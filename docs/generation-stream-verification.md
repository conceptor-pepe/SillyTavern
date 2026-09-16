# SSE 慢客户端验证

## 范围

2026-09-16，推进 T012。仅修改 SSE 传输边界，不改变路由、事件格式、用户归属或生成事务。
复用首帧失败取消与 Runner 失败收尾，不新增依赖或跨域写表。

## 实现

- `generation/http/stream.go` 每帧设置两秒写时限，覆盖 Write 和 Flush，
  返回时清除时限，模型等待不会消耗下一帧传输预算。
- 解开 Gin 响应封装一层后创建 ResponseController，避免 Gin 的 Flush 隐藏错误；
  内容仍经过 Gin Write，保留响应状态和字节数统计。
- 设置时限失败（包括不支持）、短写、写入失败、刷新失败、清除时限失败均返回错误。
  不以无界写入作为不支持时限的降级方案。
- 首帧失败通过独立上下文取消 pending；运行中失败关闭上游，
  使用既有独立五秒事务写入 failed/generation_failed，不保存不完整 assistant。
- 传输超时不等同于领域十分钟运行超时，因此不使用 generation_timeout 错误码。

## 测试证据

| 测试 | 覆盖 |
|---|---|
| TestFrameDeadline | 成功与刷新错误均清除时限，保留 Gin 字节统计 |
| TestFrameUnsupported | 不支持时限时拒绝发送，响应无正文 |
| TestFrameSetError | 设置时限失败时不写入、不刷新 |
| TestStartFlushError | 首帧刷新失败取消 pending |
| TestSlowFrame | 真实 TCP 客户端发送请求后完全不读响应，服务端返回网络超时；断言前不关闭客户端 |
| TestSlowGeneration | 生产 TLS/Cookie/MySQL，仅读取开始事件后停止读取；上游退出、任务失败、归属不变且无 assistant |

慢读生成使用本地 HTTP 模型模拟服务发送合法小帧，不能视为外部 Provider 验收。
内存测试显式实现可观察的时限能力，不把 httptest.ResponseRecorder 当作真实传输。

在 backend 执行：

```sh
AI_CHAT_TEST_MYSQL_DSN='root:root_dev@tcp(127.0.0.1:3307)/ai_chat?parseTime=true' \
AI_CHAT_TEST_REDIS_ADDR=127.0.0.1:6380 GIN_MODE=release \
go test -race ./... -count=1
go vet ./...
```

全量通过；本地凭据仅供开发容器。新增设置失败断言随后单独竞态回归。

## 审计

九文件范围：generation/http 的 stream.go、frame_test.go、slow_test.go、
handler_test.go、candidates_test.go、branch_test.go、log_test.go、disconnect_test.go，
以及 httpapi/slow_flow_test.go。

- 原始 28 条：handler_test.go 既有测试桩缺少公开函数注释 20 条，
  stream.go 错误透传 5 条，慢读上游测试中断分支 3 条。
- 增量 8 条：传输错误由调用边界既有 Zap 记录，测试上游遇连接错误直接结束请求。
  未添加豁免，未声明审计全绿。
- 新增文件及函数有中文注释；未触发长度、命名和条件嵌套规则。
- 不涉及账本、归属或权限变更；终态仍由既有条件更新仲裁。

## 剩余边界

- HTTP/2、反向代理及不透明 ResponseWriter 中间件的部署兼容性未验收。
- 两秒限制的是单帧传输，不是整个请求退出预算，也不覆盖全部数据库收尾耗时。
- 慢读与服务停机组合、真实 SIGTERM、数据库故障下最终退出未验收。
- 会话删除的原子收尾、数据实迁、浏览器真实后端联调仍属于迁移 Goal 未完成项。
