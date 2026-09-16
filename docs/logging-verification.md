# Zap 日志接入验证

更新日期：2026-09-16。范围为 API/迁移命令、GORM、MySQL 驱动、
go-redis 内部日志和 HTTP 异常恢复，不代表所有业务错误字段均已完成隐私审查。

## 接入规则

- API 和迁移命令共用 `logx.New`，输出 JSON 格式 Zap 日志。
- `db.OpenMySQL` 显式接收 Logger。GORM 和底层 MySQL 驱动分别使用连接级适配器，
  不修改驱动全局 Logger；连接失败时释放已创建的连接池。
- GORM 不展开 Trace 的 SQL 回调，不输出原始 SQL、绑定参数、重复键值。
  默认记录错误及超过 200 毫秒的慢查询；`record not found` 不在依赖层打印。
  调用 `Debug()` 也不会输出 SQL。`LogMode` 返回副本，不修改其他请求的级别。
- MySQL 错误保留编号，例如 `mysql error 1062`；取消、超时保留分类，
  其他错误仅保留类型。原始错误仍返回业务调用方，不改变错误判断语义。
- go-redis 9.16.0 的日志接口为进程级。API 在创建客户端之前调用
  `redis.SetLogger`，禁止在每个请求或并发 `Server.New` 中反复设置。
  其他可执行入口若增加 Redis，也必须在启动阶段安装适配器。
- Redis 日志以模板 SHA-256 前八字节摘要作为 `event_id`，
  用于关联同一依赖调用点；不展开模板参数和服务器原始错误。
- 当前部署为 Redis 7，显式禁用仅新版本支持的维护通知握手；
  这不是关闭错误日志，其他驱动日志仍交给 Zap。
- API 启动使用 Gin release 模式；请求 panic 经专用恢复中间件写入 Zap，
  保留请求编号、路由模板、调用栈，不记录请求头、正文和 panic 原始值。
  标准 HTTP Server 的错误 Logger 也桥接到 Zap。

本次未改变业务对象归属、数据库结构或 API 正常响应合同。
已经发送响应的流发生 panic 时不再追加 JSON 错误，避免破坏流格式。

## 测试证据

| 测试 | 验证 |
|---|---|
| `TestGormTrace` | 错误、慢查询、Info 和 Silent 级别；不展开 SQL；重复键错误原文隔离 |
| `TestGormModes` | 配置副本互不影响；GORM 日志模板和参数不进入日志 |
| `TestDependencyLogs` | Redis/MySQL 适配器保留组件和安全错误字段，不泄露参数；模板摘要稳定 |
| `TestSafeError` | nil、取消、超时和未知错误的安全分类 |
| `TestMySQLLogs` | 真实数据库插入私密值后触发重复键，Zap 仅记录错误编号，不输出 SQL |
| `TestRecoveryLogs` | Cookie、Authorization、正文和 panic 原始值不会出现在恢复日志及错误响应 |

MySQL 8.4.11、Redis 7.4.11 环境下执行：

```bash
cd backend
export AI_CHAT_TEST_MYSQL_DSN='<测试账号>:<密码>@tcp(127.0.0.1:3307)/ai_chat?parseTime=true'
export AI_CHAT_TEST_REDIS_ADDR='127.0.0.1:6380'
export GIN_MODE=release
go test -race ./... -count=1
go vet ./...
go test ./internal/httpapi -run 'Test(Generation|Cancel)Flow' -count=1 -v
```

全量竞态测试、静态检查和真实 HTTP 回归均通过；HTTP 回归输出中不再出现
原始 SQL 或 Redis 维护通知降级日志。隔离测试库清理完毕。

另在 18081 临时端口运行实际 API 入口，仅启用 Redis：
启动输出为 JSON `redis ready` 日志，`/healthz` 返回 `{"status":"ok"}`。
检查后已停止该临时进程，端口无残留监听。该启动检查不代替完整数据库集成测试。

## 规范审计

本轮十五个 Go 文件：原始审计 14 条，增量审计 5 条，不宣称脚本全绿。
已修复新增测试辅助函数的嵌套深度提示。其余增量提示人工复核：

- 两条测试 Context 提示：测试遵守标准 `func(t *testing.T)` 签名，
  真实数据库操作使用 `t.Context()`；依赖日志测试不执行网络操作。
- 三条数据库辅助函数日志提示：错误返回启动入口，由入口用 Zap 记录，
  不在连接构造、Connector 和启动层重复打印。入口已使用安全错误分类。
- 原始报告还包含迁移命令已有六参数函数、服务装配及旧连接函数的透传提示；
  本轮没有扩大重构或添加审计豁免。

## 尚未覆盖

- 部分业务 Handler 仍直接 `zap.Error(err)`，驱动错误可能含业务值，
  需逐域审查，不能据依赖层已脱敏推断全链路无泄漏。
- 请求级日志字段关联、外部 Provider 错误持久化内容、主动退出时的日志刷盘
  错误处理、所有业务写操作成功日志仍需继续核验。
- Redis 分布式取消、陈旧 pending 任务补偿、会话删除生命周期、旧数据实迁、
  浏览器真实后端联调及外部模型验收不在本次日志测试覆盖内。
