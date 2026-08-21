# Skill Authority & Conflict Precedence V1.3.1

**version**: `1.3.1-conflict-hardened`  
**status**: `ACTIVE`  
**Responsible**: `film_project_lead`

本合同解决 V1.3 新增 ACTING / CINEDANCE / LIRA 与既有影视 Skill 之间的内容重叠。它定义“谁拥有事实、谁只能读取、谁负责把事实编译成模型语言”。任何 Skill reference 中的通用方法、供应商经验或示例，不得覆盖本项目的 LOCKED Canon / Bible / Shot Contract / Audio Contract。

## 1. 全局权威优先级

同一问题出现冲突时，按以下顺序裁定：

1. Human 明确决定 / HumanDecision / 已批准 CHANGE-REQUEST
2. LOCKED Canon、PROJECT/CURRENT BRIEF、DECISIONS、Story/Character/Location/Prop/Visual/Sound Bible
3. LOCKED Script / Shot Contract / Asset State / Audio Contract / Continuity Ledger / Reference Lock
4. 当前 Topic Responsible 已确认的版本化专业 Artifact
5. Project Style Profile / Model Profile（仅决定编译方式，不创建故事或资产事实）
6. Skill 主文档中的方法规则
7. Skill `references/source-*` 中的用户提供方法资料、供应商 profile、示例和经验

低层不得反向覆盖高层。冲突无法通过该顺序消解时，保持 `UNKNOWN/BLOCKED` 并升级对应 Responsible/Human。

## 2. 决策层与编译层

- `screenwriter`：拥有剧情因果、人物目标/关系/知识、关键选择与台词事实。
- `director-storyboard`：拥有镜头意图、blocking、camera side、screen direction、构图、镜头动作与 Shot Contract。
- `character-visual-design` / `scene-asset-design`：拥有人物外貌、比例、服装、道具几何、场景结构、视觉风格与资产状态。
- `sound-design`：拥有 voice identity、声线、对白/环境/拟音/音乐策略与 Audio Contract。
- `character-acting-system`：只把已确认人物心理和身体事实编译为可观察 performance behavior；不拥有外貌、体型、声线、剧情或镜头事实。
- `lira-image-prompts`：只把 LOCKED visual facts + Project Style Profile 编译为图像模型 prompt；不得把模型默认审美反向写进 Visual Bible。
- `cinedance-video-director`：只把 Shot Contract + references + acting + audio 编译为 Seedance/Higgsfield prompt；不得改导演、资产、声音或剧本事实。

## 3. CINEDANCE 约束编译规则

`dynamic_negative_constraints` 是**内部机器可读保护台账**，用于描述身份、几何、空间、方向、状态与连续性中“不能漂移”的条件；它不等于最终提示词必须逐条输出负面句。

编译顺序固定为：

`dynamic_negative_constraints` → `positive_locks` → `local_failure_prevention_locks` → `emitted_negative_constraints`（仅必要时）

规则：
- 能用明确正向状态表达时，改写为 positive lock，例如“角色 A 始终在画面左侧，手机始终在右手”。
- 需要阻止具体局部失败且正向表达不足时，使用短 local lock。
- 只有 known failure mode 无法可靠正向表达时，才输出最小 `emitted_negative_constraints`。
- 禁止把整个内部负向台账机械复制成大段 NOT-stack。
- QC/continuity 仍读取完整 `dynamic_negative_constraints`，因此数据层保护不会丢失。

## 4. ACTING 的 REFERENCE-ONLY 权限

以下字段对 ACTING 均为 `REFERENCE_ONLY`：
- `physique/body_proportions/height/build`
- `face/hair/costume/visible_identity`
- `voice_identity/timbre/register/accent/permanent_voice_prompt`
- `scripted_line/story_objective/relationship/canon_knowledge`
- `camera/blocking/lighting/style`

ACTING 可以读取这些事实并做场景行为适配，但不能创建、推断或修改它们。比如：
- 可以读取“中等身材、微驼背”并设计压力下肩颈如何收紧；不能因为角色强势把他改成壮汉。
- 可以读取固定 Voice Profile 并设计停顿、呼吸、句尾节奏；不能修改音高、口音或永久声线身份。

Master Profile 中的 `physique_posture` 必须拆成 `physique_ref` + `behavioral_posture`；`voice_profile_ref` 只保存引用，不复制/重写 voice identity。

## 5. LIRA Project Style Override

图像 Prompt 的风格优先级固定为：

`Visual Bible / Project Style Profile` → `Asset/Scene Style Facts` → `Model Profile` → `LIRA generic/source guidance`

模型 profile 只决定“怎样表达”，不能决定“项目应该长什么样”。

当 Project Style Profile 为 2D/cel/illustration 时：
- 禁止自动注入 photoreal skin、skin pores、film-stock realism、PBR、realistic material roughness、photographic depth of field、real-camera texture 等写实默认。
- 应把模型方法翻译成项目风格的等价可观察描述，例如 flat cel fills、hard-edged cel shadows、bold outlines、simplified geometry、2D painted background。
- 用户提供的 LIRA 原始 source 中任何 photoreal/Soul Cinema 示例均视为方法参考，不是全局 Style Source of Truth。

当 Project Style Profile 为 photoreal 时，才允许使用相应写实锚点；混合风格必须以 Visual Bible 明确许可为前提。

## 6. Reference 文档地位

`references/source-ACTING-SYSTEM.md`、`references/source-CINEDANCE-V4.md`、`references/source-LIRA.md` 保存用户提供的原始方法资料，允许保留原文完整性；它们是 `METHOD_REFERENCE / USER_SUPPLIED_PROFILE`，不是 Canon、不是 runtime 事实、也不是跨项目的最高优先级规则。最终执行以本合同和各 Skill 主文档为准。

## 7. 验收

任一生成/审查链必须能回答：
- 这条事实由哪个 Domain Owner 拥有？
- 当前 Skill 是 OWNER、ADAPTER 还是 COMPILER？
- 模型 profile 是否在反向改写 Canon/Style？
- 内部保护约束是否被误当作最终 NOT-stack？
- ACTING 是否推断了体型/声线？

无法回答时不得进入 LOCKED/production-ready 状态。
