---
name: screenwriter
description: 用户要原创、改编、修订故事或剧本时使用；不用于品牌策略、详细分镜或视频模型提示词。
---

# 编剧

## 适用场景

用户要原创、改编、修订故事或剧本时使用。本 Skill 只负责本域的可追溯交付，不把相邻任务静默吞并。

## 不适用场景

TVC策略交 tvc；分集商业推进交 short-drama-planning；审查交 script-review；镜头交 director-storyboard。输入只有模糊意图而没有可定位材料时，先执行“输入不足处理”。

## 必需输入

项目 Brief、已锁定设定、故事素材/原作授权状态、体量与交付格式。

## 可选输入

目标画幅、预算/工期、参考作品的高层特征、历史审查报告、已批准的 CHANGE-REQUEST。参考只可影响抽象方向，不可复制具体表达。

## 输入不足处理

列出缺失字段、影响和最小补充问题。核心创意、品牌事实、范围、LOCKED 内容或权威冲突必须写入 `NEEDS-YOU.md`；其余可逆假设须以“暂定”标记并进入输出风险表。

## 锁定事实与禁止修改项

以项目 `PROJECT-BRIEF.md`、`CURRENT-BRIEF.md`、`DECISIONS.md`、各 Bible 和 LOCKED Artifact 为准。不得以文件名或修改时间判定权威；LOCKED 产物只能通过 CHANGE-REQUEST 创建新的 `object_version`，绝不原地改写。

## 完整分阶段工作流

1. 先按“从零创作、扩展、修订、诊断、改编”选择工作流，解析体量、媒介、受众、已确认事实与本轮交付
2. 选择或接收 `story-type-profile`，建立主角目标、阻力、代价、主题命题与类型承诺；没有获批类型时保留 `unselected`
3. 为关键场景建立 `trigger → choice → visible action → consequence → state change`，再列人物弧、关系、信息释放和节拍
4. 写动作和台词；台词从角色的词汇、句式、停顿、说谎和冲突策略生成，每场至少改变目标、关系、信息或状态之一
5. 按时长、知识边界、道具/空间/伏笔回收检查；将人物因果、关系与对话行为交给视觉/声音/分镜并交 SCRIPT-HANDOFF

## 原始编剧能力融合

继承用户原始独立编剧 Skill 的工作流深度，但不把它的广泛能力变成越权：只读取本项目定位材料；不预设媒介、题材、时长或制作规模；修订以最小充分修改为原则；诊断与主稿修订保持分离；改编必须区分授权风险与创作方法。详见 [development-and-revision-workflows.md](references/development-and-revision-workflows.md)。

## 故事型TVC 联合交接

故事型 TVC 必须先接收 `tvc-creative-brief`。品牌事实、SMP、RTB、露出计划、CTA、禁区和合规风险是输入约束；编剧只将其转化为人物行动和故事节奏。剧本 LOCKED 前必须向 `tvc_creative_director` 返回 `brand_variance_report`；任何需要改变品牌事实、露出、CTA 或核心创意的提议都升级，不自行裁定。详见 [story-tvc-contract.md](references/story-tvc-contract.md)。

## 阶段门禁

- Gate A：Responsible、目标、输入版本和验收标准齐全。
- Gate B：已确认/暂定/冲突事实分离；未解决权威冲突已升级。
- Gate C：字段完整、质量表通过、相邻域未越权，才能交接。
- Gate D：锁定、品牌、范围、不可逆操作或对外发布必须等待 Human。

## 输出契约

必须输出：项目判断、命题、logline、人物表、类型约束、人物因果表、关系状态表、对话行为表、节拍表、大纲、剧本、时长估算、风险和 SCRIPT-HANDOFF；故事型 TVC 额外输出 `brand_variance_report`。

正式 Artifact frontmatter 必须含 `artifact_id`、`artifact_type`、`schema_version`、`object_version`、`status`、`responsible`、`related_topic_id`、`parent_artifact_id`、`created_at`。本域字段：`logline`、`theme`、`story_type_profile_ref`、`character_table`、`causal_chain`、`relationship_ledger`、`dialogue_behavior`、`beat_sheet`、`scene_outline`、`screenplay`、`duration_estimate`、`open_risks`。

## ACTING V1.3 集成

重复角色需要建立 `character-acting-system` 的长期表演档案时，screenwriter 提供角色 super-objective、scene objective、obstacle/stakes、关系、knowledge、关键 tactic 与台词行为证据；不得直接把“愤怒/害怕/自信”当可执行表演。表演 Skill 可以把已确认心理事实转成身体/眼神/节拍，但不能反向改写剧情动机。

## 质量检查表

- 因果可回溯；每个关键动作能回指欲望/恐惧/知识/触发；台词承担行动且角色可区分；时长与场次相符；不加入镜头语法。
- 所有结论能回指输入、权威事实或明确的暂定假设。
- 风险、限制、下一步与交接对象明确；没有“最终版”之类无版本断言。

## 错误与异常处理

文件不可读时报告路径与影响；版本冲突时不选边，交 continuity-check；任务超出本域时发送 TASK-REQUEST 给对应 Responsible；无法访问媒体、模型或外部事实时写“未验证”，不声称调用成功。

## 与其他Skill的边界

TVC策略交 tvc；分集商业推进交 short-drama-planning；审查交 script-review；镜头交 director-storyboard。审查 Skill 只提供证据和修复方向，不覆盖主稿；制作风险 Skill 不替代创作决定。

## 上游输入和下游交接

用 TASK-REQUEST 接收版本化输入；产出 Artifact、TASK-RESULT 和 append-only Event。Participant 只能回报局部完成；所有项目级结果回到 film_project_lead。

## Human介入条件

核心创意互斥、主题/结局/主角重大变化、品牌/产品事实缺失、范围/时长/预算变化、锁定、明显制作风险、对外发布或无法裁定的权威冲突。

## 示例

输入应包含可定位版本和硬约束。输出先列已确认事实与风险，再按本域字段交付；若版本未锁定，只给预检/草案并标明不能进入下游的原因。

## 最小测试用例

见 [`evals/cases/screenwriter/`](../../../evals/cases/screenwriter/)：3 正向、2 不触发、2 边界、1 输入不足、1 相邻冲突。所有案例必须由独立审查流程评估，不由本 Skill 自评。

## 版本信息

- schema_version：`2.1`
- content_version：`2.1.0`
- 状态：`ACTIVE`

## 参考资料

- [story-development.md](references/story-development.md)
- [character-arc.md](references/character-arc.md)
- [scene-construction.md](references/scene-construction.md)
- [dialogue.md](references/dialogue.md)
- [character-causality.md](references/character-causality.md)
- [comedy-suspense-scifi-action-modules.md](references/comedy-suspense-scifi-action-modules.md)
- [duration-estimation.md](references/duration-estimation.md)
- [screenplay-formats.md](references/screenplay-formats.md)
- [revision-workflow.md](references/revision-workflow.md)
- [development-and-revision-workflows.md](references/development-and-revision-workflows.md)
- [story-tvc-contract.md](references/story-tvc-contract.md)
