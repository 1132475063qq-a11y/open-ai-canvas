# 导演分镜 参考：transitions

## 本参考解决的问题

本条用于 `director-storyboard` 的 `transitions` 决策。它把专业判断落到可核对的字段，不替代项目事实源或 Human 决定。

## 操作卡

1. 从输入记录 `artifact_id`、`object_version`、状态和权威来源。
2. 围绕本域目标执行：建立空间平面、轴线、出入口、视线和动作起终点。
3. 产出至少一个可定位证据、一个风险或限制、一个下一步/交接对象。
4. 如与 LOCKED 内容冲突，写 CHANGE-REQUEST；如无权威来源，升级而不补写。

## 必填记录

`scene`、`shot_id`、`timecode`、`duration`、`purpose`、`size_angle`、`composition`、`blocking`、`movement`、`space_anchor`、`dialogue_sound`、`transition`、`continuity_note`。任何未验证模型、平台规则、受众结论或事实必须标为“未验证”或“暂定”。

## 失败预防

禁止以审美偏好替代证据；禁止从文件名/时间戳推断版本权威；禁止让 Participant 标记 Topic 完成。交付前执行：镜头总时长与剧本匹配；轴线/视线可执行；每镜有叙事目的；未混入模型提示词；不改锁定人物动机。
