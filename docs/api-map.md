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
  "content": "用户本轮输入",
  "parent_id": "消息编号"
}
```

旧客户端可继续传 `messages`，Go 侧只提取其中最后一条 `user` 消息；角色设定和历史消息由后端按当前用户、会话和角色重新查询组装。

流式响应事件依次使用：

```text
message_start   {"generation_id":"..."}
message_delta   {"text":"..."}
message_end     {"message_id":"...","generation_id":"..."}
generation_error {"message":"generation failed"}
```

`parent_id` 只能引用当前用户当前会话中的消息，越权引用必须拒绝。

会话辅助接口：

- `GET /api/v1/chats/recent?page=1&size=20`：只返回已有消息的当前用户会话，按 `last_msg_at DESC, id DESC` 排序。
- `PUT /api/v1/chats/:id/favorite`：收藏会话。
- `DELETE /api/v1/chats/:id/favorite`：取消收藏，重复调用保持成功。
- `POST /api/v1/messages/:id/regenerate`：请求体传入 `{ "model": "provider-model" }`，基于原 assistant 消息的父节点创建新回复，不覆盖原消息。

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
