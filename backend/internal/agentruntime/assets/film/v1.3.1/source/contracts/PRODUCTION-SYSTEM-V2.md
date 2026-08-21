# Production System V2 数据契约

**schema_version**: `3.0`  
**状态**: `ACTIVE`  
**Responsible**: `film_project_lead`  
**模式**: `FILE_MODE`（结构化生产数据层；不包含 Agent runtime、Canvas、媒体模型或媒体分析。）

## 1. 目的与兼容性

本契约在 [FILEMODE-V1.2-CONTRACTS.md](FILEMODE-V1.2-CONTRACTS.md) 的 Topic、Message、
Artifact、Event 协议之上，增加 Production Source of Truth。它不替换历史 Markdown
Artifact，也不从自由 Prompt 推断事实。生产对象使用 JSON，原因是跨对象依赖、状态与
空间关系必须被无依赖校验器可靠读取。

旧项目继续可读：缺失 Production 对象表示 `NOT_MIGRATED`，不是隐式完成；迁移只能创建
新版本对象，未知字段只能是 `UNKNOWN`、`UNSET` 或 `NEEDS_REVIEW`。

## 2. 公共字段与状态命名空间

每个 `asset`、`scene`、`shot_contract`、`continuity_ledger`、`reference_lock` 和
`production_readiness_report` 必须有：

```json
{
  "object_id": "唯一 ID",
  "object_type": "明确对象类型",
  "schema_version": "3.0",
  "object_version": "1.0.0",
  "status": "DRAFT | REVIEW | LOCKED | SUPERSEDED | ARCHIVED",
  "responsible": "已注册 Agent",
  "created_at": "ISO-8601 UTC",
  "updated_at": "ISO-8601 UTC",
  "source_artifact_ids": [],
  "authority_refs": []
}
```

`status` 保持现有 Artifact 治理的语义。以下是独立的业务状态，不能混用：

| 字段 | 合法值 | 用途 |
|---|---|---|
| `lifecycle_state`（asset） | DRAFT, REVIEW, APPROVED, LOCKED, PRODUCTION_READY, DEPRECATED | 资产生产生命周期 |
| `production_state`（scene/shot） | DRAFT, READY_FOR_REVIEW, BLOCKED_BY_ASSET, BLOCKED, PRODUCTION_READY, DEPRECATED | 可生产性 |
| `execution_state`（request/result） | PLANNED, SUBMITTED, UNKNOWN, FAILED, COMPLETED, NOT_EXECUTED | 外部执行事实，不能推断媒体存在 |

`LOCKED` 对象不可原地改写。任何变更必须创建新 `object_version`，关联
`CHANGE-REQUEST`、`impact_analysis`，并使原版本进入 `SUPERSEDED` 或保留为可追溯基线。

## 3. Asset Bible

路径：`assets/ASSET-<id>.json`。支持 `Character`、`Prop`、`Vehicle`、`Creature`、
`Costume`、`UI Asset`、`Special Object`。必需字段：

```json
{
  "asset_id": "ASSET-...",
  "asset_type": "Character | Prop | Vehicle | Creature | Costume | UI Asset | Special Object",
  "identity": {"name": "...", "identity_lock": "..."},
  "geometry": {"silhouette": "...", "dimensions": {"unit": "cm", "width": 0, "height": 0, "depth": 0}},
  "appearance": {"materials": [], "colors": []},
  "protected_features": [],
  "mutable_features": [],
  "forbidden_changes": [],
  "reference_assets": [],
  "states": {},
  "views": [],
  "dependencies": [],
  "approval": {"approved_by": "", "approved_at": null},
  "revision_history": []
}
```

Character 必须另有 `character_id`、`body_proportion`、`face_features`、`hair`、
`costume`、`accessories`、`expression_set`、`pose_set`、`state_set`。Prop 必须另有
`prop_id`、`functional_parts`、`holder`、`location`、`damage_state`。角色身份和
角色状态分开：`INJURED`、`WET`、`DIRTY`、`PROP_IN_HAND` 等是状态，绝不创建新身份。

正式镜头仅可依赖 `lifecycle_state: PRODUCTION_READY` 的资产；否则镜头必须为
`BLOCKED_BY_ASSET`，并列出原因。

`approval.approval_state` 在真实项目中只能为 `APPROVED`；`MOCKED_FOR_TEST_ONLY` 仅能出
现在明示 `fixture_mode: true` 的虚构测试项目，绝不代表用户或人工已批准。

## 4. Reference Lock

路径：`references/REFERENCE-LOCKS.json`。每个条目有 `reference_id`、`target_id`、
`target_type`、`lock_level`、`allowed_changes`、`forbidden_changes`、`source_ref`、
`status`。等级：

`L0` Inspiration Only；`L1` Style Reference；`L2` Design Reference；`L3` Identity Lock；
`L4` Geometry Lock；`L5` Exact Asset Reuse。

L3 禁止身份漂移；L4 额外禁止轮廓、比例和主要结构漂移；L5 只允许 pose、expression、
camera、lighting、scene interaction 与条目明确允许的 state 改动。不存在或未锁定的
reference 不得宣称满足对应约束。

## 5. Scene Bible

路径：`scenes/SCENE-<id>.json`。每个正式场景必须有 `scene_id`、`scene_master`、
`floor_plan`、`coordinate_system`、`anchors`、`fixed_props`、`movable_props`、
`entrances`、`exits`、`windows`、`functional_zones`、`camera_map`、`action_axis`、
`lighting`、`environment_state`、`character_blocking` 与 `state_variants`。

坐标为局部逻辑空间：`X=left/right`、`Y=front/back`、`Z=height`。Anchor 使用稳定 ID，
位置优先由 `anchor_id + relative_position + offset` 表达。State variant 必须继承相同
geometry、floor plan 和 anchor map；只能声明剧情造成的状态变化。

Camera Slot 至少有 `camera_id`、`position`、`height`、`direction`、`lens_class`、
`shot_size_range`、`axis_side`。多人关系或明确运动方向使用 `action_axis`；Shot Contract
必须写 `camera_axis_side`，并由校验器给出 `AXIS_OK`、`AXIS_CROSSED` 或
`AXIS_CROSS_JUSTIFIED`。

## 6. Shot Contract 与 Blocking

路径：`shots/SHOT-<id>.json`。正式镜头必须有：

```text
shot_id, sequence_id, scene_id, story_purpose, dramatic_function,
shot_size, camera_angle, camera_height, camera_position, camera_movement,
lens_class, composition, characters, character_states, blocking,
screen_direction, props, prop_states, action_timeline, dialogue, voice,
sfx, ambience, music_policy, ui_overlay, continuity_in, continuity_out,
references, reference_levels, must_preserve, allowed_changes,
forbidden_changes, dependencies, complexity, feasibility,
fallback_strategy, validation_rules
```

`sequence_order` 是可排序的镜头顺序。`blocking` 内每个角色至少有 `start_position`、`end_position`、`anchor_id`、`facing`、
`movement_path`、`interaction_target`、`screen_direction`。`action_timeline` 是按秒且不重叠
的 `[start_s, end_s]` 动作段。`continuity_in` 与 `continuity_out` 是 Ledger entry ID。
同一 Scene 内改变 `camera_axis_side` 必须有 `axis_cross_justification`；没有说明的越轴是
`AXIS_CROSSED` 错误。

## 7. Continuity Ledger

路径：`continuity/CONTINUITY-LEDGER.json`。按镜头顺序记录 `continuity_in` 与
`continuity_out`。它需要覆盖：角色 position/orientation/pose/expression/costume/damage/
wetness/dirt/held_props/state；道具 holder/position/orientation/state/damage/visibility；
场景 lighting/weather/time/door/window/damage/movable prop；屏幕 screen direction/eye line/
axis/entrance/exit。每个镜头读取上一状态并写入下一状态；每个不连续变化必须带
`justification` 和来源。

## 8. Dependency、影响与 Readiness

`dependencies` 是显式对象 ID 列表。依赖图必须能输出 asset/scene/reference 的反向镜头
查询；锁定资产变更必须产生 `impact_analysis`。正式镜头的 Readiness Gate 必须检查：

```text
SCRIPT_LOCKED, STORYBOARD_APPROVED, ASSETS_READY, SCENE_READY,
CONTINUITY_VALID, SHOT_CONTRACT_VALID, REFERENCE_COMPLETE
```

缺一项就是 `BLOCKED` 并有结构化 `BLOCKER`。Gate 输出保存在
`production/READINESS-REPORT.json`；它是派生报告，不替代原始 Bible/Contract。

## 9. 验证与外部边界

生产校验器输出 JSON：`validator`、`status`、`issues`、`counts`、`checked_at`。严重度只可为
`INFO`、`WARNING`、`ERROR`、`BLOCKER`。本契约仅实现文件模式验证、fixture、请求/manifest
边界和确定性编译。任何媒体对象若无实际 `media_ref` 与来源，只能是 `NOT_EXECUTED`、
`UNKNOWN` 或 `MOCKED`，不得作为成片 PASS 证据。

## 10. P1 编译、UI、Audio、Feasibility、QC 与 Rework

`shot_package` 是从已通过 Readiness Gate 的 Shot Contract 派生的生产包：它固定 source
versions、scene/camera/asset/blocking/timeline/continuity/reference，并包含独立的
`audio_contract`（dialogue、voice、sfx、foley、ambience、room_tone、music、silence、
music_policy）和 `ui_contract`（ui_id、shot_id、layout、region、hierarchy、exact_text、
font_role、animation、timing、visibility、compositing）。UI 文本绝不委托给图像/视频模型
“猜对”。

`prompt_manifest` 从 Style Bible、Asset Bible、Scene Bible、Shot Contract、Ledger、
Reference Lock 和 Shot Package 编译 `positive_prompt`、`negative_prompt`、`image_prompt`、
`video_prompt`、`edit_prompt`、`animation_prompt` 与 `reference_instructions`。Negative
constraints 必须从 protected/forbidden 特征、reference level、continuity 和 scene geometry
动态导出，而不是项目专用硬编码。

`shot_feasibility_report` 用 LOW/MEDIUM/HIGH/VERY_HIGH 评估角色、动作、交互、运镜、
UI、特效、环境变化和时长；只可建议 SPLIT_SHOT、SIMPLIFY_CAMERA、SIMPLIFY_ACTION、
SEPARATE_UI、SEPARATE_EFFECT、USE_COMPOSITING，不得擅自修改核心戏剧动作。

真实 `media_qc_report` 的 verdict 才能为 PASS/PASS_WITH_WARNING/FAIL。没有真实可访问
媒体时必须为 NOT_ASSESSABLE；fixture 的模拟失败只能为 `MOCKED_FAILURE`，且必须标明
`evidence_mode: MOCKED`。FAIL 或 MOCKED_FAILURE 可创建 `rework_event`，其中要有 failure
code/evidence/expected/actual/root cause/fix/minimal scope；它表示返工计划，不表示返工或
媒体生成已完成。
