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

### 取消生成

`DELETE /api/v1/generations/:id` 使用现有登录鉴权中间件，无请求体。路径 `id` 必须是正整数。

- 只允许当前用户将 pending/running 任务更新为 cancelled，成功返回 204，无响应体。
- 数据库条件更新成功后才中断本机注册的 Provider 请求；更新失败不得中断它。
- 不存在、越权或已结束（含重复取消）的任务统一返回 404：`{"code":"NOT_FOUND","message":"generation not cancellable"}`。
- 参数错误返回 400 `INVALID_QUERY`；数据库错误返回 500 `INTERNAL_ERROR`。此接口错误响应仍为 `code/message` 对象，尚未统一到带 `data/request_id` 的信封。
- 证据：`generation/http/handler.go` 的 `Routes`、`readIDs`，`generation/http/cancel.go`，
  `generation/app/task.go` 的 `Cancel`，`generation/infra/repo.go` 的 `Move`。
- 待确认：真实数据库竞态回归、跨实例 Provider 取消尚未完成；当前中断注册表为进程内状态。

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
