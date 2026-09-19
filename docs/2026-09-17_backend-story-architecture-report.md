# 故事与角色聊天后端架构

状态：故事开聊后端 MVP 已实施。设计日期：2026-09-17；实现复核：2026-09-18。范围：Go 后端的模块、数据归属、调用关系、事务、接口与实施布局。

采用模块化单体：一个 Go API 进程、一套 MySQL，保留 Gin/GORM 和现有 SSE。新增故事、人设和上下文编排能力，扩展现有会话、消息、生成、记忆模块。首期无需引入微服务、消息总线或独立向量数据库。

**已实现范围**：故事草稿与不可变版本、单角色故事开聊、结构化开场、玩家人设快照、故事会话独立记忆、冻结上下文、三条回复建议、AI 回复编辑分叉和旧角色聊天兼容。多角色、定向改写、续写、公开社区与媒体生成仍按后续批次推进。

### 当前可用链路

1. 作者通过 `POST /api/v1/stories` 保存私有故事草稿；更新时提交 `expected_revision`，过期修订返回 409。
2. 作者通过 `POST /api/v1/stories/:id/versions` 把指定草稿修订冻结为不可变版本；重复冻结同一修订返回同一个版本。
3. 用户通过 `/api/v1/personas` 管理玩家身份。开始故事时服务端复制一份人设快照，之后修改或删除人设不会改变已有会话。
4. 客户端通过 `POST /api/v1/chats` 提交 `mode=story`、`story_version_id`、`persona_id` 和 `idempotency_key`。服务端在同一事务中创建会话、保存版本及人设快照并写入一条结构化开场根消息。
5. 客户端通过 `GET /api/v1/chats/:id/bootstrap` 获取冻结故事、人设、开场消息和会话状态；消息和生成继续使用现有聊天接口。
6. `/api/v1/chats/:id/memories` 只访问该故事会话自己的记忆。重开同一故事默认不会带入上一局记忆。
7. `POST /api/v1/chats/:id/reply-suggestions` 以当前 AI 回复为锚点生成三条玩家回复；建议结果不创建消息，也不进入记忆或摘要。
8. `POST /api/v1/messages/:id/revisions` 把编辑后的 AI 文本保存为同父节点的新分支；原回复和已有后续保持不变。

这里的核心关系是：`Story 草稿 -> StoryVersion 冻结版本 -> StorySession 会话快照 -> Conversation/Message 剧情记录`。草稿负责编辑，版本负责稳定内容，会话负责一次游玩存档，消息树负责聊天分支。生成上下文只读取版本和会话快照，不回读后来被修改的草稿或人设。

## 1. 部署与依赖

前端通过 REST 处理作品、人设、会话及消息操作，通过 SSE 接收生成事件。认证身份只从服务端鉴权中获取，客户端传入的 user_id 不能作为数据归属依据。

- MySQL：作品、版本、玩家人设、会话、消息树、候选、记忆、任务和幂等结果的真相来源。
- Redis：缓存、限流、短期建议缓存；首期 Redis 故障不能破坏消息或任务一致性。
- 文件存储：沿用现有封面能力；资产模块逐步负责上传校验、资源 ID、归属与访问地址。故事版本引用稳定资产，资产回收必须检查版本/会话引用。
- Provider：仅做文本生成、结构化输出、向量等供应商协议适配。业务层决定输入、预算、操作类型与落库方式。
- 后台任务：先保留 API 进程中的过期任务清理；后台摘要和媒体生成需要持久任务、租约与重试时再增加 worker 入口。

模块依赖方向：HTTP → 应用用例 → 领域规则/消费方接口 → 基础设施适配器。httpapi 只负责装配和路由，不组装 Prompt、不直接实现故事发布或消息分叉。

一个模块拥有自己的写入规则。跨模块流程通过窄接口调用应用能力；基础设施适配器实现这些接口。避免由生成模块直接任意更新故事表或让故事模块读取聊天私有记忆。共享 model 目录暂时保留，避免为调整目录而重写现有代码。

## 2. 功能布局

### 内容侧

**character：角色素材库（保留）**

负责角色 CRUD、角色卡导入和素材字段。角色导入故事后形成故事内副本，原角色 ID 只用于溯源。编辑或删除素材不自动改变已保存的故事版本。

**story：作品与版本（新增）**

负责标题、简介、封面、标签、世界背景、角色阵容、开场、风格示例和作品世界书；负责草稿修改、校验、不可变版本、作品访问策略。开场和世界书属于作品，不拆成两个独立 CRUD 微模块。

角色阵容使用稳定 cast_id，描述某一故事中的角色身份。角色素材与 cast 不强制一一对应；用户可直接在故事里创建角色。作品定义用类型化 JSON 聚合，关系索引用普通列；无需第一版就把每个开场段拆成一张表。

**persona：玩家身份（新增）**

负责用户私有的名字、头像、身份、性格和背景。账号资料归 user；玩家身份归 persona。故事可带推荐身份模板，用户选择后由服务端形成会话快照。人设库更新不修改已有会话。

### 游玩侧

**chat：会话生命周期（扩展）**

负责创建/继续/重命名/删除会话、收藏、当前活动叶子、会话版本号、作品版本及玩家快照。故事开聊用例在这里编排，story 不负责持有用户存档。ChatGate 继续提供当前用户可见会话的行锁事务。

**message：正式剧情与分支（扩展）**

负责用户消息、AI 回复、结构化内容段、候选选择、编辑分叉、历史父链和书签。一条 assistant 消息代表一次完整 AI 回复，内部可有多个角色气泡。每段包含 segment_id、kind、speaker_id、text。角色 ID 不能来自模型自由创造。

**generation：生成任务与操作编排（扩展）**

负责 reply、regenerate、rewrite、continue、suggestions 的任务状态、并发占用、超时、取消、重试入口、结果验证和 SSE。多角色编排先用一次模型调用输出多段，不采用每个角色单独 Agent 的方式。

建议生成仍属于 generation，结果是 suggestions 类型，不伪装成 assistant 消息。手动编辑不调用 Provider，属于 message。

**chatcontext：模型输入组装（新增、无独立表）**

负责把会话快照、当前父链、作品设定、玩家人设、记忆召回、摘要与本次操作组合为有预算的 ContextBundle。它是内部应用组件，不开放 HTTP CRUD。最终 Prompt 只在这里组装。

输入至少包括 user_id、chat_id、anchor_id、operation、target_message_id（改写时）、operation_instruction。输出包括模型消息、allowed_speakers、input_digest、预算用量、故事/人设/记忆版本信息。

**memory：记忆与摘要（重划边界）**

负责私人记忆 CRUD、关键词/向量召回、摘要生成与缓存。返回结构化记忆和摘要，不再依赖 generation/app 的 Prompt 构造函数。作品世界书从 story 版本读取，由 memory 的召回能力评估是否触发，不混入私人记忆表。

当前 memory.Build 可以暂时作为 legacy 适配入口；故事模式从第一天使用 chatcontext。最终依赖为 generation → chatcontext → memory，memory 通过独立 Summarizer/Embedder 接口使用 Provider，不能回调 generation。

### 支撑与后续能力

- user：注册登录、账号与鉴权，保留。
- preference：背景和显示偏好，保留，不加入剧情 Prompt。
- provider：LLM/embedding 协议适配，保留；未来增加图片/视频适配器，但不持有媒体业务。
- asset：后续统一封面、头像、聊天媒体的归属和资源访问。
- discovery：后续公开作品检索、标签、精选和相关推荐；拥有读模型，不拥有作品写入规则。
- community：后续作者主页、关注、评论、举报；公开资料与账号私有信息分离。
- media：后续图片/视频任务，消费显式会话快照并通过 message 接口附加结果；不塞进 story。
- billing：后续额度、钱包、账本，消费模型与媒体的用量记录；首期先记录成本相关元数据。

每个独立业务模块按 domain/app/infra/http 布局；仅内部使用的 chatcontext 无须创建空的 http 或 infra 目录。首期不为后续模块提前生成空壳。

## 3. 数据归属与版本

### 核心表

以下字段为设计建议，实施时通过版本迁移落地；JSON 中所有 ID 同样以字符串传输。

- stories：id、owner_id、draft_revision、draft_definition、playable_version_id、public_version_id（后续启用）、visibility、status。可玩版本与公开版本分开，防止保存草稿意外更新公开页面。
- story_versions：id、story_id、version_no、schema_version、definition_json、digest、created_at。唯一键 story_id + version_no。版本内容不可原地修改。
- player_personas：id、user_id、name、avatar_asset_id、description、revision、deleted_at。
- conversations 扩展：mode、story_version_id、persona_snapshot、memory_scope_id、active_leaf_id、revision、active_generation_id。CharacterID 在 story 模式允许为空，legacy 模式仍必填。
- messages / message_variants 扩展：payload_version、segments_json，保留 content 为服务端生成的兼容文本投影。一个节点的 content 与 segments 必须在同一事务保存；禁止客户端分别提交两个互相矛盾的真相。
- generations 扩展：operation、anchor_id、target_message_id、input_digest、output_kind、request_payload、result_payload、expires_at、usage_json。suggestions 结果放 result_payload，成功任务不要求一定有 message_id。
- idempotency_records：user_id、operation、key、request_hash、resource_id、response_code、expires_at。唯一键 user_id + operation + key。同 key 不同内容返回冲突。
- bookmarks（后续）：user_id、chat_id、message_id、segment_id；唯一约束防重复收藏。

版本中 cast、opening、examples、lore 有各自稳定标识和 schema 校验。public DTO 通过白名单构造，不能直接返回 definition_json：私有提示、隐藏设定、记忆和内部追踪信息不能因公开作品而暴露。

### 冻结与删除

保存作品版本冻结角色、世界观、开场和作品世界书。创建会话冻结玩家身份；新版本仅用于新会话。首期不提供旧会话原地升级版本。

素材角色/玩家人设删除不级联删除故事版本或会话快照。作品软删除停止新开，现有会话默认继续使用快照。平台强制下架与作者主动删除是不同状态，前者可明确禁止继续生成。资产引用与垃圾回收需遵守同样生命周期。

### 记忆作用域：明确存储，避免隐式共享

新增 memory_scopes，包含 id、user_id、kind、story_version_id、persona_digest、revision。story 类型归属于当前用户、指定版本和玩家身份；旧记忆仍通过原 character 作用域兼容。

默认每个新故事会话创建独立记忆空间，避免“重开故事”带入上局经历。用户选择“沿用这段关系的记忆”时才能复用兼容空间；首期可以只启用独立空间。与前一轮草案相比，这里用显式空间替代仅靠 story + persona hash 自动共享的方案，防止相同人设重开也自动串剧情。

memories 增加 scope_id；故事模式必须按 user_id + scope_id 查询。空间的 story_version_id/persona_digest 必须与会话一致；跨版本迁移或导入旧角色记忆通过显式复制，重新确认作用域。

摘要仍属于 chat_id + 祖先前缀。digest 覆盖结构化消息及发言身份、故事版本、玩家快照、摘要器/模板版本；只有选中父链参与摘要。召回记忆的 revision 加入本次生成 input_digest。摘要基于历史与冻结设定生成，不把临时操作要求、回复建议和未选候选写成事实。

## 4. 三条核心流程与事务

### 开始故事

chat/app.StartStory 读取本人可见的可玩版本和玩家身份，构造冻结快照。事务内重新确认作品状态/版本、人设读取版本，认领幂等记录，创建记忆空间、会话、结构化 assistant 开场根节点并设置 active_leaf_id，最后保存幂等结果。

无开场时允许空会话，第一条用户消息为根节点。开场创建失败则整个会话初始化回滚。试聊草稿先保存作者私有版本，再走同一开聊流程，不绕过版本机制把任意前端 JSON 直接当系统 Prompt。

### 生成回复

1. 请求进入 generation/app；鉴权、限流、检查 operation 与 anchor/target 是否匹配。
2. 短事务锁定会话，验证 expected_revision，认领幂等键和 active_generation_id，创建 pending 任务并冻结操作参数，立即提交。
3. 事务外由 chatcontext 读取被冻结的版本/玩家身份、不可变分支和一致的记忆 revision，召回/摘要并组装 ContextBundle。准备阶段也有 expires_at，失败释放占用。
4. 以条件更新将 pending 改为 running，调用 Provider。输出经角色标识、段数和长度校验，以 SSE 推送临时增量。
5. 完成短事务再次锁定会话和任务，确认仍为 running、会话可用、active_generation_id 仍匹配；保存消息与全部候选，完成任务并释放占用。
6. 仅当活动叶子/会话 revision 仍等于该任务预期值时推进 active_leaf_id；用户已切分支时保留结果在原分支，不能把用户跳回去。

不得持有数据库事务等待模型、embedding 或摘要。取消、失败、过期均以条件状态转换到终态，并仅释放自己持有的占用。首期不自动重发正在运行的模型调用；任务过期后由用户明确重试，避免不可观测的重复成本。MySQL 中的占用为准，进程内 map/Redis 仅辅助取消和通知。

suggestions 使用独立的低配额执行槽，避免长期占用剧情生成位；同用户/会话限制并发，结果绑定 anchor/input_digest。建议完成后不推进活动叶子、不创建消息；一旦用户换分支或更新上下文即标为过期。缓存命中也必须先校验访问权限。

### 编辑、重生成和续写

- 手动编辑：message/app 创建同父节点的新 assistant 消息，保留完整 segments。原节点和后续不改写；新节点成为活动分支。与活动生成冲突时返回 409，用户先取消或等待。
- regenerate/rewrite：新回复挂在目标回复的父节点上；rewrite 将原回复作为编辑对象，将要求作为本次操作参数，不作为玩家消息。
- continue：新 assistant 消息挂在指定 completed assistant 节点之后，无须伪造 user 消息。新校验按 operation 判定合法父节点，不再统一要求叶子为 user。
- reply：anchor 必须是当前合法的 completed user 消息。
- 开场根节点属于版本初始化内容，首期不编辑/重生成开场本身；通过编辑故事并新开实现。避免根节点例外进入普通消息操作。
- 候选选择与编辑都作用于整条 AI 节点，不把不同候选的角色气泡混拼。书签指向原节点，不自动跳到新版本。

## 5. Prompt 与输出协议

chatcontext 的组装顺序：运行规则 → 故事世界与阵容 → 玩家快照 → 标注为示例的风格参考 → 召回的作品世界书/私有记忆 → 当前分支摘要 → 最近完整节点 → 本次临时操作要求。

开场作为历史根节点只注入一次，不再从 FirstMessage 重复补充。历史序列化必须保留说话人和旁白类型。记忆/剧情内容使用明确的数据边界，不解释为服务端操作权限。

预算至少分为固定设定、召回、摘要、近期历史和输出预留。阵容/人设/常驻信息超过固定预算时返回 context_too_large，不能随机删除角色。近期历史按合法回复组裁剪；支持连续 assistant 的续写后，必须替换当前依赖简单 user/assistant 交替的切分假设。

首期单角色纯文本流包装成一个内容段。多角色阶段 Provider 返回受约束的 segments；先采用完整段缓冲、校验后展示的协议，不直接把半截 JSON 推给用户。模型不支持结构化输出时由适配器声明能力，采用服务器验证的文本 JSON 协议或拒绝该模式；不可把未知角色任意映射成主角。

建议输出示意（协议示例，不是可直接执行代码）：

```json
{"schema_version":1,"segments":[{"segment_id":"s1","kind":"narration","text":"雨水敲着窗。"},{"segment_id":"s2","kind":"dialogue","speaker_id":"cast_1","text":"这本书，你是不是找了很久？"}]}
```

SSE v2 事件至少包括 generation.started、segment.delta 或 segment.completed、generation.completed、generation.failed、generation.cancelled；携带 generation_id、candidate_index、segment_id 和递增 seq。最终 completed 在数据库提交后发送，包含正式 message_id。临时段不可进入后续上下文。

通过请求中的 protocol_version 协商新旧事件，旧客户端继续原 SSE。首期不承诺断线重放全部 delta；断线按现有策略取消未完成生成，重连 GET 任务及正式消息。如果完成提交与断线交错，以数据库终态为准，同幂等键不得再次调用 Provider。

## 6. API 布局

统一使用 /api/v1，沿用现有认证与分页格式；以下新增接口为建议契约。

- /stories：创建、本人作品列表；/:id 读取草稿、带 expected_revision 更新、软删除。
- /stories/:id/versions：冻结版本及查询可访问版本；保存版本不等于公开发布。
- /personas 与 /personas/:id：玩家身份 CRUD。
- POST /chats：兼容原 character_id；新增 mode=story、story_version_id、persona_id 或 recommended_persona_id、幂等键。拒绝 mode 与字段冲突。
- GET /chats/:id/bootstrap：当前会话、公共可见的故事/角色快照、玩家显示信息和活动叶子；消息单独游标分页，不返回全部历史或完整 Prompt。
- POST /chats/:id/messages：原用户消息写入，扩展 expected_revision 和幂等键。
- POST /messages/:id/revisions：AI 手动编辑分叉，提交完整 segments，不覆盖旧节点。
- POST /chats/:id/generations：新增 operation、anchor_id、target_message_id、instruction、protocol_version，保留旧 parent_id 兼容适配。
- POST /chats/:id/reply-suggestions：返回绑定上下文的三条建议；内部复用 generation 的任务状态与 Provider 能力。
- GET /generations/:id：查询归属本人的任务和结果；DELETE /generations/:id 复用取消语义。
- PUT /chats/:id/active-leaf：切分支，expected_revision 条件更新。
- /chats/:id/memories：服务端通过会话解析 memory_scope_id，不信任调用方指定任意空间；旧 /characters/:id/memories 保留。
- 公开版本后续另设 /public/stories、/public/creators；作品发布动作归 story，评论归 community，列表排序归 discovery。

标准错误：401 未登录；404 不存在或不可见；409 revision_conflict / generation_busy / idempotency_conflict；422 invalid_speaker / invalid_operation / context_too_large；429 限流；502/504 供应商失败/超时。沿用现有错误信封映射这些业务码，不能把供应商原始错误和密钥返回前端。

## 7. 工程目录与装配

在现有 backend/internal 下新增 story、persona、chatcontext；扩展 chat、message、generation、memory。各业务模块文件按用例命名，如 start_story、freeze_version、revise_reply，而非将所有业务堆在 handler.go。

模块契约优先定义在消费方 app；确需跨多个模块共享的窄契约保留 internal/port。领域值对象放各自 domain，外部模块只能用公开契约，不能实例化对方 infra。

关键接口语义：PlayableStoryReader 获取权限已验证的冻结定义；PersonaSnapshotReader 获取身份快照；BranchReader 读取已校验父链；MemoryRetriever 返回受作用域约束的事实；SummaryProvider 返回可校验摘要；ReplyCommitter 原子提交消息/候选/任务终态。

跨模块事务由用例编排与专门 infra 适配器组合 UnitOfWork/ChatGate，实现 message、chat、generation 的窄写接口。当前 DoneWriter 直接写多个 model 的路径先封装为 ReplyCommitter，随后内部改由所属模块仓储协作；不为了表归属牺牲完成事务的原子性。

httpapi/server.go 的装配逐步抽为 contents、conversation、generation 等装配函数或文件，复用同一份服务实例。不得在 HTTP 层继续查询角色、读取历史再拼 Prompt。

## 8. 实施批次与验证

**第一批：内容基础。** 新表和版本迁移；story/persona；角色导入故事；草稿与不可变版本；校验阵容/开场引用、权限、并发保存。

**第二批：可用故事聊天。** story 会话、开场初始化、玩家快照、chatcontext、memory_scope；保留 legacy 路径。验收快照稳定、幂等开聊、旧功能兼容和跨故事隔离。

**第三批：聊天辅助。** AI 编辑分叉、回复建议、服务端活动叶子、任务操作字段与结构化候选。验收编辑失效摘要、候选一致、并发冲突和建议不入剧情。

**第四批：多角色与高级操作。** 多段流协议、允许的角色集合、rewrite/continue、书签。验收坏 JSON、未知角色、中途取消、连续 assistant 历史裁剪和整轮替换。

数据库采用 expand → backfill → switch：新增可空字段/表，旧记录明确回填 mode=legacy；读取双路径；功能开关只向兼容新服务实例开放 story 请求。CharacterID 变可空等变更需显式版本迁移，不能仅依赖 AutoMigrate。新旧二进制混跑必须路由隔离故事流量；回退先关闭新开故事，保存已有新数据，不让旧服务错误读取 story 会话。

测试覆盖领域规则、真实 MySQL 事务、取消/完成竞争和跨账号读取。验证包括重复开聊、双设备改分支、生成中删除会话、生成完成时切分支、素材删除但快照仍可用、坏输出不入历史、重开默认不带旧记忆、现有 legacy 回归。

指标至少记录 context_prepare_ms、time_to_first_delta_ms、total_generation_ms、输出结构失败率、摘要缓存命中、召回降级、tokens/费用可用值、取消/超时次数和 revision 冲突。不默认记录原始 Prompt/私有记忆；缺少供应商用量时标记 unknown，不能写成零成本。

## 9. 当前证据 → 架构判断 → 改造位置

- E1：[会话模型](../backend/internal/model/chat.go) CharacterID 为非空；判断 F1：故事模式不能只加一个标题字段；路径 P1：增加 mode、版本、玩家快照与记忆空间，保留 legacy 分支。
- E2：[HTTP Prompt](../backend/internal/generation/http/prompt.go) 每次读取可变角色；[memory.Build](../backend/internal/memory/app/context.go) 引用 generation/app；判断 F2：上下文归属混杂；路径 P2：抽 chatcontext，memory 返回记忆/摘要，HTTP 只传命令。
- E3：[分支校验](../backend/internal/generation/app/branch.go) 强制 user 叶子；判断 F3：无法直接支持续写；路径 P3：按 operation 校验 anchor，同时重做摘要裁剪中的轮次假设。
- E4：[生成完成事务](../backend/internal/generation/infra/done.go) 固定 ExtraData 为 {}；判断 F4：结构化角色段会丢失；路径 P4：贯通正文、候选、选择与提交路径的 payload，并维护内容投影一致性。
- E5：[记忆模型](../backend/internal/model/memory.go) 按 user + character 存储；判断 F5：复用角色时缺故事隔离；路径 P5：显式 memory_scope，旧数据维持原作用域。
- E6：[装配入口](../backend/internal/httpapi/server.go) 集中创建存储与服务；[ChatGate](../backend/internal/chat/infra/gate.go) 已有会话行锁；判断 F6：适合增量模块化单体；路径 P6：复用事务门并拆装配，不先拆网络服务。

当前实现已通过全仓单元测试、竞态检测、`go vet`、带 `webembed` 标签的 `go vet`，以及真实 MySQL/Redis 下的故事 API 闭环与迁移幂等测试。验证覆盖版本冻结、并发修订冲突、幂等开聊、人设快照、开场只注入一次、故事记忆隔离、回复建议不入历史、AI 编辑另建分支、删除后停止新开但保留旧存档、跨账号隔离和 legacy 回归。

## 10. H5 落地状态（2026-09-18）

独立 H5 已接入本报告第一至第三批的用户主链：私有故事创建/编辑/删除、故事封面、玩家身份及头像、版本冻结后开聊、结构化开场展示、故事快照恢复、三条回复建议和 AI 回复分支改写。普通角色聊天继续沿用原接口，故事会话通过 `mode=story` 读取 bootstrap，前端不自行拼接可变草稿。

移动端使用现有紫色主题和安全区布局；375×812 浏览器回归覆盖“创建身份 → 创建故事 → 进入剧情 → 获取建议 → 改写回复”，并检查横向溢出、图片解码和输入区位置。图片在浏览器端重绘为受限 JPEG 后上传，故事正文和身份资料不写入本地存储。

公共发现流和冻结版本发布已于 2026-09-19 接入。当前还没有发布审核、创作者主页、评论、关注、相关作品推荐和通知。故事首版仍限制单角色；POV、时态、回复长度、节奏等写作风格控制、多角色结构化输出、聊天截图/书签、图片与视频消息列入后续批次。

## 11. 产品核心与上下文边界

产品核心不是三个互不相关的配置表，而是“稳定角色 × 连续关系 × 可进入的故事”：

- Character 保存稳定人格、形象和表达方式，回答“她是谁”。
- Story Version 保存本次故事的世界、角色阵容、开场和规则，回答“此刻发生在哪里”。版本冻结后，作者修改草稿不会改变用户正在经历的剧情。
- Memory Scope 保存用户与角色在指定故事和分支中的事实、约定、关系进展与摘要，回答“我们共同经历过什么”。
- Chat Session 绑定角色/故事版本、玩家身份、记忆空间和活动分支，是每次组装 Prompt 的唯一入口。

公共发布只把冻结版本挂到发现页。公开响应展示封面、标题、简介、标签、世界、角色资料和开场，不返回作者的示例提示及世界书；模型生成仍在服务端读取完整冻结版本。读者以自己的 Persona 和 Memory Scope 开聊，不能读取作者草稿或其他读者记忆。撤下作品后禁止新开，既有会话继续使用原冻结版本。

## 12. 关系连续性落地（2026-09-19）

故事阵容现在可以用 `companion_id` 绑定作者拥有的长期角色。保存草稿时校验归属，冻结版本保存角色资料副本；开聊时再次校验版本作者与角色身份，并把 `companion_id` 写入故事会话。因此模型使用稳定身份查找关系，同时仍使用当前故事版本里的姓名、性格、形象和世界快照，后续修改角色素材不会改写已经开始的剧情。

`relationships` 以 `(user_id, companion_id)` 唯一，保存关系阶段、关系现状、里程碑与乐观锁修订。同一用户用同一角色进入不同故事时复用这条记录；不同用户之间完全隔离。阶段采用明确状态而非自动增减的好感度分数，当前由用户确认后更新。

记忆分为两层同时召回：`relationships/:companion_id/memories` 是跨故事关系事实，`chats/:id/memories` 是当前故事存档事实。上下文顺序为冻结故事与玩家快照 → 关系档案 → 跨故事关系记忆 → 当前故事记忆与世界书 → 分支摘要 → 近期原文。故事未绑定长期角色时保持原有独立存档语义，不会被错误合并。

新增接口：

- `GET/PUT /api/v1/chats/:id/relationship`：服务端从本人会话解析角色，客户端不能用请求体改归属。
- `GET/POST /api/v1/relationships/:id/memories` 及单条更新、删除：必须先存在当前用户与该角色的关系。
- 创作 H5 可从自己的角色中选择长期角色；聊天 H5 的“我们的关系”面板可编辑阶段、现状、里程碑和跨故事记忆。

旧普通角色会话在迁移时回填关系档案，新建普通会话和绑定角色的故事会话都会原子创建关系。旧故事没有 `companion_id` 时继续按独立故事运行，不进行不可靠的名称匹配。

## 13. 用户确认式自动记忆（2026-09-19）

聊天页的“整理记忆”只读取当前选中的祖先分支，最多向模型发送最近 24 条已完成消息。模型输出最多六条结构化候选，并必须附带依据。普通角色聊天只允许提出 `relationship`；未绑定长期角色的故事只允许 `story`；绑定长期角色的故事允许两种范围。服务端再次校验范围、长度和重复内容，不能由模型或客户端扩大作用域。

候选保存在 `memory_candidates`，状态为 pending、accepted 或 rejected。pending 候选不会进入 Prompt；用户点击“记住”后，服务端在同一事务中锁定候选和目标关系/会话、检查配额、创建正式记忆并记录 memory_id。点击“忽略”后永久保留拒绝状态，同一会话中的相同指纹不会重复出现。重复确认返回 409，其他用户读取候选返回 404。

接受后的候选默认作为常驻事实，确保未配置向量服务时也能被召回；每个范围仍受 200 条和 6000 字节常驻预算限制。完全相同的正式记忆会复用已有记录，不重复写入。自动提取不会把剧情摘要、未选择候选回复或模型单方面陈述直接提升为关系事实。

新增接口：

- `GET /api/v1/chats/:id/memory-candidates`：列出当前会话待确认候选。
- `POST /api/v1/chats/:id/memory-candidates`：提交 `leaf_id`，从该分支提取候选。
- `POST /api/v1/memory-candidates/:id/accept`：确认并原子写入正式记忆。
- `DELETE /api/v1/memory-candidates/:id`：拒绝候选。
