# Creative Depth V1.3 Contract

**schema_version**: `3.1`  
**状态**: `DRAFT`  
**Responsible**: `film_project_lead`

这是一份对 `PRODUCTION-SYSTEM-V2.md` 的增量合同。它不替换 V2 JSON 对象，不自动选择主角、类型、主题、结局或风格；这些仍须由 Human 决定并记录在 `NEEDS-YOU.md`。

## A. Character Asset 的 Embodiment Spine

Character Asset 可增加 `embodiment_spine`。当项目 `creative_depth_v13: true` 且角色进入正式 Shot Contract 时，它必须有：

```json
{
  "causal_core": {
    "goal": "confirmed fact or NEEDS_YOU",
    "fear_or_blindspot": "confirmed fact or NEEDS_YOU",
    "pressure_trigger": "observable trigger",
    "value_conflict": "choice under pressure"
  },
  "observable_behavior": {
    "choice_pattern": [], "gesture_posture": [], "action_limits": [],
    "dialogue_evidence_refs": []
  },
  "voice_contract": {
    "voice_profile_ref": "SOUND-BIBLE / Audio Contract reference",
    "evidence_refs": [], "allowed_contrast": [], "forbidden_inference": ["do not infer appearance from voice"]
  },
  "visual_identity_refs": {
    "identity_lock": "must equal or reference asset identity lock",
    "protected_features": [], "state_separation": "identity is not state"
  },
  "consistency_checks": []
}
```

`voice_contract` 不得成为由外貌推导声线或由声线推导外貌的规则。允许反差，但必须有明确剧情功能和证据。

## B. Story Pressure Profile

路径建议：`narrative/STORY-TYPE-PROFILE.json`。`object_type` 使用既有 `story_type_profile`，不新增重复类型。

```json
{
  "primary_type": "MONSTER_IN_THE_HOUSE | GOLDEN_FLEECE | OUT_OF_THE_BOTTLE | DUDE_WITH_A_PROBLEM | RITES_OF_PASSAGE | BUDDY_LOVE | WHYDUNIT | FOOL_TRIUMPHANT | INSTITUTIONALIZED | SUPERHERO",
  "secondary_mechanisms": [],
  "approval_state": "NEEDS_YOU | APPROVED | MOCKED_FOR_TEST_ONLY",
  "audience_question": "...",
  "pressure_chain": [],
  "required_choices": [],
  "information_release": [],
  "unit_escalation": [],
  "failure_signals": [],
  "exclusion_reasons": [],
  "character_pressure_refs": [
    {
      "asset_id": "ASSET-CHAR-<ID>",
      "causal_statement": "该角色在此类型压力下必须面对的选择或代价"
    }
  ]
}
```

一个 Profile 只有一个主类型。类型影响压力、选择和后果，不是题材、情绪或营销标签。真实项目不得把 `MOCKED_FOR_TEST_ONLY` 当作批准。
`character_pressure_refs` 是 Canvas 创作因果视图的唯一类型→角色连接依据：没有明确
`asset_id` 与 `causal_statement` 时，Canvas 只能显示两个孤立节点，不能补画因果线。

## C. Observable Image Specification

Shot Contract 与 Prompt Manifest 可使用 `observable_image_spec`：

```json
{
  "frame": {"aspect_ratio": "...", "duration_s": 0},
  "environment": {"time_weather": "...", "visible_anchors": [], "lighting_evidence": []},
  "visible_characters": [{"asset_id": "...", "identity_features": [], "state": "...", "position": "...", "facing": "...", "action": "..."}],
  "visible_props": [{"asset_id": "...", "geometry_features": [], "holder_or_position": "...", "state": "..."}],
  "camera": {"shot_size": "...", "position": "...", "angle": "...", "lens_class": "...", "composition": "...", "movement": {"type": "...", "reason": "...", "timing": "..."}},
  "action_beats": [{"start_s": 0, "end_s": 0, "visible_action": "..."}],
  "atmosphere_evidence": {"light": [], "color": [], "distance_or_occlusion": [], "performance": [], "sound": []},
  "ui_audio": {"ui": [], "audio": []},
  "acceptance_checks": [],
  "minimal_rework_scope": []
}
```

抽象词只能作为检索标签，不能代替 `atmosphere_evidence`；运镜 `reason` 与 `timing` 均不可为空。Prompt 编译器必须保持独立镜头，不得用“同上/继续/上一镜”等跨镜依赖。

## D. 兼容和状态

- 既有 V2 项目没有 `creative_depth_v13` 即视为未迁移，仍按 V2 校验。
- V1.3 字段只在显式开启的项目中是 required；DRAFT/NEEDS_YOU 不是生产 ready。
- 所有执行边界保持不变：文档/编译不是 Agent runtime，Prompt 不是生成结果，无媒体不能 PASS。
