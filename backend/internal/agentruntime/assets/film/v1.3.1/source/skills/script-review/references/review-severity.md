# 剧本审查 参考：review / severity

## 本参考解决的问题

本条用于 `script-review` 的 `review / severity` 决策。它把专业判断落到可核对的字段，不替代项目事实源或 Human 决定。

## 操作卡

1. 从输入记录 `artifact_id`、`object_version`、状态和权威来源。
2. 围绕本域目标执行：逐场检查因果、动机、知识、时间、空间、道具、规则和信息重复。
3. 产出至少一个可定位证据、一个风险或限制、一个下一步/交接对象。
4. 如与 LOCKED 内容冲突，写 CHANGE-REQUEST；如无权威来源，升级而不补写。

## 必填记录

`finding_id`、`severity`、`location`、`evidence`、`impact`、`repair_direction`、`upstream_decision`、`verification_status`。任何未验证模型、平台规则、受众结论或事实必须标为“未验证”或“暂定”。

## 失败预防

禁止以审美偏好替代证据；禁止从文件名/时间戳推断版本权威；禁止让 Participant 标记 Topic 完成。交付前执行：P0/P1 可由文本证据复现；结论不声称观众必然反应；每条建议有责任人或升级路径。
