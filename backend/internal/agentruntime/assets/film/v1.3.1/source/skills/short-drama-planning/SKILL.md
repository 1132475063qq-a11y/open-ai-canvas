---
name: short-drama-planning
description: 用户要把概念、小说或大纲规划为连续短剧和分集结构时使用；不用于逐场完整剧本或镜头设计。
---

# 短剧策划与分集

## 适用场景

用户要把概念、小说或大纲规划为连续短剧和分集结构时使用。本 Skill 只负责本域的可追溯交付，不把相邻任务静默吞并。

## 不适用场景

单集写作交 screenwriter；世界规则交 worldbuilding-management；跨集矛盾交 continuity-check。输入只有模糊意图而没有可定位材料时，先执行“输入不足处理”。

## 必需输入

原作或可靠梗概、版权/改编边界、目标平台假设、总集数、单集时长和已锁定设定。

## 可选输入

目标画幅、预算/工期、参考作品的高层特征、历史审查报告、已批准的 CHANGE-REQUEST。参考只可影响抽象方向，不可复制具体表达。

## 输入不足处理

列出缺失字段、影响和最小补充问题。核心创意、品牌事实、范围、LOCKED 内容或权威冲突必须写入 `NEEDS-YOU.md`；其余可逆假设须以“暂定”标记并进入输出风险表。

## 锁定事实与禁止修改项

以项目 `PROJECT-BRIEF.md`、`CURRENT-BRIEF.md`、`DECISIONS.md`、各 Bible 和 LOCKED Artifact 为准。不得以文件名或修改时间判定权威；LOCKED 产物只能通过 CHANGE-REQUEST 创建新的 `object_version`，绝不原地改写。

## 完整分阶段工作流

1. 标注原作不可替代价值、可压缩材料、授权/事实不确定项
2. 接收或委托 `story-type-profile`，定义可重复但会升级的系列引擎、每集目标、人物选择和失败代价
3. 按阶段安排主线、副线、关系变化与信息释放，不以反转清单代替因果；每集标注类型压力如何改变人物状态
4. 逐集写开场抓力、中段升级、结尾钩子、回收和下一集承诺，并注明钩子带来的可兑现后果
5. 核对集间知识、时间、道具、人物状态，向编剧交付可写的单集任务

## 阶段门禁

- Gate A：Responsible、目标、输入版本和验收标准齐全。
- Gate B：已确认/暂定/冲突事实分离；未解决权威冲突已升级。
- Gate C：字段完整、质量表通过、相邻域未越权，才能交接。
- Gate D：锁定、品牌、范围、不可逆操作或对外发布必须等待 Human。

## 输出契约

必须输出：改编策略、故事类型 Profile、系列引擎、角色线、阶段结构、逐集选择/后果表、钩子/回收表、风险和 SERIES-HANDOFF。

正式 Artifact frontmatter 必须含 `artifact_id`、`artifact_type`、`schema_version`、`object_version`、`status`、`responsible`、`related_topic_id`、`parent_artifact_id`、`created_at`。本域字段：`adaptation_strategy`、`story_type_profile_ref`、`series_engine`、`episode_grid`、`episode_choice_consequence`、`hook_payoff_ledger`、`knowledge_ledger`、`series_handoff`。

## 质量检查表

- 每集有独立目标与变化；钩子机制不机械重复；平台规则未核验则仅为假设；不越权写完整场景。
- 所有结论能回指输入、权威事实或明确的暂定假设。
- 风险、限制、下一步与交接对象明确；没有“最终版”之类无版本断言。

## 错误与异常处理

文件不可读时报告路径与影响；版本冲突时不选边，交 continuity-check；任务超出本域时发送 TASK-REQUEST 给对应 Responsible；无法访问媒体、模型或外部事实时写“未验证”，不声称调用成功。

## 与其他Skill的边界

单集写作交 screenwriter；世界规则交 worldbuilding-management；跨集矛盾交 continuity-check。审查 Skill 只提供证据和修复方向，不覆盖主稿；制作风险 Skill 不替代创作决定。

## 上游输入和下游交接

用 TASK-REQUEST 接收版本化输入；产出 Artifact、TASK-RESULT 和 append-only Event。Participant 只能回报局部完成；所有项目级结果回到 film_project_lead。

## Human介入条件

核心创意互斥、主题/结局/主角重大变化、品牌/产品事实缺失、范围/时长/预算变化、锁定、明显制作风险、对外发布或无法裁定的权威冲突。

## 示例

输入应包含可定位版本和硬约束。输出先列已确认事实与风险，再按本域字段交付；若版本未锁定，只给预检/草案并标明不能进入下游的原因。

## 最小测试用例

见 [`evals/cases/short-drama-planning/`](../../../evals/cases/short-drama-planning/)：3 正向、2 不触发、2 边界、1 输入不足、1 相邻冲突。所有案例必须由独立审查流程评估，不由本 Skill 自评。

## 版本信息

- schema_version：`2.0`
- content_version：`2.0.0`
- 状态：`ACTIVE`

## 参考资料

- [adaptation-analysis.md](references/adaptation-analysis.md)
- [series-engine.md](references/series-engine.md)
- [story-type-adaptation.md](references/story-type-adaptation.md)
- [episode-architecture.md](references/episode-architecture.md)
- [hooks-and-payoffs.md](references/hooks-and-payoffs.md)
- [information-release.md](references/information-release.md)
- [season-arc.md](references/season-arc.md)
- [vertical-short-drama.md](references/vertical-short-drama.md)
- [episode-outline-template.md](references/episode-outline-template.md)
