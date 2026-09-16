/** 浏览器界面回归使用明确的模拟接口，不作为 MySQL 或真实 Provider 验收证据。 */
/* global document, innerWidth, innerHeight */
import assert from 'node:assert/strict';
import { mkdir } from 'node:fs/promises';

const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
const browser = await chromium.launch({ headless: true });
const origin = process.env.FRONTEND_URL || 'http://127.0.0.1:5174';
const artifacts = process.env.FRONTEND_ARTIFACTS || '/tmp/ai-chat-frontend';
await mkdir(artifacts, { recursive: true });

/** 按后端大小写兼容契约模拟登录、会话和独立候选分支。 */
async function mockAPI(page) {
    let signedIn = false;
    let messages = [];
    let chats = [];
    let sequence = 0;
    const character = { ID: 1, Name: '林夏', Description: '喜欢旅行、读书和日常分享。', FirstMessage: '今天过得怎么样？', Portrait: '/img/characters/seraphina.jpg', Tags: ['日常', '冒险'], Gender: 'female' };
    const characters = [character, { ID: 2, Name: 'Assistant', Description: '聊聊生活中的问题，或一起探索新的想法。', Portrait: '/img/characters/assistant.jpg', Tags: ['助理'] }];
    const pageData = items => ({ items, total: String(items.length) });
    await page.route('**/api/v1/**', async route => {
        const request = route.request();
        const path = new URL(request.url()).pathname.replace('/api/v1', '');
        const method = request.method();
        const body = request.postDataJSON();
        const json = (value, status = 200) => route.fulfill({ status, json: value });
        if (path === '/auth/login') { signedIn = true; return json({ user: { ID: 1, Name: '测试用户' } }); }
        if (path === '/auth/logout') { signedIn = false; return json({}); }
        if (path === '/me') return json(signedIn ? { user: { ID: 1, Name: '测试用户' } } : { error: '未登录' }, signedIn ? 200 : 401);
        if (path === '/characters' && method === 'POST') {
            assert.ok(body.portrait.startsWith('data:image/jpeg;base64,'));
            assert.deepEqual(body.tags, ['日常', '书店']);
            const created = { ...body, id: '3' };
            characters.push(created);
            return json({ data: created });
        }
        if (path === '/characters') return json(pageData(characters));
        if (/^\/characters\/\d+$/.test(path)) return json({ character: characters.find(item => String(item.ID ?? item.id) === path.split('/').at(-1)) });
        if (path === '/chats' && method === 'POST') {
            const chat = { id: '10', character_id: String(body.character_id), title: body.title };
            chats = [chat];
            return json({ data: chat });
        }
        if (path === '/chats') return json({ data: pageData(chats) });
        if (path === '/chats/10' && method === 'DELETE') { chats = []; messages = []; return json({ data: {} }); }
        if (path === '/chats/10') return json({ data: chats[0] });
        if (path === '/chats/10/messages' && method === 'POST') {
            const item = { ...body, id: String(++sequence), role: 'user', created_at: '2026-09-16T12:30:00Z' };
            messages.push(item);
            return json({ data: item });
        }
        if (path === '/chats/10/messages') return json({ data: pageData(messages) });
        if (path === '/chats/10/generations') {
            assert.equal(body.model, undefined, 'consumer requests must use the server default');
            const item = { id: String(++sequence), parent_id: body.parent_id, role: 'assistant', content: '今天很高兴见到你。', created_at: '2026-09-16T12:30:05Z' };
            messages.push(item);
            const event = (name, data) => `event: ${name}\ndata: ${JSON.stringify(data)}\n\n`;
            return route.fulfill({ contentType: 'text/event-stream', body:
                event('message_start', { generation_id: 9 }) +
                event('message_delta', { index: 0, text: item.content }) +
                event('message_end', { message_id: item.id, variants: ['7'] }) });
        }
        if (path.endsWith('/variants')) return json({ data: pageData([{ id: '7', variant_no: 1, content: '候选回复：一起聊聊今天吧。' }]) });
        if (path.endsWith('/variants/7/select')) {
            const item = { id: String(++sequence), parent_id: '1', role: 'assistant', content: '候选回复：一起聊聊今天吧。', source_variant_id: '7' };
            messages.push(item);
            return json({ data: item });
        }
        throw new Error(`Unmocked request: ${method} ${path}`);
    });
    return () => messages;
}

/** 同时验证页面边界与真实字体、位图资源，截图供人工检查。 */
async function checkLayout(page, name) {
    await page.evaluate(() => document.fonts.ready);
    const result = await page.evaluate(() => ({
        overflow: document.documentElement.scrollWidth > innerWidth,
        images: [...document.images].filter(image => image.getClientRects().length).every(image => image.complete && image.naturalWidth > 0),
        composer: document.querySelector('#composer').getBoundingClientRect().bottom,
        height: innerHeight,
    }));
    assert.equal(result.overflow, false, `${name}: horizontal overflow`);
    assert.equal(result.images, true, `${name}: broken image`);
    assert.ok(result.composer <= result.height + 1, `${name}: composer outside viewport (${result.composer}/${result.height})`);
    await page.screenshot({ path: `${artifacts}/${name}.png`, fullPage: true });
}

try {
    const page = await browser.newPage({ viewport: { width: 1440, height: 900 }, reducedMotion: 'reduce' });
    const errors = [];
    page.on('pageerror', error => errors.push(error.message));
    const messages = await mockAPI(page);
    await page.goto(origin);
    await page.locator('#auth').waitFor({ state: 'visible' });
    await page.locator('[name=handle]').fill('test');
    await page.locator('[name=password]').fill('password');
    await page.locator('#auth-submit').click();
    await page.locator('[data-character="1"] img').waitFor();
    await page.evaluate(() => Promise.all([...document.images].map(img => img.decode().catch(() => {}))));
    await page.screenshot({ path: `${artifacts}/library.png`, fullPage: true });
    await page.locator('[data-tag="助理"]').click();
    assert.equal(await page.locator('[data-character]').count(), 1);
    await page.locator('[data-tag=""]').click();
    await page.locator('#character-gender').selectOption('female');
    assert.equal(await page.locator('[data-character]').count(), 1);
    await page.locator('#character-gender').selectOption('');
    assert.equal(await page.locator('#model').count(), 0);
    await page.locator('[data-character="1"]').click();
    await page.locator('#favorite').click();
    await page.locator('#start-chat').click();
    await page.locator('#message').fill('你好 <img src=x onerror=alert(1)>');
    await page.locator('#send').click();
    await page.locator('[data-message="2"]').waitFor();
    assert.equal(await page.locator('#messages .content img').count(), 0);
    assert.equal(await page.locator('[data-message="2"] time').getAttribute('datetime'), '2026-09-16T12:30:05.000Z');
    await page.locator('#chat-background').click();
    assert.equal(await page.locator('#chat-background').getAttribute('aria-pressed'), 'true');
    await page.locator('#chat-background').click();
    await page.locator('[data-action=variants]').click();
    await page.locator('[data-action=select]').click();
    await page.locator('[data-message="3"]').waitFor();
    assert.equal(await page.locator('[data-message="2"]').count(), 0);
    await page.locator('#message').fill('接着聊');
    await page.locator('#send').click();
    await page.locator('[data-message="5"]').waitFor();
    assert.equal(messages().find(item => item.id === '4').parent_id, '3');
    await page.reload();
    await page.locator('[data-message="5"]').waitFor();
    assert.equal(await page.locator('[data-message="2"]').count(), 0);
    await checkLayout(page, 'desktop');
    await page.setViewportSize({ width: 375, height: 812 });
    await checkLayout(page, 'mobile');
    await page.evaluate(() => {
        const notice = document.querySelector('#notice');
        notice.hidden = false;
        notice.textContent = '请求失败，请稍后重试。'.repeat(12);
    });
    await checkLayout(page, 'mobile-error');
    await page.evaluate(() => { document.querySelector('#notice').hidden = true; });
    await page.setViewportSize({ width: 812, height: 375 });
    await checkLayout(page, 'landscape');
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.locator('#library').click();
    await page.locator('#create-character').click();
    await page.locator('#create-view [name=name]').fill('林间书店');
    await page.locator('#create-view [name=description]').fill('书店里，总有一本属于你的书。');
    await page.locator('#create-view [name=first_message]').fill('欢迎，今天想读些什么？');
    await page.locator('#create-view [name=tags]').fill('日常，书店');
    await page.locator('#portrait-file').setInputFiles('public/img/characters/seraphina.jpg');
    await page.waitForFunction(() => document.querySelector('#preview-image').src.startsWith('data:image/jpeg'));
    assert.equal(await page.locator('#preview-name').textContent(), '林间书店');
    await page.locator('#create-view [name=name]').scrollIntoViewIfNeeded();
    await page.screenshot({ path: `${artifacts}/create.png`, fullPage: true });
    await page.locator('[data-preview=chat]').click();
    assert.equal(await page.locator('#preview-greeting').textContent(), '欢迎，今天想读些什么？');
    await page.setViewportSize({ width: 375, height: 812 });
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
    await page.screenshot({ path: `${artifacts}/create-mobile.png`, fullPage: true });
    await page.locator('#create-submit').click();
    await page.locator('#chat-title').filter({ hasText: '林间书店' }).waitFor();
    await page.reload();
    await page.locator('#chat-title').filter({ hasText: '林间书店' }).waitFor();
    await page.locator('#chat-info').click();
    assert.equal(await page.locator('#detail-name').textContent(), '林间书店');
    await page.locator('#character-dialog [data-close]').click();
    await page.locator('#back').click();
    await page.locator('[data-delete-chat="10"]').click();
    await page.locator('#confirm-dialog [value=cancel]').click();
    assert.equal(await page.locator('[data-chat="10"]').count(), 1);
    await page.locator('[data-delete-chat="10"]').click();
    await page.locator('#confirm-dialog [value=confirm]').click();
    await page.locator('[data-chat="10"]').waitFor({ state: 'detached' });
    await page.reload();
    await page.locator('#library-view').waitFor({ state: 'visible' });
    assert.equal(await page.locator('[data-chat="10"]').count(), 0);
    await page.locator('#logout').click();
    await page.locator('#auth').waitFor({ state: 'visible' });
    assert.equal(await page.locator('#messages').textContent(), '');
    assert.deepEqual(errors, []);
    console.log(`Browser flow passed; screenshots: ${artifacts}`);
} finally {
    await browser.close();
}
