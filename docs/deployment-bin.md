# 单进程业务部署

更新日期：2026-09-16。当前默认部署方式：MySQL/Redis 使用容器，
前端页面、Go API、SSE、HTTP 全部由宿主机上的一个 `ai-chat` 二进制提供。
没有业务容器、Nginx 进程或 Node 运行时服务；Node 只参与构建和验收。
前后端代码和 API 契约仍分离，发布时将静态前端嵌入二进制。

## 当前实例

- 入口：`http://127.0.0.1:8080`，只监听本机。
- 程序：`dist/ai-chat`。
- 配置：`data/go-local/config.json`，权限 0600，含数据库密码和模型密钥。
- 账号：`data/go-local/account.json`；沿用上一轮验收账号和角色。
- 当前后台进程 PID：`data/go-local/bin.pid`；JSON 日志：`data/go-local/bin.log`。
- MySQL：`127.0.0.1:13307`，Redis：`127.0.0.1:16380`。
- 数据卷仍为 `ai-chat-local_mysql`、`ai-chat-local_redis`，未重建业务数据。
- 原 `ai-chat-local-api-1` 和 `ai-chat-local-web-1` 已停止并移除。

本地 HTTP 不使用证书，避免自签名证书导致 `ERR_CERT_AUTHORITY_INVALID`。
仅明确绑定回环 IP 且未配置 TLS 时关闭 Cookie 的 Secure，保留 HttpOnly/SameSite=Lax。
其他部署仍默认 Secure；不要把本地 HTTP 改为公网或局域网监听。
正式部署需使用可信 HTTPS；未修改系统证书信任。

## 构建与启动

在仓库根目录执行：

```sh
# 编译前端白名单资源与 Go；产物为 dist/ai-chat
node scripts/build-chat-bin.mjs

# 已有配置无需再生成；首次转换才运行，拒绝覆盖已有文件
node scripts/config-chat-bin.mjs

# 仅启动数据库容器
docker compose --env-file data/go-local/.env -f docker/docker-compose.chat-bin.yml up -d --wait

# 直接运行唯一业务进程
./dist/ai-chat --config ./data/go-local/config.json
```

当前实例已经后台运行，重复启动会报端口占用并退出。更换实例前，
先用 `ps -p "$(cat data/go-local/bin.pid)" -o command=` 确认是本项目程序，
再用 `kill -TERM "$(cat data/go-local/bin.pid)"` 正常停止。
前台运行时使用 Ctrl+C；它与 SIGTERM 都走已有请求取消和存储关闭流程。

运行不依赖工作目录中的 `public` 或 Node。配置使用 JSON，字段对应 Go Config：
`HTTPAddr`、`MySQLDSN`、`RedisAddr`、`RedisPass`、`AuthSecret`、
`ProviderURL`、`ProviderKey`、`ProviderModel`、`TLSCert`、`TLSKey`。
文件值覆盖环境变量；未知字段、弱密钥、缺少 MySQL 或不成对 TLS 配置会被拒绝。
证书路径建议用绝对路径，ProviderURL 是完整 Chat Completions URL。

新机器需要先按 `deployment-local.md` 的初始化章节生成 `.env`（本地 HTTP 不需要证书），
再运行本页的配置转换。不要同时运行旧全容器 Compose 的 API/Web。
不要删除数据卷，不要把配置和密钥加入版本库或分发到前端。

## 发布边界

构建脚本只拷贝独立聊天 HTML、CSS、JS、字体和头像到临时 embed 目录，
不会包含旧用户文件、配置或密钥。嵌入目录由 Git 忽略。
普通 `go build ./cmd/api` 仍是纯 API 构建；包含页面必须使用构建脚本
（其内部使用 `-tags webembed`）。

升级时重新构建，正常停止旧二进制，再启动新二进制。
目前为单实例，升级有短暂中断；没有自动守护或开机自启。
数据库仍走现有启动迁移机制，程序回退不等于数据库结构回退。

## 验证结果

- 真实 DeepSeek 两轮对话、口令上下文、流式草稿、刷新/重登恢复通过。
- 一次中途取消，数据库同会话为 completed=2、cancelled=1，无半条 assistant。
- 从原容器切到本机程序后，原聊天消息仍可读取。
- 实际空闲 SIGTERM 退出和二进制重启通过，重启后历史仍可读取。
- 非白名单资源、配置与用户秘密返回 404；目录不枚举。
- 新增配置边界、静态资源/HEAD/MIME/目录拒绝和监听失败退出单元测试。
- 配置真实 MySQL/Redis 的 `go test -race -tags webembed ./... -count=1` 通过。
- `go vet -tags webembed ./...` 通过。

真实浏览器测试仍用 `scripts/chat-live.browser.mjs`，会消耗模型额度。
切换或手动重启后，只验证恢复时运行：

```sh
node scripts/chat-restart.check.mjs --verify-only
```

需要已安装 Playwright，或使用 `PLAYWRIGHT_MODULE` 指定其路径。
测试默认访问本地 HTTP；其他入口通过 `AI_CHAT_TEST_ORIGIN` 指定。
测试不再忽略 HTTPS 证书错误；旧 HTTPS 验收不能作为证书信任已通过的依据。
二进制模式下不要省略 `--verify-only`：旧脚本默认会重启全容器备选部署。

## 审计和剩余项

本轮十个 Go 文件原始审计 11 条、相对 HEAD 增量 6 条，均为错误分支日志提示：
配置/启动/关闭错误交给入口 Zap；静态资源不存在返回 404；
embed 的固定路径失败分支只返回空 FS。没有新增豁免，未声称审计全绿。
函数长度、命名及 if 嵌套没有违规。仍用手写依赖装配，没有 Wire。

未改变数据归属、聊天事务、领域状态或余额规则。
尚未验证生成过程中真实 SIGTERM、异常数据库退出和公网部署；
两张旧版内置角色卡已完整迁入 `conceptor` 账号，包含标准开场白和作者备注字段；
完整旧账号、聊天和头像迁移、
安全加固及完整版本化 migration 继续按任务清单跟进。
