# 独立 Go 聊天前端

更新日期：2026-09-16。状态：已通过本机 HTTPS、真实 Go/MySQL/DeepSeek 联调；
未进行旧数据实迁和生产切流。当前前端嵌入单个 Go 二进制，
MySQL/Redis 使用容器，部署入口与证据见 `deployment-bin.md`。

## 启动

在项目根目录执行：

```bash
npm run start:frontend
```

默认页面 `http://127.0.0.1:5174/`，API 转发到 `http://127.0.0.1:8080`。
可通过 `AI_CHAT_FRONTEND_PORT` 和 `AI_CHAT_API_ORIGIN` 修改端口和 Go API 地址。
需要独立启动 Go 服务及其 MySQL、Redis 依赖；前端服务不会启动旧 Node 聊天后端。
前端未连接 Go 时显示接口不可用，不提供假登录或模拟回复。

开发服务器只监听本机。登录 Cookie 当前由 Go 设置 Secure、HttpOnly；
本地浏览器的安全上下文策略可能影响 HTTP 下的 Cookie，登录回环时应使用本地 HTTPS，
不要为此删除生产 Cookie 的 Secure 属性。

## 入口和职责

- `public/go-chat.html`：独立页面入口；旧 `public/index.html` 不改动。
- `public/scripts/api-client.js`：同源 `/api/v1` 请求与鉴权错误。
- `public/scripts/api-events.js`：SSE 字节解析和异常清理。
- `public/scripts/go-chat/data.js`：响应兼容、ID、分页、分支和用户隔离偏好。
- `public/scripts/go-chat/session.js`：消息持久化、生成、取消、候选和分支状态。
- `public/scripts/go-chat/view.js`：安全文本渲染。
- `public/scripts/go-chat/main.js`：登录、角色目录与页面操作编排。
- `scripts/serve-frontend.mjs`：静态文件和不缓冲的 API 流代理，不承载业务逻辑。

## 本轮可用范围

登录/注册/退出、角色搜索与详情、本地收藏、会话列表与创建、
消息历史、保存提问后生成、流式回复、停止、重试、重新生成、
候选回复选择、分支继续聊天和刷新恢复。
仅允许编辑没有后续分支的用户消息，删除也限制为无后代叶节点。
所有服务端文本通过 `textContent` 展示，不执行回复中的 HTML。
本地只存用户隔离的模型、候选数量、收藏和会话/分支编号，不存消息正文或密钥。

## 验证

```bash
npm run test:go-frontend
```

30 项 Node 测试覆盖客户端、SSE、会话状态和真实 HTTP 代理传输；
代理测试使用临时端口的模拟上游，不连接业务数据库。

启动开发服务后，使用已安装的 Playwright 执行：

```bash
node scripts/go-chat.browser.mjs
```

未安装到本项目时，可用 `PLAYWRIGHT_MODULE` 指向已有的 Playwright ESM 入口。
可配置 `FRONTEND_URL`、`FRONTEND_ARTIFACTS`，默认截图目录 `/tmp/ai-chat-frontend`。
浏览器脚本明确拦截并模拟 API，检查登录、选角色、候选分支追问、
刷新恢复、退出清理、HTML 注入保护、位图加载和桌面/手机/横屏边界。
逐字流的跨块与中止由独立网络/解析测试覆盖，不把一次性模拟 SSE 当真实流体验验收。

## 尚未完成

- 真实 Go + MySQL + Redis + Provider 全链路，包括事务、取消落库和重复提交。
- 旧 consumer shell 默认入口切换、旧数据迁移验收与灰度回退演练。
- 角色接口尚无头像字段，当前使用已有默认头像。
- 收藏只保存在当前浏览器，尚未跨设备同步。
- 模型名称手工输入；Provider 密钥仅在后端配置，没有前端密钥输入。
- 角色/用户接口仍有大写字段，创建会话仍要求数字角色 ID；适配器拒绝不安全数字。
- 消息 Markdown、附件、复杂角色编辑、群聊和记忆不属于本轮入口实现。
- 后端会话软删除及候选来源 schema 已补齐并通过真实 MySQL 测试，见 `docs/mysql-verification.md`；
  删除期间生成任务终止等生命周期、统一迁移发布和前后端端到端删除验证仍待完成。

## 部署边界

生产部署静态资源和 Go API，使用同源 HTTPS 反向代理；
`/api/v1` 转发到 Go，关闭 SSE 代理缓冲并配置适合长连接的超时。
发布静态入口时保留 `css`、`scripts`、`img`、`webfonts` 相对目录；
不要把数据目录、配置文件或 Provider 密钥发布到静态根目录。
开发 Express 代理不是生产网关。

## 最新验收补充

2026-09-16 已新增独立 Docker/Nginx HTTPS 本机部署，并通过真实浏览器、Go、
MySQL、DeepSeek 的两轮聊天、流式显示、上下文、取消、刷新及重登恢复验收。
原 `scripts/go-chat.browser.mjs` 仍为模拟 UI 回归；
真实验收入口为 `scripts/chat-live.browser.mjs`，详细步骤和限制见 `deployment-local.md`。
旧默认入口未切换，上面的开发服务器说明仍适用于开发环境。
