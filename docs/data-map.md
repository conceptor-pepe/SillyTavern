# 现有数据来源与迁移盘点

## 1. 盘点时间

2026-09-14

## 2. 用户数据

用户由 `src/users.js` 管理，使用用户目录和本地持久化存储。每个用户拥有独立目录，目录包含角色、聊天、资源、设置、备份等数据。

Go 迁移目标：

```text
users
user_settings
```

密码字段、账号版本、启用状态和目录迁移规则必须在正式迁移前单独确认。

## 3. 角色数据

角色主要从用户角色目录中的 PNG 文件读取，角色卡数据内嵌在 PNG 中，并转换为 Spec V2 结构。

当前已确认的角色字段：

```text
name
description
personality
scenario
first_mes
mes_example
creator
creator_notes
tags
extensions
```

角色统计还包括：

```text
date_added
date_last_chat
chat_size
data_size
```

Go 迁移目标：

```text
characters
character_data
assets
```

旧 PNG 文件不能在数据库迁移完成前删除，应保留原始文件作为回退和校验来源。

## 4. 聊天数据

聊天当前保存在用户聊天目录下，按角色目录组织，文件扩展名为 `.jsonl`。每一行是一个 JSON 消息对象。

当前行为：

- `/api/chats/save` 接收完整数组并写回 JSONL
- `/api/chats/get` 读取整份 JSONL 并解析为数组
- `/api/chats/rename` 通过文件复制和删除完成重命名
- `/api/chats/delete` 删除 JSONL 文件
- 最近聊天通过文件修改时间扫描

Go 迁移目标：

```text
conversations
messages
message_variants
```

JSONL 中未确认的扩展字段先保存到 `messages.extra_data`，不能在未知字段未盘点前丢弃。

## 5. 文件和图片

当前有独立的图片和文件接口，资源主要使用用户目录文件系统保存。

迁移策略：

1. 先登记旧文件路径
2. 数据库保存逻辑资源 ID 和原始路径
3. 再迁移到对象存储或统一资源目录
4. 校验数据库记录与文件实际存在性

## 6. 数据迁移校验

必须核对：

```text
用户数量
角色数量
会话数量
消息数量
角色归属
会话归属
消息顺序
消息分支
头像和附件路径
```

## 7. 当前未知项

- 2026-09-14 基于仓库当前 `data/default-user` 快照统计：`chats` 下 9 个 JSONL 文件、`backups` 下 70 个 JSONL 文件；原始非空行 74 行，经识别 `chat_metadata` 元数据行后得到 65 条可迁移消息，9 个文件无解析错误。该统计是当前工作区快照，不代表线上全量。
- 2026-09-15 复核用户与角色迁移来源：用户账号位于 `data/_storage` 的 KV 文件；角色 PNG 内含 `tEXt` 的 `chara`/`ccv3` Base64 角色卡 JSON，可读取核心字段和原始扩展数据。
- 用户 KV 已确认包含 `handle`、`name`、`enabled`；旧记录的 `password` 为空，不能直接作为新系统登录凭证。
- JSONL 消息完整字段需要用真实用户目录抽样确认。
- 当前工作区未发现可直接迁移的真实数据目录。
- 角色 PNG 写回格式和图片元数据迁移需要独立测试。
