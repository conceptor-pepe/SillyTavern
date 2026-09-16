# 生成超时补偿验证

## 范围

2026-09-16，推进 T012-2，修复周期清理漏掉等待任务和缺失启动时间的运行任务。
首轮仅修改生成域存储规则；后续接入运行截止时间和清理协程生命周期。
原有 Domain/App 仓储接口、HTTP 路由及一分钟调度间隔不变。

## 业务边界

- 发起者是内部清理调度，处理所有用户的超时任务；不改变用户、会话和来源时间。
- 复用 `Service.Expire -> Repo.Expire`，不新增平行清理入口，不跨业务域更新表。
- pending 使用 UTC `created_at`；running 优先使用 Unix 秒 `started_at`，NULL 时回退创建时间。
- 严格早于截止时间才处理；已有 completed、failed、cancelled 不再修改。
- 单次 UPDATE 同时检查时间和状态，写入 failed、generation_timeout、通用错误描述和结束时间。
- 无资金或额度变动；现有调度边界通过 Zap 记录失败或实际变更数量。
- 完成写入仍复用既有事务：已超时任务不满足 running 条件，消息和候选一起回滚。

## 验证

新增 `backend/internal/generation/infra/expire_mysql_test.go`：

- 13 种组合：pending 的过期/边界/新建/异常启动时间；running 的过期/边界/新启动；
  running 缺失启动时间的过期/边界/新建；三个已有终态。
- 每个组合检查实际变更数量、重复执行幂等、归属和来源时间保持、未过期记录不变。
- 超时后迟到的生成结果不能提交消息或候选，不能从 pending 重新启动，超时错误不被覆盖。

在隔离真实 MySQL 测试库和 Redis 环境执行：

```sh
cd backend
AI_CHAT_TEST_MYSQL_DSN='root:root_dev@tcp(127.0.0.1:3307)/ai_chat?parseTime=true' \
AI_CHAT_TEST_REDIS_ADDR=127.0.0.1:6380 GIN_MODE=release \
go test -race ./... -count=1
go vet ./...
```

以上通过；格式化和 `git diff --check` 通过。测试凭据仅用于本地开发容器。

## 审计

范围：generation/infra 的 repo.go、expire.go、expire_mysql_test.go，
generation/app/task.go，以及仅更新清理注释的 httpapi/server.go。

- 原始审计 8 条：2 条测试函数 Context 提示、6 条既有仓储/服务初始化错误透传提示。
- 增量审计 2 条：标准 `TestXxx(*testing.T)` 签名无法接受 context 参数，
  两个测试的实际数据库及仓储调用均传 `t.Context()`。
- 未增加日志豁免，不宣称审计脚本全绿；初始化错误由入口记录，清理错误由调度边界记录。
- 新增文件和函数有中文注释，函数长度及条件嵌套未触发审计规则。

## 运行与调度补充

2026-09-16 后续实现：

- `domain.RunLimit` 统一十分钟标准，Runner 对执行上下文设置 deadline；开始、请求头等待、
  流读取和消息写入共用时限。比它更短的调用方 deadline 仍优先生效。
- 超时关闭 Provider HTTP 请求，失败收尾使用独立五秒上下文。将运行上下文错误加入原因链，
  防止响应体关闭错误掩盖超时；保留条件更新，已取消或已补偿的任务不会改写。
- 数据库可查询的错误描述仅使用 `generation failed` / `generation timed out`，
  不持久化 Provider URL、凭据或驱动原始错误；其他 Handler 日志尚未全域完成脱敏。
- 清理模块依赖生成域 `Expire` 能力，不直接读取或更新业务表。
  启动即扫、每分钟串行重扫，每次查询五秒时限；未配置 Provider 也清理数据库遗留任务。
- Stop 等待清理退出才释放数据库，等待超时保留连接并返回错误；
  再次调用 Stop 可以继续收尾。初始化失败会尝试释放已有 MySQL/Redis 连接并合并错误。
- 调度失败日志使用 Zap 安全错误分类，成功日志包含数量、目标状态及超时错误码；
  正常关停造成的取消不产生日志噪声。

### 补充测试

- `generation/app/timeout_test.go`：请求头/流读取总时限、失败状态、流关闭、取消表释放、
  默认时限与领域策略一致、可查询错误脱敏。
- `generation/app/timeout_mysql_test.go`：生产 OpenAI 兼容适配器访问真实本地 HTTP 服务，
  分别模拟响应头等待和流正文阻塞；确认对端感知断连、MySQL 写入超时终态、没有半条消息。
  测试只缩短 Runner 私有时限，不修改生产默认值，也不把本地模拟上游当作外部 Provider 验收。
- 首次响应头场景失败源于模拟服务未消费请求正文；补齐正文读取后，两次生成域/HTTP 回归通过。
- `httpapi/cleanup_test.go`：启动及周期调用、数据库上下文时限、停止等待及超时、
  失败日志脱敏、成功计数和关停静默。
- `httpapi/cleanup_mysql_test.go`：真实生产装配在无 Provider 配置时立即补偿陈旧任务，
  Stop 返回时清理已退出、MySQL 连接已关闭。
- `generation/infra/expire_race_test.go`：同一任务上并发执行 Expire 与 SaveDone/Cancel，
  每类四轮；仅一个终态生效，失败完成事务不留下消息或候选。
  这是实际数据库竞争测试，与 Go `-race` 内存竞态检查分别提供证据。

沿用上文环境命令，全量 `go test -race ./... -count=1`、`go vet ./...`、格式化和
`git diff --check` 通过。装配使用手写构造，仓库没有 Wire 生成入口，本轮无新增依赖。

### 补充审计

本轮九文件：generation/domain/generation.go，generation/app 的 run.go、timeout_test.go、
timeout_mysql_test.go，generation/infra/expire_race_test.go，以及 httpapi 的 server.go、
cleanup.go、cleanup_test.go、cleanup_mysql_test.go。

- 原始 16 条：6 条 Runner 错误透传提示、6 条初始化/连接错误透传提示、
  3 条标准测试签名 Context 提示、1 条测试桩错误透传提示。
- 增量 5 条：3 条标准 `TestXxx(*testing.T)` Context 提示（实际调用均传上下文）；
  1 条测试仓储原样传播状态拒绝；1 条 New 汇总初始化与关闭错误，由可执行入口统一记 Zap。
- 未增加豁免。Runner 的业务边界日志仍由 HTTP Handler 记录；
  该项是现有分层审计提示，不将脚本结果标记为全绿。
- 新增函数和文件均有中文注释；函数行数、命名、条件嵌套未触发审计规则。

## 剩余边界

- 2026-09-16 已补齐数据库终态轮询：其他实例取消或清理后，原实例停止上游。
  两套独立服务装配的实际 HTTP/MySQL 测试通过，见 `docs/generation-cancel-verification.md`。
  未引入 Redis 广播，未验证独立操作系统进程或多机部署。
- SSE 已增加单帧两秒写入与刷新时限，真实 TCP/TLS 慢客户端失败收尾已验证，
  见 `docs/generation-stream-verification.md`；HTTP/2 与反向代理仍待验收。
- HTTP Shutdown 超时保留依赖、请求取消及收尾等待已验证，见下节；
  真实 SIGTERM、慢读与停机组合和数据库故障下最终退出仍待验收。
- 初始化部分失败后的 Redis 释放已有代码路径，尚无针对该错误组合的专门测试。
- 会话删除与生成任务的原子收尾仍待实现。
- T012 和迁移 Goal 继续保持进行中；本次无外部 Provider 或浏览器真实后端验收。

## 停机资源保护补充

2026-09-16，`Server.Stop` 不再于 HTTP Shutdown 超时后关闭存储连接。
HTTP 与清理协程都退出才调用 closeClients，其他情况返回合并错误，供调用方重试。
没有修改用户归属、任务状态转换或接口响应，也没有新增业务域依赖。

`backend/internal/httpapi/shutdown_mysql_test.go` 使用实际 HTTP 服务和隔离 MySQL：
让请求进入 Handler 并等待，确认第一次停止返回 DeadlineExceeded，连接池仍可 Ping；
释放请求后，Handler 自身的数据库调用成功、HTTP 客户端返回，再次停止成功且数据库关闭。
该测试验证 Server 生命周期，不模拟为生产 API 路由或系统信号端到端测试。

沿用上文环境命令，全量竞态测试和 go vet 通过。两个修改文件已 gofmt。
原始审计 8 条日志提示：server.go 7 条、测试请求协程 1 条；
相对 HEAD 的增量审计 3 条，包括前轮 New 初始化透传、本轮 Stop 错误透传、
测试客户端把错误交给测试主线程。可执行入口已有初始化及停止 Zap 日志；
没有增加豁免，不宣称审计全绿。

该阶段主程序 `cmd/api/main.go` 只调用一次十秒 Stop，失败后记录并退出；
后续已接入请求主动取消和收尾等待，详见下节。连接保留只保证 Server 方法不提前关闭依赖，
不代表进程退出后连接还能存活。

## 请求退出等待补充

2026-09-16，新增 `httpapi/drain.go`，在所有生产路由外登记 Handler 生命周期。
归属、请求值和原始 deadline 不变，只合并服务停止的取消信号；
停机不直接修改任何业务表，仍由 Runner.fail 使用独立五秒上下文持久化失败。

- 入口登记和停止标记共用互斥锁，停止后不增加活动业务请求，返回 503。
- 停止取消上下文后仍等待 Handler 返回，包括独立上下文的终态写入。
- HTTP Shutdown、登记器等待和清理协程等待共用调用方预算；任一失败都不释放依赖。
- 连接关闭通过 sync.Once 保护，重复停止返回相同关闭结果，不重复关闭 Redis。
- 主程序保留十秒停止预算，成功记录 `api stopped`，失败记录脱敏 Zap 错误后退出。
  未能落库的任务仍依赖下次启动/周期补偿；这不是故障下保证即时终态的承诺。

新增及调整验证：

| 测试 | 证据 |
|---|---|
| `drain_test.go` | 请求收到取消后暂缓返回，等待超时；新请求 503；释放后重试成功；空登记器及重复停止 |
| `shutdown_mysql_test.go` | 使用登记器且业务暂不响应取消，超时保留连接；独立收尾上下文可查询真实 MySQL |
| `shutdown_flow_test.go` | 生产服务的真实 TLS/Cookie/模型 HTTP 连接；Stop 后上游关闭、任务 failed/generation_failed、归属不变、无 assistant，之后连接池已关闭 |
| `flow_fixture_test.go` | TLS 测试直接使用生产 Server 的 http.Server，不再只复用 Handler 而让 Stop 作用于未监听的对象 |

全量真实 MySQL/Redis 竞态测试、go vet 通过；七文件 gofmt 完成，无 Wire 入口。
审计范围为 cmd/api/main.go、httpapi 的 server.go、drain.go、drain_test.go、
flow_fixture_test.go、shutdown_flow_test.go、shutdown_mysql_test.go。
原始 8 条、增量 3 条错误分支日志提示：Server 初始化/停止透传到入口 Zap，
测试请求错误通过通道交给断言。无新增豁免，不标记脚本全绿。

剩余验收：真实操作系统信号、慢读与停机组合、数据库故障下独立五秒收尾失败及最终退出。
SSE 单帧阻塞已有两秒时限及真实网络测试，详见慢客户端验证文档。
当前成功生成停机证据来自真实网络和数据库，但不是独立进程 SIGTERM 测试。
