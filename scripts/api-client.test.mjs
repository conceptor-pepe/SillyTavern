/** Go API Client 的独立回归测试，不需要 Node 服务或真实 Provider。 */
import assert from 'node:assert/strict';
import { test } from 'node:test';
import { apiClient, streamGeneration, streamRegeneration } from '../public/scripts/api-client.js';
import { readEvents } from '../public/scripts/api-events.js';

const encoder = new TextEncoder();
const end = 'event: message_end\ndata: {"message_id":12,"variants":null}\n\n';

/** 逐块供给字节，便于复现 UTF-8、CRLF 和事件边界拆分。 */
function stream(chunks, cancel = () => {}) {
    let index = 0;
    return new ReadableStream({
        pull(controller) {
            if (index === chunks.length) return controller.close();
            const chunk = chunks[index++];
            controller.enqueue(typeof chunk === 'string' ? encoder.encode(chunk) : chunk);
        },
        cancel,
    });
}

test('SSE preserves candidate indexes across every byte boundary', async () => {
    const source = ': heartbeat\r\n\r\nevent: message_delta\r\ndata: {"index":1,\r\ndata: "text":"中文"}\r\n\r\n'
        + end.replaceAll('\n', '\r\n');
    const bytes = encoder.encode(source);
    const body = stream(Array.from(bytes, byte => Uint8Array.of(byte)));
    const events = [];
    await readEvents(body, { message_delta: value => events.push(value) });
    assert.deepEqual(events, [{ index: 1, text: '中文' }]);
    assert.equal(body.locked, false);
});

test('SSE supports CR-only lines and interleaved candidates', async () => {
    const values = [];
    const delta = index => `event: message_delta\rdata: {"index":${index},"text":"x"}\r\r`;
    await readEvents(stream([delta(1), delta(0), end.replaceAll('\n', '\r')]), {
        message_delta: value => values.push(value.index),
    });
    assert.deepEqual(values, [1, 0]);
});

test('EOF without a complete message_end rejects', async () => {
    for (const source of ['', 'event: message_delta\ndata: {"text":"partial"}\n\n', end.trimEnd()]) {
        const body = stream([source]);
        await assert.rejects(readEvents(body), { code: 'STREAM_INTERRUPTED' });
        assert.equal(body.locked, false);
    }
});

test('generation_error notifies then rejects and cancels reading', async () => {
    let cancelled = false;
    let reported;
    const body = stream([
        'event: generation_error\ndata: {"message":"generation failed"}\n\n',
        end,
    ], () => { cancelled = true; });
    await assert.rejects(readEvents(body, {
        generation_error: value => { reported = value.message; },
    }), { code: 'GENERATION_FAILED' });
    assert.equal(reported, 'generation failed');
    assert.equal(cancelled, true);
    assert.equal(body.locked, false);
});

test('invalid JSON and handler exceptions release the reader', async () => {
    const body = stream(['event: message_delta\ndata: {bad}\n\n', end]);
    await assert.rejects(readEvents(body), { code: 'INVALID_EVENT' });
    assert.equal(body.locked, false);
    const failed = stream([end, ': heartbeat\n\n']);
    const error = new Error('render failed');
    await assert.rejects(readEvents(failed, { message_end: () => { throw error; } }), error);
    assert.equal(failed.locked, false);
});

test('aborted reads preserve the original error and release the lock', async () => {
    const error = new DOMException('Aborted', 'AbortError');
    const body = new ReadableStream({ start(controller) { controller.error(error); } });
    await assert.rejects(readEvents(body), error);
    assert.equal(body.locked, false);
});

test('candidate APIs preserve string IDs, encode paths and return data', async t => {
    const calls = [];
    t.mock.method(globalThis, 'fetch', async (url, options) => {
        calls.push({ url, options });
        return Response.json({ code: 'OK', data: { id: '9007199254740993' } });
    });
    await apiClient.variants('a/b', '?page=2&size=10');
    const selected = await apiClient.selectVariant('9007199254740993', 'a/b');
    assert.equal(selected.id, '9007199254740993');
    assert.equal(calls[0].url, '/api/v1/messages/a%2Fb/variants?page=2&size=10');
    assert.equal(calls[1].url, '/api/v1/messages/9007199254740993/variants/a%2Fb/select');
    assert.equal(calls[1].options.method, 'POST');
    assert.equal(calls[1].options.credentials, 'include');
    assert.equal(calls[1].options.body, undefined);
});

test('generation and regeneration share SSE transport and abort signal', async t => {
    const calls = [];
    const signal = new AbortController().signal;
    t.mock.method(globalThis, 'fetch', async (url, options) => {
        calls.push({ url, options });
        return new Response(stream([end]), { headers: { 'Content-Type': 'text/event-stream; charset=utf-8' } });
    });
    await streamGeneration('10', { model: 'test', parent_id: '11', n: 2 }, {}, signal);
    await streamRegeneration('12', { model: 'test', n: 2 });
    assert.equal(calls[0].url, '/api/v1/chats/10/generations');
    assert.equal(calls[1].url, '/api/v1/messages/12/regenerate');
    assert.equal(calls[0].options.signal, signal);
    assert.equal(calls[0].options.credentials, 'include');
    assert.equal(calls[0].options.headers.Accept, 'text/event-stream');
    assert.deepEqual(JSON.parse(calls[0].options.body), { model: 'test', parent_id: '11', n: 2 });
});

test('HTTP errors keep status and code for both request types', async t => {
    t.mock.method(globalThis, 'fetch', async () =>
        Response.json({ code: 'MESSAGE_CONFLICT', message: 'conflict' }, { status: 409 }));
    await assert.rejects(apiClient.selectVariant('1', '2'), { status: 409, code: 'MESSAGE_CONFLICT' });
    await assert.rejects(streamGeneration('1', {}), { status: 409, code: 'MESSAGE_CONFLICT' });
});

test('non-SSE responses reject instead of silently completing', async t => {
    t.mock.method(globalThis, 'fetch', async () =>
        new Response('<html>login</html>', { headers: { 'Content-Type': 'text/html' } }));
    await assert.rejects(streamGeneration('1', {}), /Expected an event stream/);
});
