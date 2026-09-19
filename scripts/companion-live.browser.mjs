/** 本机真实 API/模型验收：专用随机账号与合成剧情，不修改既有用户资料。 */
import assert from 'node:assert/strict';
import { randomBytes } from 'node:crypto';
import { mkdir, writeFile } from 'node:fs/promises';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
const browser = await chromium.launch({ headless: true });
const origin = process.env.AI_CHAT_TEST_ORIGIN || 'http://127.0.0.1:8080';
const directory = new URL('../data/go-local/evidence/', import.meta.url);
await mkdir(directory, { recursive: true });

try {
    const context = await browser.newContext();
    const account = { handle: `memory-${Date.now()}`, password: randomBytes(20).toString('hex'), name: '记忆功能验收' };
    const request = context.request;
    async function api(path, method = 'GET', data) {
        const response = await request.fetch(`${origin}/api/v1${path}`, { method, data, timeout: 150000 });
        assert.ok(response.ok(), `${method} ${path}: ${response.status()}`);
        const body = await response.json();
        return body.data ?? body;
    }
    await api('/auth/register', 'POST', account);
    const created = await api('/characters', 'POST', { name: '记忆验收角色', description: '只依据当前资料回答，不编造。', personality: '认真记住明确约定', gender: 'female', age: '25' });
    const id = String(created.ID);
    await api(`/characters/${id}`, 'PUT', { name: '夜间书店', gender: 'female', age: '26', scenario: '书店', message_sample: '你：晚上好。角色：欢迎回来。' });
    const name = `小鹿${randomBytes(3).toString('hex')}`;
    const passphrase = `meet-${randomBytes(4).toString('hex')}`;
    const memory = await api(`/characters/${id}/memories`, 'POST', { kind: 'fact', content: `用户的专属昵称是 ${name}。`, pinned: true, enabled: true });
    await api(`/characters/${id}/memories`, 'POST', { kind: 'lore', content: '书店位于月港。', keywords: ['书店'], enabled: true });
    const chat = await api('/chats', 'POST', { character_id: Number(id), title: '长上下文验收' });
    let parent = null;
    const filler = '今天我们在书店讨论最近读过的书，也聊了一些普通的天气和晚餐安排。这些是日常闲聊，没有新的重要约定。'.repeat(2);
    for (let i = 0; i < 121; i++) {
        let content = `${i}: ${filler}`;
        if (i === 0) content = `重要约定：我们以后见面的暗号是 ${passphrase}。这是最重要的约定，请记住。`;
        if (i === 120) content = '请只回答我的专属昵称，以及第一条消息约定的见面暗号。';
        const message = await api(`/chats/${chat.id}/messages`, 'POST', { content, parent_id: parent });
        parent = message.id;
    }
    const response = await request.post(`${origin}/api/v1/chats/${chat.id}/generations`, { data: { parent_id: parent }, timeout: 180000 });
    assert.equal(response.status(), 200);
    const stream = await response.text();
    assert.ok(/event:\s*message_end/.test(stream), 'generation did not complete');
    const history = await api(`/chats/${chat.id}/messages?page=2&size=100`);
    const answer = history.items.at(-1).content;
    assert.ok(answer.includes(name), 'long-term fact missing');
    assert.ok(answer.includes(passphrase), 'oldest message lost from summary');
    const page = await context.newPage();
    await page.goto(origin);
    await page.locator('[data-character]').waitFor();
    await page.locator(`[data-character="${id}"]`).click();
    await page.locator('#manage-memories').click();
    await page.locator('.memory-entry').filter({ hasText: name }).waitFor();
    await page.setViewportSize({ width: 375, height: 812 });
    await page.screenshot({ path: new URL('companion-memory-mobile.png', directory).pathname, fullPage: true });
    await page.locator('#memory-dialog [data-close]').click();
    await page.locator(`[data-chat="${chat.id}"]`).click();
    await page.locator('#background-settings').click();
    await page.locator('#background-dialog').waitFor({ state: 'visible' });
    await page.waitForFunction(() => !document.querySelector('#background-file').disabled);
    await page.locator('#background-file').setInputFiles('public/img/characters/assistant.jpg');
    await page.waitForFunction(() => document.querySelector('#background-preview').src.startsWith('data:image/jpeg'));
    await page.locator('#background-form button[type=submit]').click();
    await page.locator('#background-dialog').waitFor({ state: 'hidden' });
    const second = await browser.newContext();
    const login = await second.request.post(`${origin}/api/v1/auth/login`, { data: account });
    assert.ok(login.ok());
    const remote = await second.request.get(`${origin}/api/v1/me/background`);
    const background = (await remote.json()).data;
    assert.ok(background.enabled && background.image.startsWith('data:image/jpeg'));
    const foreign = await browser.newContext();
    await foreign.request.post(`${origin}/api/v1/auth/register`, { data: { ...account, handle: `${account.handle}-other` } });
    assert.equal((await foreign.request.get(`${origin}/api/v1/characters/${id}/memories`)).status(), 404);
    assert.equal((await foreign.request.put(`${origin}/api/v1/characters/${id}`, { data: { name: '越权修改' } })).status(), 404);
    await api(`/characters/${id}/memories/${memory.id}`, 'DELETE');
    assert.ok(!(await api(`/characters/${id}/memories`)).items.some(x => x.id === memory.id));
    await api(`/characters/${id}`, 'DELETE');
    assert.equal((await request.get(`${origin}/api/v1/characters/${id}`)).status(), 404);
    assert.ok((await api(`/chats/${chat.id}/messages`)).total > 0);
    await writeFile(new URL('companion-report.json', directory), JSON.stringify({
        testedAt: new Date().toISOString(), realProvider: true, historyMessages: 121,
        characterEdit: true, characterSoftDelete: true, historyPreserved: true,
        longTermRecall: true, oldestMessageSummaryRecall: true, backgroundAcrossSessions: true,
        ownershipIsolation: true, forgetMemory: true,
    }, null, 2), { mode: 0o600 });
    console.log('Real companion flow passed: 121-message summary, persistent memory, background sync, ownership, edit/delete.');
} finally {
    await browser.close();
}
