/** 独立前端开发服务器：提供静态页面，并原样转发 Go API 和 SSE。 */
import express from 'express';
import http from 'node:http';
import https from 'node:https';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../public/', import.meta.url));

/** 不加载旧 server-main，不读取用户数据、Provider 密钥或 Node 登录状态。 */
export function createFrontend(upstream = 'http://127.0.0.1:8080') {
    const target = new URL(upstream);
    if (!['http:', 'https:'].includes(target.protocol)) throw new Error('Invalid API protocol');
    const app = express();
    app.disable('x-powered-by');
    app.use('/api/v1', (req, res) => proxyAPI(req, res, target));
    app.get('/', (_req, res) => res.sendFile(path.join(root, 'go-chat.html')));
    app.use(express.static(root, { index: false, dotfiles: 'deny' }));
    return app;
}

/** 使用流式管道，不缓冲 SSE；浏览器断开时同步关闭到 Go 的请求。 */
function proxyAPI(req, res, target) {
    const transport = target.protocol === 'https:' ? https : http;
    const upstream = transport.request(new URL(req.originalUrl, target), {
        method: req.method,
        headers: { ...req.headers, host: target.host },
    }, response => {
        res.writeHead(response.statusCode, response.headers);
        response.on('error', () => res.destroy());
        response.pipe(res);
    });
    upstream.on('error', () => {
        if (res.headersSent) return res.destroy();
        res.status(502).json({ code: 'API_UNAVAILABLE', message: 'Go API 暂时不可用' });
    });
    req.on('aborted', () => upstream.destroy());
    res.on('close', () => { if (!res.writableEnded) upstream.destroy(); });
    req.pipe(upstream);
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
    const port = Number(process.env.AI_CHAT_FRONTEND_PORT || 5174);
    const upstream = process.env.AI_CHAT_API_ORIGIN || 'http://127.0.0.1:8080';
    const server = createFrontend(upstream).listen(port, '127.0.0.1', () => {
        console.log(`AI Chat frontend: http://127.0.0.1:${server.address().port}`);
    });
    server.on('error', error => { console.error(error.message); process.exitCode = 1; });
}
