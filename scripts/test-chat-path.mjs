import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const settings = JSON.parse(readFileSync(new URL('../data/default-user/settings.json', import.meta.url)));
const s = settings.oai_settings;
assert.equal(s.chat_completion_source, 'custom', 'This smoke test requires the custom OpenAI-compatible connection');
const base = 'http://127.0.0.1:8000';
const csrf = await fetch(`${base}/csrf-token`, { signal: AbortSignal.timeout(5000) });
assert.equal(csrf.status, 200);
const { token } = await csrf.json();
assert.ok(token);
const headers = {
    'Content-Type': 'application/json',
    'X-CSRF-Token': token,
    Cookie: csrf.headers.getSetCookie().map(cookie => cookie.split(';')[0]).join('; '),
};
const connection = {
    chat_completion_source: 'custom', custom_url: s.custom_url,
    custom_include_headers: s.custom_include_headers,
};
async function post(path, body) {
    const response = await fetch(`${base}${path}`, {
        method: 'POST', headers, body: JSON.stringify(body),
        signal: AbortSignal.timeout(60000),
    });
    assert.equal(response.status, 200, `${path}: HTTP ${response.status}`);
    return response;
}

const status = await (await post('/api/backends/chat-completions/status', connection)).json();
assert.ok(status.data?.some(model => model.id === s.custom_model), 'Configured model is missing');
const messages = [{ role: 'user', content: 'Reply with exactly: CHAT_PATH_OK' }];
const count = await (await post(`/api/tokenizers/openai/count?model=${encodeURIComponent(s.custom_model)}`, messages)).json();
assert.ok(count.token_count > 0, 'Token counting failed');
const response = await post('/api/backends/chat-completions/generate', {
    ...connection, model: s.custom_model, messages, stream: true,
    max_tokens: s.openai_max_tokens,
    custom_include_body: s.custom_include_body,
    custom_exclude_body: s.custom_exclude_body,
});
const stream = await response.text();
assert.ok(stream.includes('data: [DONE]'), 'Stream did not finish');
let content = '';
for (const line of stream.split('\n')) {
    if (!line.startsWith('data: ') || line.trim() === 'data: [DONE]') continue;
    const chunk = JSON.parse(line.slice(6));
    assert.ok(!chunk.error, 'Provider returned a stream error');
    content += chunk.choices?.[0]?.delta?.content ?? '';
}
assert.ok(content.includes('CHAT_PATH_OK'), 'No expected assistant text received');
console.log('PASS: CSRF -> model status -> token count -> generation -> assistant text -> DONE');
