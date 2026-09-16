# 现有接口与聊天链路盘点

## 1. 盘点时间

2026-09-14

## 2. 服务入口

当前 Node/Express 在 `src/server-startup.js` 的 `setupPrivateEndpoints` 中注册私有接口。接口没有统一版本号，主要使用 `/api/*`。

## 3. 核心接口

| 现有接口 | 作用 | Go 迁移目标 |
|---|---|---|
| `POST /api/users/login` | 用户登录 | `POST /api/v1/auth/login` |
| `POST /api/users/logout` | 用户退出 | `POST /api/v1/auth/logout` |
| `GET /api/users/me` | 当前用户 | `GET /api/v1/me` |
| `POST /api/characters/all` | 角色列表 | `GET /api/v1/characters` |
| `POST /api/characters/get` | 角色详情 | `GET /api/v1/characters/:id` |
| `POST /api/characters/chats` | 角色下会话列表 | `GET /api/v1/characters/:id/chats` |
| `POST /api/chats/get` | 读取 JSONL 会话 | `GET /api/v1/chats/:id/messages` |
| `POST /api/chats/save` | 保存 JSONL 会话 | 由消息写入接口替代 |
| `POST /api/chats/rename` | 重命名会话 | `PATCH /api/v1/chats/:id` |
| `POST /api/chats/delete` | 删除会话 | `DELETE /api/v1/chats/:id` |
| `POST /api/chats/recent` | 最近会话 | `GET /api/v1/chats/recent` |
| `POST /api/backends/chat-completions/generate` | AI 生成 | `POST /api/v1/chats/:id/generations` |
| `POST /api/images/upload` | 图片上传 | `POST /api/v1/assets` |
| `POST /api/images/delete` | 图片删除 | `DELETE /api/v1/assets/:id` |

## 4. 旧接口兼容结论

- 第一阶段不直接修改旧接口。
- Go API 使用 `/api/v1`，由前端 API Client 逐步切换。
- 旧 `avatar_url`、`file_name`、JSONL 字段需要在迁移适配层中保留读取能力。
- `POST /api/chats/save` 是整份 JSONL 覆盖式保存，Go 侧改为消息级写入，但必须保留历史顺序和分支关系。

## 5. Go 生成接口契约

`POST /api/v1/chats/:id/generations` 使用登录 Cookie，推荐请求体为：

```json
{
  "model": "provider-model",
  "n": 2,
  "content": "用户本轮输入",
  "parent_id": "消息编号"
}
```

旧客户端可继续传 `messages`，Go 侧只提取其中最后一条 `user` 消息；角色设定和历史消息由后端按当前用户、会话和角色重新查询组装。

流式响应事件依次使用：

```text
message_start   {"generation_id":8}
message_delta   {"text":"...","index":0}
message_end     {"message_id":12,"generation_id":8,"variants":null}
generation_error {"message":"generation failed"}
```

指定 `parent_id` 时，必须引用当前用户当前会话中已完成、内容非空的 user 消息。
后端从该消息沿父链读取上下文，不再追加重复用户文本；`content` 可省略，传入时必须与该已保存消息内容完全一致。
推荐先通过消息创建接口保存本轮 user 消息，再用返回的 ID 发起生成。

父链上下文最多保留最近 100 条，按祖先到 user 叶节点排序；兄弟回复不参与。
不足 100 条却未到根、重复节点、循环、跨会话或未完成祖先均拒绝，返回 400 `INVALID_BRANCH`；
不可见父消息也使用该公开错误，不泄漏具体归属。数据库故障仍为 500。
重新生成必须有已保存的 user 父消息，缺失父节点不再回退到完整会话。
无 `parent_id` 的旧客户端仍走原来的历史兼容路径，其线性窗口与用户消息持久化尚待收口，不代表已支持完整分支。
代码依据：`generation/http/prompt.go`、`generation/app/branch.go`、`message/infra/branch.go`。

候选契约（2026-09-16，代码依据：`generation/http/handler.go` 的 `stream`、`regenerate`、`run`，
`generation/app/candidates.go` 和 `message/domain/repo.go`）：

- 两个 POST 生成入口都接受整数 `n`；省略或 0 表示默认一条，1–4 指定数量，负值或超过 4 返回 400。
- `message_delta.index` 是从 0 开始的候选编号；各候选可能交错发送，客户端应分别累积。
- 主消息使用编号 0 的文本。多候选时，`message_end.variants` 包含全部候选，字段为字符串 `id`、字符串 `message_id`、整数 `variant_no`、字符串 `content`、可选 JSON `extra_data`；单候选时为 null。
- 当前顶层 SSE `message_id`、`generation_id` 实际编码为数字，候选对象 ID 编码为字符串；统一 ID 编码仍待处理，前端不得套用旧文档中的顶层字符串示例。
- 主消息、候选记录和 completed 状态在同一 GORM 事务内保存；供应商缺少请求的候选或返回越界编号时生成失败。失败前已发送的增量不是落库成功证据。
- 候选历史查询和选择 API 已接通，契约见下节；前端切换尚未接通；真实 MySQL 回滚、真实 Provider 多候选及浏览器验收待完成。

### 候选历史查询

本节新增接口共 1 个：`GET /api/v1/messages/:id/variants`，使用现有登录鉴权中间件。
路径 `id` 为正整数消息 ID；查询参数 `page` 默认为 1、最小 1，`size` 默认为 20、范围 1–100；无请求体。

```json
{
  "code": "OK",
  "message": "ok",
  "data": {
    "items": [{"id": "21", "message_id": "12", "variant_no": 0, "content": "reply", "extra_data": {}}],
    "total": "1",
    "page": 1,
    "size": 20
  },
  "request_id": "..."
}
```

- `id`、`message_id`、`total` 是字符串；`variant_no`、`page`、`size` 是整数；`extra_data` 为可选 JSON。
- 只返回当前用户可见消息的未删除候选；消息或会话已删除同样不可见。按 `variant_no ASC, id ASC` 排序。
- 无候选或超出末页时 `items` 为 `[]`；未保存候选的单回复消息不虚构候选，主文本仍从消息历史读取。
- 参数非法：400 `INVALID_QUERY`；不存在、越权、已删除消息：404 `MESSAGE_NOT_FOUND`；内部故障：500 `INTERNAL_ERROR`，不返回底层错误。
- 证据：`message/http/handler.go` 的 `Routes`、`readArgs`、`fail`，`message/http/variants.go`，`message/app/variants.go`，`message/infra/repo.go` 的 `ListVariants`、`variants`、`owned`，`message/domain/repo.go` 和 `reply/reply.go`。
- 待确认：真实 MySQL 执行与并发删除回归尚未验收；本接口不承担候选选择或分支切换。

### 选择候选回复

`POST /api/v1/messages/:id/variants/:variant/select` 使用现有登录鉴权，无请求体。
两个路径参数均为正整数 ID；`:variant` 为候选记录 ID，不是 `variant_no`。

- 原消息必须属于当前用户、未删除且为 completed assistant；候选必须属于该原消息且未删除。
- 事务内创建同会话、同父节点的新 assistant 消息，不覆盖原消息或移动已有后续分支。即使选择候选 0，也创建独立分支。
- 新消息记录 `source_variant_id`；重复选择同一候选返回同一个已保存消息。已选分支软删除后再次选择返回 409，不隐式恢复。
- 返回 200，采用统一信封，`data` 为 Message：字符串 `id`、`conversation_id`、可空字符串 `parent_id`、可选字符串 `source_variant_id`，以及 `role/content/status/variant_no/extra_data`；当前选择结果不包含 `variants`。
- 400 `INVALID_QUERY` 表示参数非法；404 `MESSAGE_NOT_FOUND` 隐藏不存在/越权/候选不匹配的差异；409 `MESSAGE_CONFLICT` 表示原消息或已选分支状态不允许；500 `INTERNAL_ERROR` 隐藏数据库错误。
- 前端保留原消息 ID 作为候选列表来源；继续聊天时先保存 user 消息，令其 `parent_id` 指向选择结果的消息 ID，再用新 user 消息 ID 发起生成。
- 代码依据：`message/http/select.go`、`message/app/select.go`、`message/infra/select.go`、`message/domain/repo.go`、`model/message.go`。
- 部署要求：先执行现有模型迁移，添加 `messages.source_variant_id` 可空列和唯一索引。真实 MySQL 并发/回滚与浏览器流程仍待验收。

会话辅助接口：

- `GET /api/v1/chats/recent?page=1&size=20`：只返回已有消息的当前用户会话，按 `last_msg_at DESC, id DESC` 排序。
- `PUT /api/v1/chats/:id/favorite`：收藏会话。
- `DELETE /api/v1/chats/:id/favorite`：取消收藏，重复调用保持成功。
- `POST /api/v1/messages/:id/regenerate`：请求体传入 `{ "model": "provider-model" }`，基于原 assistant 消息的父节点创建新回复，不覆盖原消息。

### 生成与删除并发

2026-09-16，本节列出七个受影响的现有登录入口，不增加接口或参数：

| 方法 | 路径 | Handler | 本次约束 |
|---|---|---|---|
| POST | `/api/v1/chats/:id/generations` | `Handler.stream` | 创建事务再次锁定并检查当前用户的未删除会话 |
| POST | `/api/v1/messages/:id/regenerate` | `Handler.regenerate` | 复用同一生成创建事务及完成事务 |
| DELETE | `/api/v1/chats/:id` | `Handler.remove` | 删除、清理会话收藏、取消未结束任务同事务提交 |
| PUT | `/api/v1/chats/:id/favorite` | `Handler.favorite` | 会话锁内收藏，已删除会话返回 404 |
| DELETE | `/api/v1/chats/:id/favorite` | `Handler.favorite` | 可见会话取消收藏幂等，已删除会话返回 404 |
| GET | `/api/v1/generations/:id` | `Handler.find` | 已删除会话关联任务返回 404 |
| DELETE | `/api/v1/generations/:id` | `Handler.cancel` | 删除已取消的任务再次取消返回 404 |

- 前置查询成功后若会话先被删除，创建返回 404：
  `{"code":"NOT_FOUND","message":"chat not found"}`，不启动 SSE。
- 完成事务也检查会话。流已开始后会话被删除，迟到完成不会保存消息或候选，
  失败沿现有 `generation_error` 路径返回，不能再修改已发送的 HTTP 状态码。
- 删除之前提交的完整回复保留历史存储事实，消息查询继续过滤已删除会话。
- 会话删除无请求体；成功返回 200 和现有 `reply.OK` 空对象数据。
  重复删除及其他用户会话删除保持 200 且无副作用，不暴露归属；事务失败返回 500。
- 删除仅清理当前用户、当前会话且 kind=chat 的收藏，只取消 pending/running；
  已完成、失败、取消任务保留原有消息关联、错误和结束时间。任一步失败整体回滚。
- 任务启动在同一会话锁下检查可见性；删除先提交则不能转 running。
  启动先提交的执行可能已获准调用 Provider，删除通过任务监听关闭连接，
  不承诺删除响应时远端已经停止，也不承诺从未发出远端请求。
- 待确认：独立进程、HTTP/2 和代理环境尚未验收；同进程两套服务实例的真实网络测试已通过。
- 代码依据：`chat/infra/gate.go` 的 `WithChat`，`generation/infra/repo.go` 的 `Create`，
  `generation/infra/done.go` 的 `SaveDone`，`generation/http/handler.go` 的 `fail`；
  `chat/http/routes.go`、`chat/http/handler.go` 的 `remove/favorite`、
  `chat/app/remove.go` 的 `Delete`、`chat/infra/delete.go`、`chat/infra/favorite.go`，
  `generation/infra/chat.go` 的 `CancelChat`、`generation/infra/repo.go` 的 `Find/Move`。

### 取消生成

本次更新 1 个已有接口，不新增路由或响应字段：

| 方法 | 路径 | Handler | 鉴权 |
|---|---|---|---|
| DELETE | `/api/v1/generations/:id` | `Handler.cancel` | `RequireAuth`，Cookie 优先、Bearer 其次 |

无请求体。路径 `id` 必须是正整数；未登录返回 401。生产装配需配置 MySQL 和 Provider URL 才注册生成路由。

- 只允许当前用户将 pending/running 任务更新为 cancelled，成功返回 204，无响应体。
- 数据库条件更新成功后才中断本机注册的 Provider 请求；更新失败不得中断它。
- 任务运行于其他实例时，原实例每秒通过生成域查询 MySQL 状态，每次查询时限两秒；
  发现非 running 状态或查询失败后取消上游上下文。204 表示取消已持久化，不表示远端连接已经关闭。
- 跨实例停止不是即时广播，不承诺一秒内断连；调度、数据库延迟和 SSE 客户端写阻塞均影响收尾。
  完成事务继续检查 running，取消先提交时不保存 assistant 消息和候选。
- 不存在、越权或已结束（含重复取消）的任务统一返回 404：`{"code":"NOT_FOUND","message":"generation not cancellable"}`。
- 参数错误返回 400 `INVALID_QUERY`；数据库错误返回 500 `INTERNAL_ERROR`。此接口错误响应仍为 `code/message` 对象，尚未统一到带 `data/request_id` 的信封。
- 证据：`generation/http/handler.go` 的 `Routes`、`readIDs`，`generation/http/cancel.go`，
  `generation/app/task.go` 的 `Cancel`，`generation/infra/repo.go` 的 `Move`，
  `generation/app/watch.go` 的 `watchTask/checkTask`、`generation/app/run.go` 的 `RunEvents`，
  `httpapi/server.go` 的 `connect`、`httpapi/auth.go` 的 `RequireAuth/readToken`。
- 验证：真实 MySQL 竞争测试及两套独立服务装配的远端取消/超时清理已通过，
  详见 `docs/generation-cancel-verification.md`。
- SSE 单帧写入和刷新已有两秒时限，真实 TCP/TLS 慢读收尾通过测试；
  代码依据为 `generation/http/stream.go` 的 `writeFrame`，不等同于整个请求的退出时限。
- 会话删除联动已通过真实 TLS/MySQL 两套服务实例测试，详见 `docs/chat-lifecycle-verification.md`。
- 待确认：独立操作系统进程、多机部署、真实外部 Provider、HTTP/2 与代理部署、
  慢读与停机组合尚未验收。

## 6. 生成入口证据

前端 `public/scripts/openai.js` 的 `sendOpenAIRequest`：

1. 组装 `generate_data`
2. 请求 `/api/backends/chat-completions/generate`
3. 根据 `stream` 读取响应 Body
4. 解析增量 JSON
5. 累积文本、候选回复、工具调用和状态

Go 迁移时需要把 Provider 响应解析和前端展示格式解耦。

## 7. 当前未确认项

- 现有前端实际保存消息的调用链还需结合 `public/script.js` 等动态入口继续追踪。
- 旧用户存储、密码算法和账号迁移字段需要进一步确认。
- 不同 Provider 的完整请求字段不能只从 OpenAI 入口推断。
