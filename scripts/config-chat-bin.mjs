/** 复用现有验收密钥，生成仅本机可读的二进制配置，不覆盖已有文件。 */
import { readFile, writeFile } from 'node:fs/promises';
import { parseEnv } from 'node:util';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const root = fileURLToPath(new URL('../', import.meta.url));
const dir = path.join(root, 'data/go-local');
const env = parseEnv(await readFile(path.join(dir, '.env'), 'utf8'));
const cfg = {
    HTTPAddr: '127.0.0.1:8080',
    MySQLDSN: `ai_chat:${env.CHAT_DB_PASSWORD}@tcp(127.0.0.1:13307)/ai_chat?parseTime=true&charset=utf8mb4`,
    RedisAddr: '127.0.0.1:16380',
    AuthSecret: env.AI_CHAT_AUTH_SECRET,
    ProviderURL: env.AI_CHAT_PROVIDER_URL,
    ProviderKey: env.AI_CHAT_PROVIDER_KEY,
    ProviderModel: env.AI_CHAT_PROVIDER_MODEL,
    TLSCert: '',
    TLSKey: '',
};
await writeFile(path.join(dir, 'config.json'), JSON.stringify(cfg, null, 2), { flag: 'wx', mode: 0o600 });
console.log('Created data/go-local/config.json (secrets not printed).');
