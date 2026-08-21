---
name: ai-production-feasibility-review
description: 制作前需判断 AI 图像/视频工作流的风险、拆分和降级方案时使用；不用于无依据承诺成本或模型能力。
---

# AI制作可行性审查

## 适用场景

制作前需判断 AI 图像/视频工作流的风险、拆分和降级方案时使用。本 Skill 只负责本域的可追溯交付，不把相邻任务静默吞并。

## 不适用场景

镜头意图归 director；提示词归 ai-video-shot-prompt；最终验收归 QC。输入只有模糊意图而没有可定位材料时，先执行“输入不足处理”。

## 必需输入

剧本/分镜、资产方案、已知模型信息、工期和后期能力。

## 可选输入

目标画幅、预算/工期、参考作品的高层特征、历史审查报告、已批准的 CHANGE-REQUEST。参考只可影响抽象方向，不可复制具体表达。

## 输入不足处理

列出缺失字段、影响和最小补充问题。核心创意、品牌事实、范围、LOCKED 内容或权威冲突必须写入 `NEEDS-YOU.md`；其余可逆假设须以“暂定”标记并进入输出风险表。

## 锁定事实与禁止修改项

以项目 `PROJECT-BRIEF.md`、`CURRENT-BRIEF.md`、`DECISIONS.md`、各 Bible 和 LOCKED Artifact 为准。不得以文件名或修改时间判定权威；LOCKED 产物只能通过 CHANGE-REQUEST 创建新的 `object_version`，绝不原地改写。

## 完整分阶段工作流

1. 将场次拆成角色、环境、动作、交互、文字和镜头运动风险
2. 按已知模型证据、项目资产与未知项分级；记录能力证据来源和日期，不报精确价格或未经运行的成功率
3. 识别角色一致性、手部、多人、反射、屏幕和特效的失败模式
4. 为每个中高风险镜头给预资产、拆分、后期和叙事保真降级；区分“提示词已写”“计划尝试”“有结果媒体”三种事实
5. 标出阻断项与需要 Human 接受的风险

## 阶段门禁

- Gate A：Responsible、目标、输入版本和验收标准齐全。
- Gate B：已确认/暂定/冲突事实分离；未解决权威冲突已升级。
- Gate C：字段完整、质量表通过、相邻域未越权，才能交接。
- Gate D：锁定、品牌、范围、不可逆操作或对外发布必须等待 Human。

## 输出契约

必须输出：镜头风险矩阵、前置资产、生成/后期边界、拆分简化、重试等级、阻断项和 AI-PRODUCTION-HANDOFF。

正式 Artifact frontmatter 必须含 `artifact_id`、`artifact_type`、`schema_version`、`object_version`、`status`、`responsible`、`related_topic_id`、`parent_artifact_id`、`created_at`。本域字段：`shot_risk`、`risk_reason`、`asset_prerequisite`、`generation_method`、`post_method`、`split_plan`、`fallback`、`retry_level`、`blocker`。

## Production System V2 集成

以 `Shot Package`、`Prompt Manifest`、Reference Lock 和 `Asset/Scene Bible` 生成逐镜头 `Shot Feasibility Report`。逐项标出角色漂移、手部/道具交互、多人、空间/反射、屏幕 UI、长运镜、口型和声音风险，并给不改变核心剧情的拆镜、预资产、后期或叙事保真降级。结论只能是可验证的计划与风险；没有已运行模型或媒体证据时，`execution_state` 必须为 NOT_EXECUTED。

以 [`PRODUCTION-SYSTEM-V2.md`](../../../contracts/PRODUCTION-SYSTEM-V2.md) 的可行性、降级与执行证据边界为准。

## 质量检查表

- 不只给可/不可；高风险有替代；未知模型能力标注未知；降级不偷偷改核心剧情。
- 所有结论能回指输入、权威事实或明确的暂定假设。
- 风险、限制、下一步与交接对象明确；没有“最终版”之类无版本断言。

## 错误与异常处理

文件不可读时报告路径与影响；版本冲突时不选边，交 continuity-check；任务超出本域时发送 TASK-REQUEST 给对应 Responsible；无法访问媒体、模型或外部事实时写“未验证”，不声称调用成功。

## 与其他Skill的边界

镜头意图归 director；提示词归 ai-video-shot-prompt；最终验收归 QC。审查 Skill 只提供证据和修复方向，不覆盖主稿；制作风险 Skill 不替代创作决定。

## 上游输入和下游交接

用 TASK-REQUEST 接收版本化输入；产出 Artifact、TASK-RESULT 和 append-only Event。Participant 只能回报局部完成；所有项目级结果回到 film_project_lead。

## Human介入条件

核心创意互斥、主题/结局/主角重大变化、品牌/产品事实缺失、范围/时长/预算变化、锁定、明显制作风险、对外发布或无法裁定的权威冲突。

## 示例

输入应包含可定位版本和硬约束。输出先列已确认事实与风险，再按本域字段交付；若版本未锁定，只给预检/草案并标明不能进入下游的原因。

## 最小测试用例

见 [`evals/cases/ai-production-feasibility-review/`](../../../evals/cases/ai-production-feasibility-review/)：3 正向、2 不触发、2 边界、1 输入不足、1 相邻冲突。所有案例必须由独立审查流程评估，不由本 Skill 自评。

## 版本信息

- schema_version：`3.0`
- content_version：`3.0.0`
- 状态：`ACTIVE`

## 参考资料

- [risk-matrix.md](references/risk-matrix.md)
- [character-consistency.md](references/character-consistency.md)
- [action-interaction.md](references/action-interaction.md)
- [text-screen-reflection.md](references/text-screen-reflection.md)
- [generation-vs-post.md](references/generation-vs-post.md)
- [asset-reuse.md](references/asset-reuse.md)
- [fallback-strategies.md](references/fallback-strategies.md)
- [attempt-result-evidence.md](references/attempt-result-evidence.md)
