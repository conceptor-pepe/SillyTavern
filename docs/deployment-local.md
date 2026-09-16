# 本机部署与真实聊天验收

> 2026-09-16 更新：当前已按用户要求切换为 **MySQL/Redis 容器 + 单个业务二进制**。
> 默认启动请看 `deployment-bin.md`。下方保留早期全容器方案与验收记录，
> 不代表当前仍在运行 Go/Nginx 容器。不要与二进制模式同时启动。

## 交付范围

2026-09-16：独立 Go API、Nginx 静态前端、MySQL、Redis 已部署到本机 Docker。
入口为 `https://localhost:8443`，只绑定 `127.0.0.1`。不是公网部署，也没有切换旧系统入口。

模型使用旧系统当前启用的 DeepSeek 配置，实际 `/models` 请求确认
`deepseek-flash` 可用。密钥只复制到被 Git 忽略、权限为 0600 的
`data/go-local/.env`，不进入前端、镜像或文档。没有修改旧配置、旧账号或旧聊天。

本机验收专用项目名为 `ai-chat-local`，具有独立网络、数据库和卷；
已有 `ai-chat-mysql`、`ai-chat-redis` 开发容器不受影响。

## 启动

在仓库根目录执行。首次初始化需要 Node.js、OpenSSL、Docker Compose；
初始化脚本拒绝覆盖已有 `.env`，已经初始化的机器直接执行第二条命令。

```sh
node scripts/init-chat-local.mjs --from-legacy
docker compose --env-file data/go-local/.env -f docker/docker-compose.chat-local.yml up -d --build
docker compose --env-file data/go-local/.env -f docker/docker-compose.chat-local.yml ps
```

不用旧配置时，通过环境变量提供 `AI_CHAT_PROVIDER_URL`、
`AI_CHAT_PROVIDER_KEY`、`AI_CHAT_PROVIDER_MODEL`，运行初始化脚本时不加 `--from-legacy`。
URL 必须是完整 HTTPS Chat Completions 地址，不是只有域名的 base URL。
不要把密钥直接写在会保存历史的 shell 命令中。

API 使用非 root 用户和只读文件系统；MySQL、Redis、API 不映射宿主机端口。
Nginx 只复制聊天所需静态文件，API 流代理关闭缓冲。
登录保留 Secure、HttpOnly Cookie，没有为 HTTP 开发环境降低安全属性。

## 登录

已准备专用验收账号及一个“聊天助手”角色。账号与随机密码位于：

```text
data/go-local/account.json
```

打开本机入口后用此账号登录，模型填写 `deepseek-flash`，候选选择 `1 条`。
新注册账号不会自动获得角色：目前没有角色创建 UI，验收脚本仅为专用账号写入角色。
这不是正式角色导入或旧数据迁移。

本机证书有效期 30 天，未安装系统信任。浏览器首次访问会提示自签名证书；
仅对此本机地址确认后继续。自动化测试使用 `ignoreHTTPSErrors`，
不代表证书链已经可信；生产环境必须换成域名和可信证书。

## 验收脚本

```sh
node scripts/chat-live.browser.mjs
node scripts/chat-restart.check.mjs
npm run test:go-frontend
```

脚本需要能导入 Playwright，可通过 `PLAYWRIGHT_MODULE` 指定现有安装：

```sh
PLAYWRIGHT_MODULE=/Users/con/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright/index.mjs \
node scripts/chat-live.browser.mjs
```

真实聊天脚本每次新建一个会话，调用模型三次：两次简短对话，一次开始后取消。
会产生真实模型费用，并保留合成验收数据。无 `page.route` 或模拟 Provider。
角色夹具通过专用数据库写入，注册、登录、会话、消息、生成、取消均走正式 HTTP API。

已通过的路径：

| 验收项 | 证据 |
|---|---|
| 注册、登录、退出、重新登录 | 浏览器真实 Cookie；退出后 `/me` 返回 401 |
| HTTPS → Nginx → Gin → DeepSeek | 三个生成响应均为 HTTP 200、`text/event-stream` |
| 逐步显示真实回复 | MutationObserver 实际观察到非空流式草稿 |
| 两轮上下文 | 第二轮正确回忆第一轮随机合成口令 |
| 完成落库和刷新恢复 | assistant 消息持久化，刷新及重新登录后保留 |
| 停止生成 | MySQL cancelled，未留下第三条 assistant，无 pending/running |
| 桌面与手机 | 1440×900、375×812 截图，无横向溢出、可见图片加载成功 |
| 整栈空闲停止和启动 | 历史消息恢复，原模型无需再次调用 |
| 静态发布白名单 | 用户秘密、`.env`、旧 settings 和旧应用入口不可访问 |

取证文件在 Git 忽略目录，避免误提交账号及聊天内容：

```text
data/go-local/evidence/live-report.json
data/go-local/evidence/restart-report.json
data/go-local/evidence/desktop.png
data/go-local/evidence/mobile.png
```

首次执行在末尾 SSE 响应体取证断言失败，但真实业务路径已通过；
已改用真实响应头、流式草稿、前端完成协议和数据库结果组合断言，重新执行通过。
不把 Chromium 无法读取已释放响应体误认为后端生成失败。
前端原有 30 项 Node 回归通过；携带真实 MySQL/Redis 配置的
`go test -race ./... -count=1`、`go vet ./...` 及 `git diff --check` 均通过。

## 运维与回退

```sh
# 只停止此部署，不删除数据卷，也不停止旧系统
docker compose --env-file data/go-local/.env -f docker/docker-compose.chat-local.yml stop

# 再次启动，保留现有数据
docker compose --env-file data/go-local/.env -f docker/docker-compose.chat-local.yml up -d

# 验证配置语法，不打印含密钥的完整配置
docker compose --env-file data/go-local/.env -f docker/docker-compose.chat-local.yml config --quiet
```

禁止把 `down -v` 当作常规回退步骤，它会删除数据库卷。
当前回退只需停止新入口并继续使用未修改的旧系统。
这不是正式新旧数据双写或生产数据回滚演练。

## 尚未验收

- 公网服务器、域名、可信证书、备份恢复、容量和监控告警。
- 真实模型多候选、模型错误/余额不足、超长上下文和供应商切换。
- 生成中真实进程 SIGTERM、多实例代理取消、HTTP/2 与数据库故障组合。
- 旧数据正式导入、差异对账、生产切流及数据回退。
- 全量版本化 migration、账号会话撤销/CSRF/限流及逐域敏感日志审查。

本轮未修改 Go 业务实现，没有新增 Go 审计豁免。
部署可用不等于整个迁移 Goal 完成，相关主任务保持进行中。
