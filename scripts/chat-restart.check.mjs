/** 重启专用本机部署栈并核对历史消息，不再次消耗模型请求。 */
import assert from 'node:assert/strict';
import { readFile, writeFile } from 'node:fs/promises';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../', import.meta.url));
const dir = `${root}data/go-local`;
const compose = ['compose', '--env-file', `${dir}/.env`, '-f', `${root}docker/docker-compose.chat-local.yml`];
const { request } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
const account = JSON.parse(await readFile(`${dir}/account.json`, 'utf8'));
const previous = JSON.parse(await readFile(`${dir}/evidence/live-report.json`, 'utf8'));
assert.equal(previous.status, 'passed');
if (!process.argv.includes('--verify-only')) {
    execFileSync('docker', [...compose, 'stop'], { stdio: 'pipe' });
    execFileSync('docker', [...compose, 'up', '-d', '--wait'], { stdio: 'pipe' });
}
const client = await request.newContext({
    baseURL: process.env.AI_CHAT_TEST_ORIGIN || 'http://127.0.0.1:8080',
});

/** 代理启动与 API 启动存在短暂时差，仅对尚未就绪的状态做有界重试。 */
async function waitReady() {
    for (let attempt = 0; attempt < 60; attempt++) {
        try {
            const result = await client.get('/api/v1/me', { timeout: 2000 });
            if (result.status() === 401) return;
        } catch {
            // 进程尚未绑定端口时允许暂时连接失败。
        }
        await new Promise(resolve => setTimeout(resolve, 1000));
    }
    throw new Error('Local deployment did not become ready');
}

try {
    await waitReady();
    const login = await client.post('/api/v1/auth/login', { data: account });
    assert.equal(login.status(), 200);
    const chats = await (await client.get('/api/v1/chats?page=1&size=100')).json();
    let restored = false;
    for (const chat of chats.data.items) {
        const result = await (await client.get(`/api/v1/chats/${chat.id}/messages?page=1&size=100`)).json();
        const ids = new Set(result.data.items.map(item => item.id));
        if (previous.message_ids.every(id => ids.has(id))) restored = true;
    }
    assert.equal(restored, true, 'persisted live conversation missing after restart');
    for (const url of ['/data/default-user/secrets.json', '/.env', '/settings.json', '/scripts/app.js']) {
        assert.ok([403, 404].includes((await client.get(url)).status()), `unexpected public resource: ${url}`);
    }
    const report = { checked_at: new Date().toISOString(), status: 'passed',
        idle_restart: !process.argv.includes('--verify-only'),
        history_restored: restored, static_allowlist: true, active_generation_shutdown: 'not tested' };
    await writeFile(`${dir}/evidence/restart-report.json`, JSON.stringify(report, null, 2), { mode: 0o600 });
    console.log(JSON.stringify(report, null, 2));
} finally {
    await client.dispose();
}
