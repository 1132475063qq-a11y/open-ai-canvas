---
name: final-film-quality-review
description: 已有成片、粗剪、样片或逐镜头结果需要验收时使用；不用于未观看实际材料便声称通过。
---

# 成片质量审查

## 适用场景

已有成片、粗剪、样片或逐镜头结果需要验收时使用。本 Skill 只负责本域的可追溯交付，不把相邻任务静默吞并。

## 不适用场景

剧本文本审查交 script-review；连续性归 continuity；收口交 film_project_lead。输入只有模糊意图而没有可定位材料时，先执行“输入不足处理”。

## 必需输入

可访问媒体/逐镜头结果、时间码、交付规格、品牌事实和验收标准。

## 可选输入

目标画幅、预算/工期、参考作品的高层特征、历史审查报告、已批准的 CHANGE-REQUEST。参考只可影响抽象方向，不可复制具体表达。

## 输入不足处理

列出缺失字段、影响和最小补充问题。核心创意、品牌事实、范围、LOCKED 内容或权威冲突必须写入 `NEEDS-YOU.md`；其余可逆假设须以“暂定”标记并进入输出风险表。

## 锁定事实与禁止修改项

以项目 `PROJECT-BRIEF.md`、`CURRENT-BRIEF.md`、`DECISIONS.md`、各 Bible 和 LOCKED Artifact 为准。不得以文件名或修改时间判定权威；LOCKED 产物只能通过 CHANGE-REQUEST 创建新的 `object_version`，绝不原地改写。

## 完整分阶段工作流

1. 确认可访问媒体和规格；不可访问则只报告无法验收
2. 逐镜审叙事、表演、剪辑、视觉一致性、物理/AI伪影、口型和声音
3. 核对品牌/产品/CTA、画幅、字幕、技术交付和平台规范
4. 按观众影响和交付阻断级别排序，指定修复责任人
5. 对修复版按同一时间码复检并给出结论

## 阶段门禁

- Gate A：Responsible、目标、输入版本和验收标准齐全。
- Gate B：已确认/暂定/冲突事实分离；未解决权威冲突已升级。
- Gate C：字段完整、质量表通过、相邻域未越权，才能交接。
- Gate D：锁定、品牌、范围、不可逆操作或对外发布必须等待 Human。

## 输出契约

必须输出：REJECT/CONDITIONAL PASS/PASS/POLISH 报告，含时间码、证据、修复责任人和复检结果。

V1.2 终审先检查 `GenerationResult.media_ref` 是否可访问。缺失媒体时 verdict 必须为 `NOT_ASSESSABLE`，不得借用 Prompt、Storyboard 或静态契约检查结果写 PASS。

正式 Artifact frontmatter 必须含 `artifact_id`、`artifact_type`、`schema_version`、`object_version`、`status`、`responsible`、`related_topic_id`、`parent_artifact_id`、`created_at`。本域字段：`timecode`、`category`、`severity`、`evidence`、`audience_impact`、`repair`、`owner`、`regenerate`、`recheck`、`verdict`。

## Production System V2 集成

输出逐镜 `Media QC Report`，并把视觉、动作、道具、空间、UI、声音、口型和成片规格问题映射至 `Shot Contract`、Reference Lock 与 `Continuity Ledger` 的可定位证据。只有实际可访问媒体可给 PASS/CONDITIONAL PASS/REJECT；`MOCKED` 仅能得到 MOCKED_FAILURE，缺媒体只能得到 NOT_ASSESSABLE。问题必须由最小 `Rework Event` 指向责任对象、修复范围和复检门槛；审查者不自行改写正式产物。

以 [`PRODUCTION-SYSTEM-V2.md`](../../../contracts/PRODUCTION-SYSTEM-V2.md) 的 Media QC、Rework Event 与证据边界为准。

## 质量检查表

- 无媒体绝不 PASS；局部通过不等于整片通过；每条不是主观感觉。
- 所有结论能回指输入、权威事实或明确的暂定假设。
- 风险、限制、下一步与交接对象明确；没有“最终版”之类无版本断言。

## 错误与异常处理

文件不可读时报告路径与影响；版本冲突时不选边，交 continuity-check；任务超出本域时发送 TASK-REQUEST 给对应 Responsible；无法访问媒体、模型或外部事实时写“未验证”，不声称调用成功。

## 与其他Skill的边界

剧本文本审查交 script-review；连续性归 continuity；收口交 film_project_lead。审查 Skill 只提供证据和修复方向，不覆盖主稿；制作风险 Skill 不替代创作决定。

## 上游输入和下游交接

用 TASK-REQUEST 接收版本化输入；产出 Artifact、TASK-RESULT 和 append-only Event。Participant 只能回报局部完成；所有项目级结果回到 film_project_lead。

## Human介入条件

核心创意互斥、主题/结局/主角重大变化、品牌/产品事实缺失、范围/时长/预算变化、锁定、明显制作风险、对外发布或无法裁定的权威冲突。

## 示例

输入应包含可定位版本和硬约束。输出先列已确认事实与风险，再按本域字段交付；若版本未锁定，只给预检/草案并标明不能进入下游的原因。

## 最小测试用例

见 [`evals/cases/final-film-quality-review/`](../../../evals/cases/final-film-quality-review/)：3 正向、2 不触发、2 边界、1 输入不足、1 相邻冲突。所有案例必须由独立审查流程评估，不由本 Skill 自评。

## 版本信息

- schema_version：`3.0`
- content_version：`3.0.0`
- 状态：`ACTIVE`

## 参考资料

- [narrative-review.md](references/narrative-review.md)
- [visual-review.md](references/visual-review.md)
- [ai-artifact-review.md](references/ai-artifact-review.md)
- [audio-review.md](references/audio-review.md)
- [brand-review.md](references/brand-review.md)
- [technical-delivery.md](references/technical-delivery.md)
- [final-qc-report.md](references/final-qc-report.md)
- [media-result-boundary.md](references/media-result-boundary.md)
