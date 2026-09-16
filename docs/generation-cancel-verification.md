# 跨实例生成停止验证

## 范围与事实

2026-09-16，推进 T012-1/T012-2。保留原 DELETE 路由及响应，复用生成域
`Service.Find/Cancel/Expire`，不跨域访问业务表，不引入新依赖或 Redis 消息广播。

- 用户只能取消自己的 pending/running 任务；数据库条件更新先成功，再尝试本机中断。
- MySQL 已提交状态是跨实例依据。每个运行任务每秒查询一次，每次最多两秒；
  非 running、任务消失或查询错误均取消上游，避免未知状态下继续付费生成。
- 原用户、会话及归属不变，无额度账本变更。
- 原实例失败收尾仍受状态条件保护，不能覆盖其他实例写入的 cancelled/failed。
- 上游读取及关闭完成后停止监听并等待退出，再提交结果；主动关闭查询不会取消正常结果提交。
- 监听退出至结果提交之间仍可能有远端取消，已有 SaveDone 事务的 running 条件负责最终仲裁。
- 每个活动任务约增加一条查询/秒，不是瞬时通知；实际延迟取决于调度、数据库和客户端写入。

## 证据与测试

| 文件 | 验证内容 |
|---|---|
| `backend/internal/generation/app/watch_test.go` | 所有终态、归属参数、查询时限、数据库错误、主动停止与查询退出、取消原因保留 |
| `backend/internal/httpapi/remote_flow_test.go` | 两套独立生产 Server/连接池/Runner，共享数据库及签名配置；真实 TLS、登录 Cookie 和 MySQL |
| `backend/internal/generation/http/log_test.go` | 执行失败 Zap 日志隐藏原始错误，保留用户/会话/任务/请求编号；SSE 不泄漏错误 |

跨实例集成测试覆盖：

1. 实例 A 发起生成，本地 HTTP 模型服务保持真实上游连接。
2. 实例 B 上的第二用户读取/取消被拒绝，不影响原任务。
3. 本人在 B 调用 DELETE，或 B 使用同一个生成域 Expire 用例模拟超时清理。
4. A 感知终态，关闭上游；原 SSE 返回 generation_error，没有 message_end。
5. 数据库保留 cancelled 或 failed/generation_timeout，不留下 assistant 消息，重复取消返回 404。

两套服务运行于同一测试进程，但取消表与连接池独立；不是独立进程、多机或外部 Provider 验收。
查询故障目前由单元测试注入，没有把真实 MySQL 停机作为故障演练。

## 验证命令

在 backend 目录执行：

```sh
AI_CHAT_TEST_MYSQL_DSN='root:root_dev@tcp(127.0.0.1:3307)/ai_chat?parseTime=true' \
AI_CHAT_TEST_REDIS_ADDR=127.0.0.1:6380 GIN_MODE=release \
go test -race ./... -count=1
go vet ./...
```

均通过。数据库测试使用隔离临时库；上述凭据仅为本地开发容器配置。
本轮六文件已 gofmt；生产装配使用手写构造，没有 Wire 生成入口。

## 审计

六文件范围：generation/app 的 run.go、watch.go、watch_test.go，
generation/http 的 handler.go、log_test.go，以及 httpapi/remote_flow_test.go。

- 原始审计 24 条错误分支日志提示：Runner 7 条、监听 2 条、已有 Handler 15 条。
- 增量审计 4 条：RunEvents 两个错误转入 fail 的分支，watchTask/checkTask 各一条。
- 新监听错误由 cancel cause 传给 Runner.fail，再由 Handler.run 统一使用 Zap 记录；
  本轮将此执行边界改为 SafeError，避免数据库或上游原始文本进入日志。
- 没有增加日志豁免，未将脚本标记为全绿。Handler 原有部分参数拒绝无日志、
  其他错误路径未统一脱敏，仍需独立收口。
- 中文文件/函数注释、函数长度、命名和条件嵌套未触发规则。

## 后续边界

- SSE 已设置单帧两秒写入与刷新时限，并通过真实 TCP/TLS 慢读测试，
  见 `docs/generation-stream-verification.md`；HTTP/2 与代理部署仍待验收。
- 会话删除尚未原子取消关联任务，新监听不等同于删除生命周期已完成。
- HTTP Shutdown 超时保留连接、重试停止、主动取消活动请求及等待业务收尾已验证，
  见超时验证文档；真实 SIGTERM、慢读与停机组合和数据库故障下最终退出仍待验收。
- 需补多进程部署与容量验证，评估每个活动任务每秒读取的数据库负载。
- 当前 T012 和迁移 Goal 继续进行，不将本轮证据扩展为全量迁移验收。
