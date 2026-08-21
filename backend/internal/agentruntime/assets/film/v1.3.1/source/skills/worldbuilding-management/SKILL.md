---
name: worldbuilding-management
description: 项目需要建立或变更世界规则、制度、历史和角色知识边界时使用；不用于为单场戏临时补万能设定。
---

# 世界观与设定管理

## 适用场景

项目需要建立或变更世界规则、制度、历史和角色知识边界时使用。本 Skill 只负责本域的可追溯交付，不把相邻任务静默吞并。

## 不适用场景

剧情执行交 screenwriter；视觉解释交 visual；冲突交 continuity。输入只有模糊意图而没有可定位材料时，先执行“输入不足处理”。

## 必需输入

故事概念、已有 Bible、冲突点、变更请求和锁定决定。

## 可选输入

目标画幅、预算/工期、参考作品的高层特征、历史审查报告、已批准的 CHANGE-REQUEST。参考只可影响抽象方向，不可复制具体表达。

## 输入不足处理

列出缺失字段、影响和最小补充问题。核心创意、品牌事实、范围、LOCKED 内容或权威冲突必须写入 `NEEDS-YOU.md`；其余可逆假设须以“暂定”标记并进入输出风险表。

## 锁定事实与禁止修改项

以项目 `PROJECT-BRIEF.md`、`CURRENT-BRIEF.md`、`DECISIONS.md`、各 Bible 和 LOCKED Artifact 为准。不得以文件名或修改时间判定权威；LOCKED 产物只能通过 CHANGE-REQUEST 创建新的 `object_version`，绝不原地改写。

## 完整分阶段工作流

1. 提出核心世界假设与剧情用途
2. 定义规则、代价、限制、例外及观众获知顺序
3. 补足制度、历史、地理和信息传播到剧情所需深度
4. 以 `canon_fact` 维护角色知识边界与正史来源：每条可核对事实带 evidence、authority_level、known_by、change_policy 与影响引用
5. 审核变更影响，未批准不写入 canon

## 阶段门禁

- Gate A：Responsible、目标、输入版本和验收标准齐全。
- Gate B：已确认/暂定/冲突事实分离；未解决权威冲突已升级。
- Gate C：字段完整、质量表通过、相邻域未越权，才能交接。
- Gate D：锁定、品牌、范围、不可逆操作或对外发布必须等待 Human。

## 输出契约

必须输出：可追溯 STORY-BIBLE、规则/代价/限制、知识表、冲突报告和 CHANGE-REQUEST。

V1.2 Canon/Bible 不以无来源散文取代事实记录；没有权威的设定应为 `UNKNOWN` / WORKING，不得写成 LOCKED。

正式 Artifact frontmatter 必须含 `artifact_id`、`artifact_type`、`schema_version`、`object_version`、`status`、`responsible`、`related_topic_id`、`parent_artifact_id`、`created_at`。本域字段：`premise`、`rule`、`cost`、`limit`、`knowledge_boundary`、`canon_source`、`change_impact`。

## 质量检查表

- 每条设定有剧情用途和边界；无万能补丁；推测不进入正史。
- 所有结论能回指输入、权威事实或明确的暂定假设。
- 风险、限制、下一步与交接对象明确；没有“最终版”之类无版本断言。

## 错误与异常处理

文件不可读时报告路径与影响；版本冲突时不选边，交 continuity-check；任务超出本域时发送 TASK-REQUEST 给对应 Responsible；无法访问媒体、模型或外部事实时写“未验证”，不声称调用成功。

## 与其他Skill的边界

剧情执行交 screenwriter；视觉解释交 visual；冲突交 continuity。审查 Skill 只提供证据和修复方向，不覆盖主稿；制作风险 Skill 不替代创作决定。

## 上游输入和下游交接

用 TASK-REQUEST 接收版本化输入；产出 Artifact、TASK-RESULT 和 append-only Event。Participant 只能回报局部完成；所有项目级结果回到 film_project_lead。

## Human介入条件

核心创意互斥、主题/结局/主角重大变化、品牌/产品事实缺失、范围/时长/预算变化、锁定、明显制作风险、对外发布或无法裁定的权威冲突。

## 示例

输入应包含可定位版本和硬约束。输出先列已确认事实与风险，再按本域字段交付；若版本未锁定，只给预检/草案并标明不能进入下游的原因。

## 最小测试用例

见 [`evals/cases/worldbuilding-management/`](../../../evals/cases/worldbuilding-management/)：3 正向、2 不触发、2 边界、1 输入不足、1 相邻冲突。所有案例必须由独立审查流程评估，不由本 Skill 自评。

## 版本信息

- schema_version：`2.0`
- content_version：`2.0.0`
- 状态：`ACTIVE`

## 参考资料

- [world-premise.md](references/world-premise.md)
- [rules-costs-limits.md](references/rules-costs-limits.md)
- [institutions-society.md](references/institutions-society.md)
- [technology-magic-systems.md](references/technology-magic-systems.md)
- [knowledge-boundaries.md](references/knowledge-boundaries.md)
- [canon-change-control.md](references/canon-change-control.md)
- [story-bible.md](references/story-bible.md)
- [canon-fact-contract.md](references/canon-fact-contract.md)
