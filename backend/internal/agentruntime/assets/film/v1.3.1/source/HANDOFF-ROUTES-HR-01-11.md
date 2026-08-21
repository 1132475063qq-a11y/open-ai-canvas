# Handoff Routes (HR-01 ~ HR-11) 定义

本文档包含完整的 Handoff Routes 定义，用于插入到 ROUTING-MATRIX.md

---

## 4. Handoff Routes (HR-01 ~ HR-11)

### HR-01: short_drama_planner → narrative_screenwriter

**Handoff ID**: HR-01  
**From Agent**: `short_drama_planner`  
**To Agent**: `narrative_screenwriter`

**输入 Artifacts**:
- `short-drama-plan` (artifact_type)
- artifact_id: `ARTIFACT-XXX-short-drama-plan-v{version}`
- Handoff document: `SHORT-DRAMA-PLAN.md`
- Status: locked

**输出 Artifacts**:
- `script` (artifact_type)
- artifact_id: `ARTIFACT-XXX-script-v{version}`
- Handoff document: `SCRIPT-HANDOFF.md`
- Status: draft → locked (after review)

**触发条件**:
- short_drama_planner 完成策划方案并锁定
- 发送 `request` 消息给 narrative_screenwriter

**工作流程**:
1. short_drama_planner 锁定 `short-drama-plan`
2. 生成 `SHORT-DRAMA-PLAN.md` handoff document
3. 发送 `request` 给 narrative_screenwriter
4. narrative_screenwriter 接收并确认
5. narrative_screenwriter 调用 screenwriter Skill 创作剧本
6. 完成后发送 `completion_notification`

**Message 类型**: `request`

**Message Payload 示例**:
```yaml
message_type: request
handoff_id: HR-01
task_type: screenplay_creation
from_agent: short_drama_planner
to_agent: narrative_screenwriter
inputs:
  - artifact_id: ARTIFACT-001-short-drama-plan-v1.0
    artifact_type: short-drama-plan
    handoff_document: SHORT-DRAMA-PLAN.md
    status: locked
expected_outputs:
  - artifact_type: script
    description: 分集剧本（10集，每集90秒）
```

**停止边界**:
- narrative_screenwriter 完成剧本并经过 script-review
- 剧本状态改为 locked
- 发送 `completion_notification` 给 film_project_lead

**升级条件**:
- 剧本内容与策划方案冲突 → 回复 short_drama_planner 协商
- 分集数量或时长超出约束 → 升级到 film_project_lead

**Human 介入条件**:
- 重大情节或角色调整需要用户确认
- 策划方案与剧本创作意图冲突

**串行/并行规则**:
- 串行：必须等待 short-drama-plan locked 后才能开始
- 下游串行：剧本必须 locked 后才能交给 director_storyboard_artist

---

### HR-02: tvc_creative_director → narrative_screenwriter

**Handoff ID**: HR-02  
**From Agent**: `tvc_creative_director`  
**To Agent**: `narrative_screenwriter`

**输入 Artifacts**:
- `tvc-creative-brief` (artifact_type)
- artifact_id: `ARTIFACT-XXX-tvc-creative-brief-v{version}`
- Handoff document: `TVC-CREATIVE-BRIEF.md`
- Status: locked

**输出 Artifacts**:
- `tvc-script` (artifact_type)
- artifact_id: `ARTIFACT-XXX-tvc-script-v{version}`
- Handoff document: `TVC-SCRIPT-HANDOFF.md`
- Status: draft → locked

**触发条件**:
- tvc_creative_director 完成创意 brief 并锁定
- 需要故事化的 TVC

**Message 类型**: `request`

**停止边界**:
- TVC 剧本锁定并经过审查
- 品牌核心信息确认无误

**升级条件**:
- 故事与品牌定位冲突 → 回复 tvc_creative_director
- 需要修改品牌核心信息 → 升级到 film_project_lead

**Human 介入条件**:
- 品牌信息修改需要用户确认
- 创意风险较高

**串行/并行规则**:
- 串行：必须等待 tvc-creative-brief locked

---

### HR-03: narrative_screenwriter → director_storyboard_artist

**Handoff ID**: HR-03  
**From Agent**: `narrative_screenwriter`  
**To Agent**: `director_storyboard_artist`

**输入 Artifacts**:
- `script` 或 `tvc-script` (artifact_type)
- artifact_id: `ARTIFACT-XXX-script-v{version}`
- Handoff document: `SCRIPT-HANDOFF.md`
- Status: locked (必须经过 script-review)

**输出 Artifacts**:
- `storyboard` (artifact_type)
- artifact_id: `ARTIFACT-XXX-storyboard-v{version}`
- Handoff document: `STORYBOARD-HANDOFF.md`
- Status: draft → locked

**触发条件**:
- narrative_screenwriter 完成剧本并经过 script-review
- 剧本状态改为 locked
- 发送 `completion_notification` 给 director_storyboard_artist

**工作流程**:
1. narrative_screenwriter 完成剧本
2. 调用 script-review Skill 或请求 quality_control_editor 审查
3. 审查通过后锁定剧本
4. 生成 `SCRIPT-HANDOFF.md`
5. 发送 `completion_notification` 给 director_storyboard_artist 和 film_project_lead
6. director_storyboard_artist 接收并创建分镜

**Message 类型**: `completion_notification`

**Message Payload 示例**:
```yaml
message_type: completion_notification
handoff_id: HR-03
topic_id: TOPIC-001-script-creation
from_agent: narrative_screenwriter
to_agent: director_storyboard_artist
completed_at: 2026-08-10T14:00:00Z
final_outputs:
  - artifact_id: ARTIFACT-002-script-v1.0
    artifact_type: script
    handoff_document: SCRIPT-HANDOFF.md
    status: locked
next_steps: 请 director_storyboard_artist 基于剧本创建分镜
```

**停止边界**:
- 剧本经过 script-review 并锁定后，narrative_screenwriter 停止主动修改
- 如需修改，必须通过 CHANGE-REQUEST 流程

**升级条件**:
- 剧本长度或结构超出项目约束 → 升级到 film_project_lead
- 需要重大情节调整 → 写入 NEEDS-YOU.md

**Human 介入条件**:
- 锁定剧本需要用户最终确认
- 重大创意方向调整

**串行/并行规则**:
- 串行：必须等待 script locked
- 下游可以并行：director_storyboard_artist 可以同时启动多个协作（visual, sound）

---

### HR-04: director_storyboard_artist → visual_development_designer

**Handoff ID**: HR-04  
**From Agent**: `director_storyboard_artist`  
**To Agent**: `visual_development_designer`

**输入 Artifacts**:
- `storyboard` (artifact_type)
- artifact_id: `ARTIFACT-XXX-storyboard-v{version}`
- Handoff document: `STORYBOARD-HANDOFF.md`
- Status: locked

**输出 Artifacts**:
- `character-design` (artifact_type)
- `scene-design` (artifact_type)
- `visual-style-guide` (artifact_type)
- Handoff documents: `CHARACTER-DESIGN.md`, `SCENE-DESIGN.md`, `VISUAL-STYLE-GUIDE.md`
- Status: draft → locked

**触发条件**:
- director_storyboard_artist 完成分镜并锁定
- 发送 `request` 给 visual_development_designer

**Message 类型**: `request`

**停止边界**:
- 视觉设计方案锁定
- ai_production_supervisor 确认接收

**升级条件**:
- 视觉风格与剧本冲突 → 回复 director_storyboard_artist
- 资产设计超出制作能力 → 升级到 ai_production_supervisor
- 需要重大风格调整 → 写入 NEEDS-YOU.md

**Human 介入条件**:
- 视觉风格需要用户选择
- 角色或场景设计需要确认

**串行/并行规则**:
- 串行：必须等待 storyboard locked
- 并行：与 HR-05 (sound_designer) 可以并行执行

---

### HR-05: director_storyboard_artist → sound_designer

**Handoff ID**: HR-05  
**From Agent**: `director_storyboard_artist`  
**To Agent**: `sound_designer`

**输入 Artifacts**:
- `script` (artifact_type, locked)
- `storyboard` (artifact_type, locked)
- Handoff document: `STORYBOARD-HANDOFF.md`

**输出 Artifacts**:
- `sound-design-plan` (artifact_type)
- Handoff document: `SOUND-HANDOFF.md`
- Status: draft → locked

**触发条件**:
- director_storyboard_artist 完成分镜
- 需要声音设计协作

**Message 类型**: `request`

**停止边界**:
- 声音设计方案锁定

**升级条件**:
- 声音设计与剧本或视觉冲突 → 回复 director_storyboard_artist 协商
- 音频资源超出项目预算 → 升级到 film_project_lead

**Human 介入条件**:
- 需要定制音乐或音效
- 音频预算超出约束

**串行/并行规则**:
- 串行：必须等待 storyboard locked
- 并行：与 HR-04 (visual_development_designer) 可以并行执行

---

### HR-06: visual_development_designer → ai_production_supervisor

**Handoff ID**: HR-06  
**From Agent**: `visual_development_designer`  
**To Agent**: `ai_production_supervisor`

**输入 Artifacts**:
- `character-design` (artifact_type, locked)
- `scene-design` (artifact_type, locked)
- `visual-style-guide` (artifact_type, locked)
- Handoff documents: `CHARACTER-DESIGN.md`, `SCENE-DESIGN.md`, `VISUAL-STYLE-GUIDE.md`

**输出 Artifacts**:
- `production-feasibility-report` (artifact_type, partial)
- Handoff document: `PRODUCTION-FEASIBILITY-REPORT.md` (视觉部分)

**触发条件**:
- visual_development_designer 完成所有视觉设计并锁定
- 发送 `notification` 给 ai_production_supervisor

**Message 类型**: `notification`

**停止边界**:
- ai_production_supervisor 确认接收设计方案
- 等待 sound_designer 的输入后才能完成完整的可行性报告

**升级条件**:
- 关键资产无法用 AI 实现 → 回复 visual_development_designer 调整
- 需要重大技术调整 → 升级到 film_project_lead

**Human 介入条件**:
- 制作成本或时间超出预算

**串行/并行规则**:
- 并行：与 HR-07 (sound_designer → ai_production_supervisor) 并行
- ai_production_supervisor 需要等待 HR-06 和 HR-07 都完成后才能输出完整报告

---

### HR-07: sound_designer → ai_production_supervisor

**Handoff ID**: HR-07  
**From Agent**: `sound_designer`  
**To Agent**: `ai_production_supervisor`

**输入 Artifacts**:
- `sound-design-plan` (artifact_type, locked)
- Handoff document: `SOUND-HANDOFF.md`

**输出 Artifacts**:
- `production-feasibility-report` (artifact_type, partial)
- Handoff document: `PRODUCTION-FEASIBILITY-REPORT.md` (声音部分)

**触发条件**:
- sound_designer 完成声音设计方案并锁定
- 发送 `notification` 给 ai_production_supervisor

**Message 类型**: `notification`

**停止边界**:
- ai_production_supervisor 确认接收声音方案
- 等待 visual_development_designer 的输入后才能完成完整的可行性报告

**升级条件**:
- 音频方案无法实现 → 回复 sound_designer 调整

**Human 介入条件**:
- 音频预算超出约束

**串行/并行规则**:
- 并行：与 HR-06 (visual_development_designer → ai_production_supervisor) 并行

---

### HR-08: ai_production_supervisor → quality_control_editor

**Handoff ID**: HR-08  
**From Agent**: `ai_production_supervisor`  
**To Agent**: `quality_control_editor`

**输入 Artifacts**:
- `production-feasibility-report` (artifact_type, locked)
- 所有其他 Artifacts (storyboard, designs, sound-design-plan)
- Handoff document: `PRODUCTION-FEASIBILITY-REPORT.md`

**输出 Artifacts**:
- `qc-report` (artifact_type)
- Handoff document: `QC-REPORT.md`

**触发条件**:
- ai_production_supervisor 完成制作可行性报告（合并视觉和声音评估）
- 报告锁定后发送 `notification` 给 quality_control_editor

**Message 类型**: `notification`

**停止边界**:
- quality_control_editor 完成 QC 报告并锁定
- 发送 `completion_notification` 给 film_project_lead

**升级条件**:
- 发现重大质量问题 → 升级到 film_project_lead
- 连续性错误无法修复 → 回复相关 Agent

**Human 介入条件**:
- 技术规范不符合要求需要用户决策

**串行/并行规则**:
- 串行：必须等待 production-feasibility-report locked

---

### HR-09: quality_control_editor → film_project_lead

**Handoff ID**: HR-09  
**From Agent**: `quality_control_editor`  
**To Agent**: `film_project_lead`

**输入 Artifacts**:
- `qc-report` 或 `final-qc-report` (artifact_type, locked)
- 所有其他 Artifacts (完整项目交付物)
- Handoff document: `QC-REPORT.md` 或 `FINAL-QC-REPORT.md`

**输出 Artifacts**:
- `project-summary` (artifact_type)
- Handoff document: `PROJECT-SUMMARY.md`

**触发条件**:
- quality_control_editor 完成质量控制并锁定报告
- 发送 `completion_notification` 给 film_project_lead

**Message 类型**: `completion_notification`

**Message Payload 示例**:
```yaml
message_type: completion_notification
handoff_id: HR-09
topic_id: TOPIC-005-quality-control
from_agent: quality_control_editor
to_agent: film_project_lead
completed_at: 2026-08-12T16:00:00Z
final_outputs:
  - artifact_id: ARTIFACT-010-qc-report-v1.0
    artifact_type: qc-report
    handoff_document: QC-REPORT.md
    status: locked
next_steps: 项目收口，准备交付
```

**停止边界**:
- film_project_lead 完成项目收口报告
- 交付给用户

**升级条件**:
- 交付物不符合要求 → 分配修复任务

**Human 介入条件**:
- 需要对外发布决策
- 交付物需要用户最终确认

**串行/并行规则**:
- 串行：必须等待 qc-report locked
- 最终收口：film_project_lead 是项目级唯一最终收口人

---

### HR-10: film_project_lead → 任意 Agent (项目启动)

**Handoff ID**: HR-10  
**From Agent**: `film_project_lead`  
**To Agent**: 任意 Agent (根据用户需求)

**输入 Artifacts**:
- `project-requirements` (artifact_type)
- 用户提供的资源和时间约束

**输出 Artifacts**:
- Topic 分配
- 初始 `request` 消息

**触发条件**:
- 项目启动
- 用户提供项目需求

**Message 类型**: `request`

**停止边界**:
- 所有 Topics 分配完成
- 相关 Agents 开始工作

**升级条件**:
- 项目范围需要调整 → 写入 NEEDS-YOU.md
- 资源不足 → 写入 NEEDS-YOU.md

**Human 介入条件**:
- 重大项目决策
- 资源分配需要确认

**串行/并行规则**:
- 并行：film_project_lead 可以同时分配多个独立 Topics

---

### HR-11: 任意 Agent → film_project_lead (项目收口)

**Handoff ID**: HR-11  
**From Agent**: 任意 Agent  
**To Agent**: `film_project_lead`

**输入 Artifacts**:
- 各 Topic 的最终 Artifacts (全部 locked)

**输出 Artifacts**:
- `project-summary` (artifact_type)
- Handoff document: `PROJECT-SUMMARY.md`
- Status: locked

**触发条件**:
- 接收到 quality_control_editor 的 completion_notification (HR-09)
- 所有 Topics 完成

**Message 类型**: `completion_notification`

**停止边界**:
- 项目收口报告交付
- 用户确认接收

**升级条件**:
- 交付物不符合要求 → 分配修复任务
- 需要对外发布决策 → 写入 NEEDS-YOU.md

**Human 介入条件**:
- 最终交付需要用户确认

**串行/并行规则**:
- 串行：必须等待所有 Topics 完成
- film_project_lead 是唯一最终 Responsible

---

## 5. 路由统计

### 5.1 Intent Routes 统计

| IR ID | 用户意图 | 主 Agent | 主 Skill |
|---|---|---|---|
| IR-01 | 原创短片故事 | narrative_screenwriter | screenwriter |
| IR-02 | TVC策略 | tvc_creative_director | tvc |
| IR-03 | 故事型TVC | tvc_creative_director | tvc + screenwriter |
| IR-04 | 小说改短剧 | short_drama_planner | short-drama-planning |
| IR-05 | 写单集剧本 | narrative_screenwriter | screenwriter |
| IR-06 | 检查剧本 | quality_control_editor | script-review |
| IR-07 | 做分镜 | director_storyboard_artist | director-storyboard |
| IR-08 | 视频生成提示词 | ai_production_supervisor | ai-video-shot-prompt |
| IR-09 | 角色设计 | visual_development_designer | character-visual-design |
| IR-10 | 场景设计 | visual_development_designer | scene-asset-design |
| IR-11 | 世界观 | narrative_screenwriter | worldbuilding-management |
| IR-12 | 连续性检查 | quality_control_editor | continuity-check |
| IR-13 | 声音方案 | sound_designer | sound-design-plan |
| IR-14 | 制作可行性 | ai_production_supervisor | ai-production-feasibility-review |
| IR-15 | 成片审查 | quality_control_editor | final-film-quality-review |

**总计**: 15 条 Intent Routes

### 5.2 Handoff Routes 统计

| HR ID | From Agent | To Agent | Message Type |
|---|---|---|---|
| HR-01 | short_drama_planner | narrative_screenwriter | request |
| HR-02 | tvc_creative_director | narrative_screenwriter | request |
| HR-03 | narrative_screenwriter | director_storyboard_artist | completion_notification |
| HR-04 | director_storyboard_artist | visual_development_designer | request |
| HR-05 | director_storyboard_artist | sound_designer | request |
| HR-06 | visual_development_designer | ai_production_supervisor | notification |
| HR-07 | sound_designer | ai_production_supervisor | notification |
| HR-08 | ai_production_supervisor | quality_control_editor | notification |
| HR-09 | quality_control_editor | film_project_lead | completion_notification |
| HR-10 | film_project_lead | 任意 Agent | request |
| HR-11 | 任意 Agent | film_project_lead | completion_notification |

**总计**: 11 条 Handoff Routes

### 5.3 Artifact 流转路径

```
用户需求 (project-requirements)
    ↓
策划方案 (short-drama-plan / tvc-creative-brief)
    ↓ HR-01 / HR-02
剧本 (script / tvc-script)
    ↓ HR-03
分镜脚本 (storyboard)
    ↓ HR-04 / HR-05 (并行)
┌─────────┴─────────┐
角色设计  场景设计  声音设计
character-design  scene-design  sound-design-plan
└─────────┬─────────┘
    ↓ HR-06 / HR-07 (并行)
制作可行性报告 (production-feasibility-report)
    ↓ HR-08
QC 报告 (qc-report / final-qc-report)
    ↓ HR-09
项目收口报告 (project-summary)
    ↓ HR-11
交付给用户
```

---

**最后更新**: 2026-08-08  
**版本**: 1.0

## V1.3 Skill 挂载说明

V1.3 不新增 HR-12 等新的长期 Agent→Agent 边；ACTING、CINEDANCE、LIRA 都在既有 Domain/Handoff 内作为 Task/RoutingDecision 选中的专业 Skill。

- 编剧/导演交接中可携带 `character-acting-profile` / `scene-acting-adaptation`。
- 视觉开发→AI制作可携带 `image-prompt-pack` 及通过 QC 的 reference refs。
- AI制作仍由 `ai-video-shot-prompt` 作为总入口；目标为 Seedance/Higgsfield 时在同一 Responsible 下调用 `cinedance-video-director`。

因此 HR 仍为 11 条，避免因方法 Skill 增长破坏长期 Agent 责任拓扑。
