/** 白名单打包前端并构建单进程业务程序，运行时不需要 Node。 */
import { cp, mkdir, rm } from 'node:fs/promises';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import './prepare-chat-assets.mjs';

const root = fileURLToPath(new URL('../', import.meta.url));
const target = path.join(root, 'backend/internal/webui/assets');
await rm(target, { recursive: true, force: true });
await mkdir(target, { recursive: true });
const entries = [
    ['go-chat.html', 'index.html'],
    ['css/go-chat.css'], ['css/fontawesome.min.css'], ['css/solid.min.css'],
    ['webfonts'], ['img/ai4.png'], ['img/characters'], ['scripts/api-client.js'], ['scripts/api-events.js'],
    ['scripts/go-chat'],
];
for (const [source, output = source] of entries) {
    await mkdir(path.dirname(path.join(target, output)), { recursive: true });
    await cp(path.join(root, 'public', source), path.join(target, output), { recursive: true });
}
await mkdir(path.join(root, 'dist'), { recursive: true });
execFileSync('go', ['build', '-tags', 'webembed', '-trimpath', '-o', path.join(root, 'dist/ai-chat'), './cmd/api'],
    { cwd: path.join(root, 'backend'), stdio: 'inherit' });
console.log('Built dist/ai-chat with embedded frontend.');
