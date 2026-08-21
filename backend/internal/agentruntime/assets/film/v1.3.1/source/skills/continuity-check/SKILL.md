---
name: continuity-check
description: 需跨剧本、分集、分镜、角色、场景和提示词定位连续性冲突时使用；不用于自行裁定未锁定事实。
---

# 连续性检查

## 适用场景

需跨剧本、分集、分镜、角色、场景和提示词定位连续性冲突时使用。本 Skill 只负责本域的可追溯交付，不把相邻任务静默吞并。

## 不适用场景

剧本问题交 screenwriter；分镜问题交 director；最终判断交 quality-control-editor。输入只有模糊意图而没有可定位材料时，先执行“输入不足处理”。

## 必需输入

多个版本化 Artifact、权威来源、检查范围和时间线。

## 可选输入

目标画幅、预算/工期、参考作品的高层特征、历史审查报告、已批准的 CHANGE-REQUEST。参考只可影响抽象方向，不可复制具体表达。

## 输入不足处理

列出缺失字段、影响和最小补充问题。核心创意、品牌事实、范围、LOCKED 内容或权威冲突必须写入 `NEEDS-YOU.md`；其余可逆假设须以“暂定”标记并进入输出风险表。

## 锁定事实与禁止修改项

以项目 `PROJECT-BRIEF.md`、`CURRENT-BRIEF.md`、`DECISIONS.md`、各 Bible 和 LOCKED Artifact 为准。不得以文件名或修改时间判定权威；LOCKED 产物只能通过 CHANGE-REQUEST 创建新的 `object_version`，绝不原地改写。

## 完整分阶段工作流

1. 建立 artifact/version/source 清单，不以修改时间判权威
2. 抽取时间、地点、人物因果/关系状态、外形/服装/动作、道具、知识、声音锚点、规则和 Shot/Prompt/Attempt/Result 状态 ledger
3. 对比剧本、Bible、场景资产、分镜、提示词、生成链和声音方案，标出冲突 A/B 与位置
4. 找到锁定权威；无权威则升级而不选边
5. 按责任人返回修复建议并复检新版本

## 阶段门禁

- Gate A：Responsible、目标、输入版本和验收标准齐全。
- Gate B：已确认/暂定/冲突事实分离；未解决权威冲突已升级。
- Gate C：字段完整、质量表通过、相邻域未越权，才能交接。
- Gate D：锁定、品牌、范围、不可逆操作或对外发布必须等待 Human。

## 输出契约

必须输出：可定位冲突报告、权威来源、修复建议、决策人和修复状态。

V1.2 连续性报告必须注明 `media_state`：尚无可访问媒体时只检查文本/结构化约束，不能断言画面连续性或成片质量。

正式 Artifact frontmatter 必须含 `artifact_id`、`artifact_type`、`schema_version`、`object_version`、`status`、`responsible`、`related_topic_id`、`parent_artifact_id`、`created_at`。本域字段：`issue_id`、`severity`、`source_versions`、`location`、`fact_a`、`fact_b`、`authority`、`owner`、`repair_status`。

## Production System V2 集成

维护逐镜 `Continuity Ledger`，对 character_state、prop_state、scene_state、knowledge_state、audio_state、screen_direction、action_axis、UI 状态和 Reference Lock 逐项记录输入/输出。镜头的 blocking 起点必须继承上一状态，跨场或跨轴必须写理由。无媒体时结论仅限结构化/文本连续性；可访问媒体的逐镜对照另交 final-film-quality-review。

以 [`PRODUCTION-SYSTEM-V2.md`](../../../contracts/PRODUCTION-SYSTEM-V2.md) 的 Continuity Ledger、镜头轴线与 Reference Lock 规则为准。

## 质量检查表

- 每个冲突双方可定位；不静默修主稿；没有权威时标 UNSOLVED。
- 所有结论能回指输入、权威事实或明确的暂定假设。
- 风险、限制、下一步与交接对象明确；没有“最终版”之类无版本断言。

## 错误与异常处理

文件不可读时报告路径与影响；版本冲突时不选边，交 continuity-check；任务超出本域时发送 TASK-REQUEST 给对应 Responsible；无法访问媒体、模型或外部事实时写“未验证”，不声称调用成功。

## 与其他Skill的边界

剧本问题交 screenwriter；分镜问题交 director；最终判断交 quality-control-editor。审查 Skill 只提供证据和修复方向，不覆盖主稿；制作风险 Skill 不替代创作决定。

## 上游输入和下游交接

用 TASK-REQUEST 接收版本化输入；产出 Artifact、TASK-RESULT 和 append-only Event。Participant 只能回报局部完成；所有项目级结果回到 film_project_lead。

## Human介入条件

核心创意互斥、主题/结局/主角重大变化、品牌/产品事实缺失、范围/时长/预算变化、锁定、明显制作风险、对外发布或无法裁定的权威冲突。

## 示例

输入应包含可定位版本和硬约束。输出先列已确认事实与风险，再按本域字段交付；若版本未锁定，只给预检/草案并标明不能进入下游的原因。

## 最小测试用例

见 [`evals/cases/continuity-check/`](../../../evals/cases/continuity-check/)：3 正向、2 不触发、2 边界、1 输入不足、1 相邻冲突。所有案例必须由独立审查流程评估，不由本 Skill 自评。

## 版本信息

- schema_version：`3.0`
- content_version：`3.0.0`
- 状态：`ACTIVE`

## 参考资料

- [timeline-ledger.md](references/timeline-ledger.md)
- [character-state-ledger.md](references/character-state-ledger.md)
- [costume-prop-ledger.md](references/costume-prop-ledger.md)
- [location-continuity.md](references/location-continuity.md)
- [knowledge-state.md](references/knowledge-state.md)
- [version-diff.md](references/version-diff.md)
- [continuity-report.md](references/continuity-report.md)
- [cross-media-continuity.md](references/cross-media-continuity.md)
