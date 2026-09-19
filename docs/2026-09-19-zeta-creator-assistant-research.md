# Zeta Creator Assistant 研究与我们的 H5 方案

研究日期：2026-09-19

## 结论

Zeta 的创建助手不是把一个长表单机械地拆成六页，而是一条“用户给方向，AI 逐层补全，用户逐层确认”的内容生成流水线。

它先收集一句概念和若干氛围标签，然后依次生成开场、前史和角色阵容候选。每个候选都允许选择、编辑或完全自写。确认角色后，服务端生成完整 Plot，再进入角色图片处理。最终结果被交给正式的 Plot 编辑器继续完善。

我们目前已经有可以承载结果的 Story Definition、版本冻结、玩家人设、故事会话、记忆和公开发现。主要缺口在创建体验和生成编排层：现有 H5 直接展示完整故事表单，用户必须一次理解世界、角色、开场、示例和世界书；后端没有“创建会话、分步生成候选、保存选择、生成草稿”的应用服务。

应保留现有高级编辑器，并在它前面增加“快速创建”。快速创建产出的仍是同一个 Story Draft，不创建第二套作品模型。

## 已验证的六步流程

### 第 1 步：Concept

- 用户自由输入一句故事概念。
- 输入上限 200 字符。
- 示例是“一名时间旅行者被困在最糟糕的星期一”。
- 提交前做内容词检查。
- 这是唯一完全由用户先写的核心输入。

### 第 2 步：Vibe

- 支持多选。
- 当前公开界面固定项为 Romance、Fantasy、Action、Slice of Life、Drama、Mystery。
- 每项带一句解释，降低用户理解成本。
- 允许 “I’ll write my own”，输入自定义氛围。
- 至少选一项才能继续。
- 固定项由服务端接口返回，不写死在页面，因此可以分语言、灰度和调整排序。

### 第 3 步：Opening

- 服务端根据 Concept 和 Vibe 动态生成若干 `{id, title, description}` 候选。
- 用户点开候选后才能确认，避免误触直接进入下一步。
- 候选可以先编辑标题和描述再确认。
- 允许完全自写。
- 请求失败时允许重试或返回前一步修改；存在每日配额错误。

### 第 4 步：Backstory

- 根据 Concept、Vibe 和已选 Opening 动态生成若干前史候选。
- 交互与 Opening 相同：选择、预览、编辑、确认、自写。
- 前史是故事为何走到开场这一刻的因果层，不等同于世界书。

### 第 5 步：Characters

- 服务端根据前四步生成角色阵容，支持多角色。
- 阵容中明确区分玩家角色 `isUserCharacter` 和故事角色。
- 故事角色字段包括姓名、角色定位、性格、与玩家的关系。
- 玩家角色包含姓名和性格，不展示“与玩家关系”。
- 用户可横向浏览、逐个编辑、删除故事角色、增加角色。
- 非玩家角色姓名必填、不能重复、最长 25 字符，并做名称合规检查。
- 角色定位必填。

### 第 6 步：Cast & Images

- 前五步确认后先异步生成完整 Plot，页面提示通常约 30 秒。
- 生成结果至少包含角色描述、角色图像 Prompt，并作为整体结果交给正式创建流程。
- 用户可以逐个修改角色姓名、描述和图像 Prompt。
- 图片风格来自服务端列表；页面存在 Clean、Smart、Star、Heroic、Dream 等回退样式。
- 支持为所有有 Prompt 的角色生成图片，也允许直接跳过图片进入 Plot。
- 失败和内容违规分别处理；每个角色有一次免费重试，后续重试消耗 Pieces。

## 这套流程真正有效的原因

1. **渐进式披露。** 用户每次只回答一个认知问题，不必先理解完整 Plot 数据结构。
2. **AI 生成的是候选，不是最终答案。** 用户保留选择、修改和自写权，错误成本较低。
3. **后一步依赖前一步。** Opening、Backstory 和 Characters 不是相互独立的随机生成，它们共享一份累积 Brief。
4. **结构化输出。** 候选和角色都使用明确字段，最终才能稳定映射到 Plot 编辑器和运行时 Prompt。
5. **快速创建与高级编辑分层。** 助手负责从零到可编辑草稿；正式编辑器负责精修、样式、世界书和发布。
6. **服务端控制选项。** 固定 Vibe 和图像风格由接口返回，便于运营、国际化和实验。

## Zeta 创建体系不止六步助手

六步助手只解决“从空白到第一版”。其正式创建器还提供 Roleplay Style：叙事视角、时态、回复长度、对白/动作比例、节奏、世界难度、最多两个类型和一种文风。官方还把回复选项、状态信息框等运行时能力放在 Style/Intro 配置中。

因此不能把六步助手当成完整的 Zeta 创建器。它是引流入口，后面仍有高级配置层。

## 与我们现状的对应关系

| Zeta 结果 | 我们已有承载 | 当前缺口 |
| --- | --- | --- |
| Concept | Story 的 title、hook、world 可承载生成结果 | 缺少独立 brief 和生成过程 |
| Vibe | Definition.tags | 标签目前只是文本，没有服务端 taxonomy、说明和版本 |
| Opening | Definition.opening | 现有表单只能手写旁白和第一句，缺候选生成及多段开场编排 |
| Backstory | Definition.world 可部分承载 | 应补独立 premise/backstory，避免与世界规则混写 |
| Characters | Definition.cast | 当前校验只允许一个角色；缺玩家角色在故事定义中的推荐模板、角色关系字段 |
| Cast & Images | Cast.portrait、Story.cover | 只有上传，没有媒体生成任务、图像 Prompt、风格和重试账本 |
| 高级 Style | 当前 Prompt 的默认行为 | 缺 narrative style 结构及 Prompt 映射 |

## 我们的产品边界

我们的核心仍然是“角色关系在故事中持续发展”：

- Story 定义这一局发生在哪个世界、从哪个场景开始、有哪些角色。
- Companion/Relationship 定义长期陪伴对象及跨故事关系。
- Persona 定义用户在本次故事中的身份。
- Memory Scope 保存这一局发生过的事实；Relationship Memory 保存经过用户确认、可以跨故事延续的关系事实。

创建助手只负责生产 Story Draft，不能直接写入用户长期记忆，也不能替用户确认关系事实。

## 推荐的 H5 快速创建流程

首版采用六步，但文案和字段按我们的核心调整：

1. **一句灵感**：自由输入，最多 200 字；可选“从模板开始”。
2. **故事氛围**：恋爱、日常、幻想、悬疑、戏剧、冒险；多选，允许自定义。主题色继续使用我们的紫色。
3. **关系起点**：陌生初遇、青梅竹马、契约关系、久别重逢、敌对暧昧、已有恋人；这是 AI 女友产品需要比 Zeta 更早明确的变量。
4. **开场选择**：AI 返回三条结构化候选，支持编辑和自写。
5. **角色确认**：首版只生成一个主要 Companion 加一个玩家 Persona 模板；后端数据结构按数组设计，为多角色保留升级路径。
6. **预览并生成草稿**：展示标题、Hook、世界背景、关系起点、开场和角色；用户确认后写入正式 Story Draft，再进入现有高级编辑器。

图片生成不应阻塞首版快速创建。第六步先支持上传/稍后设置；媒体任务模块完成后再增加风格选择和异步生成。

## 后端布局

新增 `backend/internal/storyassistant`，它是创建编排模块，不侵入 `story` 的领域规则：

```text
storyassistant/
  domain/
    session.go       创建会话、步骤、状态、过期时间
    brief.go         concept、vibes、relationship、opening、cast
    candidate.go     候选及来源、排序、生成批次
    taxonomy.go      固定选项的稳定 key 与展示 DTO
  app/
    start.go         创建/恢复助手会话
    choose.go        校验并保存当前步骤
    generate.go      调模型生成下一步候选
    regenerate.go    重试与配额/idempotency
    materialize.go   转换为 story.Definition 并创建 Story Draft
  infra/
    repo.go          MySQL 会话与候选持久化
    generator.go     LLM 结构化输出适配器
  http/
    handler.go
    routes.go
```

`storyassistant` 依赖 `story` 的应用服务创建草稿；`story` 不依赖助手。生成模块只返回结构化候选，不直接写 Story。最终 `materialize` 通过显式映射和 `story` 现有校验创建草稿。

建议的核心表：

- `story_assistant_sessions`：id、user_id、status、current_step、brief_json、taxonomy_version、revision、expires_at、created_at、updated_at。
- `story_assistant_generations`：id、session_id、step、request_key、input_digest、status、model、attempt、error_code、created_at、finished_at。
- `story_assistant_candidates`：id、generation_id、stable_key、position、payload_json、selected_at。

候选必须持久化，刷新 H5 后仍能恢复；`request_key` 和 `input_digest` 防止重复扣费和重复生成。修改上游步骤时，应清除所有下游选择和候选。

固定选项使用稳定 key，例如 `romance`、`slice_of_life`，展示名和说明由 `/api/v1/story-assistant/taxonomy` 返回。Story 最终保存 key，而不是保存中文展示文字。

## API 草案

- `GET /api/v1/story-assistant/taxonomy`：Vibe、关系起点及可用版本。
- `POST /api/v1/story-assistant/sessions`：创建会话或从模板开始。
- `GET /api/v1/story-assistant/sessions/:id`：恢复进度、选择和候选。
- `PUT /api/v1/story-assistant/sessions/:id/brief`：保存 Concept/Vibe/关系起点，使用 revision 做乐观锁。
- `POST /api/v1/story-assistant/sessions/:id/steps/:step/generations`：生成或重生成候选，带 idempotency key。
- `PUT /api/v1/story-assistant/sessions/:id/steps/:step/selection`：选择、编辑或提交自写内容。
- `POST /api/v1/story-assistant/sessions/:id/materialize`：创建正式 Story Draft，返回 story_id 和 revision。

`materialize` 之后仍需用户在高级编辑器中主动发布；快速创建不能直接公开作品。

## Prompt 和结构化输出

每一步只给模型必要的累积 Brief，并要求 JSON Schema 输出：

- Opening 候选：`title`、`narration`、`first_dialogue`、`emotional_hook`。
- Cast 候选：`name`、`role`、`description`、`personality`、`relationship_to_player`、`suggested_persona`。
- Final Draft：`title`、`hook`、`world`、`cast[]`、`opening[]`、`examples`、`lore[]`。

后端必须做长度、枚举、角色引用、重复姓名和开场 speaker_id 校验。模型输出不应直接透传到数据库。

## 实施顺序

1. 建 taxonomy、session、candidate 数据结构和恢复接口。
2. 实现“一句灵感、氛围、关系起点”三个纯结构步骤。
3. 接入 Opening 三候选生成、编辑、自写、重试和下游失效。
4. 接入单 Companion + Persona 模板生成，并映射到现有 Story Draft。
5. 将现有长表单改为高级编辑器，快速创建完成后跳转进入。
6. 增加 Style 数据和 Prompt 映射。
7. 媒体模块就绪后加入图片风格、异步生成和重试计费。

## 证据与限制

- 公开创建助手页面和其当前前端资源确认了六步名称、字段顺序、候选交互、角色编辑、图片步骤、接口边界和错误处理。
- Vibe 固定项以用户提供的当前截图为准；接口需要登录，未来可能由 Zeta 服务端调整。
- Opening、Backstory 和 Characters 的具体候选内容由 AI 动态产生，不存在一套可以完整抄录的固定选项。
- 未提交最终生成或发布动作，也未验证付费重试的实际扣费流程。
- Zeta 官方公告确认高级 Roleplay Style、回复选项和状态信息框；这些属于正式创建器和运行时能力，不属于六步助手本身。

## 官方来源

- Creator Assistant：https://zeta-ai.io/en/plots/assistant
- Plot 定位：https://zeta-ai.io/en/announcements/9736
- Roleplay Styles：https://zeta-ai.io/en/Announcements/9109
- Options Feature：https://zeta-ai.io/en/announcements/10594
- Info Boxes：https://zeta-ai.io/en/announcements/9604
- Auto Plots & Image Generation：https://web-cdn.zeta-ai.io/en/announcements/11023
