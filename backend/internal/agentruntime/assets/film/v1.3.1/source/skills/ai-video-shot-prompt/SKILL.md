---
name: ai-video-shot-prompt
description: 导演分镜需转为可独立生成的 AI 视频镜头提示词时使用；不用于改导演意图或写跨段依赖文学描述。
---

# AI视频镜头提示词

## 适用场景

导演分镜需转为可独立生成的 AI 视频镜头提示词时使用。本 Skill 只负责本域的可追溯交付，不把相邻任务静默吞并。

## 不适用场景

镜头意图归 director-storyboard；风险归 feasibility；失败成片诊断归 short-drama-video-prompt-doctor。输入只有模糊意图而没有可定位材料时，先执行“输入不足处理”。

## 必需输入

分镜、角色/场景/道具锚点、模型 profile、画幅时长和声音规则。

## 可选输入

目标画幅、预算/工期、参考作品的高层特征、历史审查报告、已批准的 CHANGE-REQUEST。参考只可影响抽象方向，不可复制具体表达。

## 输入不足处理

列出缺失字段、影响和最小补充问题。核心创意、品牌事实、范围、LOCKED 内容或权威冲突必须写入 `NEEDS-YOU.md`；其余可逆假设须以“暂定”标记并进入输出风险表。

## 锁定事实与禁止修改项

以项目 `PROJECT-BRIEF.md`、`CURRENT-BRIEF.md`、`DECISIONS.md`、各 Bible 和 LOCKED Artifact 为准。不得以文件名或修改时间判定权威；LOCKED 产物只能通过 CHANGE-REQUEST 创建新的 `object_version`，绝不原地改写。

## 完整分阶段工作流

1. 读取 ShotDecision 的目的、观众信息和不可改视觉锚点；未知模型能力标为未验证
2. 为每段重复写完整可见人物（外形、服装状态、姿态）、场景、空间关系、道具和起始动作状态
3. 将抽象情绪编译为可见行为、构图、光线、声音和时间轴；按时间段组织单一动作链与镜头运动
4. 加入禁止项、独立验收点和锚点来源；禁止“同上、继续、承接上一段、那个男人”等跨镜依赖
5. 按手部、多人互动、文字、反射和复杂运动分级，给出拆分/降级；为每版提示词预留 Attempt/Result/QC/Retry 引用而不声称已调用模型

## 阶段门禁

- Gate A：Responsible、目标、输入版本和验收标准齐全。
- Gate B：已确认/暂定/冲突事实分离；未解决权威冲突已升级。
- Gate C：字段完整、质量表通过、相邻域未越权，才能交接。
- Gate D：锁定、品牌、范围、不可逆操作或对外发布必须等待 Human。

## 输出契约

必须输出：逐镜头独立提示词、**内部动态保护约束台账**、连续性检查点、失败风险、重试与降级方案。内部保护约束不等于最终模型 Prompt 的负面词块。

每镜 V1.2 还必须输出 `prompt_version`、`source_shot_decision_ref`、`anchor_refs`、`visible_start_state`、`timed_action`、`camera_reason`、`observable_atmosphere`、`acceptance_checks` 与 `generation_chain_ref`。提示词采用 `AI-SHOT-PROMPT-SPEC.md`，不是跨镜文学描述。

正式 Artifact frontmatter 必须含 `artifact_id`、`artifact_type`、`schema_version`、`object_version`、`status`、`responsible`、`related_topic_id`、`parent_artifact_id`、`created_at`。本域字段：`shot_id`、`generation_context`、`timed_action`、`camera`、`anchors`、`dynamic_negative_constraints`（内部保护台账）、`positive_locks`、`emitted_negative_constraints`、`sound`、`continuity_checks`、`risk`、`fallback`。

## Production System V2 集成

从已通过的 `Shot Package` 编译独立 `Prompt Manifest`：每条提示词写清看得见的角色外形/服装状态、空间锚点、物体、动作、景别机位、构图、光线、运镜、时间与声音/UI 条件；不得引用“同上”“上一镜头”。`dynamic_negative_constraints` 必须由角色保护特征、道具几何、空间禁变项、屏幕方向和 Reference Lock 自动继承，但它是内部机器可读保护台账。最终 Prompt 编译时优先改写为 positive locks / local locks；只有无法正向表达的已知失败模式才进入 `emitted_negative_constraints`。提示词只是计划，未产生可访问媒体前不得写生成成功。

以 [`PRODUCTION-SYSTEM-V2.md`](../../../contracts/PRODUCTION-SYSTEM-V2.md) 的 Shot Package、Prompt Manifest 与动态负向约束规则为准。

## Creative Depth V1.3 集成

将导演的 `Observable Image Specification` 保留在 Prompt Manifest 中，再编译正向、负向、图像、视频、编辑和动画提示词。每段先写可见环境/锚点，再写角色身份与状态、道具几何和位置、按秒动作、相机、可观察光色/遮挡/声音；不以“高级、电影感、压迫”替代事实。输出必须同时保留结构化验收点和最小返工范围；不能生成可访问媒体时仍为 `NOT_EXECUTED`。

详见 [`CREATIVE-DEPTH-V1.3.md`](../../../contracts/CREATIVE-DEPTH-V1.3.md)。

## CINEDANCE / ACTING V1.3.1 集成

当目标是 Seedance 2.0 / Higgsfield Seedance 且 Shot Contract 已锁定时，`ai-video-shot-prompt` 作为总入口调用 `cinedance-video-director` 的专业编译顺序：active references → location map → first-frame blocking → optics → camera → action timing → physics → lighting → audio。人物表演不再用抽象情绪填充，优先消费 `character-acting-system` 的 `scene-acting-adaptation`。如果目标模型不是 Seedance/Higgsfield，保留相同的空间/行为原则但回退通用模型 profile。

三个 Skill 都只能生成计划/Prompt Artifact；没有真实 Attempt/Result/Event 与可访问媒体时，不得声称模型调用或成片成功。约束编译统一遵守 [`SKILL-AUTHORITY-PRECEDENCE-V1.3.1.md`](../../../contracts/SKILL-AUTHORITY-PRECEDENCE-V1.3.1.md)：数据层保留完整保护台账，模型 Prompt 不默认复制 NOT-stack。

## 质量检查表

- 每段脱离前段仍可理解；动作数量适合时长；角色/服装/空间不遗漏；没有模型未验证规格伪装为事实。
- 所有结论能回指输入、权威事实或明确的暂定假设。
- 风险、限制、下一步与交接对象明确；没有“最终版”之类无版本断言。

## 错误与异常处理

文件不可读时报告路径与影响；版本冲突时不选边，交 continuity-check；任务超出本域时发送 TASK-REQUEST 给对应 Responsible；无法访问媒体、模型或外部事实时写“未验证”，不声称调用成功。

## 与其他Skill的边界

镜头意图归 director-storyboard；风险归 feasibility；失败成片诊断归 short-drama-video-prompt-doctor。审查 Skill 只提供证据和修复方向，不覆盖主稿；制作风险 Skill 不替代创作决定。

## 上游输入和下游交接

用 TASK-REQUEST 接收版本化输入；产出 Artifact、TASK-RESULT 和 append-only Event。Participant 只能回报局部完成；所有项目级结果回到 film_project_lead。

## Human介入条件

核心创意互斥、主题/结局/主角重大变化、品牌/产品事实缺失、范围/时长/预算变化、锁定、明显制作风险、对外发布或无法裁定的权威冲突。

## 示例

输入应包含可定位版本和硬约束。输出先列已确认事实与风险，再按本域字段交付；若版本未锁定，只给预检/草案并标明不能进入下游的原因。

## 最小测试用例

见 [`evals/cases/ai-video-shot-prompt/`](../../../evals/cases/ai-video-shot-prompt/)：3 正向、2 不触发、2 边界、1 输入不足、1 相邻冲突。所有案例必须由独立审查流程评估，不由本 Skill 自评。

## 版本信息

- schema_version：`3.2`
- content_version：`3.2.1`
- 状态：`ACTIVE`

## 参考资料

- [model-profiles/generic-video-model.md](references/model-profiles/generic-video-model.md)
- [model-profiles/seedance-user-profile.md](references/model-profiles/seedance-user-profile.md)
- [independent-shot-contract.md](references/independent-shot-contract.md)
- [prompt-generation-loop.md](references/prompt-generation-loop.md)
