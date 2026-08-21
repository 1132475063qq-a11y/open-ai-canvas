---
name: sound-design
description: 短剧、动画、TVC 或 AI 视频需要声音方案与交接时使用；不用于以模糊形容词代替可执行声音计划。
---

# 声音设计

## 适用场景

短剧、动画、TVC 或 AI 视频需要声音方案与交接时使用。本 Skill 只负责本域的可追溯交付，不把相邻任务静默吞并。

## 不适用场景

镜头节奏归 director；制作限制归 feasibility；成片复检归 final QC。输入只有模糊意图而没有可定位材料时，先执行“输入不足处理”。

## 必需输入

剧本/分镜、人物情绪、平台、语言/字幕、音乐许可和无 BGM 约束。

## 可选输入

目标画幅、预算/工期、参考作品的高层特征、历史审查报告、已批准的 CHANGE-REQUEST。参考只可影响抽象方向，不可复制具体表达。

## 输入不足处理

列出缺失字段、影响和最小补充问题。核心创意、品牌事实、范围、LOCKED 内容或权威冲突必须写入 `NEEDS-YOU.md`；其余可逆假设须以“暂定”标记并进入输出风险表。

## 锁定事实与禁止修改项

以项目 `PROJECT-BRIEF.md`、`CURRENT-BRIEF.md`、`DECISIONS.md`、各 Bible 和 LOCKED Artifact 为准。不得以文件名或修改时间判定权威；LOCKED 产物只能通过 CHANGE-REQUEST 创建新的 `object_version`，绝不原地改写。

## 完整分阶段工作流

1. 提取叙事重点、对白信息和移动端可懂度
2. 定义角色音色、节奏、情绪变化和录制限制
3. 逐镜安排环境、拟音、叙事音效、音乐或静默
4. 设计声画转场、对白优先级和响度风险
5. 输出 cue sheet 与无 BGM/版权边界
6. 将声音 Profile 与角色具身锚点对照：只校验角色 ID、状态、发声条件和行为证据一致，不从外貌反推声线。

## 阶段门禁

- Gate A：Responsible、目标、输入版本和验收标准齐全。
- Gate B：已确认/暂定/冲突事实分离；未解决权威冲突已升级。
- Gate C：字段完整、质量表通过、相邻域未越权，才能交接。
- Gate D：锁定、品牌、范围、不可逆操作或对外发布必须等待 Human。

## 输出契约

必须输出：角色音色 Brief、cue sheet、环境/拟音/音效/音乐方案、混音优先级和 SOUND-HANDOFF。

V1.2 的角色声音 Profile 必填 `pitch_register`、`timbre_texture`、`pace_articulation`、`breath_pause`、`emotional_range`、`lie_silence_pattern` 与可定位对白证据。

正式 Artifact frontmatter 必须含 `artifact_id`、`artifact_type`、`schema_version`、`object_version`、`status`、`responsible`、`related_topic_id`、`parent_artifact_id`、`created_at`。本域字段：`voice_profile`、`dialogue_priority`、`ambience`、`foley`、`sfx`、`music_policy`、`silence`、`mix_priority`。

## Production System V2 集成

将声音写成 `Audio Contract` 并挂到 Shot Contract：角色只按 character asset_id、状态、发声条件和可定位台词证据绑定，不从身高、眼镜或体型推导嗓音。逐镜写对白、环境、拟音、SFX、静默、音乐许可与混音优先级；声音状态变化进入连续性台账。无可访问音频时只能审查声音方案，不能声称混音或媒体质量通过。

以 [`PRODUCTION-SYSTEM-V2.md`](../../../contracts/PRODUCTION-SYSTEM-V2.md) 的 Audio Contract、身份/状态分离和媒体证据边界为准。

## 质量检查表

- 对白/拟音/环境/音乐分层；无 BGM 时未暗加音乐；不复制受版权作品。
- 所有结论能回指输入、权威事实或明确的暂定假设。
- 风险、限制、下一步与交接对象明确；没有“最终版”之类无版本断言。

## 错误与异常处理

文件不可读时报告路径与影响；版本冲突时不选边，交 continuity-check；任务超出本域时发送 TASK-REQUEST 给对应 Responsible；无法访问媒体、模型或外部事实时写“未验证”，不声称调用成功。

## 与其他Skill的边界

镜头节奏归 director；制作限制归 feasibility；成片复检归 final QC。审查 Skill 只提供证据和修复方向，不覆盖主稿；制作风险 Skill 不替代创作决定。

## 上游输入和下游交接

用 TASK-REQUEST 接收版本化输入；产出 Artifact、TASK-RESULT 和 append-only Event。Participant 只能回报局部完成；所有项目级结果回到 film_project_lead。

## Human介入条件

核心创意互斥、主题/结局/主角重大变化、品牌/产品事实缺失、范围/时长/预算变化、锁定、明显制作风险、对外发布或无法裁定的权威冲突。

## 示例

输入应包含可定位版本和硬约束。输出先列已确认事实与风险，再按本域字段交付；若版本未锁定，只给预检/草案并标明不能进入下游的原因。

## 最小测试用例

见 [`evals/cases/sound-design/`](../../../evals/cases/sound-design/)：3 正向、2 不触发、2 边界、1 输入不足、1 相邻冲突。所有案例必须由独立审查流程评估，不由本 Skill 自评。

## 版本信息

- schema_version：`3.0`
- content_version：`3.0.0`
- 状态：`ACTIVE`

## 参考资料

- [voice-profile.md](references/voice-profile.md)
- [embodiment-voice-link.md](references/embodiment-voice-link.md)
- [dialogue-direction.md](references/dialogue-direction.md)
- [ambience-foley.md](references/ambience-foley.md)
- [narrative-sfx.md](references/narrative-sfx.md)
- [music-policy.md](references/music-policy.md)
- [silence-transition.md](references/silence-transition.md)
- [mobile-mix.md](references/mobile-mix.md)
- [sound-cue-sheet.md](references/sound-cue-sheet.md)
