---
name: story-type-engine
description: 为影视、短剧、动画短片或TVC选择并落实十种故事类型引擎时使用；不把类型公式当作未经确认的剧情事实。
---

# 故事类型引擎

## 适用场景

当项目要在十种 Save the Cat 故事类型中比较、选择、适配，或审查类型承诺是否真正驱动人物选择、后果和结构时使用。适用于电影、剧集、竖屏短剧、动画与故事型 TVC。

## 不适用场景

不用于把类型名当作灵感标签就直接写完整剧本；不替代 screenwriter 的分场写作、short-drama-planning 的分集推进、worldbuilding-management 的正史规则或 script-review 的独立审查。

## 必需输入

项目 Brief、已确认的核心冲突/受众承诺、媒介体量、锁定事实和当前未决项。

## 可选输入

多个概念、已有剧本/分集表、项目参考的高层特征、已批准 CHANGE-REQUEST。参考只能帮助识别机制，不能复制具体情节。

## 输入不足处理

没有可选核心冲突时，输出比较问题和 `selected_type: unselected`，不假装选择已被批准。主角、结局、主题或核心创意互斥，必须在 `NEEDS-YOU.md` 写成可选 HumanDecision。

## 锁定事实与禁止修改项

读取 PROJECT-BRIEF、CURRENT-BRIEF、DECISIONS、各 Bible 与 LOCKED Artifact。类型模型只能解释或暴露冲突，不能重写 LOCKED 主角、结局、品牌事实或正史；任何修改创建新 object_version。

## 完整分阶段工作流

1. 列出项目的观众问题、主角位置、对抗力量、代价与最终变化，分清已确认和假设。
2. 用十类的最小引擎、必需压力和失败征兆进行比较；输出“适合/不适合/证据不足”。
3. 为候选类型填写 Story Type Profile：类型承诺、核心关系、因果压力、不可缺少的选择与后果。
4. 按媒介改写为段落或分集压力：每个单元必须推动人物选择或代价，而不是重复类型标志。
5. 将选型交给 short-drama-planning/screenwriter，将审查点交给 script-review；选型未获 Human 确认时保持 DRAFT。

## 阶段门禁

- Gate A：输入版本、媒介、项目约束与可选类型明确。
- Gate B：每个候选都有“为什么适合/为什么不适合”的项目证据；不是靠相似片名。
- Gate C：类型承诺已翻译为因果链、人物状态变化和至少一个可审查失败模式。
- Gate D：主角、主题、结局、品牌/预算范围、锁定或权威冲突一律升级 Human，不能用类型书替代决定。

## 输出契约

输出 `story-type-profile` Artifact，含 `selected_type`、type promise、audience question、engine elements、项目化因果链、媒介适配、排除理由、未决项和下游输入。每个类型只作为工具性结构；不承诺公式会带来市场结果。

## Creative Depth V1.3 集成

将类型交付为 `Story Pressure Profile`：仅一个主类型、可选从属机制、观众问题、压力链、不可缺少的选择、信息释放、每单元升级、失败征兆与排除理由。类型必须改变人物在压力下的选择和后果，不能只贴“悬疑/爱情/喜剧”标签。主类型、主题、主角或结局尚未由 Human 决定时，`approval_state` 必须为 `NEEDS_YOU`；真实项目不得用 fixture mock 代替批准。

详见 [`CREATIVE-DEPTH-V1.3.md`](../../../contracts/CREATIVE-DEPTH-V1.3.md)。

## 质量检查表

- 十类都可被比较，不把“悬疑”或“爱情”误当作类型引擎。
- 类型元素改变人物选择、关系、信息或代价；否则是装饰。
- 选型和终局未决定时标 DRAFT / NEEDS-YOU，不把推荐写成锁定。
- 每条结论回指项目材料或明确假设，不复制参考作品具体表达。

## 错误与异常处理

如果一个项目混合多种机制，指定主引擎和从属机制，并说明冲突位置；如果两种类型会导向不同的主角/结局，停止在对比并升级。找不到权威来源时标 UNKNOWN，不从文件日期推断。

## 与其他Skill的边界

本 Skill 决定“故事压力如何运作”，不写镜头、不设计视觉或声音，也不进行成片验收。screenwriter 把类型转成剧本；short-drama-planning 把它转成系列引擎；script-review 独立验证落地效果。

## 上游输入和下游交接

上游是 film_project_lead 的 Task/RoutingDecision、项目 Brief 与 Canon。下游向 short-drama-planning 交付系列压力表，向 screenwriter 交付因果/人物变化约束，向 script-review 交付类型审查基准；所有 Topic 结论由 film_project_lead 收口。

## Human介入条件

选择会改变主角、核心关系、主题、结局、品牌信息、范围/预算或锁定事实；十类均不匹配而需要发明新引擎；权威材料冲突无法裁定。

## 示例

输入“普通人被困于封闭场所，外部威胁逐步逼近”时，可以把“屋里有怪物”列为候选，但仍需证明封闭空间、威胁、道德/旧错或代价如何使人物选择升级；缺任何一项，输出“证据不足”，不能自动套入。

## 最小测试用例

见 [`evals/cases/story-type-engine/`](../../../evals/cases/story-type-engine/)。案例覆盖正向比较、非触发、类型混合、输入不足与 LOCKED 冲突；它们是静态触发契约，不是运行时 Agent 证明。

## 版本信息

- schema_version：`3.1`
- content_version：`3.1.0`
- 状态：`ACTIVE`

## 参考资料

- [ten-types.md](references/ten-types.md)
- [type-selection.md](references/type-selection.md)
- [format-adaptation.md](references/format-adaptation.md)
