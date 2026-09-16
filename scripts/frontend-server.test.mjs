/** 静态入口与 API 流代理的网络回归，不连接真实 Go 或 Provider。 */
import assert from 'node:assert/strict';
import http from 'node:http';
import { once } from 'node:events';
import { test } from 'node:test';
import { createFrontend } from './serve-frontend.mjs';

/** 自动关闭监听与连接，避免回归测试遗留服务进程。 */
async function listen(t, handler) {
    const server = http.createServer(handler);
    server.listen(0, '127.0.0.1');
    await once(server, 'listening');
    t.after(() => { server.closeAllConnections(); server.close(); });
    return `http://127.0.0.1:${server.address().port}`;
}

test('standalone root and modules load without legacy Node runtime', async t => {
    const url = await listen(t, createFrontend());
    const response = await fetch(url);
    assert.equal(response.status, 200);
    assert.match(await response.text(), /scripts\/go-chat\/main.js/);
    for (const asset of ['/scripts/go-chat/main.js', '/css/go-chat.css', '/img/ai4.png']) {
        assert.equal((await fetch(url + asset)).status, 200);
    }
});

test('proxy preserves API path, query, body, cookie and response status', async t => {
    let received;
    const origin = await listen(t, async (req, res) => {
        let body = '';
        for await (const chunk of req) body += chunk;
        received = { path: req.url, method: req.method, cookie: req.headers.cookie, body };
        res.writeHead(201, { 'Content-Type': 'application/json', 'Set-Cookie': 'session=new; HttpOnly' });
        res.end('{"id":"1"}');
    });
    const url = await listen(t, createFrontend(origin));
    const result = await fetch(`${url}/api/v1/chats?page=2`, {
        method: 'POST', headers: { Cookie: 'session=old' }, body: '{"title":"test"}',
    });
    assert.equal(result.status, 201);
    assert.equal(result.headers.get('set-cookie'), 'session=new; HttpOnly');
    assert.deepEqual(received, {
        path: '/api/v1/chats?page=2', method: 'POST', cookie: 'session=old', body: '{"title":"test"}',
    });
});

test('SSE forwards first chunk before upstream finishes and abort closes upstream', { timeout: 5000 }, async t => {
    let close;
    const closed = new Promise(resolve => { close = resolve; });
    const origin = await listen(t, (_req, res) => {
        res.writeHead(200, { 'Content-Type': 'text/event-stream' });
        res.write('event: message_delta\ndata: {"text":"first"}\n\n');
        res.on('close', close);
    });
    const url = await listen(t, createFrontend(origin));
    const controller = new AbortController();
    const response = await fetch(`${url}/api/v1/chats/1/generations`, { signal: controller.signal });
    const reader = response.body.getReader();
    assert.match(new TextDecoder().decode((await reader.read()).value), /first/);
    controller.abort();
    await closed;
    reader.releaseLock();
});

test('unavailable upstream returns generic 502 without infrastructure details', async t => {
    const temporary = http.createServer();
    temporary.listen(0, '127.0.0.1');
    await once(temporary, 'listening');
    const port = temporary.address().port;
    await new Promise(resolve => temporary.close(resolve));
    const url = await listen(t, createFrontend(`http://127.0.0.1:${port}`));
    const response = await fetch(`${url}/api/v1/me`);
    assert.equal(response.status, 502);
    assert.equal((await response.json()).code, 'API_UNAVAILABLE');
});
