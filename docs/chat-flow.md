# 现有 AI 聊天流程盘点

## 1. 已确认流程

```text
用户在前端输入消息
  -> 前端生成上下文和 Provider 参数
  -> POST /api/backends/chat-completions/generate
  -> Node 路由调用对应模型后端
  -> 返回流式或非流式结果
  -> 前端解析增量结果
  -> 前端更新聊天显示和候选回复
  -> 通过聊天保存流程写入 JSONL
```

## 2. 流式行为

前端在 `public/scripts/openai.js` 中：

- 检查响应状态
- 将响应 Body 转换为事件流
- 读取事件数据
- `[DONE]` 表示结束
- 累积 `text`
- 处理多候选回复 `swipes`
- 处理工具调用和推理状态

Go SSE 设计必须至少支持：

```text
message_start
message_delta
message_end
generation_error
generation_done
```

## 3. Go 目标流程

```text
创建用户消息
  -> 创建 generation(pending)
  -> 校验会话和角色
  -> 组装 Prompt
  -> generation(running)
  -> 调用 Provider
  -> SSE 推送增量
  -> 保存完整 AI 消息
  -> generation(completed)
```

失败、取消和超时必须分别记录状态，不能依赖前端推断。

## 4. 必须保留的行为

- 多轮上下文
- 角色设定和场景
- 开场白
- 流式回复
- 停止生成
- 重新生成
- 候选回复
- 历史消息刷新恢复

## 5. 需要后续确认的行为

- 消息保存是由哪个前端函数最终触发。
- 重新生成时旧消息和新候选的准确父子关系。
- Provider 工具调用是否属于第一期范围。
- 内容安全拦截时前端和后端的状态约定。
