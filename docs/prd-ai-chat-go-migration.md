# AI Chat Go 后端与前后端分离 PRD

## 1. 文档信息

| 项目 | 内容 |
|---|---|
| 状态 | 规划中 |
| 日期 | 2026-09-14 |
| 技术栈 | Go、Gin、GORM、MySQL、Redis、SSE |
| 迁移方式 | 增量迁移、并行运行、逐步切流 |
| 当前系统 | Node/Express 单体与现有前端 |

## 2. 需求本质

在保持现有 AI 聊天核心能力的前提下，将后端逐步迁移到 Go，并完成前后端分离，使角色、会话、消息和 AI 生成任务拥有清晰的业务边界、可持久化的数据模型和可独立演进的 API。

核心不变量：

- 用户只能访问自己的角色、会话和消息
- 用户消息和 AI 消息不能丢失或重复保存
- 流式生成结束后必须形成可重新加载的完整消息
- 生成任务状态必须可追踪、可取消、可恢复
- 旧数据迁移后数量和归属不能改变

## 3. 用户和入口

### 3.1 普通用户

- 注册和登录
- 选择角色
- 创建会话
- 发送消息
- 接收流式回复
- 停止或重新生成
- 查看历史会话

### 3.2 系统

- AI Provider
- 数据迁移程序
- 生成超时清理任务
- 日志和监控系统

## 4. 第一阶段范围

### 必须保留

- 用户注册、登录、退出
- 角色列表和角色详情
- 角色头像
- 创建和删除会话
- 历史消息
- 多轮上下文
- 流式 AI 回复
- 停止生成
- 重新生成和消息分支
- 消息编辑和删除
- 最近聊天
- 基础收藏

### 暂缓

- 群聊
- 完整长期记忆
- 复杂世界书
- 插件系统
- 图片生成
- 语音识别和语音合成
- 全量本地模型适配
- 创作者后台
- Electron 桌面端

## 5. 核心流程

```text
登录
  -> 获取角色
  -> 创建会话
  -> 保存用户消息
  -> 创建生成任务
  -> 组装 Prompt
  -> 调用 Provider
  -> SSE 推送增量文本
  -> 保存 AI 消息
  -> 更新生成状态
```

## 6. API 初稿

```text
POST /api/v1/auth/register
POST /api/v1/auth/login
POST /api/v1/auth/logout
GET  /api/v1/me

GET  /api/v1/characters
GET  /api/v1/characters/:id

GET  /api/v1/chats
POST /api/v1/chats
GET  /api/v1/chats/:id
DELETE /api/v1/chats/:id

GET  /api/v1/chats/:id/messages
POST /api/v1/chats/:id/messages
POST /api/v1/chats/:id/generations
GET  /api/v1/generations/:id/stream
POST /api/v1/generations/:id/cancel

POST /api/v1/messages/:id/regenerate
PATCH /api/v1/messages/:id
DELETE /api/v1/messages/:id
```

## 7. 数据对象

第一期表：

```text
users
characters
conversations
messages
message_variants
generations
assets
favorites
```

消息必须支持 `parent_id`，用于重新生成和分支对话。

## 8. 非功能要求

- API 支持独立部署
- 所有用户数据按用户隔离
- 生成请求支持超时和取消
- 关键写操作可审计
- API 返回 `request_id`
- MySQL 事务保证消息和任务状态一致
- 大列表分页
- Provider Key 不落日志
- 支持旧数据重复迁移校验

## 9. 验收标准

- 登录后可加载角色
- 可创建会话并刷新恢复
- 用户消息保存成功
- AI 回复可流式显示
- 生成中可以停止
- Provider 失败后任务状态为 `failed`
- 重新生成不会覆盖原消息
- 用户不能读取其他用户数据
- Go 服务重启后历史消息不丢失
- 前端独立运行时可以调用 Go API
- 迁移前后用户、角色、会话、消息数量可核对

## 10. 风险

| 风险 | 应对 |
|---|---|
| 旧 JSON 数据结构不统一 | 先做只读盘点和迁移报告 |
| 前端依赖旧接口 | 增加 API 适配层，分批替换 |
| Provider 行为不一致 | 第一阶段只实现 OpenAI 兼容协议 |
| 流式断开导致脏状态 | 统一生成状态和超时清理 |
| 旧功能范围过大 | 第一阶段冻结群聊、插件和语音 |
