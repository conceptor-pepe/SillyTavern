# MySQL 集成测试

更新日期：2026-09-16。

## 环境与执行

本轮实际运行 MySQL 8.4.11，容器为项目 Compose 的 `ai-chat-mysql`，本机端口 3307。
Docker daemon 已启动；Redis 7 镜像重试拉取成功，`ai-chat-redis` 健康，
实际版本 7.4.11，本机端口 6380；没有使用缓存 Redis 8 替代。

```bash
docker compose -f docker/docker-compose.ai-chat.yml up -d --wait mysql redis
cd backend
export AI_CHAT_TEST_MYSQL_DSN='<专用测试账号>:<密码>@tcp(127.0.0.1:3307)/ai_chat?parseTime=true'
export AI_CHAT_TEST_REDIS_ADDR='127.0.0.1:6380'
export GIN_MODE=release
go test -race ./... -count=1
go vet ./...
```

测试账号必须允许创建和删除数据库。仅用于本机或专用测试 MySQL，
不得使用生产连接或生产管理员凭据。
`internal/testdb.Open` 忽略 DSN 中已有库名，为每次测试创建 `ai_chat_test_<UUID>`，
注册清理函数后再建立测试连接，测试结束只删除对应随机库。
异常终止整个测试进程时可能遗留测试库，应人工确认名称后清理。
缺少 DSN 会明确 Skip；跳过不能视为数据库测试通过。

## 本轮通过的证据

| 测试 | 覆盖 |
|---|---|
| `TestDeleteMySQL` | 越权删除不生效；重复删除；记录保留；列表、最近、详情、归属、改名和收藏过滤删除会话；子消息与候选不可见 |
| `TestSourceSchema` | 001 初始结构已有消息经 002 升级后保持不变，候选来源唯一键拒绝重复 |
| `TestBranchMySQL` | 递归父链顺序、兄弟分支隔离、用户和会话归属、消息及会话删除过滤 |
| `TestSelectionMySQL` | 重复候选选择复用独立分支；原消息不覆盖；越权拒绝；删除分支不隐式恢复 |
| `TestConcurrentSelection` | 八个并发请求选择同一候选，返回同一个消息 ID，最终只存一条来源记录 |
| `TestDoneMySQL` | 消息、候选、generation 完成状态一起提交；重复完成的新增数据回滚 |
| `TestDoneRollback` | cancelled 任务、其他用户、重复候选编号失败时，不遗留主消息或候选，不改写任务状态 |

`go test -race ./... -count=1` 全量通过；`go vet ./...` 通过。
测试完成后查询 `information_schema.SCHEMATA`，没有残留的 `ai_chat_test_%` 数据库。

## 测试夹具修正

原分支测试使用 MySQL 临时表，递归 SQL 多次引用同一临时表报
`Error 1137: Can't reopen table`。原候选测试复用单连接 GORM 实例，
测试计数查询条件污染后续查询。本轮改成独立库的普通表，并为计数使用独立 Context 会话。
这两项属于测试隔离修复，不以修改生产分支 SQL 来迁就测试夹具。

## 结构升级

- SQL 管理的库：先执行 `migrations/001_init.sql`，再执行 `002_message_source.sql`。
- 002 新增 `messages.source_variant_id` 及唯一索引，允许旧消息字段为空。
- 002 是一次性 DDL；已经通过 AutoMigrate 创建该字段/索引的库不得重复执行。
- API 启动仍走已有 AutoMigrate。统一版本化迁移执行和发布记录尚未完成，不能把新增 SQL 文件当作完整迁移发布方案。

## 审计与边界

本轮九个 Go 文件的全文件审计报告 14 条提示，增量审计报告 8 条，未标记工具审计通过。
人工复核增量提示：

- 6 条测试 Context 提示：Go 测试签名必须为 `func(t *testing.T)`，相关数据库调用使用 `t.Context()`；
  `TestChatScope` 本身为 DryRun，不建立数据库连接。
- 1 条仓储日志提示：`chat/infra.Repo.Set` 返回错误，调用方 `chat/http.Handler.fail` 使用 Zap 记录用户、
  请求和会话标识；不在 Repository 重复记录同一错误。
- 1 条并发测试错误日志提示：失败先通过 `t.Errorf` 记录，再继续接收其他协程结果；
  不是静默忽略生产错误。

未包含用户原有 `migration/infra/scan.go` 改动；本轮未修改其代码。
会话软删除不等于完整删除生命周期：删除期间正在运行的 Provider 任务终止、
收藏关系清理、生成任务查询可见性仍需继续检查。
本轮不是浏览器到真实 Provider 的端到端验收，也不是旧数据迁移实跑验收。

## HTTP 集成验证补充

2026-09-16 新增生产 `httpapi.New` 装配的真实网络测试。每次运行使用隔离
MySQL 库、真实 Redis 连接和 TLS 测试服务；注册及登录由 CookieJar 接收
Secure Cookie，没有伪造用户令牌。角色通过数据库准备，其余业务操作通过 HTTP。
模型上游是本地 HTTP SSE 模拟服务，不是外部付费 Provider。

| 测试 | 覆盖 |
|---|---|
| `TestGenerationFlow` | 匿名拒绝、注册、退出、错误密码、正确登录、会话创建、保存用户消息、双候选 SSE、任务完成、父节点、历史读取、候选重复选择幂等 |
| `TestCancelFlow` | 真实第二用户无法读取或取消任务；本人取消关闭上游连接；cancelled 终态不被失败收尾覆盖；不保存半条 assistant 消息；重复取消返回 404 |
| `TestStartDisconnect` | 首帧发送前请求取消，不留下 pending 任务 |
| `TestStartWriteError` | 底层写入报错但 Context 尚未取消时，仍取消待执行任务 |
| `TestStartCancelled` | Runner 开始时 Context 已取消，独立超时收尾将 pending 标记 failed，保留已有 cancelled |

首帧失败原先会直接返回，已通过失败测试复现并修复。SSE 先编码完整帧，
再检查实际写入错误或短写；首帧失败通过独立五秒 Context 调用原有 Cancel 用例。
Runner 的 Start 失败也复用原有 Fail 收尾，不新建一套状态更新规则。

上述环境下重新执行 `go test -race ./... -count=1` 和 `go vet ./...`，全量通过。
测试结束再次确认无 `ai_chat_test_%` 库残留。
Redis 在此验证连接与启动装配，未据此宣称跨实例取消或分布式任务协调完成。

七个本轮 Go 文件的全文件审计报告 25 条，增量报告 4 条，仍不标记脚本全绿：
一条属于 `startRepo.Move` 测试桩；三条属于 `send` 的 Context、编码和写入错误透传。
首尾事件调用方通过 `logSend` 记录 Zap，增量发送失败由 Runner 返回并在 `run`
记录 Zap。没有新增豁免或在每层重复打印错误。

本次实际装配也暴露两个既有日志缺口：GORM 默认 Logger 仍输出原始 SQL，
go-redis 在 Redis 7 下打印维护通知握手降级提示。业务 Handler 使用 Zap，
但依赖日志尚未全部统一，后续必须收口并验证敏感字段不落日志。
后续同日已完成依赖适配与真实重复键脱敏测试，见 `docs/logging-verification.md`；
以上描述保留为该轮发现记录，不代表当前依赖日志状态。
进程崩溃或收尾数据库不可用导致的陈旧 pending 清理、删除期间取消、
真实外部 Provider、浏览器真实后端联调、旧数据迁移及灰度回退仍待完成。
