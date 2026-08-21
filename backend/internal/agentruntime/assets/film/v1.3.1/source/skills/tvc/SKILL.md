---
name: tvc
description: 用户提供品牌、产品或客户 Brief 并需要 TVC 策略与创意时使用；不用于虚构产品事实或直接出详细分镜。
---

# TVC策划与创意

## 适用场景

用户提供品牌、产品或客户 Brief 并需要 TVC 策略与创意时使用。本 Skill 只负责本域的可追溯交付，不把相邻任务静默吞并。

## 不适用场景

故事化剧本交 screenwriter；镜头交 director-storyboard；AI风险交 ai-production-feasibility-review。输入只有模糊意图而没有可定位材料时，先执行“输入不足处理”。

## 必需输入

客户 Brief、品牌/产品事实、受众、传播目标、禁用表述、媒介和时长。

## 可选输入

目标画幅、预算/工期、参考作品的高层特征、历史审查报告、已批准的 CHANGE-REQUEST。参考只可影响抽象方向，不可复制具体表达。

## 输入不足处理

列出缺失字段、影响和最小补充问题。核心创意、品牌事实、范围、LOCKED 内容或权威冲突必须写入 `NEEDS-YOU.md`；其余可逆假设须以“暂定”标记并进入输出风险表。

## 锁定事实与禁止修改项

以项目 `PROJECT-BRIEF.md`、`CURRENT-BRIEF.md`、`DECISIONS.md`、各 Bible 和 LOCKED Artifact 为准。不得以文件名或修改时间判定权威；LOCKED 产物只能通过 CHANGE-REQUEST 创建新的 `object_version`，绝不原地改写。

## 完整分阶段工作流

1. 将产品事实、证据等级、品牌禁区和未确认项拆表
2. 界定产品型、情绪型、故事型、形象片或社媒短广告；不明时提出最小问题
3. 提炼一个可验证的单一核心信息与 Reason to Believe，不把愿望写成事实
4. 生成至少三个机制不同的概念并比较受众相关性、品牌可见性、原创性和制作约束；评分不能覆盖硬门槛
5. 确定产品/品牌/CTA 时间点；故事型先交编剧，再由本 Skill 品牌复核，并按十阶段将后续工作路由给对应专业域

## AI TVC v2.1 专业融合

继承十阶段工作流、创意评分、变更影响、广告事实台账、品类/合规/AI 风险和不可信输入防护；它们被拆为可按需读取的参考，而不是把 TVC Agent 变成编剧、导演、声音、可行性和 QC 的替身。详见 [tvc-ten-stage-workflow.md](references/tvc-ten-stage-workflow.md) 与 [tvc-risk-and-governance.md](references/tvc-risk-and-governance.md)。

## 阶段门禁

- Gate A：Responsible、目标、输入版本和验收标准齐全。
- Gate B：已确认/暂定/冲突事实分离；未解决权威冲突已升级。
- Gate C：字段完整、质量表通过、相邻域未越权，才能交接。
- Gate D：锁定、品牌、范围、不可逆操作或对外发布必须等待 Human。

## 输出契约

必须输出：Brief 审计、受众洞察、单一核心信息、RTB、至少三项概念比较、评分/硬门槛、露出/CTA、合规清单、变更影响（如适用）和 TVC-CREATIVE-HANDOFF。

正式 Artifact frontmatter 必须含 `artifact_id`、`artifact_type`、`schema_version`、`object_version`、`status`、`responsible`、`related_topic_id`、`parent_artifact_id`、`created_at`。本域字段：`fact_ledger`、`audience_tension`、`single_minded_proposition`、`rtb`、`concept_options`、`exposure_plan`、`cta`、`compliance_risk`。

## 质量检查表

- 每个功能/数据有 Brief 来源；品牌不被故事吞没；露出与 CTA 可定位；不把竞品或法规猜测当事实；评分不覆盖事实、合规、范围或 Human 门禁。
- 所有结论能回指输入、权威事实或明确的暂定假设。
- 风险、限制、下一步与交接对象明确；没有“最终版”之类无版本断言。

## 错误与异常处理

文件不可读时报告路径与影响；版本冲突时不选边，交 continuity-check；任务超出本域时发送 TASK-REQUEST 给对应 Responsible；无法访问媒体、模型或外部事实时写“未验证”，不声称调用成功。

## 与其他Skill的边界

故事化剧本交 screenwriter；镜头交 director-storyboard；AI风险交 ai-production-feasibility-review。审查 Skill 只提供证据和修复方向，不覆盖主稿；制作风险 Skill 不替代创作决定。

## 上游输入和下游交接

用 TASK-REQUEST 接收版本化输入；产出 Artifact、TASK-RESULT 和 append-only Event。Participant 只能回报局部完成；所有项目级结果回到 film_project_lead。

## Human介入条件

核心创意互斥、主题/结局/主角重大变化、品牌/产品事实缺失、范围/时长/预算变化、锁定、明显制作风险、对外发布或无法裁定的权威冲突。

## 示例

输入应包含可定位版本和硬约束。输出先列已确认事实与风险，再按本域字段交付；若版本未锁定，只给预检/草案并标明不能进入下游的原因。

## 最小测试用例

见 [`evals/cases/tvc/`](../../../evals/cases/tvc/)：3 正向、2 不触发、2 边界、1 输入不足、1 相邻冲突。所有案例必须由独立审查流程评估，不由本 Skill 自评。

## 版本信息

- schema_version：`2.0`
- content_version：`2.0.0`
- 状态：`ACTIVE`

## 参考资料

- [client-brief.md](references/client-brief.md)
- [brand-product-analysis.md](references/brand-product-analysis.md)
- [audience-insight.md](references/audience-insight.md)
- [single-minded-proposition.md](references/single-minded-proposition.md)
- [concept-development.md](references/concept-development.md)
- [product-brand-exposure.md](references/product-brand-exposure.md)
- [tvc-duration-structures.md](references/tvc-duration-structures.md)
- [compliance-risk.md](references/compliance-risk.md)
- [creative-presentation.md](references/creative-presentation.md)
- [tvc-ten-stage-workflow.md](references/tvc-ten-stage-workflow.md)
- [tvc-risk-and-governance.md](references/tvc-risk-and-governance.md)
