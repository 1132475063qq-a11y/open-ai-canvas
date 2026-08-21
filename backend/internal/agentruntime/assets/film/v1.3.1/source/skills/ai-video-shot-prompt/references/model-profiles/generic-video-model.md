# AI视频镜头提示词 参考：model / profiles/generic / video / model

## 本参考解决的问题

本条用于 `ai-video-shot-prompt` 的 `model / profiles/generic / video / model` 决策。它把专业判断落到可核对的字段，不替代项目事实源或 Human 决定。

## 操作卡

1. 从输入记录 `artifact_id`、`object_version`、状态和权威来源。
2. 围绕本域目标执行：为每段重复写完整人物、服装、场景、空间和当前动作前提。
3. 产出至少一个可定位证据、一个风险或限制、一个下一步/交接对象。
4. 如与 LOCKED 内容冲突，写 CHANGE-REQUEST；如无权威来源，升级而不补写。

## 必填记录

`shot_id`、`generation_context`、`timed_action`、`camera`、`anchors`、`negative_constraints`、`sound`、`continuity_checks`、`risk`、`fallback`。任何未验证模型、平台规则、受众结论或事实必须标为“未验证”或“暂定”。

## 失败预防

禁止以审美偏好替代证据；禁止从文件名/时间戳推断版本权威；禁止让 Participant 标记 Topic 完成。交付前执行：每段脱离前段仍可理解；动作数量适合时长；角色/服装/空间不遗漏；没有模型未验证规格伪装为事实。
