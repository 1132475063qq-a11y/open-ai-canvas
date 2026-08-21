# Artifact 类型注册表

**schema_version**: `3.2`  
**状态**: `ACTIVE`  
**更新日期**: `2026-08-10`  
**维护者**: `film_project_lead`

这是文件模式中 Artifact 的唯一类型词表。新建 Artifact 必须使用下表的
`canonical_artifact_type`；历史 Artifact 的旧拼写不改写，通过兼容映射解析，
以保护已有版本和审计链。

## 使用规则

- `artifact_type` 是小写 kebab-case 的 canonical 值；不得把状态、版本、文件名
  或 handoff 名称塞入该字段。
- 每一份 Artifact 还要写 `artifact_domain`，便于路由和未来 Canvas 分组。
- 旧值只有在本表 `legacy_aliases` 中有映射时可读；新模板和新 E2E 不允许使用别名。
- 类型变更、类型合并和未列类型必须由 `film_project_lead` 创建
  `routing-decision`，不可凭文件名推断。

## Canonical 类型

| canonical_artifact_type | artifact_domain | Responsible | 用途 |
|---|---|---|---|
| `project-requirements` | project | film_project_lead | 项目约束与验收 |
| `task` | orchestration | film_project_lead | 可执行任务卡 |
| `routing-decision` | orchestration | film_project_lead | 路由依据和版本选择 |
| `human-decision` | governance | film_project_lead | Human 已确认决定的快照 |
| `story-type-profile` | narrative | short_drama_planner | 故事类型引擎及适配 |
| `short-drama-plan` | narrative | short_drama_planner | 系列和分集策划 |
| `script` | narrative | narrative_screenwriter | 剧本/分集剧本 |
| `tvc-creative-brief` | brand | tvc_creative_director | TVC 创意 Brief |
| `tvc-script` | brand | narrative_screenwriter | TVC 剧本 |
| `canon-bible` | canon | narrative_screenwriter | 世界、事实和知识边界 |
| `character-causal-bible` | character | narrative_screenwriter | 欲望到后果的人物因果 |
| `character-embodiment-bible` | character | visual_development_designer | 外形、动作、声音的统一锚点 |
| `relationship-ledger` | character | narrative_screenwriter | 关系状态与相互影响 |
| `character-acting-profile` | character | narrative_screenwriter | 重复角色的长期表演行为 Source Profile |
| `scene-acting-adaptation` | direction | director_storyboard_artist | 将长期表演档案重写为当前场景/镜头的可观察行为 |
| `image-prompt-pack` | visual | visual_development_designer | 角色/场景/道具/编辑的图像模型路由与提示词包 |
| `character-design` | visual | visual_development_designer | 角色视觉资产 |
| `scene-design` | visual | visual_development_designer | 空间、道具和机位资产 |
| `visual-style-guide` | visual | visual_development_designer | 全局视觉规则 |
| `sound-design-plan` | sound | sound_designer | 声音和角色声线方案 |
| `storyboard` | direction | director_storyboard_artist | 分镜和镜头决策 |
| `shot-decision-sheet` | direction | director_storyboard_artist | 逐镜目的、观众信息和运镜理由 |
| `ai-video-prompts` | ai-production | ai_production_supervisor | 独立镜头提示词包 |
| `generation-attempt` | ai-production | ai_production_supervisor | 一次生成请求的记录；不等于已生成 |
| `generation-result` | ai-production | ai_production_supervisor | 已取得媒体/结果的索引；无文件则 UNKNOWN |
| `generation-qc-report` | ai-production | quality_control_editor | 逐尝试的结果 QC |
| `production-feasibility-report` | ai-production | ai_production_supervisor | 风险和降级计划 |
| `script-review-report` | review | quality_control_editor | 剧本文本审查 |
| `continuity-report` | review | quality_control_editor | 跨媒介连续性审查 |
| `qc-report` | review | quality_control_editor | 阶段质量审查 |
| `final-qc-report` | review | quality_control_editor | 有媒体时的终审 |
| `project-summary` | project | film_project_lead | 项目收口 |
| `asset-bible` | production | visual_development_designer | 结构化角色、道具、UI 等生产资产 |
| `scene-bible` | production | visual_development_designer | 结构化空间、坐标、Anchor、机位与状态变体 |
| `shot-contract` | production | director_storyboard_artist | 正式镜头的生产 Source of Truth |
| `continuity-ledger` | production | film_project_lead | 每镜 read-in / write-out 状态账本 |
| `reference-lock-ledger` | production | visual_development_designer | L0–L5 参考锁与允许/禁止变化 |
| `dependency-graph` | production | film_project_lead | 镜头到资产/场景的派生依赖图 |
| `production-readiness-report` | production | film_project_lead | 生产就绪门的派生可追溯结果 |
| `style-bible` | production | visual_development_designer | 全局视觉、材质、光线、UI 与镜头语言 |
| `shot-package` | production | ai_production_supervisor | 由正式镜头编译的版本固定生产包 |
| `prompt-manifest` | production | ai_production_supervisor | 独立提示词与动态负向约束的编译结果 |
| `shot-feasibility-report` | production | ai_production_supervisor | 镜头复杂度、降级与回退建议 |
| `media-qc-report` | production | quality_control_editor | 真实或明示模拟媒体结果的 QC 边界 |
| `rework-event` | production | ai_production_supervisor | 从失败证据派生的最小范围返工计划 |
| `impact-analysis` | production | film_project_lead | LOCKED 对象变更前的反向依赖和复检范围计划 |

## 历史兼容映射

历史产物可能使用下列值。解析器把它们映射到 canonical 值，但不会修改原文件：

| legacy_alias | canonical_artifact_type |
|---|---|
| `worldbuilding-bible` | `canon-bible` |
| `character-bible` | `character-causal-bible` |
| `visual-asset` | `character-design` |
| `ai-prompts` | `ai-video-prompts` |
| `feasibility` | `production-feasibility-report` |
| `sound` | `sound-design-plan` |
| `script-review` | `script-review-report` |
| `continuity` | `continuity-report` |
| `qc` | `qc-report` |
| `execution-summary` | `project-summary` |
| `input` | `project-requirements` |
| `series-plan` | `short-drama-plan` |
| `world-knowledge` | `canon-bible` |
| `episode-script` | `script` |
| `AI-PRODUCTION-FEASIBILITY` / `feasibility-report` | `production-feasibility-report` |
| `AI-VIDEO-PROMPT-PACK` / `AI_VIDEO_PROMPT_PACK` / `AI_VIDEO_SHOT_PROMPTS` | `ai-video-prompts` |
| `AI_PRODUCTION_HANDOFF` / `AI_PRODUCTION_REVIEW` | `production-feasibility-report` |
| `CHARACTER-DESIGN` | `character-design` |
| `CONCEPT_OPTIONS` | `short-drama-plan` |
| `CONTINUITY-REPORT` / `CONTINUITY_REPORT` | `continuity-report` |
| `E2E-EXECUTION-SUMMARY` / `EXECUTION_SUMMARY` | `project-summary` |
| `FINAL-FILM-QC-REPORT` / `FINAL_FILM_QUALITY_REVIEW` | `final-qc-report` |
| `INPUT_FACTS` / `PROJECT-BRIEF` / `PROJECT_BRIEF` / `project-brief` | `project-requirements` |
| `LOCATION-BIBLE` | `scene-design` |
| `QUALITY_CONTROL_REPORT` | `qc-report` |
| `SCREENPLAY` / `SCRIPT_HANDOFF` | `script` |
| `SCRIPT-REVIEW` / `SCRIPT_REVIEW` | `script-review-report` |
| `SERIES_PLAN` / `series-plan` | `short-drama-plan` |
| `SOUND-BRIEF` / `SOUND_CUE_SHEET` / `SOUND_PLAN` | `sound-design-plan` |
| `STORY-BIBLE` / `STORY_BIBLE` / `world-bible` | `canon-bible` |
| `STORYBOARD` | `storyboard` |
| `TVC_CREATIVE_HANDOFF` / `tvc-creative-handoff` | `tvc-creative-brief` |
| `VISUAL_ASSET_BIBLE` | `character-design` |

## 版本和状态

Artifact 的版本写在 `object_version`，状态只能是 `DRAFT`、`REVIEW`、`LOCKED`、
`SUPERSEDED` 或 `ARCHIVED`。`LOCKED` 仍必须有对应的 RECORDED
`artifact.locked` Event；本注册表不把 Markdown frontmatter 当作状态事实。

## 变更记录

| 日期 | 版本 | 变更 | 说明 |
|---|---|---|---|
| 2026-08-08 | 1.0 | 初版 | 18 个类型，未连接校验器。 |
| 2026-08-09 | 2.0 | V1.2 | 增加 Canon、Task、HumanDecision、生成闭环和兼容映射。 |
| 2026-08-09 | 3.0 | Production System P0 | 增加结构化资产、场景、镜头、连续性、参考锁、依赖图与就绪门类型；历史 Artifact 不改写。 |
| 2026-08-09 | 3.1 | Production System V2 | 增加 impact-analysis；它只规划影响，不授权变更。 |
| 2026-08-10 | 3.2 | Higgsfield/Performance/Image Prompt V1.3 | 增加 acting profile、scene acting adaptation 与 image prompt pack；不改变媒体证据边界。 |
