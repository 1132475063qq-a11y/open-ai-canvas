# 短剧策划与分集 参考：episode / architecture

## 本参考解决的问题

本条用于 `short-drama-planning` 的 `episode / architecture` 决策。它把专业判断落到可核对的字段，不替代项目事实源或 Human 决定。

## 操作卡

1. 从输入记录 `artifact_id`、`object_version`、状态和权威来源。
2. 围绕本域目标执行：定义可重复但会升级的系列引擎、每集目标和失败代价。
3. 产出至少一个可定位证据、一个风险或限制、一个下一步/交接对象。
4. 如与 LOCKED 内容冲突，写 CHANGE-REQUEST；如无权威来源，升级而不补写。

## 必填记录

`adaptation_strategy`、`series_engine`、`episode_grid`、`hook_payoff_ledger`、`knowledge_ledger`、`series_handoff`。任何未验证模型、平台规则、受众结论或事实必须标为“未验证”或“暂定”。

## 失败预防

禁止以审美偏好替代证据；禁止从文件名/时间戳推断版本权威；禁止让 Participant 标记 Topic 完成。交付前执行：每集有独立目标与变化；钩子机制不机械重复；平台规则未核验则仅为假设；不越权写完整场景。
