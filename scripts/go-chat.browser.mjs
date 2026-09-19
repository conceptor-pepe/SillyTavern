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
    let background = { image: '', enabled: false };
    let memories = [];
    let stories = [];
    let publicStories = [];
    let personas = [];
    let storyMessages = [];
    let storyVersion = null;
    let relationship = { id: '70', companion_id: '1', stage: 'acquaintance', narrative: '', milestones: [], revision: '1' };
    let relationshipMemories = [];
    let memoryCandidates = [];
    const character = { ID: 1, Name: '林夏', Description: '喜欢旅行、读书和日常分享。', FirstMessage: '今天过得怎么样？', Portrait: '/img/characters/seraphina.jpg', Tags: ['日常', '冒险'], Gender: 'female' };
    const characters = [character, { ID: 2, Name: 'Assistant', Description: '聊聊生活中的问题，或一起探索新的想法。', Portrait: '/img/characters/assistant.jpg', Tags: ['助理'] }];
    const pageData = items => ({ items, total: String(items.length) });
    await page.route('**/api/v1/**', async route => {
        const request = route.request();
        const path = new URL(request.url()).pathname.replace('/api/v1', '');
        const method = request.method();
        const body = request.postDataJSON();
        const json = (value, status = 200) => route.fulfill({ status, json: value });
        if (path === '/me/background') { if (method === 'PUT') background = body; return json({ data: background }); }
        if (/^\/characters\/\d+\/memories/.test(path)) {
            const memoryID = path.split('/')[4];
            if (method === 'POST') { const item = { ...body, id: String(memories.length + 1) }; memories.push(item); return json({ data: item }); }
            if (method === 'PUT') { memories = memories.map(x => x.id === memoryID ? { ...body, id: memoryID } : x); return json({ data: memories.find(x => x.id === memoryID) }); }
            if (method === 'DELETE') { memories = memories.filter(x => x.id !== memoryID); return json({ data: {} }); }
            return json({ data: { items: memories } });
        }
        if (/^\/relationships\/1\/memories/.test(path)) {
            const memoryID = path.split('/').at(-1);
            if (method === 'POST') { const item = { ...body, id: String(80 + relationshipMemories.length) }; relationshipMemories.push(item); return json({ data: item }); }
            if (method === 'DELETE') { relationshipMemories = relationshipMemories.filter(item => item.id !== memoryID); return json({ data: {} }); }
            return json({ data: { items: relationshipMemories } });
        }
        if (/^\/characters\/\d+$/.test(path) && method === 'PUT') {
            const index = characters.findIndex(x => String(x.ID ?? x.id) === path.split('/').at(-1));
            characters[index] = { ...body, id: path.split('/').at(-1) };
            return json({ data: characters[index] });
        }
        if (/^\/characters\/\d+$/.test(path) && method === 'DELETE') {
            const index = characters.findIndex(x => String(x.ID ?? x.id) === path.split('/').at(-1));
            characters.splice(index, 1); return json({ data: {} });
        }
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
        if (path === '/personas' && method === 'POST') {
            const item = { ...body, id: String(50 + personas.length), revision: '1' };
            personas.push(item);
            return json({ data: item });
        }
        if (path === '/personas') return json({ data: pageData(personas) });
        if (/^\/personas\/\d+$/.test(path) && method === 'PUT') {
            const id = path.split('/').at(-1);
            const item = { ...body, id, revision: String(Number(body.expected_revision) + 1) };
            personas = personas.map(value => value.id === id ? item : value);
            return json({ data: item });
        }
        if (/^\/personas\/\d+$/.test(path) && method === 'DELETE') {
            const id = path.split('/').at(-1);
            personas = personas.filter(value => value.id !== id);
            return json({ data: {} });
        }
        if (path === '/stories' && method === 'POST') {
            const item = { ...body, id: String(30 + stories.length), revision: '1' };
            stories.push(item);
            return json({ data: item });
        }
        if (path === '/stories') return json({ data: pageData(stories) });
        if (path === '/public/stories') return json({ data: pageData(publicStories) });
        if (/^\/public\/stories\/\d+$/.test(path)) {
            const id = path.split('/').at(-1);
            const item = publicStories.find(value => value.story_id === id);
            return item ? json({ data: item }) : json({ code: 'NOT_FOUND', message: 'not found' }, 404);
        }
        if (/^\/stories\/\d+$/.test(path) && method === 'PUT') {
            const id = path.split('/').at(-1);
            const item = { definition: body.definition, id, revision: String(Number(body.expected_revision) + 1) };
            stories = stories.map(value => value.id === id ? item : value);
            return json({ data: item });
        }
        if (/^\/stories\/\d+$/.test(path) && method === 'DELETE') {
            const id = path.split('/').at(-1);
            stories = stories.filter(value => value.id !== id);
            return json({ data: {} });
        }
        if (/^\/stories\/\d+\/versions$/.test(path) && method === 'POST') {
            const story = stories.find(value => value.id === path.split('/')[2]);
            storyVersion = { id: '40', story_id: story.id, revision: story.revision, definition: story.definition, digest: 'browser-test' };
            return json({ data: storyVersion });
        }
        if (/^\/stories\/\d+\/publication$/.test(path) && method === 'POST') {
            const id = path.split('/')[2];
            const story = stories.find(value => value.id === id);
            storyVersion = { id: '40', story_id: story.id, revision: story.revision, definition: story.definition, digest: 'browser-test' };
            Object.assign(story, { published_version_id: storyVersion.id, published_at: '2026-09-19T08:00:00Z' });
            publicStories = [{ story_id: story.id, version_id: storyVersion.id, author_id: '1', author_name: '测试用户', author_handle: 'test', published_at: story.published_at, definition: story.definition }];
            return json({ data: story });
        }
        if (/^\/stories\/\d+\/publication$/.test(path) && method === 'DELETE') {
            const id = path.split('/')[2];
            const story = stories.find(value => value.id === id);
            delete story.published_version_id;
            delete story.published_at;
            publicStories = publicStories.filter(value => value.story_id !== id);
            return json({ data: story });
        }
        if (path === '/chats' && method === 'POST') {
            if (body.mode === 'story') {
                const chat = { id: '20', mode: 'story', title: storyVersion.definition.title, story_version_id: storyVersion.id };
                chats = [chat, ...chats];
                storyMessages = [{ id: '101', parent_id: null, role: 'assistant', content: `${storyVersion.definition.opening[0].text}\n${storyVersion.definition.opening[1].text}`, created_at: '2026-09-18T08:00:00Z' }];
                return json({ data: { chat, version: storyVersion, persona: personas.find(value => value.id === body.persona_id), opening_id: '101' } });
            }
            const chat = { id: '10', character_id: String(body.character_id), title: body.title };
            chats = [chat];
            return json({ data: chat });
        }
        if (path === '/chats') return json({ data: pageData(chats) });
        if (path === '/chats/20') return json({ data: chats.find(value => value.id === '20') });
        if (path === '/chats/20/relationship') {
            if (method === 'PUT') relationship = { ...body, id: relationship.id, companion_id: '1', revision: String(Number(relationship.revision) + 1) };
            return json({ data: relationship });
        }
        if (path === '/chats/20/memory-candidates') {
            if (method === 'POST') memoryCandidates = [
                { id: '90', scope: 'relationship', content: '用户喜欢被叫作小鹿。', evidence: '用户明确确认了称呼。' },
                { id: '91', scope: 'story', content: '两人在雨夜书店相遇。', evidence: '当前分支的开场已经发生。' },
            ];
            return json({ data: { items: memoryCandidates } });
        }
        if (path === '/memory-candidates/90/accept' && method === 'POST') {
            memoryCandidates = memoryCandidates.filter(item => item.id !== '90');
            return json({ data: { id: '90', status: 'accepted' } });
        }
        if (path === '/memory-candidates/91' && method === 'DELETE') {
            memoryCandidates = memoryCandidates.filter(item => item.id !== '91');
            return json({ data: { id: '91', status: 'rejected' } });
        }
        if (path === '/chats/20/bootstrap') return json({ data: {
            chat: chats.find(value => value.id === '20'), version: storyVersion, persona: personas[0], relationship, opening_id: '101',
        } });
        if (path === '/chats/20/messages') return json({ data: pageData(storyMessages) });
        if (path === '/chats/20/reply-suggestions' && method === 'POST') {
            assert.equal(body.parent_id, '101');
            return json({ data: { anchor_id: '101', items: ['问她为什么在这里', '环顾四周寻找线索', '沉默地递出雨伞'] } });
        }
        if (path === '/messages/101/revisions' && method === 'POST') {
            const item = { ...storyMessages[0], id: '102', content: body.content };
            storyMessages.push(item);
            return json({ data: item });
        }
        if (path === '/chats/10' && method === 'DELETE') { chats = []; messages = []; return json({ data: {} }); }
        if (path === '/chats/10/relationship') return json({ code: 'NOT_FOUND', message: 'not found' }, 404);
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
    await page.waitForFunction(() => document.querySelector('#chat-background').getAttribute('aria-pressed') === 'true');
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
    await page.locator('#portrait-file').setInputFiles('public/img/characters/seraphina.jpg');
    await page.waitForFunction(() => document.querySelector('#preview-image').src.startsWith('data:image/jpeg'));
    assert.equal(await page.locator('#preview-name').textContent(), '林间书店');
    await page.locator('#create-character-form [data-wizard-next]').click();
    await page.locator('#create-character-form [data-choice-value="温柔"]').click();
    assert.equal(await page.locator('#create-view [name=tags]').inputValue(), '温柔');
    await page.locator('#create-view [name=tags]').fill('日常，书店');
    await page.locator('#create-character-form [data-wizard-next]').click();
    await page.locator('#create-view [name=first_message]').fill('欢迎，今天想读些什么？');
    await page.locator('#create-character-form [data-wizard-next]').click();
    assert.equal(await page.locator('#create-character-form').getAttribute('data-wizard-current'), '3');
    await page.locator('#character-review-summary').scrollIntoViewIfNeeded();
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
    await page.locator('#edit-character').click();
    await page.locator('#create-view [name=name]').fill('夜间书店');
    await page.locator('#create-character-form [data-wizard-next]').click();
    await page.locator('#create-character-form [data-wizard-next]').click();
    await page.locator('#create-view details summary').click();
    await page.locator('#create-view [name=message_sample]').fill('你：晚安。角色：梦里见。');
    await page.locator('#create-character-form [data-wizard-next]').click();
    await page.locator('#create-submit').click();
    await page.locator('#detail-name').filter({ hasText: '夜间书店' }).waitFor();
    await page.locator('#manage-memories').click();
    await page.locator('#memory-dialog').waitFor({ state: 'visible' });
    await page.locator('#memory-form [name=content]').fill('请叫我小鹿');
    await page.locator('#memory-form button[type=submit]').click();
    await page.locator('.memory-entry').filter({ hasText: '请叫我小鹿' }).waitFor();
    await page.locator('[data-memory-edit="1"]').click();
    await page.locator('#memory-form [name=content]').fill('周五一起看电影');
    await page.locator('#memory-form button[type=submit]').click();
    await page.locator('.memory-entry').filter({ hasText: '周五一起看电影' }).waitFor();
    await page.screenshot({ path: `${artifacts}/memories-mobile.png`, fullPage: true });
    await page.locator('#memory-dialog [data-close]').click();
    await page.locator('[data-chat="10"]').click();
    await page.locator('#background-settings').click();
    await page.locator('#background-dialog').waitFor({ state: 'visible' });
    await page.waitForFunction(() => !document.querySelector('#background-file').disabled);
    await page.locator('#background-file').setInputFiles('public/img/characters/assistant.jpg');
    await page.waitForFunction(() => document.querySelector('#background-preview').src.startsWith('data:image/jpeg') || !document.querySelector('#notice').hidden);
    assert.equal(await page.locator('#notice').isVisible(), false, await page.locator('#notice').textContent());
    await page.locator('#background-form button[type=submit]').click();
    await page.locator('#background-dialog').waitFor({ state: 'hidden' });
    await page.reload();
    await page.locator('#chat-view').waitFor({ state: 'visible' });
    assert.equal(await page.locator('#chat-background').getAttribute('aria-pressed'), 'true');
    assert.ok((await page.locator('.chat-column').getAttribute('style')).includes('data:image/jpeg'));
    assert.deepEqual(await page.locator('.chat-column').evaluate(column => ({
        foreground: getComputedStyle(column, '::after').backgroundSize,
        fill: getComputedStyle(column, '::before').backgroundSize,
    })), { foreground: 'contain', fill: 'cover' });
    await page.screenshot({ path: `${artifacts}/background-contain-mobile.png`, fullPage: true });
    await page.locator('#chat-info').click();
    await page.locator('#manage-memories').click();
    await page.locator('.memory-entry').filter({ hasText: '周五一起看电影' }).waitFor();
    await page.locator('[data-memory-delete="1"]').click();
    await page.locator('.memory-entry').waitFor({ state: 'detached' });
    await page.locator('#memory-dialog [data-close]').click();
    await page.locator('#chat-info').click();
    await page.locator('#delete-character').click();
    await page.locator('#confirm-dialog [value=confirm]').click();
    await page.locator('[data-character="3"]').waitFor({ state: 'detached' });
    await page.locator('#library-view').waitFor({ state: 'visible' });

    await page.locator('[data-delete-chat="10"]').click();
    await page.locator('#confirm-dialog [value=cancel]').click();
    assert.equal(await page.locator('[data-chat="10"]').count(), 1);
    await page.locator('[data-delete-chat="10"]').click();
    await page.locator('#confirm-dialog [value=confirm]').click();
    await page.locator('[data-chat="10"]').waitFor({ state: 'detached' });
    await page.reload();
    await page.locator('#library-view').waitFor({ state: 'visible' });
    assert.equal(await page.locator('[data-chat="10"]').count(), 0);

    // H5 故事主链：身份 → 故事 → 冻结版本 → 开场 → 回复建议 → AI 回复分支改写。
    await page.locator('#stories-nav').click();
    await page.locator('#story-view').waitFor({ state: 'visible' });
    await page.locator('#manage-personas').click();
    await page.locator('#persona-dialog').waitFor({ state: 'visible' });
    await page.locator('#persona-avatar-file').setInputFiles('public/img/characters/assistant.jpg');
    await page.waitForFunction(() => document.querySelector('#persona-avatar-preview').src.startsWith('data:image/jpeg'));
    await page.locator('#persona-form [name=name]').fill('阿远');
    await page.locator('#persona-form [name=description]').fill('刚搬到城里的插画师');
    await page.locator('#persona-form button[type=submit]').click();
    await page.locator('.persona-item').filter({ hasText: '阿远' }).waitFor();
    await page.locator('#persona-dialog [data-close]').click();
    await page.locator('#create-story').click();
    await page.locator('#story-cover-file').setInputFiles('public/img/characters/seraphina.jpg');
    await page.waitForFunction(() => document.querySelector('#story-cover-preview').src.startsWith('data:image/jpeg'));
    await page.locator('#story-form [name=title]').fill('雨夜书店');
    await page.locator('#story-form [name=hook]').fill('午夜之后，书架会替来客保守一个秘密。');
    await page.locator('#story-form [data-choice-value="日常"]').click();
    assert.equal(await page.locator('#story-form [name=tags]').inputValue(), '日常');
    await page.locator('#story-form [name=tags]').fill('都市，治愈');
    await page.locator('#story-form [data-wizard-next]').click();
    await page.locator('#story-form [name=world]').fill('现代城市的一间旧书店，雨夜中时间会偶尔停住。');
    await page.locator('#story-form [name=companion_id]').selectOption('1');
    await page.locator('#story-form [name=character_name]').fill('林夏');
    await page.locator('#story-form [name=character_description]').fill('书店老板，似乎认识每一位来客。');
    await page.locator('#story-form [name=character_personality]').fill('温柔、敏锐，说话留有余地。');
    await page.locator('#story-form [data-wizard-next]').click();
    await page.locator('#story-form [name=opening_narration]').fill('雨水沿着玻璃缓慢落下，门铃忽然响了。');
    await page.locator('#story-form [name=opening_dialogue]').fill('你终于来了，我为你留了一本书。');
    await page.locator('#story-form [data-wizard-next]').click();
    await page.locator('#story-form [data-wizard-next]').click();
    assert.equal(await page.locator('#story-form').getAttribute('data-wizard-current'), '4');
    assert.equal(await page.locator('#story-form [data-wizard-step="4"]').isVisible(), true);
    await page.waitForTimeout(250);
    await page.screenshot({ path: `${artifacts}/story-editor-mobile.png`, fullPage: true });
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
    await page.locator('#story-form button[type=submit]').click();
    await page.locator('#my-stories').click();
    await page.locator('[data-story="30"]').waitFor();
    await page.locator('[data-story="30"]').click();
    await page.locator('#publish-story').click();
    await page.locator('#discover-stories.active').waitFor();
    await page.locator('[data-story="30"][data-story-scope="public"]').waitFor();
    await page.screenshot({ path: `${artifacts}/stories-mobile.png`, fullPage: true });
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
    await page.locator('[data-story="30"][data-story-scope="public"]').click();
    await page.locator('#story-dialog').waitFor({ state: 'visible' });
    await page.locator('#start-story').click();
    await page.locator('[data-message="101"]').waitFor();
    await page.locator('#relationship-settings').click();
    await page.locator('#relationship-form [name=stage]').selectOption('close');
    await page.locator('#relationship-form [name=narrative]').fill('我们已经互相信任。');
    await page.locator('#relationship-form button[type=submit]').click();
    await page.locator('#relationship-memory-form [name=content]').fill('她答应叫我小鹿。');
    await page.locator('#relationship-memory-form button[type=submit]').click();
    await page.locator('#relationship-memory-list').filter({ hasText: '她答应叫我小鹿' }).waitFor();
    await page.screenshot({ path: `${artifacts}/relationship-mobile.png`, fullPage: true });
    await page.locator('#relationship-dialog [data-close]').click();
    await page.locator('#memory-candidates').click();
    await page.locator('#extract-memory-candidates').click();
    await page.locator('.candidate-entry').first().waitFor();
    assert.equal(await page.locator('.candidate-entry').count(), 2);
    await page.locator('[data-candidate-accept="90"]').click();
    await page.locator('[data-candidate-accept="90"]').waitFor({ state: 'detached' });
    await page.locator('[data-candidate-reject="91"]').click();
    await page.locator('.candidate-entry').waitFor({ state: 'detached' });
    await page.locator('#memory-candidate-dialog [data-close]').click();
    await page.locator('#suggest-replies').click();
    await page.locator('#reply-suggestions button').first().waitFor();
    assert.equal(await page.locator('#reply-suggestions button').count(), 3);
    await page.locator('#reply-suggestions button').first().click();
    assert.equal(await page.locator('#message').inputValue(), '问她为什么在这里');
    await page.locator('[data-message="101"] .message-menu summary').click();
    await page.locator('[data-message="101"] [data-action=revise]').click();
    await page.locator('#edit-content').fill('雨声里，她把那本书轻轻推到你面前。');
    await page.locator('#edit-form button[type=submit]').click();
    await page.locator('[data-message="102"]').waitFor();
    await checkLayout(page, 'story-chat-mobile');

    await page.locator('#logout').click();
    await page.locator('#auth').waitFor({ state: 'visible' });
    assert.equal(await page.locator('#messages').textContent(), '');
    assert.deepEqual(errors, []);
    console.log(`Browser flow passed; screenshots: ${artifacts}`);
} finally {
    await browser.close();
}
