/** 从已安装依赖复制聊天 Markdown 运行库，开发与二进制打包共用同一版本。 */
import { cp, mkdir } from 'node:fs/promises';

const target = new URL('../public/scripts/go-chat/vendor/', import.meta.url);
await mkdir(target, { recursive: true });
await cp(new URL('../node_modules/showdown/dist/showdown.min.js', import.meta.url), new URL('showdown.min.js', target));
await cp(new URL('../node_modules/dompurify/dist/purify.min.js', import.meta.url), new URL('purify.min.js', target));
