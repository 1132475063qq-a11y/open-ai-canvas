# 剧本到分镜提示词标准

**文档类型**：`METHOD_REFERENCE / PROMPT_DELIVERY_STANDARD`  
**适用对象**：`ai-video-shot-prompt` 及其上游 `director-storyboard`  
**版本**：`1.0.0`  
**状态**：`ACTIVE`

## 1. 目标

把已确认的剧本内容稳定地转换为：

1. 可拍摄、可检查的分镜决策；
2. 每镜独立、可执行的视频生成提示词；
3. 可追溯的角色、场景、道具、动作、镜头和声音约束；
4. 能交给 Seedance 2.5 或其他视频模型继续编译的 `Prompt Manifest`。

本标准解决的是“每次如何规范输出”，不是替编剧改剧情，也不是证明模型已经生成视频。

## 2. 总体工作流

严格按以下顺序执行：

```text
剧本事实
  -> 场次与戏剧目的
  -> 镜头拆分
  -> Shot Contract
  -> 分镜决策表
  -> 独立视频 Prompt
  -> 连续性与生成风险检查
```

### 2.1 读取和分级事实

将输入内容分为三类：

| 类别           | 处理规则                                                   |
| -------------- | ---------------------------------------------------------- |
| 已确认事实     | 直接写入分镜和 Prompt，并保留来源引用                      |
| 可逆假设       | 明确标记为“暂定”，写入风险表，不伪装成剧本事实             |
| 缺失或冲突事实 | 标记 `UNKNOWN` / `BLOCKED`，列出最小补充问题，不能自行拍板 |

剧本负责人物目标、关系、剧情因果、关键选择和台词；分镜负责镜头意图、调度、构图、轴线、节奏和可见画面；提示词只负责把已确认内容编译成模型可以执行的语言。

### 2.2 镜头拆分原则

每个镜头只承载一条主要可见动作链和一个清晰的观众信息变化。以下情况通常应拆镜：

- 观众需要从不知道变为知道一个重要信息；
- 角色、空间或动作轴发生不可忽略的变化；
- 景别、机位或镜头运动承担不同叙事目的；
- 复杂多人互动、手部操作、文字、反射或特效无法在一个镜头内稳定完成；
- 一个镜头的动作数量超过时长可承载的范围。

不要为了“看起来丰富”堆叠无叙事作用的运镜和切换。镜头数量、时长和复杂度必须能回指剧本节奏和制作限制。

## 3. 每镜标准字段

每个镜头必须按以下字段输出。字段缺失时写 `UNKNOWN`，不要留空或用“同上”代替。

### 3.1 分镜决策字段

```text
shot_id                 唯一镜头 ID，例如 S01-SH03
scene_id                场次 ID
sequence_order          镜头顺序
source_script_ref       剧本段落、页码或台词来源
dramatic_purpose        该镜头在剧情中的唯一主要目的
viewer_knows_before     镜头开始前观众知道什么
viewer_learns_after     镜头结束后观众获得什么信息或感受
duration_s              时长，单位为秒
shot_size               景别
camera_angle            角度
camera_position         机位和空间位置
lens_class              焦段类别或视觉效果
composition             构图和画面层次
movement                运镜、起止时机和 movement_reason
blocking                人物/道具的起点、终点、距离、朝向和互动目标
screen_direction        屏幕方向
action_axis             动作轴线
space_anchors           可定位的场景锚点
visible_start_state     第一帧可见状态
action_timeline         按阶段或时间点排列的动作
dialogue_sound          对白、拟音、环境声、音乐和静默
transition              与前后镜头的转场和连续性理由
continuity_in           继承的角色、道具、空间和声音状态
continuity_out          镜头结束后交给下一镜的状态
acceptance_checks       可观察的验收条件
```

### 3.2 视频 Prompt 字段

```text
prompt_version
target_model_profile
task_mode                T2V / image-reference / video-reference / edit / extend / first-last-frame / multi-material
active_reference_roles   当前镜头实际使用的素材和职责
scene_context            当前镜头的完整场景上下文
location_map             空间关系和场景锚点
first_frame_blocking     第一帧人物、道具和空间占位
timed_action             动作顺序、阶段和结束状态
camera_prompt            景别、机位、构图、焦段和运镜
physics_prompt           重量、接触、惯性、液体、布料和道具反应
lighting_prompt          光源方向、色温、阴影和曝光逻辑
audio_prompt             说话者、对白时机、环境声和拟音
positive_locks           必须保持的正向约束
local_failure_locks      针对已知失败模式的局部保护
preservation_contract    编辑/延长任务必须保持的母版状态
acceptance_checks        生成后可检查的结果条件
generation_chain_ref     与 Attempt/Result/QC 的关联
```

## 4. 标准输出顺序

AI 每次输出分镜提示词时，使用以下固定顺序：

### A. 输入状态

先列出：

- 使用的剧本版本和来源段落；
- 角色、场景、道具和声音资料版本；
- 目标模型和任务模式；
- 已确认事实；
- 暂定假设；
- 阻塞项和需要用户确认的问题。

### B. 镜头表

先用表格给出全段镜头概览：

| 镜头     | 时长 | 景别/机位       | 主要动作       | 叙事目的     | 风险     |
| -------- | ---: | --------------- | -------------- | ------------ | -------- |
| S01-SH01 |   4s | 中近景 / 侧前方 | A 抬头看向门口 | 建立威胁来源 | 视线方向 |

概览中的镜头数量、顺序和总时长必须与下面的逐镜内容一致。

### C. 逐镜分镜决策

每镜先写分镜事实，再写模型 Prompt。推荐格式：

```markdown
## S01-SH01

### Shot Contract

- 来源：S01 / 剧本第 3 段
- 戏剧目的：...
- 时长：4 秒
- 第一帧：...
- 空间与 Blocking：...
- 景别/机位/构图：...
- 运镜与理由：...
- 动作时间轴：...
- 声音：...
- 连续性输入/输出：...
- 验收条件：...

### Video Prompt

...

### Internal Protection Ledger

- 必须保持：...
- 已知风险：...
- 局部保护：...
```

### D. 交接摘要

最后列出：

- 可直接进入生成的镜头；
- 因输入不足而 `BLOCKED` 的镜头；
- 需要拆分或降级的镜头；
- 素材缺口；
- 连续性风险；
- 下一步应交给的 Agent 或 API Provider。

## 5. Prompt 编写规则

### 5.1 每镜必须自洽

每条 Prompt 脱离上一镜也必须能理解。禁止使用：

- “同上”；
- “继续上一镜”；
- “承接前一段”；
- “那个男人”“她”而没有身份描述；
- 未绑定来源的 `@素材`；
- 场次编号、脚本页眉或内部制作备注。

如果角色在本镜出现，重复写明其可见外貌、服装状态、姿态和位置；如果角色不在本镜，不要为了连续性把其写进 Prompt。

### 5.2 先空间，再风格

Prompt 的事实顺序优先是：

```text
场景上下文
-> 素材职责
-> 空间地图
-> 第一帧占位
-> 人物与道具状态
-> 动作时间轴
-> 镜头与构图
-> 物理
-> 光线
-> 声音
-> 必须保持项
```

不要用“高级、电影感、压迫、唯美”替代空间、动作和灯光事实。

### 5.3 时间轴必须可执行

普通叙事使用连续阶段；需要严格节拍时使用时间点。每个阶段写清动作和结束状态，时间段连续且不重叠。例如：

```text
0-2s: A 站在厨房门口，手仍握着湿雨伞，先看向桌面。
2-4s: A 缓慢走到桌边，把雨伞靠在左侧墙边。
4-6s: A 发现桌上的信封，右手停在信封上方，没有立即打开。
```

时间戳是节奏预算，不保证模型在绝对帧点完成动作；动作的先后关系和最终状态必须明确。

### 5.4 参考素材必须声明职责

每个素材写清“采用什么”和“不采用什么”：

```text
人物 A 使用 @角色图1 的脸部特征、发型和服装；不继承参考图的姿势和背景。
场景使用 @场景图2 的空间布局和窗光方向；不使用图中人物和文字。
道具使用 @道具图3 的结构、颜色和材质；保持道具在 A 的右手。
```

多素材不能只列文件名。没有职责、来源或使用位置的素材应从 Prompt 中移除或标记为 `BLOCKED`。

### 5.5 负面约束采用正向优先

先把保护项写成正向事实：

```text
A 始终位于画面左侧，手机始终握在右手，窗光从画面右后方照入。
```

只有已知失败模式无法通过正向表达可靠控制时，才补充最小的局部禁止项。内部保护台账可以完整记录约束，但不应机械复制成大段负面词。

## 6. 任务模式补充规则

### 6.1 文生视频 / 参考素材

明确哪些信息来自文字，哪些信息来自图片、视频或音频。参考素材只承担声明的职责，不自动覆盖剧本事实、镜头构图或角色状态。

### 6.2 视频编辑

把原视频写成唯一母版，明确编辑对象、编辑范围、目标素材职责和必须保持项。不能把“编辑”写成全片重新生成。

### 6.3 视频延长

向后延长从原视频尾帧开始，向前延长与原视频首帧衔接。人物、道具、背景、运动趋势、光线和声音必须连续；新素材不能替代原视频边界状态。

### 6.4 首帧/首尾帧

首帧承担开场构图和人物占位；首尾帧保持同一画幅。比例、时长、分辨率等由 UI/API 控制时，不重复写成模型事实。

## 7. 交付前强制检查

逐镜检查以下项目，任何一项不通过都不能标记为 `READY_FOR_GENERATION`：

- [ ] 来源剧本段落可定位，未改变剧情事实；
- [ ] 每镜有唯一叙事目的；
- [ ] 总时长与剧本或用户要求一致；
- [ ] 第一帧人物、道具和空间占位明确；
- [ ] 人物位置、身体朝向、视线和移动方向明确；
- [ ] 场景锚点、动作轴和镜头侧位可执行；
- [ ] 动作时间段连续、不重叠，数量适合镜头时长；
- [ ] 参考素材都有职责、来源和禁止继承项；
- [ ] 对白有说话者和时机，声音不会错配；
- [ ] 编辑/延长任务有 `preservation_contract`；
- [ ] 没有“同上”“继续”“上一镜”等跨镜依赖；
- [ ] 没有把 UI 参数伪装成已验证的模型能力；
- [ ] 风险、阻塞项和降级方案已写明；
- [ ] 没有真实执行证据时，状态仍为 `PLANNED` / `NOT_EXECUTED`。

## 8. 最小标准示例

输入：剧本中，夜晚的便利店里，林夏发现收银台下有一封带血的信。她先看向门口，再蹲下取信。

规范化输出的核心应是：

```markdown
## S02-SH04

### Shot Contract

- 戏剧目的：让观众确认林夏发现异常，同时保留门外是否有人这一悬念。
- 时长：6 秒。
- 第一帧：林夏位于画面左侧收银台内，收银台下方的信封只露出一角；便利店门位于画面右后方。
- Blocking：林夏先保持半蹲，头部转向右后方的门，再把视线落到收银台下；右手伸向信封，左手扶住台面。
- 运镜与理由：从中近景缓慢向前推进，保持门和信封同时可见，服务于悬念递进。
- 声音：冰柜低频声持续；门外风铃无声；林夏的呼吸在转头后变得清晰。
- 验收条件：第一帧同时看见林夏、门和信封；取信动作发生在看门之后；信封始终在收银台下方。

### Video Prompt

At night inside a small convenience store, Lin Xia is half-crouched behind the checkout counter on the left side of the frame. A blood-stained envelope is only partly visible beneath the counter, while the entrance door remains visible in the rear right. In the first frame, Lin Xia is already visible, the envelope is already visible, and the door is part of the composition. From 0-2 seconds, she turns her head and gaze toward the entrance without standing up. From 2-4 seconds, her gaze drops from the door to the envelope. From 4-6 seconds, she reaches with her right hand and carefully pulls the envelope out while her left hand supports the counter. The camera makes a slow, restrained push-in that keeps Lin Xia, the door, and the envelope visible together. Cold fluorescent light from above, a faint blue spill from the entrance, realistic contact, weight, and cloth movement. Low refrigerator hum and clear breathing after she looks toward the door. Preserve Lin Xia on the left, the entrance in the rear right, the envelope beneath the counter, and the order of the three actions: look at the door, look at the envelope, take the envelope.
```

## 9. 事实边界

这份标准不定义具体 API 地址、认证信息、模型 ID、价格、分辨率能力或响应字段。上述内容必须从当前 Provider、模型 Profile 和 UI 配置读取。提示词完成不等于任务已创建，任务已创建不等于媒体已生成，媒体生成也不等于 QC 通过。
