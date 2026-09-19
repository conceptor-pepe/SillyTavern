/** 独立前端会话回归：使用内存接口验证分支和异步边界。 */
import assert from 'node:assert/strict';
import { test } from 'node:test';
import { allPages, branchPath, characterView, idText, preferences } from '../public/scripts/go-chat/data.js';
import { ChatSession } from '../public/scripts/go-chat/session.js';

const options = { model: 'test', n: 2 };
const question = { id: '1', parent_id: null, role: 'user', content: '问题' };
const answer = { id: '2', parent_id: '1', role: 'assistant', content: '回复' };

/** 内存存储模拟同一浏览器中多个账号的偏好。 */
function storage() {
    const values = new Map();
    return { getItem: key => values.get(key), setItem: (key, value) => values.set(key, value) };
}

/** 写操作同步到服务端数组，使重新加载读取真实的模拟持久化结果。 */
function fixture(initial = [], chat = { id: '10' }) {
    const db = initial.map(item => ({ ...item }));
    const calls = [];
    const api = {
        chat: async id => ({ ...chat, id }),
        chatBootstrap: async id => ({ story: { title: '夜航' }, chat_id: id }),
        relationship: async () => null,
        messages: async () => ({ items: db.map(item => ({ ...item })), total: String(db.length) }),
        addMessage: async (_id, body) => {
            calls.push('save');
            const item = { ...body, id: String(db.length + 1), role: 'user' };
            db.push(item);
            return { ...item };
        },
        cancelGeneration: async id => { calls.push(`cancel:${id}`); },
        deleteMessage: async id => { db.splice(db.findIndex(item => item.id === id), 1); },
        editMessage: async (id, body) => Object.assign(db.find(item => item.id === id), body),
        reviseAssistant: async (id, body) => {
            const source = db.find(item => item.id === id);
            const item = { ...source, ...body, id: String(db.length + 1), parent_id: source.parent_id };
            db.push(item);
            return { ...item };
        },
        replySuggestions: async (id, body) => {
            calls.push(`suggest:${id}:${body.parent_id}:${body.model || ''}`);
            return { items: ['继续追问', '换个话题'] };
        },
    };
    const prefs = preferences('10', storage());
    const generate = async (_id, body, handlers) => {
        calls.push('generate');
        const item = { id: String(db.length + 1), parent_id: body.parent_id, role: 'assistant', content: '回复' };
        db.push(item);
        handlers.message_end({ message_id: item.id });
    };
    return { db, calls, api, prefs, session: new ChatSession({ api, prefs, generate }) };
}

test('IDs reject unsafe numbers and preserve large strings', () => {
    assert.equal(idText('9007199254740993'), '9007199254740993');
    for (const value of [9007199254740992, null, 0, -1, '01', 'bad']) assert.throws(() => idText(value));
    assert.equal(characterView({ ID: 1, Name: '角色' }).name, '角色');
});

test('pagination reads beyond 100 and rejects incomplete lists', async () => {
    const queries = [];
    const items = await allPages(async query => {
        queries.push(query);
        return { items: Array(queries.length === 1 ? 100 : 1).fill('item'), total: '101' };
    });
    assert.equal(items.length, 101);
    assert.equal(queries[1], '?page=2&size=100');
    await assert.rejects(allPages(async () => ({ items: [], total: '1' })), /不完整/);
});

test('branch path excludes siblings and rejects cycles or missing ancestors', () => {
    const sibling = { ...answer, id: '3' };
    assert.deepEqual(branchPath([question, answer, sibling], '3'), [question, sibling]);
    assert.throws(() => branchPath([answer], '2'), /不完整/);
    assert.throws(() => branchPath([{ id: '1', parent_id: '1' }], '1'), /不完整/);
});

test('preferences isolate users and restore selected branch', async () => {
    const store = storage();
    const a = preferences('1', store);
    const b = preferences('2', store);
    a.set('leaf:10', '2');
    assert.equal(b.get('leaf:10'), null);
    const f = fixture([question, answer, { ...answer, id: '3' }]);
    f.session.prefs = a;
    await f.session.open('10');
    assert.equal(f.session.leaf, '2');
    f.session.choose('3');
    await f.session.open('10');
    assert.equal(f.session.leaf, '3');
});

test('send saves user first and generates from its persisted ID', async () => {
    const f = fixture([question, answer]);
    await f.session.open('10');
    await f.session.send(' 下一个问题 ', options);
    assert.deepEqual(f.calls, ['save', 'generate']);
    assert.equal(f.db[2].parent_id, '2');
    assert.equal(f.db[3].parent_id, '3');
    assert.equal(f.session.leaf, '4');
});

test('failed generation retains question and retry does not duplicate it', async () => {
    const f = fixture();
    await f.session.open('10');
    const generate = f.session.generate;
    f.session.generate = async () => { throw new Error('offline'); };
    await assert.rejects(f.session.send('问题', options), /offline/);
    assert.equal(f.session.leaf, '1');
    assert.equal(f.session.busy, false);
    f.session.generate = generate;
    await f.session.retry('1', options);
    assert.equal(f.calls.filter(item => item === 'save').length, 1);
    assert.equal(f.db.length, 2);
});

test('invalid generation options do not persist a question', async () => {
    const f = fixture();
    await f.session.open('10');
    await assert.rejects(f.session.send('问题', { model: 'test', n: 5 }));
    assert.equal(f.db.length, 0);
});

test('active generation prevents duplicate submits and branch changes', async () => {
    const f = fixture();
    await f.session.open('10');
    let finish;
    f.session.generate = () => new Promise(resolve => { finish = resolve; });
    const pending = f.session.send('问题', options);
    await Promise.resolve();
    await assert.rejects(f.session.send('重复', options), /尚未完成/);
    await assert.rejects(f.session.open('11'), /尚未完成/);
    assert.throws(() => f.session.choose(null), /停止/);
    finish();
    await pending;
    assert.equal(f.db.length, 1);
});

test('candidate selection preserves original reply and descendants', async () => {
    const child = { id: '3', parent_id: '2', role: 'user', content: '追问' };
    const f = fixture([question, answer, child]);
    f.api.selectVariant = async () => ({ ...answer, id: '4', content: '候选回复' });
    await f.session.open('10');
    await f.session.select('2', '7');
    assert.deepEqual(f.session.path.map(item => item.id), ['1', '4']);
    assert.equal(f.session.messages.find(item => item.id === '2').content, '回复');
    assert.equal(f.session.messages.find(item => item.id === '3').parent_id, '2');
});

for (const status of [null, 404, 500]) {
    test(`stop aborts stream and cleans up even when cancel returns ${status}`, async () => {
        const f = fixture();
        await f.session.open('10');
        f.api.cancelGeneration = async () => {
            if (status) throw Object.assign(new Error('cancel failed'), { status });
        };
        f.session.generate = async (_id, _body, handlers, signal) => {
            handlers.message_start({ generation_id: 9 });
            await new Promise((_resolve, reject) => signal.addEventListener('abort', () =>
                reject(new DOMException('Aborted', 'AbortError')), { once: true }));
        };
        const pending = f.session.send('问题', options);
        await Promise.resolve();
        if (status === 500) await assert.rejects(f.session.stop(), /cancel failed/);
        else await f.session.stop();
        await pending;
        assert.equal(f.session.active, null);
        assert.equal(f.session.busy, false);
        await f.session.stop();
    });
}

test('only terminal user messages may be edited and parents cannot be deleted', async () => {
    const f = fixture([question, answer]);
    await f.session.open('10');
    await assert.rejects(f.session.edit('修改'), /用户消息/);
    f.session.choose('1');
    await assert.rejects(f.session.edit('修改'), /后续分支/);
    await assert.rejects(f.session.remove(), /后续分支/);
    f.session.choose('2');
    await f.session.remove();
    await f.session.edit('修改');
    assert.equal(f.session.path[0].content, '修改');
});

test('explicit empty branch remains empty after reopening', async () => {
    const f = fixture([question, answer]);
    await f.session.open('10');
    f.session.choose(null);
    await f.session.open('10');
    assert.equal(f.session.leaf, null);
    assert.deepEqual(f.session.path, []);
});

test('regeneration targets original assistant without saving another question', async () => {
    const f = fixture([question, answer]);
    let target;
    f.session.regenerate = async (id, _body, handlers) => {
        target = id;
        f.db.push({ ...answer, id: '3', content: '重新生成' });
        handlers.message_end({ message_id: '3' });
    };
    await f.session.open('10');
    await f.session.retry('2', options);
    assert.equal(target, '2');
    assert.equal(f.session.leaf, '3');
    assert.equal(f.db[1].content, '回复');
    assert.deepEqual(f.calls, []);
});

test('assistant revision creates a sibling branch and preserves the source reply', async () => {
    const f = fixture([question, answer]);
    await f.session.open('10');
    await f.session.revise(' 更自然的回复 ');
    assert.deepEqual(f.session.path.map(item => item.id), ['1', '3']);
    assert.equal(f.db.find(item => item.id === '2').content, '回复');
    assert.equal(f.db.find(item => item.id === '3').content, '更自然的回复');
    assert.equal(f.db.find(item => item.id === '3').parent_id, '1');
});

test('reply suggestions anchor to the assistant leaf without changing history', async () => {
    const f = fixture([question, answer]);
    await f.session.open('10');
    const result = await f.session.suggestions('guided');
    assert.deepEqual(result.items, ['继续追问', '换个话题']);
    assert.deepEqual(f.calls, ['suggest:10:2:guided']);
    assert.equal(f.db.length, 2);
    f.session.choose('1');
    await assert.rejects(f.session.suggestions(), /AI 回复/);
});

test('story chat loads its frozen bootstrap alongside message history', async () => {
    const f = fixture([question, answer], { id: '10', mode: 'story' });
    await f.session.open('10');
    assert.deepEqual(f.session.story, { story: { title: '夜航' }, chat_id: '10' });
    assert.equal(f.session.path.at(-1).id, '2');
});

test('stop before generation ID aborts connection without cancel request', async () => {
    const f = fixture();
    await f.session.open('10');
    f.session.generate = async (_id, _body, _handlers, signal) =>
        new Promise((_resolve, reject) => signal.addEventListener('abort', () =>
            reject(new DOMException('Aborted', 'AbortError')), { once: true }));
    const pending = f.session.send('问题', options);
    await Promise.resolve();
    await f.session.stop();
    await pending;
    assert.deepEqual(f.calls, ['save']);
    assert.equal(f.session.active, null);
});
