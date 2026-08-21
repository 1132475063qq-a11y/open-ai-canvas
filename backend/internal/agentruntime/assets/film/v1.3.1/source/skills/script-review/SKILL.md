---
name: script-review
description: 用户要求独立检查剧本逻辑、人物、节奏或连续性时使用；不用于未经请求地重写正式主稿。
---

# 剧本审查

## 适用场景

用户要求独立检查剧本逻辑、人物、节奏或连续性时使用。本 Skill 只负责本域的可追溯交付，不把相邻任务静默吞并。

## 不适用场景

修订交 screenwriter；跨产物冲突交 continuity-check；最终成片交 final-film-quality-review。输入只有模糊意图而没有可定位材料时，先执行“输入不足处理”。

## 必需输入

可定位剧本、object_version、审查范围、权威事实源与制作约束。

## 可选输入

目标画幅、预算/工期、参考作品的高层特征、历史审查报告、已批准的 CHANGE-REQUEST。参考只可影响抽象方向，不可复制具体表达。

## 输入不足处理

列出缺失字段、影响和最小补充问题。核心创意、品牌事实、范围、LOCKED 内容或权威冲突必须写入 `NEEDS-YOU.md`；其余可逆假设须以“暂定”标记并进入输出风险表。

## 锁定事实与禁止修改项

以项目 `PROJECT-BRIEF.md`、`CURRENT-BRIEF.md`、`DECISIONS.md`、各 Bible 和 LOCKED Artifact 为准。不得以文件名或修改时间判定权威；LOCKED 产物只能通过 CHANGE-REQUEST 创建新的 `object_version`，绝不原地改写。

## 完整分阶段工作流

1. 确认审查版本和锁定事实，列出无法验证的输入
2. 逐场检查因果、动机、知识、时间、空间、道具、规则和信息重复；关键动作必须能追到触发、选择、后果和状态变化
3. 检查类型承诺、关系双向变化、台词行为、时长和初步制作风险；TVC 额外核品牌信息
4. 按 P0-P3 只记录有证据的问题，区分硬冲突与偏好
5. 汇总最小修复路径，返还 Responsible；不改主稿

## 阶段门禁

- Gate A：Responsible、目标、输入版本和验收标准齐全。
- Gate B：已确认/暂定/冲突事实分离；未解决权威冲突已升级。
- Gate C：字段完整、质量表通过、相邻域未越权，才能交接。
- Gate D：锁定、品牌、范围、不可逆操作或对外发布必须等待 Human。

## 输出契约

必须输出：P0-P3 审查报告；每项含位置、证据、影响、修复方向与上游决定标记。

正式 Artifact frontmatter 必须含 `artifact_id`、`artifact_type`、`schema_version`、`object_version`、`status`、`responsible`、`related_topic_id`、`parent_artifact_id`、`created_at`。本域字段：`finding_id`、`severity`、`location`、`evidence`、`type_contract_ref`、`causal_chain_break`、`dialogue_behavior_conflict`、`impact`、`repair_direction`、`upstream_decision`、`verification_status`。

## 质量检查表

- P0/P1 可由文本证据复现；结论不声称观众必然反应；每条建议有责任人或升级路径。
- 所有结论能回指输入、权威事实或明确的暂定假设。
- 风险、限制、下一步与交接对象明确；没有“最终版”之类无版本断言。

## 错误与异常处理

文件不可读时报告路径与影响；版本冲突时不选边，交 continuity-check；任务超出本域时发送 TASK-REQUEST 给对应 Responsible；无法访问媒体、模型或外部事实时写“未验证”，不声称调用成功。

## 与其他Skill的边界

修订交 screenwriter；跨产物冲突交 continuity-check；最终成片交 final-film-quality-review。审查 Skill 只提供证据和修复方向，不覆盖主稿；制作风险 Skill 不替代创作决定。

## 上游输入和下游交接

用 TASK-REQUEST 接收版本化输入；产出 Artifact、TASK-RESULT 和 append-only Event。Participant 只能回报局部完成；所有项目级结果回到 film_project_lead。

## Human介入条件

核心创意互斥、主题/结局/主角重大变化、品牌/产品事实缺失、范围/时长/预算变化、锁定、明显制作风险、对外发布或无法裁定的权威冲突。

## 示例

输入应包含可定位版本和硬约束。输出先列已确认事实与风险，再按本域字段交付；若版本未锁定，只给预检/草案并标明不能进入下游的原因。

## 最小测试用例

见 [`evals/cases/script-review/`](../../../evals/cases/script-review/)：3 正向、2 不触发、2 边界、1 输入不足、1 相邻冲突。所有案例必须由独立审查流程评估，不由本 Skill 自评。

## 版本信息

- schema_version：`2.0`
- content_version：`2.0.0`
- 状态：`ACTIVE`

## 参考资料

- [causal-logic.md](references/causal-logic.md)
- [character-motivation.md](references/character-motivation.md)
- [type-and-causal-review.md](references/type-and-causal-review.md)
- [timeline-spatial-review.md](references/timeline-spatial-review.md)
- [dialogue-review.md](references/dialogue-review.md)
- [duration-review.md](references/duration-review.md)
- [review-severity.md](references/review-severity.md)
- [review-report-template.md](references/review-report-template.md)
