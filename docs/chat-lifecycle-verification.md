# 会话删除生命周期验证

## 当前阶段

2026-09-16，T012-4 进行中。已实现删除与生成写入之间的数据库仲裁，
删除时原子取消生成任务并清理收藏；真实 MySQL 与跨实例 TLS 删除测试通过。

## 事实与边界

- 会话和生成任务的用户归属不变，无账本或额度变更。
- 会话行锁及可见性归会话域所有；`internal/port.ChatGate` 仅暴露带上下文的回调。
- `chat/infra.Gate.WithChat` 开事务并以 `FOR UPDATE` 读取指定用户未删除会话，
  失败不执行回调；`repo.InTx/DB` 向各域仓储传递同一事务连接。
- 事务上下文仅限同一数据库的同步调用链，不允许跨数据库、跨请求或异步复用。
  嵌套事务使用 GORM 保存点，最外层提交前行锁不释放。
- 生产装配显式将 Gate 注入生成仓储与完成写入器，构造参数必填；
  不提供省略可见性检查的默认实现。
- 生成 Create、SaveDone 及兼容消息 Create 都在锁内写入自己拥有的数据。
  SaveDone 继续保留用户、会话、任务和 running 条件，不能覆盖取消/超时终态。
- `chat/app.Remover` 在会话锁内调用会话删除和生成应用服务的 `CancelChat`。
  各域只写自己的数据；`port.ChatTasks` 暴露取消能力，不在会话仓储更新生成表。
- 会话软删除、当前用户该会话收藏清理、pending/running 取消及结束时间同事务提交。
  已有终态完整保留。回调失败整体回滚，不能将回调中的不存在错误误当成幂等成功。
- 收藏写入、生成启动/完成及任务查询复用会话锁；删除后不再返回任务事实。
  运行中的任务监听发现不可见后取消 Provider，不要求本机持有原始 Runner。

## 场景矩阵

| 顺序或条件 | 期望 | 当前证据 |
|---|---|---|
| 删除已提交，再创建生成或保存结果 | 拒绝，无新增任务/消息/候选 | 真实 MySQL TestDeletedChatWrites |
| 其他用户创建或保存结果 | 拒绝，不改归属及原始任务 | 同上 |
| 完成事务持有会话锁，再删除 | 删除等待，不得越过完成事务 | TestChatDeleteLock，删除报错且上下文已到期，会话保持未删除 |
| 写入成功后会话回调返回错误 | 任务、消息、候选与完成状态一起回滚 | TestChatWriteRollback |
| 前置归属检查成功，创建事务发现删除 | HTTP 404，不启动 SSE | TestCreateChatGone，错误注入路由测试 |
| 已运行生成时删除 | 原子取消，原实例监听后关闭上游 | TestRemoteRemove：真实 TLS/Cookie/MySQL、两套生产装配 |
| 删除与收藏/创建/启动/完成并发 | 不留收藏或活动任务；完成先提交则保留完整历史回复 | TestRemoveRace |
| 删除取消写入后失败 | 会话、收藏及任务完整回滚 | TestRemoveRollback，注入领域不存在错误也不能吞掉 |
| 已有终态、重复/越权删除 | 终态完整保留，幂等且无跨用户副作用 | TestRemoveTasks |
| 删除后查询/启动/收藏/生成 | 查询隐藏、写入拒绝，无迟到完成 | TestRemoveTasks、TestRemoteRemove、TestDeletedChatWrites |

MySQL 锁测试使用实际连接和上下文时限，不用固定睡眠模拟数据库冲突。
回滚测试使用真实会话 Gate，在写入后返回错误，确认生成仓储未逃逸外层事务。
已有完成、超时与取消竞争测试补齐真实会话数据，继续走生产 Guard，不使用放行桩。

## 验证命令

在 backend 执行：

```sh
AI_CHAT_TEST_MYSQL_DSN='root:root_dev@tcp(127.0.0.1:3307)/ai_chat?parseTime=true' \
AI_CHAT_TEST_REDIS_ADDR=127.0.0.1:6380 GIN_MODE=release \
go test -race ./... -count=1
go vet ./...
```

全量通过；开发凭据仅用于本地隔离测试库。删除保护测试随后使用真实 MySQL 重复三轮。
重复测试曾暴露断言对错误链的错误假设：GORM 拼接回滚错误时原截止错误可能仅保留文本。
已改为检查删除错误、实际上下文截止状态及会话仍未删除，不依赖错误字符串或放宽为任意错误。
生产采用手写装配，无 Wire 入口；没有新增外部依赖。

## 审计

十四文件：port/chat.go、repo/scope.go、chat/infra/gate.go，
generation/infra 的 repo.go、done.go、chat_mysql_test.go、done_mysql_test.go、
expire_mysql_test.go、expire_race_test.go，generation/app/timeout_mysql_test.go，
generation/http 的 handler.go、chat_test.go，httpapi 的 server.go、remote_flow_test.go。

- 原始 39 条：标准测试签名 Context 提示 9 条，错误分支日志提示 30 条。
- 相对 HEAD 增量 16 条：Context 提示 9 条，错误透传提示 7 条。
  部分测试文件此前未提交，增量工具会将其全部计入。
- 测试实际使用 t.Context 或派生时限，标准 TestXxx 签名不接受 context 参数。
- Gate/生成仓储错误沿既有调用链交给 HTTP 边界 Zap；装配与停止错误由入口记录；
  测试回调把错误交给断言。未增加豁免，未声明审计全绿。
- 新增文件/函数均有中文注释，长度、命名与 if 嵌套未触发规则。

## 本轮补充验证

- `go test -race ./internal/generation/infra ./internal/httpapi -run 'TestRemove|TestRemoteRemove' -count=3` 通过。
- 携带真实 MySQL/Redis 环境变量的全量 `go test -race ./... -count=1` 通过；`go vet ./...` 通过。
- 本轮生产实现及三个新测试共十五文件单独审计：原始 39 条（错误分支 34、测试 Context 4、reflect 1），
  相对 HEAD 增量 13 条（错误分支 8、测试 Context 4、reflect 1）。
  reflect 仅用于比较事务前后完整数据库记录；测试通过 t.Context 传递上下文。
  透传错误由 HTTP Zap 或启动入口处理，测试错误由断言处理；未添加审计豁免，未声明全绿。
- 首次审计发现测试辅助函数嵌套超限，已改为平级判断；最终无函数长度、命名或嵌套规则提示。
- 新增文件、函数均有中文职责注释；未新增依赖，仍使用手工依赖装配。

## 剩余边界

1. 两套服务实例仍在同一操作系统进程；独立进程、多机、代理和 HTTP/2 未验收。
2. 启动先提交时执行已获准调用 Provider，删除依靠监听最终取消，不承诺远端瞬时停止。
3. Favorite 已增加专用历史索引升级、命名锁及状态记录，并通过真实 MySQL 升级/漂移/恢复/并发测试；仍不是完整通用版本化迁移系统，见任务清单最新记录。
4. 全局日志审计及处理器错误脱敏仍需收口。其他用户任务、其他会话任务混合批量取消范围还需独立矩阵测试。

T012-4 和迁移 Goal 继续保持进行中；本轮结果不代表整个迁移已完成。
