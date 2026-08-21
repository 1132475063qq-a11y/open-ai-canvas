---
name: scene-asset-design
description: 场景或道具需成为空间稳定、可复用设计资产时使用；不用于只写气氛或任意改变剧情动线。
---

# 场景资产设计

## 适用场景

场景或道具需成为空间稳定、可复用设计资产时使用。本 Skill 只负责本域的可追溯交付，不把相邻任务静默吞并。

## 不适用场景

故事事实归 screenwriter；分镜归 director-storyboard；资产风险归 feasibility。输入只有模糊意图而没有可定位材料时，先执行“输入不足处理”。

## 必需输入

地点功能、剧本动作、人物/道具需求、时间天气和画幅。

## 可选输入

目标画幅、预算/工期、参考作品的高层特征、历史审查报告、已批准的 CHANGE-REQUEST。参考只可影响抽象方向，不可复制具体表达。

## 输入不足处理

列出缺失字段、影响和最小补充问题。核心创意、品牌事实、范围、LOCKED 内容或权威冲突必须写入 `NEEDS-YOU.md`；其余可逆假设须以“暂定”标记并进入输出风险表。

## 锁定事实与禁止修改项

以项目 `PROJECT-BRIEF.md`、`CURRENT-BRIEF.md`、`DECISIONS.md`、各 Bible 和 LOCKED Artifact 为准。不得以文件名或修改时间判定权威；LOCKED 产物只能通过 CHANGE-REQUEST 创建新的 `object_version`，绝不原地改写。

## 完整分阶段工作流

1. 定义剧情功能与必须完成的动线
2. 画文字平面、出入口、前中后景、站位和遮挡
3. 区分固定与可移动道具并编号定位
4. 安排光源、时间天气版本和可拍机位
5. 输出多角度资产清单及不可改变锚点，并将空间关系、固定物、可见光源和可拍机位写为可供分镜/提示词复用的 Anchor

## 阶段门禁

- Gate A：Responsible、目标、输入版本和验收标准齐全。
- Gate B：已确认/暂定/冲突事实分离；未解决权威冲突已升级。
- Gate C：字段完整、质量表通过、相邻域未越权，才能交接。
- Gate D：锁定、品牌、范围、不可逆操作或对外发布必须等待 Human。

## 输出契约

必须输出：LOCATION-BIBLE、文字平面图、锚点/道具/机位表、光线版本、资产清单和提示词。

V1.2 中每个空间 Anchor 需要 `asset_id`、空间关系、固定外观、光线条件、camera-relevant constraint 和 evidence；提示词不能凭氛围词猜测房间几何。

正式 Artifact frontmatter 必须含 `artifact_id`、`artifact_type`、`schema_version`、`object_version`、`status`、`responsible`、`related_topic_id`、`parent_artifact_id`、`created_at`。本域字段：`layout`、`entrances`、`fixed_props`、`movable_props`、`camera_zones`、`lighting_variants`、`asset_list`、`negative_constraints`。

## Production System V2 集成

场景必须输出为可引用的 `Scene Bible`：局部 XYZ 原点、锚点、门窗/关键道具、地面平面图、Camera Map 与 Action Axis 都是可验证字段，而非只写氛围词。区分固定空间结构与可变状态（灯光、门的开合、道具位置）；镜头只能从已声明的 camera_slot 和 anchor 出发。空间不完整时标 BLOCKED/NEEDS-YOU，不以临时文字补造动线。

以 [`PRODUCTION-SYSTEM-V2.md`](../../../contracts/PRODUCTION-SYSTEM-V2.md) 的 Scene、空间锚点与镜头轴线规则为准。

## LIRA V1.3 集成

场景设计先建立 Scene Bible、空间 anchor、入口/出口、灯光方向和未来 camera zones，再由 `lira-image-prompts` 编译 location/image prompt。反打或新机位必须从同一空间事实重新写物件相对位置，不把一张漂亮正面图当完整空间。新视角只有经过 continuity/QC 后才能成为正式 reference。

## 质量检查表

- 动线支持剧本；固定物位置不冲突；机位能看到必须信息。
- 所有结论能回指输入、权威事实或明确的暂定假设。
- 风险、限制、下一步与交接对象明确；没有“最终版”之类无版本断言。

## 错误与异常处理

文件不可读时报告路径与影响；版本冲突时不选边，交 continuity-check；任务超出本域时发送 TASK-REQUEST 给对应 Responsible；无法访问媒体、模型或外部事实时写“未验证”，不声称调用成功。

## 与其他Skill的边界

故事事实归 screenwriter；分镜归 director-storyboard；资产风险归 feasibility。审查 Skill 只提供证据和修复方向，不覆盖主稿；制作风险 Skill 不替代创作决定。

## 上游输入和下游交接

用 TASK-REQUEST 接收版本化输入；产出 Artifact、TASK-RESULT 和 append-only Event。Participant 只能回报局部完成；所有项目级结果回到 film_project_lead。

## Human介入条件

核心创意互斥、主题/结局/主角重大变化、品牌/产品事实缺失、范围/时长/预算变化、锁定、明显制作风险、对外发布或无法裁定的权威冲突。

## 示例

输入应包含可定位版本和硬约束。输出先列已确认事实与风险，再按本域字段交付；若版本未锁定，只给预检/草案并标明不能进入下游的原因。

## 最小测试用例

见 [`evals/cases/scene-asset-design/`](../../../evals/cases/scene-asset-design/)：3 正向、2 不触发、2 边界、1 输入不足、1 相邻冲突。所有案例必须由独立审查流程评估，不由本 Skill 自评。

## 版本信息

- schema_version：`3.0`
- content_version：`3.0.0`
- 状态：`ACTIVE`

## 参考资料

- [spatial-layout.md](references/spatial-layout.md)
- [set-dressing.md](references/set-dressing.md)
- [fixed-props.md](references/fixed-props.md)
- [camera-zones.md](references/camera-zones.md)
- [lighting-variants.md](references/lighting-variants.md)
- [location-bible.md](references/location-bible.md)
- [multi-angle-assets.md](references/multi-angle-assets.md)
