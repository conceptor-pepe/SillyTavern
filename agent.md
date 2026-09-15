# AI Chat 后端编码规范

## 1. 适用范围

本规范适用于 AI Chat Go 后端。技术栈固定为：

- Go
- Gin
- GORM
- MySQL 8
- Redis
- SSE
- 轻量 DDD
- Zap

前后端分离后，Go 后端提供 `/api/v1` 接口，前端不得依赖 Go 服务端渲染页面。

## 2. 设计原则

### 2.1 轻量 DDD

按业务域划分代码，不引入过度复杂的 DDD 模式。

第一期业务域：

- `user`：用户和鉴权
- `character`：角色
- `chat`：会话
- `message`：消息
- `gen`：AI 生成任务
- `provider`：模型供应商

后续业务域：

- `memory`：记忆
- `asset`：图片和附件
- `group`：群聊

推荐结构：

```text
internal/<domain>/
  domain/       实体、值对象、领域规则、领域接口
  app/          用例编排
  infra/        GORM、MySQL、Redis、Provider 实现
  interfaces/   Gin Handler、路由、请求响应
```

依赖方向：

```text
interfaces -> app -> domain
infra      -> domain
```

领域层不得依赖 Gin、GORM、MySQL、Redis 或第三方 Provider SDK。

### 2.2 单一职责

一个文件只负责一个清晰职责：

```text
chat_handler.go   HTTP 参数和响应
chat_service.go   聊天用例
chat_repo.go      数据访问
chat_model.go     GORM 模型
chat_route.go     路由注册
```

禁止把路由、SQL、Prompt 组装和模型调用混在一个文件中。

### 2.3 中文注释

所有新增 Go 文件和函数必须有中文注释。

- 每个文件顶部必须说明文件职责
- 每个公开函数必须写 GoDoc 中文注释
- 私有函数只要包含业务规则，也必须写中文注释
- 注释说明“为什么这样做”和业务边界，不重复代码字面含义
- 复杂状态转换、事务边界、兼容旧数据的逻辑必须单独说明
- 注释不使用空泛描述，例如“处理数据”“执行逻辑”

提交前必须检查新增或修改的每个 Go 文件：

- 文件首部有中文职责注释
- 公开函数有中文 GoDoc
- 承载业务规则的私有函数有中文注释
- 测试辅助函数也要说明其模拟的边界

示例：

```go
// CreateChat 创建用户与角色之间的新会话。
func CreateChat(ctx context.Context, uid, charID int64) (*Chat, error) {
    // 会话必须绑定创建者，避免后续查询出现跨用户数据。
    return nil, nil
}
```

文件注释示例：

```go
// chat_service.go 负责会话创建、查询和删除用例。
package chat
```

## 3. 函数规则

### 3.1 长度

- 单个函数最多 50 行
- Handler 建议不超过 25 行
- Repository 函数建议不超过 25 行
- 超过 50 行必须按业务动作拆分
- 不得通过无意义的 `step1`、`step2` 拆分来规避规则

### 3.2 命名

- 函数名使用动词加对象
- 函数名最多 4 个英文单词
- 包名使用小写短词
- 避免 `ProcessData`、`ExecuteLogic`、`HandleRequest` 等抽象名称

推荐：

```go
CreateChat
GetChat
ListChats
SaveMsg
LoadMsgs
BuildPrompt
StreamChat
StopGen
CheckUser
```

### 3.3 条件嵌套

- `if` 嵌套最多 2 层
- 优先使用提前返回
- 复杂条件提取为有业务含义的短函数
- 禁止通过多层嵌套隐藏权限、状态或数据范围判断

### 3.4 其他复杂度限制

- 单个文件建议不超过 400 行，超过后按职责拆分
- 单个函数参数建议不超过 5 个，超过后使用请求结构体
- 单个函数只允许一个主要返回结果
- 禁止使用全局可变业务状态
- 禁止在业务代码中直接使用 `panic`
- 禁止忽略错误返回值
- 禁止用 `interface{}` 或 `map[string]any` 替代明确业务结构
- 禁止为了复用创建没有业务含义的通用 Base 类
- 新增依赖必须说明用途，避免重复引入库

## 4. 分层规则

### 4.1 Handler

Handler 只做：

1. 读取和绑定参数
2. 调用应用用例
3. 转换错误
4. 返回响应

Handler 不得直接写 GORM 查询、组装 Prompt 或调用 Provider。

### 4.2 App

App 层负责完整业务用例：

- 权限检查
- 状态检查
- 调用领域规则
- 调用 Repository
- 调用 Provider
- 事务编排
- 返回业务结果

App 层不依赖 Gin Context，使用标准 `context.Context`。

### 4.3 Domain

Domain 层只表达业务规则：

- 会话是否属于用户
- 消息是否允许编辑
- 生成任务状态是否允许转换
- 消息分支如何建立

不得在 Domain 层执行数据库或网络请求。

### 4.4 Infra

Infra 层实现外部依赖：

- GORM Repository
- Redis
- Provider HTTP 客户端
- 文件存储

Infra 不负责用户权限和业务流程。

## 5. API 规则

统一使用 `/api/v1`。

响应格式：

```json
{
  "code": "OK",
  "message": "ok",
  "data": {},
  "request_id": "..."
}
```

错误状态：

```text
400 参数错误
401 未登录
403 无权限
404 数据不存在
409 状态冲突
429 请求过多
500 服务错误
```

不得把数据库错误、API Key 或 Provider 原始错误直接返回给前端。

## 6. 数据库规则

- 使用 MySQL 8 和 `utf8mb4`
- 时间统一存 UTC
- 所有表包含 `created_at`、`updated_at`
- 可删除业务数据使用 `deleted_at`
- 用户数据查询必须带 `user_id`
- 列表接口必须分页
- 外键字段必须建索引
- 金额和额度使用 `DECIMAL`
- JSON 仅用于确实需要扩展的数据
- 避免 GORM `Preload` 无限嵌套
- 避免循环内查询造成 N+1
- 多表写入使用事务

## 7. AI 生成规则

必须区分：

```text
message_id       消息 ID
generation_id    生成任务 ID
provider_task_id 供应商任务 ID
```

生成状态：

```text
pending
running
completed
failed
cancelled
```

SSE 事件：

```text
message_start
message_delta
message_end
generation_error
generation_done
```

客户端断开时必须取消 Provider 请求。生成失败必须更新任务状态，不能留下永久 `running`。

## 8. 日志和安全

后端统一使用 Zap 记录结构化日志，禁止在业务代码中直接使用标准库 `log`、`fmt.Println` 或 `println` 输出日志。

日志边界：

- Handler 记录请求结果、操作者和资源标识
- App 记录跨仓储、Provider 或事务失败
- Infra 只补充连接、SQL、Provider 请求失败所需的技术字段，不重复打印完整业务内容
- Domain 不引入日志依赖；业务上下文由调用它的 App 或 Handler 记录
- 统一注入 `*zap.Logger`，禁止在业务函数内临时创建全局 Logger
- 错误日志必须使用 `zap.Error(err)`，禁止只拼接错误字符串
- 成功日志使用 `Info`，可恢复或用户输入问题使用 `Warn`，系统故障使用 `Error`

日志至少包含：

```text
request_id
user_id
chat_id
message_id
generation_id
provider
model
duration_ms
error_code
```

禁止记录：

- API Key
- Authorization
- Cookie
- 用户完整隐私内容
- 完整 Prompt
- Provider 完整响应

写操作必须有成功日志，失败必须有结构化错误日志。

## 9. 测试和提交

- 新增业务规则必须有单元测试
- 核心链路必须有集成测试
- 测试必须覆盖权限、重复提交、超时、取消和事务失败
- 提交前执行 `gofmt`
- 提交前执行 `go vet`
- 提交前执行相关 `go test`
- 不混入无关格式化和重命名
- 不覆盖工作区已有的用户修改

提交前的最低检查命令：

```bash
gofmt -w <changed.go files>
go test ./...
go test -race ./...
go vet ./...
rg -n 'log\\.|fmt\\.Print|println' backend --glob '*.go'
```

最后一条命令必须无业务代码命中；测试中的必要输出也应优先改为测试断言或 Zap。
