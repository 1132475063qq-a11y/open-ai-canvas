---
name: character-visual-design
description: 人物设定需要变成可复用角色卡、三视图和图像提示词包时使用；不用于只给空泛审美词或改变人物设定。
---

# 角色视觉设计

## 适用场景

人物设定需要变成可复用角色卡、三视图和图像提示词包时使用。本 Skill 只负责本域的可追溯交付，不把相邻任务静默吞并。

## 不适用场景

人物正史归 worldbuilding/screenwriter；空间归 scene-asset-design；连续性归 continuity-check。输入只有模糊意图而没有可定位材料时，先执行“输入不足处理”。

## 必需输入

角色功能、身份、经济条件、心理状态、世界观和媒介风格。

## 可选输入

目标画幅、预算/工期、参考作品的高层特征、历史审查报告、已批准的 CHANGE-REQUEST。参考只可影响抽象方向，不可复制具体表达。

## 输入不足处理

列出缺失字段、影响和最小补充问题。核心创意、品牌事实、范围、LOCKED 内容或权威冲突必须写入 `NEEDS-YOU.md`；其余可逆假设须以“暂定”标记并进入输出风险表。

## 锁定事实与禁止修改项

以项目 `PROJECT-BRIEF.md`、`CURRENT-BRIEF.md`、`DECISIONS.md`、各 Bible 和 LOCKED Artifact 为准。不得以文件名或修改时间判定权威；LOCKED 产物只能通过 CHANGE-REQUEST 创建新的 `object_version`，绝不原地改写。

## 完整分阶段工作流

1. 提取不可改身份、功能和剧情状态
2. 定义远中近三层轮廓、比例、脸发、体态和识别细节
3. 将服装、材质、配色与职业/经济/动作需求关联
4. 制作中性、情绪、剧情状态和三视图需求
5. 固定可复现锚点，列出可变项与禁改项
6. 追加 `character-embodiment-bible`：记录身高体型、面部/发型/眼镜、服装状态、姿态/手势、行动能力和声线 Profile 引用；每项注明证据、允许反差和禁止漂移。

人格只影响可观察的行为选择，不能从“粗犷”“文静”等词反推出外貌、身材、性别表达或声线。没有权威证据的外观/声线组合必须标 UNKNOWN 或升级 `NEEDS-YOU.md`。

## 阶段门禁

- Gate A：Responsible、目标、输入版本和验收标准齐全。
- Gate B：已确认/暂定/冲突事实分离；未解决权威冲突已升级。
- Gate C：字段完整、质量表通过、相邻域未越权，才能交接。
- Gate D：锁定、品牌、范围、不可逆操作或对外发布必须等待 Human。

## 输出契约

必须输出：视觉 Brief、角色卡、三视图需求、状态版、一致性锚点和提示词包。

V1.2 还必须交付具身字段：`height_build`、`face_hair_glasses`、`posture_gesture`、`action_capability`、`voice_profile_ref`、`allowed_contrast` 与 `forbidden_drift`。

正式 Artifact frontmatter 必须含 `artifact_id`、`artifact_type`、`schema_version`、`object_version`、`status`、`responsible`、`related_topic_id`、`parent_artifact_id`、`created_at`。本域字段：`silhouette`、`proportion`、`face_hair`、`costume_logic`、`palette`、`material`、`state_variants`、`consistency_anchors`、`prompt_pack`。

## Production System V2 集成

角色必须先成为 `Asset Bible` 中可引用的 Character Asset，再进入镜头。把身份锚点（脸型、眼镜、身高体态、发型、服装与不可变特征）和可变状态（服装污损、受伤、情绪、手持物）分开记录；不得以“声音粗犷”等听觉描述反推外貌。输出须含 asset_id、state_id、L3 Reference Lock 需求与对 `Shot Contract` 的可用性；外貌未锁定时只能给 DRAFT，不得冒充稳定角色。

以 [`PRODUCTION-SYSTEM-V2.md`](../../../contracts/PRODUCTION-SYSTEM-V2.md) 的 Asset、Reference Lock 与状态规则为准。

## Creative Depth V1.3 集成

为每个进入正式镜头的角色维护 `Embodiment Spine`：因果内核、可观察选择/姿态/行动限制、台词和声音证据、视觉身份引用、可变状态与一致性检查。声音与外形各自引用独立证据；允许反差但须写剧情功能，禁止以声线、性格、眼镜、身高或体型互相反推。信息不足时保留 `NEEDS_YOU`/`UNKNOWN`，不为图像提示词擅自补写。

详见 [`CREATIVE-DEPTH-V1.3.md`](../../../contracts/CREATIVE-DEPTH-V1.3.md)。

## LIRA V1.3 集成

视觉设计先锁定角色身份、比例、服装、材质、状态变体和 Reference Lock，再由 `lira-image-prompts` 编译目标图像模型的角色/角色 sheet prompt。模型路由与平台参数属于 Prompt 层，不能反过来修改 Character Asset 的 canon。用户提供的 Soul / AI Cast 等能力表按项目 profile 保存，未有 runtime 证据时标为未验证。

## 质量检查表

- 设计能区分角色且服务剧情；不依赖真实人物；状态变化可追溯。
- 所有结论能回指输入、权威事实或明确的暂定假设。
- 风险、限制、下一步与交接对象明确；没有“最终版”之类无版本断言。

## 错误与异常处理

文件不可读时报告路径与影响；版本冲突时不选边，交 continuity-check；任务超出本域时发送 TASK-REQUEST 给对应 Responsible；无法访问媒体、模型或外部事实时写“未验证”，不声称调用成功。

## 与其他Skill的边界

人物正史归 worldbuilding/screenwriter；空间归 scene-asset-design；连续性归 continuity-check。审查 Skill 只提供证据和修复方向，不覆盖主稿；制作风险 Skill 不替代创作决定。

## 上游输入和下游交接

用 TASK-REQUEST 接收版本化输入；产出 Artifact、TASK-RESULT 和 append-only Event。Participant 只能回报局部完成；所有项目级结果回到 film_project_lead。

## Human介入条件

核心创意互斥、主题/结局/主角重大变化、品牌/产品事实缺失、范围/时长/预算变化、锁定、明显制作风险、对外发布或无法裁定的权威冲突。

## 示例

输入应包含可定位版本和硬约束。输出先列已确认事实与风险，再按本域字段交付；若版本未锁定，只给预检/草案并标明不能进入下游的原因。

## 最小测试用例

见 [`evals/cases/character-visual-design/`](../../../evals/cases/character-visual-design/)：3 正向、2 不触发、2 边界、1 输入不足、1 相邻冲突。所有案例必须由独立审查流程评估，不由本 Skill 自评。

## 版本信息

- schema_version：`3.2`
- content_version：`3.2.0`
- 状态：`ACTIVE`

## 参考资料

- [silhouette-proportion.md](references/silhouette-proportion.md)
- [face-hair-body.md](references/face-hair-body.md)
- [costume-storytelling.md](references/costume-storytelling.md)
- [material-color.md](references/material-color.md)
- [expression-pose.md](references/expression-pose.md)
- [turnaround-character-sheet.md](references/turnaround-character-sheet.md)
- [consistency-anchor.md](references/consistency-anchor.md)
- [embodiment-contract.md](references/embodiment-contract.md)
- [prompt-pack.md](references/prompt-pack.md)
