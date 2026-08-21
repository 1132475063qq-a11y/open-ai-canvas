# 场景资产设计 参考：camera / zones

## 本参考解决的问题

本条用于 `scene-asset-design` 的 `camera / zones` 决策。它把专业判断落到可核对的字段，不替代项目事实源或 Human 决定。

## 操作卡

1. 从输入记录 `artifact_id`、`object_version`、状态和权威来源。
2. 围绕本域目标执行：画文字平面、出入口、前中后景、站位和遮挡。
3. 产出至少一个可定位证据、一个风险或限制、一个下一步/交接对象。
4. 如与 LOCKED 内容冲突，写 CHANGE-REQUEST；如无权威来源，升级而不补写。

## 必填记录

`layout`、`entrances`、`fixed_props`、`movable_props`、`camera_zones`、`lighting_variants`、`asset_list`、`negative_constraints`。任何未验证模型、平台规则、受众结论或事实必须标为“未验证”或“暂定”。

## 失败预防

禁止以审美偏好替代证据；禁止从文件名/时间戳推断版本权威；禁止让 Participant 标记 Topic 完成。交付前执行：动线支持剧本；固定物位置不冲突；机位能看到必须信息。
