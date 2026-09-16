/** 本机真实端到端验收：不拦截 API，使用 HTTPS、MySQL 和已配置的真实模型。 */
import assert from 'node:assert/strict';
import { readFile, writeFile, mkdir } from 'node:fs/promises';
import { randomBytes } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../', import.meta.url));
const dir = `${root}data/go-local`;
const origin = process.env.AI_CHAT_TEST_ORIGIN || 'http://127.0.0.1:8080';
const compose = ['compose', '--env-file', `${dir}/.env`, '-f', `${root}docker/docker-compose.chat-local.yml`];
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
const env = await readFile(`${dir}/.env`, 'utf8');
const model = env.match(/^AI_CHAT_PROVIDER_MODEL='([^']+)'$/m)?.[1];
assert.ok(model, 'configured model missing');
await mkdir(`${dir}/evidence`, { recursive: true, mode: 0o700 });

/** 只使用容器自己的凭据；SQL 输出不包含密码、Token 或真实用户文本。 */
function sql(query) {
    return execFileSync('docker', [...compose, 'exec', '-T', 'mysql', 'sh', '-c',
        'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql --default-character-set=utf8mb4 -uroot -N -B ai_chat'],
    { input: query, encoding: 'utf8', cwd: root }).trim();
}

/** 随机账号仅保存在忽略目录，可供用户后续登录本机验收环境。 */
async function account() {
    try {
        return { ...JSON.parse(await readFile(`${dir}/account.json`, 'utf8')), fresh: false };
    } catch (error) {
        if (error.code !== 'ENOENT') throw error;
    }
    const value = { handle: `live-${Date.now()}`, password: randomBytes(18).toString('hex'), name: '本机验收' };
    await writeFile(`${dir}/account.json`, JSON.stringify(value, null, 2), { flag: 'wx', mode: 0o600 });
    return { ...value, fresh: true };
}

/** 角色写入仅用于验收数据准备，所有用户、会话、消息和生成操作走正式 API。 */
function seedRole(uid) {
    assert.match(String(uid), /^[1-9][0-9]*$/);
    sql(`INSERT INTO characters
        (user_id,name,description,personality,scenario,first_message,message_sample,creator,creator_notes,tags,extra_data,created_at,updated_at)
        SELECT ${uid},'聊天助手','本机真实模型验收角色','用简洁中文回复','日常聊天','你好','','验收','','[]','{}',UTC_TIMESTAMP(3),UTC_TIMESTAMP(3)
        WHERE NOT EXISTS (SELECT 1 FROM characters WHERE user_id=${uid} AND name='聊天助手');`);
}

/** 等待真实持久化 assistant 消息，失败时尽快报告 UI 错误。 */
async function send(page, content, count) {
    await page.locator('#message').fill(content);
    await page.locator('#send').click();
    await page.waitForFunction(expected => {
        const error = document.querySelector('#notice');
        const ready = document.querySelectorAll('[data-role=assistant]').length === expected;
        return ready || (!error.hidden && error.textContent);
    }, count, { timeout: 120000 });
    assert.equal(await page.locator('#notice').isVisible(), false, await page.locator('#notice').textContent());
    await page.waitForFunction(() => !document.querySelector('#send').disabled);
    return page.locator('[data-role=assistant] .content').last().textContent();
}

/** 保存桌面与移动端证据，并检查静态资源及布局边界。 */
async function screenshot(page, name, width, height) {
    await page.setViewportSize({ width, height });
    await page.evaluate(() => document.fonts.ready);
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
    assert.equal(await page.evaluate(() => [...document.images].filter(x => x.getClientRects().length)
        .every(x => x.complete && x.naturalWidth > 0)), true);
    await page.screenshot({ path: `${dir}/evidence/${name}.png`, fullPage: true });
}

const user = await account();
const browser = await chromium.launch({ headless: true });
const report = { started_at: new Date().toISOString(), origin, model, api_mocked: false, streams: [] };
try {
    const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
    const page = await context.newPage();
    const errors = [];
    page.on('pageerror', error => errors.push(error.message));
    page.on('response', response => {
        if (!/\/generations$/.test(new URL(response.url()).pathname)) return;
        report.streams.push({ status: response.status(), type: response.headers()['content-type'] });
    });
    await page.goto(origin);
    await page.locator('#auth').waitFor({ state: 'visible' });
    if (user.fresh) {
        await page.locator('#auth-mode').click();
        await page.locator('[name=name]').fill(user.name);
    }
    await page.locator('[name=handle]').fill(user.handle);
    await page.locator('[name=password]').fill(user.password);
    await page.locator('#auth-submit').click();
    await page.locator('#workspace').waitFor({ state: 'visible' });
    const identity = await (await context.request.get(`${origin}/api/v1/me`)).json();
    const uid = identity.user.id ?? identity.user.ID;
    seedRole(uid);
    const cookies = await context.cookies();
    assert.ok(cookies.some(x => x.name === 'ai_chat_token' && x.secure && x.httpOnly));
    await page.locator('#library').click();
    await page.locator('[data-character]').first().click();
    await page.locator('#start-chat').click();
    await page.locator('#model').fill(model);
    await page.locator('#count').selectOption('1');
    await page.evaluate(() => {
        window.liveDraftSeen = false;
        new MutationObserver(() => {
            if (document.querySelector('.draft .content')?.textContent) window.liveDraftSeen = true;
        }).observe(document.querySelector('#messages'), { childList: true, subtree: true });
    });
    const marker = `青松${randomBytes(3).toString('hex')}`;
    await send(page, `你好！请记住本次测试口令“${marker}”。仅用一句简短中文确认。`, 1);
    const reply = await send(page, '刚才我让你记住的口令是什么？只回复口令。', 2);
    assert.ok(reply.includes(marker), 'real model did not recall prior context');
    assert.equal(await page.evaluate(() => window.liveDraftSeen), true, 'no visible streaming draft');
    const ids = await page.locator('[data-message]').evaluateAll(nodes => nodes.map(x => x.dataset.message));
    ids.forEach(id => assert.match(id, /^[1-9][0-9]*$/));
    report.message_ids = ids;
    report.context_recalled = true;
    await page.reload();
    await page.locator(`[data-message="${ids.at(-1)}"]`).waitFor();
    assert.ok((await page.locator('[data-role=assistant] .content').last().textContent()).includes(marker));
    report.refresh_persisted = true;
    await screenshot(page, 'desktop', 1440, 900);
    await screenshot(page, 'mobile', 375, 812);

    await page.locator('#message').fill('请逐行列出从1到1000的数字，每行解释这个数字，不要省略。');
    await page.locator('#send').click();
    await page.waitForFunction(() => Boolean(document.querySelector('.draft .content')?.textContent), null, { timeout: 120000 });
    await page.locator('#stop').click();
    await page.waitForFunction(() => !document.querySelector('#send').disabled, null, { timeout: 15000 });
    assert.equal(await page.locator('[data-role=assistant]').count(), 2);
    await page.locator('#logout').click();
    await page.locator('#auth').waitFor({ state: 'visible' });
    assert.equal((await context.request.get(`${origin}/api/v1/me`)).status(), 401);
    await page.locator('[name=handle]').fill(user.handle);
    await page.locator('[name=password]').fill(user.password);
    await page.locator('#auth-submit').click();
    await page.locator(`[data-message="${ids.at(-1)}"]`).waitFor();
    report.relogin_persisted = true;
    report.tasks = sql(`SELECT status,COUNT(*) FROM generations WHERE user_id=${uid}
        AND conversation_id=(SELECT conversation_id FROM messages WHERE id=${ids[0]} AND user_id=${uid})
        GROUP BY status;`);
    assert.match(report.tasks, /completed\t2/);
    assert.match(report.tasks, /cancelled\t1/);
    assert.doesNotMatch(report.tasks, /pending|running|failed/);
    assert.equal(report.streams.length, 3);
    assert.ok(report.streams.every(x => x.status === 200 && x.type?.includes('text/event-stream')));
    assert.deepEqual(errors, []);
    report.status = 'passed';
    console.log(JSON.stringify(report, null, 2));
} catch (error) {
    report.status = 'failed';
    report.error = error.message;
    throw error;
} finally {
    await writeFile(`${dir}/evidence/live-report.json`, JSON.stringify(report, null, 2), { mode: 0o600 });
    await browser.close();
}
