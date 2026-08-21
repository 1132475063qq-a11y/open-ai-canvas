---
name: director-storyboard
description: 锁定或接近锁定的剧本需要转为可拍摄分镜和镜头清单时使用；不用于重写核心剧情或模型专属提示词。
---

# 导演分镜

## 适用场景

锁定或接近锁定的剧本需要转为可拍摄分镜和镜头清单时使用。本 Skill 只负责本域的可追溯交付，不把相邻任务静默吞并。

## 不适用场景

角色/场景资产交 visual；声音交 sound-design；模型提示词交 ai-video-shot-prompt。输入只有模糊意图而没有可定位材料时，先执行“输入不足处理”。

## 必需输入

LOCKED/REVIEW 剧本、角色/场景/道具 Bible、画幅、制作限制和声音约束。

## 可选输入

目标画幅、预算/工期、参考作品的高层特征、历史审查报告、已批准的 CHANGE-REQUEST。参考只可影响抽象方向，不可复制具体表达。

## 输入不足处理

列出缺失字段、影响和最小补充问题。核心创意、品牌事实、范围、LOCKED 内容或权威冲突必须写入 `NEEDS-YOU.md`；其余可逆假设须以“暂定”标记并进入输出风险表。

## 锁定事实与禁止修改项

以项目 `PROJECT-BRIEF.md`、`CURRENT-BRIEF.md`、`DECISIONS.md`、各 Bible 和 LOCKED Artifact 为准。不得以文件名或修改时间判定权威；LOCKED 产物只能通过 CHANGE-REQUEST 创建新的 `object_version`，绝不原地改写。

## 完整分阶段工作流

1. 按场次提取戏剧目的、观众已知/将知/被隐藏的信息、情绪的可见证据和不可改剧情事实
2. 建立空间平面、轴线、出入口、视线和动作起终点；先决定角色/道具如何在画面内可见
3. 为每个信息变化选择景别、机位、构图、调度和镜头时长，并写明这种选择而非另一种选择的叙事理由
4. 为每一个静止或运动镜头写 `movement_reason`；把氛围分解为光、色、距离、遮挡、表演、声音和剪辑关系，避免无目的炫技
5. 安排镜头间信息关系、转场、声画关系和节奏；逐项回读剧本，标记建议性结构改动后再交视觉/声音/AI

## 阶段门禁

- Gate A：Responsible、目标、输入版本和验收标准齐全。
- Gate B：已确认/暂定/冲突事实分离；未解决权威冲突已升级。
- Gate C：字段完整、质量表通过、相邻域未越权，才能交接。
- Gate D：锁定、品牌、范围、不可逆操作或对外发布必须等待 Human。

## 输出契约

必须输出：镜头清单、分镜说明、空间锚点、连续性备注和 STORYBOARD-HANDOFF。

V1.2 每镜还必须有 `dramatic_purpose`、`viewer_knows_before`、`viewer_learns_or_feels_after`、`withheld_information`、`visible_evidence`、`movement_reason`、`atmosphere_evidence`；使用 `SHOT-DECISION-SHEET.md` 的字段，不以“电影感/压迫感/推近”取代决定。

正式 Artifact frontmatter 必须含 `artifact_id`、`artifact_type`、`schema_version`、`object_version`、`status`、`responsible`、`related_topic_id`、`parent_artifact_id`、`created_at`。本域字段：`scene`、`shot_id`、`timecode`、`duration`、`purpose`、`size_angle`、`composition`、`blocking`、`movement`、`space_anchor`、`dialogue_sound`、`transition`、`continuity_note`。

## Production System V2 集成

每个可制作镜头必须输出 `Shot Contract`，明确 scene_id、sequence_order、画幅、景别、机位、焦段/镜头语言、运镜、camera_slot、start/end blocking、screen_direction、action_axis、可见资产状态、UI/声音提示和镜头间转场理由。镜头意图是导演事实；模型专属语法留给提示词编译。不得用“电影感”“压迫感”替代可见画面、调度和运镜。

以 [`PRODUCTION-SYSTEM-V2.md`](../../../contracts/PRODUCTION-SYSTEM-V2.md) 的 Shot Contract、Camera Map 与动作轴线规则为准。

## Creative Depth V1.3 集成

为可制作镜头交付 `Observable Image Specification`：画幅/时长、可见空间锚点与光线、每个角色的身份特征/状态/位置/朝向/动作、关键道具的几何/位置/状态、相机位置/景别/角度/焦段/构图、动作时间段、声画/UI、可见氛围证据、验收点和最小返工范围。把“压迫、温暖、危险”等抽象词拆成光、色、距离/遮挡、表演、声音和调度；每次运镜写开始/停止时机与叙事理由。

详见 [`CREATIVE-DEPTH-V1.3.md`](../../../contracts/CREATIVE-DEPTH-V1.3.md)。

## ACTING V1.3 集成

分镜需要人物可演性时，director-storyboard 消费 `character-acting-system` 的 scene adaptation，把 behavior beat 与 blocking、distance、gaze、physical business 对齐。导演拥有镜头与调度，ACTING 拥有表演行为；任何表演建议若会改变角色目标、关键台词或关系，必须返回 screenwriter/Human。

## 质量检查表

- 镜头总时长与剧本匹配；轴线/视线可执行；每镜有叙事目的；未混入模型提示词；不改锁定人物动机。
- 所有结论能回指输入、权威事实或明确的暂定假设。
- 风险、限制、下一步与交接对象明确；没有“最终版”之类无版本断言。

## 错误与异常处理

文件不可读时报告路径与影响；版本冲突时不选边，交 continuity-check；任务超出本域时发送 TASK-REQUEST 给对应 Responsible；无法访问媒体、模型或外部事实时写“未验证”，不声称调用成功。

## 与其他Skill的边界

角色/场景资产交 visual；声音交 sound-design；模型提示词交 ai-video-shot-prompt。审查 Skill 只提供证据和修复方向，不覆盖主稿；制作风险 Skill 不替代创作决定。

## 上游输入和下游交接

用 TASK-REQUEST 接收版本化输入；产出 Artifact、TASK-RESULT 和 append-only Event。Participant 只能回报局部完成；所有项目级结果回到 film_project_lead。

## Human介入条件

核心创意互斥、主题/结局/主角重大变化、品牌/产品事实缺失、范围/时长/预算变化、锁定、明显制作风险、对外发布或无法裁定的权威冲突。

## 示例

输入应包含可定位版本和硬约束。输出先列已确认事实与风险，再按本域字段交付；若版本未锁定，只给预检/草案并标明不能进入下游的原因。

## 最小测试用例

见 [`evals/cases/director-storyboard/`](../../../evals/cases/director-storyboard/)：3 正向、2 不触发、2 边界、1 输入不足、1 相邻冲突。所有案例必须由独立审查流程评估，不由本 Skill 自评。

## 版本信息

- schema_version：`3.2`
- content_version：`3.2.0`
- 状态：`ACTIVE`

## 参考资料

- [shot-purpose.md](references/shot-purpose.md)
- [shot-decision-method.md](references/shot-decision-method.md)
- [shot-size-angle.md](references/shot-size-angle.md)
- [composition-blocking.md](references/composition-blocking.md)
- [axis-eyeline.md](references/axis-eyeline.md)
- [camera-movement.md](references/camera-movement.md)
- [editing-rhythm.md](references/editing-rhythm.md)
- [transitions.md](references/transitions.md)
- [storyboard-format.md](references/storyboard-format.md)
- [vertical-9x16.md](references/vertical-9x16.md)
- [action-scene-coverage.md](references/action-scene-coverage.md)
