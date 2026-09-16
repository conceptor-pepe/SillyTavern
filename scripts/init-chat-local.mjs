/** 初始化本机部署的随机凭据和自签名证书；拒绝覆盖已有配置。 */
import { mkdir, readFile, writeFile, chmod } from 'node:fs/promises';
import { randomBytes } from 'node:crypto';
import { spawnSync } from 'node:child_process';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../', import.meta.url));
const dir = path.join(root, 'data/go-local');
const random = () => randomBytes(32).toString('hex');

/** 只显式复用旧系统当前启用的密钥，绝不打印配置值。 */
async function providerConfig() {
    if (!process.argv.includes('--from-legacy')) {
        return {
            url: process.env.AI_CHAT_PROVIDER_URL,
            key: process.env.AI_CHAT_PROVIDER_KEY,
            model: process.env.AI_CHAT_PROVIDER_MODEL,
        };
    }
    const settings = JSON.parse(await readFile(path.join(root, 'data/default-user/settings.json'), 'utf8'));
    const secrets = JSON.parse(await readFile(path.join(root, 'data/default-user/secrets.json'), 'utf8'));
    const current = settings.oai_settings;
    if (current.chat_completion_source !== 'custom') throw new Error('Legacy provider is not custom');
    const url = new URL(current.custom_url);
    url.pathname = `${url.pathname.replace(/\/$/, '')}/chat/completions`;
    return { url: url.href, key: secrets.api_key_custom?.find(item => item.active)?.value, model: current.custom_model };
}

/** 使用保守字符集写 Compose env，防止换行与变量替换改变凭据含义。 */
function envText(values) {
    return Object.entries(values).map(([key, value]) => {
        if (!value || /[\r\n'$\\]/.test(value)) throw new Error(`Invalid or missing ${key}`);
        return `${key}='${value}'`;
    }).join('\n') + '\n';
}

/** 生成证书只作用于本目录，不安装系统信任或修改全局证书。 */
function makeCert() {
    const result = spawnSync('openssl', [
        'req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '30',
        '-keyout', path.join(dir, 'tls/localhost.key'), '-out', path.join(dir, 'tls/localhost.crt'),
        '-subj', '/CN=localhost', '-addext', 'subjectAltName=DNS:localhost,IP:127.0.0.1',
    ], { stdio: 'ignore' });
    if (result.status !== 0) throw new Error('Certificate generation failed');
}

const provider = await providerConfig();
if (new URL(provider.url).protocol !== 'https:') throw new Error('Provider requires HTTPS');
const content = envText({
    CHAT_DB_PASSWORD: random(), CHAT_ROOT_PASSWORD: random(), AI_CHAT_AUTH_SECRET: random(),
    AI_CHAT_PROVIDER_URL: provider.url, AI_CHAT_PROVIDER_KEY: provider.key, AI_CHAT_PROVIDER_MODEL: provider.model,
});
await mkdir(path.join(dir, 'tls'), { recursive: true, mode: 0o700 });
await chmod(dir, 0o700);
await writeFile(path.join(dir, '.env'), content, { flag: 'wx', mode: 0o600 });
makeCert();
await chmod(path.join(dir, 'tls/localhost.key'), 0o600);
console.log('Local deployment initialized at data/go-local (secrets not printed).');
