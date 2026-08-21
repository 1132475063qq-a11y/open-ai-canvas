---
name: character-acting-system
description: 角色表演需要从抽象情绪转成压力下的目标、策略、节拍、身体、眼神、听与反应时使用；产出长期表演档案与逐场适配。
---

# ACTING 角色表演系统

## 适用场景

用于角色表演设计、人物长期 acting master profile、逐场 scene adaptation、对白表演、眼神/呼吸/身体节奏、多人关系和 AI 视频 performance layer。核心原则是：**表演是压力下的行为，不是情绪展示**。

## 不适用场景

不用于决定角色外貌/服装、镜头焦段/构图、灯光调色、剧情大改、配乐设计或模型平台路由。没有人物目标/关系/场景事实时，不通过“愤怒、害怕、悲伤”等词自行补戏。

## 必需输入

角色因果/关系资料、当前场景目标、对手/对象、障碍与 stakes、场景动作/姿势、对白（如有）、角色长期行为/声线证据、当前身体状态。重复角色优先读取已有 `character-acting-profile`。

## 可选输入

super-objective、历史 scene adaptation、导演 blocking、声音方案、角色习惯/tic、软化对象、伤势/疲劳/醉酒等身体状态、多人空间距离。

## 输入不足处理

若 scene objective、对象、障碍或 stakes 缺失，先从锁定剧本/character-causal-bible 回读；仍缺失则请求 `narrative_screenwriter`。如果只有“他很生气/她很紧张”，把该词标为不可直接执行的状态描述，并要求或推导其可观察行为依据，但不改变剧情事实。

## 锁定事实与禁止修改项

角色的 super-objective、关系、知识状态、身体条件、声线身份、重要 tic 与 facade/crack 触发条件由上游 Artifact 锁定。**身体外形/体型/身高/比例与永久 Voice Identity 对 ACTING 均为 REFERENCE_ONLY**：本 Skill 只能读取并适配行为，不能创建、推断或改写这些事实。Scene adaptation 可以重新表达但不能与 master profile 矛盾；不能因为姿势限制就删除行为能量，应把它转移为等价可观察动作。

## 完整分阶段工作流

1. **Scene engine**：为每个在场角色确认 Objective、Obstacle/Stakes、Tactic、Beat、Subtext；目标必须是指向具体对象的动作动词，不是情绪状态。
2. **Body first**：确定重心、姿态、开放/封闭、呼吸、节奏、physical business、人物距离和 status；台词发生在身体正在做的事情之上。
3. **Listening**：让反应在对方句尾前开始；困难回答前有 thought-before-word；重要信息出现时写 assessment moment；节拍变化必须在姿势、视线、呼吸、速度或距离上可见。
4. **Eye life**：连续微扫视、目标性 gaze、自然 blink、live catchlight；眼睛先于头和语言抵达目标，静止必须是选择而不是冻结。
5. **Master → Scene**：重复角色先写 150–220 词的长期 master profile，再对当前场景重写；`physique_ref` 只引用 Character Visual/Asset，`behavioral_posture` 才是 ACTING 可拥有的行为字段；按 seated/running/hiding 等姿势转换行为出口。
6. **Voice lock**：`voice_profile_ref` 只引用 Sound Bible/Audio Contract，不在 ACTING 中重写音高、音色、口音或永久 Voice prompt。表演可改变停顿、呼吸、节奏、音量策略等场景行为，但 voice identity 不变。

## 阶段门禁

- Gate A：当前场景每个角色都有明确 objective + obstacle/stakes。
- Gate B：至少 2 个可观察行为通道（身体/眼神/呼吸/physical business/距离）承载状态。
- Gate C：scene adaptation 不与 master profile、声音档案或 continuity 冲突。
- Gate D：需要改变角色动机、关系、关键台词或剧情后果时返回编剧/Human。

## 输出契约

长期角色输出 canonical `character-acting-profile`；逐场输出 `scene-acting-adaptation`。Master Profile 至少包含：`character_id`、`physique_ref`（REFERENCE_ONLY）、`behavioral_posture`、`psychological_engine`、`voice_profile_ref`（REFERENCE_ONLY）、`signature_tics_with_triggers`、`stress_tic`、`concealment_behavior`、`facade_and_crack_trigger`、`gait`、`eye_life`、`softening_target`、`source_refs`。不得使用 ACTING Artifact 反向更新 Character Visual Bible 或 Voice Profile。Scene adaptation 至少包含：`scene_id/shot_id`、`objective`、`obstacle_stakes`、`tactics`、`beats`、`subtext`、`body_state`、`business`、`proxemics_status`、`eye_life`、`speech_behavior`、`observable_change`。

## V1.3.1 Authority Override

遵守 [`SKILL-AUTHORITY-PRECEDENCE-V1.3.1.md`](../../../contracts/SKILL-AUTHORITY-PRECEDENCE-V1.3.1.md)。ACTING 的角色是 ADAPTER，不是 visual/voice/story owner。`physique_ref` 与 `voice_profile_ref` 都是只读引用；任何 source reference 中关于“physique carries biography”或完整 vocal profile 的写法，只能在上游已经提供对应事实后用于行为解释，不能据此创造外貌或声线。

## Production System V2 集成

`scene-acting-adaptation` 可作为 Shot Contract/Prompt Manifest 的 performance evidence，但它不拥有镜头或资产事实。复杂动作先保证身体物理可实现，再交 `ai-production-feasibility-review`。详见 [`PRODUCTION-SYSTEM-V2.md`](../../../contracts/PRODUCTION-SYSTEM-V2.md)。

## Creative Depth V1.3 集成

ACTING 把人物内核落到“选择 → 行动 → 后果”的可见行为层，与 Character Causal Bible 和 Embodiment Spine 对齐；严禁根据声音/性格反推体型或外貌。详见 [`CREATIVE-DEPTH-V1.3.md`](../../../contracts/CREATIVE-DEPTH-V1.3.md)。

## 质量检查表

- 没有把“愤怒/害怕/悲伤/自信”等抽象状态当作主要表演指令。
- 每个 beat 都能看到行为变化；策略失败后人物会调整 tactic。
- 对白前有 thought，听与反应持续存在；不是等台词 cue 才“开机”。
- 身体状态与声音一致：刚跑完的人不能稳定无喘地说长句。
- 眼睛有自然 micro-saccade、blink 和 gaze target；没有死盯镜头。
- Physical business、距离/status、facade/crack 至少有一项真正进入当前场景。
- Scene adaptation 是重写而非直接粘贴 master profile。

## 错误与异常处理

角色 master profile 不存在时先创建 DRAFT；关系/目标冲突交编剧与 continuity；镜头姿势不允许某行为时做行为转换而非删除；声线矛盾交 sound_designer；模型表现能力未知时只交行为层，不承诺生成质量。

## 与其他Skill的边界

`screenwriter` 决定人物动机、选择与台词；本 Skill 把它们编译为表演行为。`director-storyboard` 决定调度和镜头；`cinedance-video-director` / `ai-video-shot-prompt` 把 scene adaptation 编译进视频提示词；`sound-design` 拥有 voice identity；`character-visual-design` 拥有外貌和服装。

## 上游输入和下游交接

Master profile 上游主要来自 `narrative_screenwriter` + character causal/relationship evidence；scene adaptation 上游来自剧本和导演 blocking。下游交给 `director_storyboard_artist`、`ai_production_supervisor` 与 QC；局部结果通过 Message 返回 Topic Responsible。

## Human介入条件

需要改变人物核心欲望、关系、关键选择、台词、结局、角色生理事实或锁定声线时升级 Human；如果表演方向存在两个互斥且都会改变人物理解的方案，也应进入 Needs You。

## 示例

坏指令：“他很愤怒地说话。”可执行版本应描述：他仍低声说，先把杯子对齐桌沿以维持控制；对方提到钱时手停住半拍，眼睛先抬到对方脸上，呼吸收紧，随后把句尾压短，不提高音量——目标是逼对方承认，而不是展示愤怒。

## 最小测试用例

见 [`evals/cases/character-acting-system/`](../../../evals/cases/character-acting-system/)：3 正向、2 不触发、2 边界、1 输入不足、1 相邻冲突。评测验证结构和边界，不证明任何视频模型真的产生优质表演。

## 版本信息

- schema_version：`3.2`
- content_version：`3.2.1`
- 状态：`ACTIVE`

## 参考资料

- [ACTING SYSTEM 原始用户资料](references/source-ACTING-SYSTEM.md)
- [Master Profile 规则](references/master-profile.md)
- [Scene Adaptation 规则](references/scene-adaptation.md)
