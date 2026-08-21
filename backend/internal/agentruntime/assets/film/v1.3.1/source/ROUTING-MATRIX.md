# Agent 路由矩阵（扩展版）

本文档定义 Agent 间的有方向协作接口，包括输入输出 Artifact、触发条件、停止边界、升级条件和消息类型。

---

## 1. 路由矩阵总览

### 1.1 协作拓扑

```
项目启动
    ↓
film_project_lead (项目 Lead)
    ↓
┌───────────────────┬─────────────────────┐
│                   │                     │
short_drama_planner tvc_creative_director  │
    ↓                   ↓                 │
narrative_screenwriter ←┘                 │
    ↓                                     │
director_storyboard_artist                │
    ↓                                     │
┌───────┴────────┬──────────────────┐    │
│                │                  │    │
visual_development_designer  sound_designer  │
    └────────────┴──────────────────┘    │
                 ↓                        │
        ai_production_supervisor          │
                 ↓                        │
        quality_control_editor            │
                 ↓                        │
        film_project_lead (收口) ─────────┘
```

---

## 2. 路由体系说明

本矩阵定义两种路由：

### 2.1 Intent Routes (IR-01 ~ IR-15)

**定义**: 用户需求 → 主 Agent → 主 Skill → Topic

Intent Routes 描述用户如何触发 Agent Team 工作，每条 IR 定义：
- 用户触发条件（关键词、需求模式）
- 负责的主 Agent 和主 Skill
- Topic Responsible（唯一负责人）
- 输入/输出 Artifact 类型
- 可协作的 Agent 和 Skill
- 上游依赖和下游交接
- 停止边界和 Human 升级条件

### 2.2 Handoff Routes (HR-01 ~ HR-11)

**定义**: Agent → Agent 协作路由

Handoff Routes 描述 Agent 间如何交接工作，每条 HR 定义：
- 发送方和接收方 Agent
- 输入/输出 Artifact 类型和 Handoff 文档
- 触发条件和消息类型
- 停止边界和升级条件
- 串行/并行规则

---

## 3. Intent Routes (IR-01 ~ IR-15)

### IR-01: 原创短片故事

**用户触发条件**:
- "写一个原创短片故事"
- "帮我创作一个3分钟短片剧本"
- "我想做一个关于XX主题的短片"

**主 Agent**: `narrative_screenwriter`  
**主 Skill**: `screenwriter`  
**Topic Responsible**: `narrative_screenwriter`

**输入 Artifacts**:
- `project-requirements` (可选，由 film_project_lead 提供)

**输出 Artifacts**:
- `script` (artifact_type)
- Handoff document: `SCRIPT-HANDOFF.md`

**可协作 Agents**: 
- `film_project_lead` (项目整合)
- `quality_control_editor` (剧本审查)

**可协作 Skills**:
- `worldbuilding-management` (世界观设定)
- `script-review` (剧本审查)

**Message 类型**: `request` (从 film_project_lead) 或直接启动

**上游依赖**: 无（可以作为项目起点）

**下游交接**: 
- 通常触发 HR-03 (narrative_screenwriter → director_storyboard_artist)
- 或触发 IR-06 (剧本审查)

**停止边界**:
- 剧本经过 script-review 并锁定后，narrative_screenwriter 停止主动修改
- 发送 `completion_notification` 给 film_project_lead

**Human 升级条件**:
- 核心创意方向需要用户选择
- 主题或结局需要用户确认
- 剧本长度超出项目约束

---

### IR-02: TVC策略

**用户触发条件**:
- "帮我做一个TVC策略"
- "品牌XX需要一个广告创意"
- "我要做产品YY的TVC"

**主 Agent**: `tvc_creative_director`  
**主 Skill**: `tvc`  
**Topic Responsible**: `tvc_creative_director`

**输入 Artifacts**:
- `project-requirements` (必需，包含品牌资料)

**输出 Artifacts**:
- `tvc-creative-brief` (artifact_type)
- Handoff document: `TVC-CREATIVE-BRIEF.md`

**可协作 Agents**:
- `film_project_lead` (项目整合)
- `narrative_screenwriter` (故事型TVC需要)

**可协作 Skills**:
- `screenwriter` (故事型TVC)
- `ai-production-feasibility-review` (可行性评估)

**Message 类型**: `request` (从 film_project_lead)

**上游依赖**: 
- 需要 `project-requirements` (品牌资料、传播目标)

**下游交接**:
- 纯策略型：直接给 film_project_lead 收口
- 故事型：触发 HR-02 (tvc_creative_director → narrative_screenwriter)

**停止边界**:
- 创意 brief 锁定后，tvc_creative_director 停止主动修改
- 如需故事化，交接给 narrative_screenwriter

**Human 升级条件**:
- 品牌核心信息需要确认
- 创意方向与品牌定位冲突
- 传播目标不明确

---

### IR-03: 故事型TVC

**用户触发条件**:
- "写一个故事型TVC剧本"
- "品牌XX的广告需要讲一个故事"
- "把这个创意变成一个微电影广告"

**主 Agent**: `tvc_creative_director`  
**主 Skill**: `tvc` + `screenwriter` (协作)  
**Topic Responsible**: `tvc_creative_director`

**输入 Artifacts**:
- `project-requirements` (品牌资料)
- `tvc-creative-brief` (可选，如果已有策略)

**输出 Artifacts**:
- `tvc-script` (artifact_type)
- Handoff document: `TVC-SCRIPT-HANDOFF.md`

**可协作 Agents**:
- `narrative_screenwriter` (剧本创作)
- `quality_control_editor` (审查)

**可协作 Skills**:
- `screenwriter` (故事创作)
- `script-review` (审查)

**Message 类型**: `request`

**上游依赖**:
- 需要品牌资料和创意方向

**下游交接**:
- 触发 HR-03 (narrative_screenwriter → director_storyboard_artist)
- 或触发 IR-06 (剧本审查)

**停止边界**:
- TVC 剧本经过审查并锁定
- 品牌核心信息确认无误

**Human 升级条件**:
- 品牌信息修改
- 产品事实需要确认
- 创意风险较高

---

### IR-04: 小说改短剧

**用户触发条件**:
- "把这个小说改编成短剧"
- "帮我把XX故事改成10集短剧"
- "做一个小说改编的短剧策划"

**主 Agent**: `short_drama_planner`  
**主 Skill**: `short-drama-planning`  
**Topic Responsible**: `short_drama_planner`

**输入 Artifacts**:
- `project-requirements` (包含原小说、平台要求)

**输出 Artifacts**:
- `short-drama-plan` (artifact_type)
- Handoff document: `SHORT-DRAMA-PLAN.md`

**可协作 Agents**:
- `narrative_screenwriter` (后续剧本创作)
- `film_project_lead` (项目整合)

**可协作 Skills**:
- `worldbuilding-management` (原作世界观)
- `continuity-check` (改编一致性)

**Message 类型**: `request`

**上游依赖**:
- 需要原小说或故事素材

**下游交接**:
- 触发 HR-01 (short_drama_planner → narrative_screenwriter)

**停止边界**:
- 分集策划方案锁定
- 改编策略确认

**Human 升级条件**:
- 分集数量或时长超出约束
- 原作改编尺度需要确认
- IP授权问题

---

### IR-05: 写单集剧本

**用户触发条件**:
- "写第X集的剧本"
- "帮我创作这一集的剧本"
- "基于这个大纲写剧本"

**主 Agent**: `narrative_screenwriter`  
**主 Skill**: `screenwriter`  
**Topic Responsible**: `narrative_screenwriter`

**输入 Artifacts**:
- `short-drama-plan` (如果是短剧的一集)
- `worldbuilding-bible` (如果已有)
- `project-requirements` (可选)

**输出 Artifacts**:
- `script` (artifact_type)
- Handoff document: `SCRIPT-HANDOFF.md`

**可协作 Agents**:
- `quality_control_editor` (审查)
- `short_drama_planner` (策划对齐)

**可协作 Skills**:
- `worldbuilding-management` (世界观)
- `script-review` (审查)

**Message 类型**: `request` (从 short_drama_planner 或 film_project_lead)

**上游依赖**:
- 如果是短剧，需要 `short-drama-plan`

**下游交接**:
- 触发 IR-06 (剧本审查)
- 或触发 HR-03 (进入分镜阶段)

**停止边界**:
- 剧本经过审查并锁定

**Human 升级条件**:
- 剧情与策划方案冲突
- 重大情节或角色调整

---

### IR-06: 检查剧本

**用户触发条件**:
- "帮我审查这个剧本"
- "检查剧本的连续性和质量"
- "对剧本做专业审查"

**主 Agent**: `quality_control_editor`  
**主 Skill**: `script-review`  
**Topic Responsible**: `quality_control_editor`

**输入 Artifacts**:
- `script` (必需)

**输出 Artifacts**:
- `script-review-report` (artifact_type)
- Handoff document: `SCRIPT-REVIEW-REPORT.md`

**可协作 Agents**:
- `narrative_screenwriter` (根据审查意见修订)

**可协作 Skills**:
- `continuity-check` (连续性检查)

**Message 类型**: `request` (从 narrative_screenwriter 或 film_project_lead)

**上游依赖**:
- 需要 `script` (draft 状态即可)

**下游交接**:
- 如果审查不通过，返回 narrative_screenwriter 修订
- 如果审查通过，script 状态改为 locked，触发后续流程

**停止边界**:
- 审查报告交付并确认

**Human 升级条件**:
- 发现重大剧本问题需要用户决策
- 审查意见与创作意图冲突

---

### IR-07: 做分镜

**用户触发条件**:
- "基于剧本做分镜"
- "帮我创建分镜脚本"
- "把这个剧本转成分镜"

**主 Agent**: `director_storyboard_artist`  
**主 Skill**: `director-storyboard`  
**Topic Responsible**: `director_storyboard_artist`

**输入 Artifacts**:
- `script` (必需，locked 状态)

**输出 Artifacts**:
- `storyboard` (artifact_type)
- Handoff document: `STORYBOARD-HANDOFF.md`

**可协作 Agents**:
- `sound_designer` (声音设计协作)
- `visual_development_designer` (视觉设计协作)

**可协作 Skills**:
- `continuity-check` (连续性检查)

**Message 类型**: `completion_notification` (从 narrative_screenwriter)

**上游依赖**:
- 需要 `script` (locked)

**下游交接**:
- 触发 HR-04 (director_storyboard_artist → visual_development_designer)
- 触发 HR-05 (director_storyboard_artist → sound_designer)

**停止边界**:
- 分镜脚本锁定后不再改变镜头结构

**Human 升级条件**:
- 分镜复杂度超出制作能力
- 镜头设计与剧本冲突
- 需要重大视觉调整

---

### IR-08: 视频生成提示词

**用户触发条件**:
- "生成AI视频提示词"
- "把分镜转成Sora提示词"
- "创建视频生成prompts"

**主 Agent**: `ai_production_supervisor`  
**主 Skill**: `ai-video-shot-prompt`  
**Topic Responsible**: `ai_production_supervisor`

**输入 Artifacts**:
- `storyboard` (必需)
- `character-design` (推荐)
- `scene-design` (推荐)
- `visual-style-guide` (推荐)

**输出 Artifacts**:
- `ai-video-prompts` (artifact_type)
- Handoff document: `AI-VIDEO-PROMPTS.md`

**可协作 Agents**:
- `director_storyboard_artist` (镜头对齐)
- `visual_development_designer` (视觉对齐)

**可协作 Skills**:
- `director-storyboard` (分镜理解)
- `cinedance-video-director` (Seedance/Higgsfield 专业编译)
- `character-acting-system` (人物 performance layer)

**Message 类型**: `request`

**上游依赖**:
- 需要 `storyboard` (locked)

**下游交接**:
- 通常作为最终输出，交给 film_project_lead 收口
- 或进入实际视频生成流程（超出本 Agent Team 范围）

**停止边界**:
- 提示词集合交付并确认

**Human 升级条件**:
- 提示词无法准确描述镜头需求
- AI 生成能力限制需要调整分镜

---

### IR-09: 角色设计

**用户触发条件**:
- "设计XX角色"
- "帮我做角色视觉设计"
- "创建角色设定图"

**主 Agent**: `visual_development_designer`  
**主 Skill**: `character-visual-design`  
**Topic Responsible**: `visual_development_designer`

**输入 Artifacts**:
- `script` (必需，包含角色描述)
- `storyboard` (推荐，了解角色出场)
- `worldbuilding-bible` (推荐，世界观对齐)

**输出 Artifacts**:
- `character-design` (artifact_type)
- Handoff document: `CHARACTER-DESIGN.md`

**可协作 Agents**:
- `narrative_screenwriter` (角色设定对齐)
- `ai_production_supervisor` (制作可行性)

**可协作 Skills**:
- `worldbuilding-management` (世界观一致性)
- `continuity-check` (设计一致性)
- `lira-image-prompts` (角色图像 prompt / 编辑路由)

**Message 类型**: `request` (从 director_storyboard_artist)

**上游依赖**:
- 需要 `script` (至少 draft)

**下游交接**:
- 触发 HR-06 (visual_development_designer → ai_production_supervisor)

**停止边界**:
- 角色设计锁定

**Human 升级条件**:
- 角色视觉风格需要用户选择
- 设计与剧本描述冲突

---

### IR-10: 场景设计

**用户触发条件**:
- "设计XX场景"
- "帮我做场景视觉设计"
- "创建场景资产设定"

**主 Agent**: `visual_development_designer`  
**主 Skill**: `scene-asset-design`  
**Topic Responsible**: `visual_development_designer`

**输入 Artifacts**:
- `script` (必需，包含场景描述)
- `storyboard` (必需，场景镜头需求)
- `worldbuilding-bible` (推荐)

**输出 Artifacts**:
- `scene-design` (artifact_type)
- Handoff document: `SCENE-DESIGN.md`

**可协作 Agents**:
- `director_storyboard_artist` (场景调度对齐)
- `ai_production_supervisor` (制作可行性)

**可协作 Skills**:
- `worldbuilding-management` (世界观一致性)
- `continuity-check` (空间一致性)
- `lira-image-prompts` (场景图像 prompt / 反打与新视角)

**Message 类型**: `request` (从 director_storyboard_artist)

**上游依赖**:
- 需要 `storyboard` (locked)

**下游交接**:
- 触发 HR-06 (visual_development_designer → ai_production_supervisor)

**停止边界**:
- 场景设计锁定

**Human 升级条件**:
- 场景复杂度超出制作能力
- 设计与剧本冲突

---

### IR-11: 世界观

**用户触发条件**:
- "建立XX世界观"
- "帮我做世界观设定"
- "创建story bible"

**主 Agent**: `narrative_screenwriter` (故事世界观) 或 `visual_development_designer` (视觉世界观)  
**主 Skill**: `worldbuilding-management`  
**Topic Responsible**: `narrative_screenwriter` (默认) 或 `visual_development_designer` (视觉为主时)

**输入 Artifacts**:
- `script` (推荐，提取世界观元素)
- `project-requirements` (可选)

**输出 Artifacts**:
- `worldbuilding-bible` (artifact_type)
- Handoff document: `WORLDBUILDING-BIBLE.md`

**可协作 Agents**:
- `narrative_screenwriter` 和 `visual_development_designer` 协作
- `short_drama_planner` (系列世界观)

**可协作 Skills**:
- `screenwriter` (故事世界观)
- `character-visual-design` (角色世界观)
- `scene-asset-design` (场景世界观)

**Message 类型**: `request`

**上游依赖**:
- 通常在项目早期创建，或在创作过程中逐步完善

**下游交接**:
- 作为其他 Artifacts 的参考输入（script, character-design, scene-design）

**停止边界**:
- 世界观 bible 锁定
- 核心设定确认

**Human 升级条件**:
- 世界观规则需要用户确认
- 世界观与已有内容冲突

---

### IR-12: 连续性检查

**用户触发条件**:
- "检查连续性"
- "审查剧本/分镜的一致性"
- "找出连续性错误"

**主 Agent**: `quality_control_editor`  
**主 Skill**: `continuity-check`  
**Topic Responsible**: `quality_control_editor`

**输入 Artifacts**:
- `script` (可选)
- `storyboard` (可选)
- `character-design` (可选)
- `scene-design` (可选)
- `worldbuilding-bible` (推荐)

**输出 Artifacts**:
- `continuity-report` (artifact_type)
- Handoff document: `CONTINUITY-REPORT.md`

**可协作 Agents**:
- `narrative_screenwriter` (剧本连续性)
- `director_storyboard_artist` (分镜连续性)
- `visual_development_designer` (视觉连续性)

**可协作 Skills**:
- `worldbuilding-management` (设定一致性)

**Message 类型**: `request`

**上游依赖**:
- 需要至少一种 Artifact (script, storyboard, design)

**下游交接**:
- 如发现问题，返回相关 Agent 修复
- 如无问题，报告交给 film_project_lead

**停止边界**:
- 连续性报告交付

**Human 升级条件**:
- 发现无法自动解决的连续性冲突

---

### IR-13: 声音方案

**用户触发条件**:
- "设计声音方案"
- "帮我做声音设计"
- "创建音效和音乐策略"

**主 Agent**: `sound_designer`  
**主 Skill**: `sound-design`  
**Topic Responsible**: `sound_designer`

**输入 Artifacts**:
- `script` (推荐)
- `storyboard` (必需)
- `visual-style-guide` (推荐)

**输出 Artifacts**:
- `sound-design-plan` (artifact_type)
- Handoff document: `SOUND-HANDOFF.md`

**可协作 Agents**:
- `director_storyboard_artist` (镜头声音对齐)
- `ai_production_supervisor` (制作可行性)

**可协作 Skills**:
- `director-storyboard` (理解镜头需求)

**Message 类型**: `request` (从 director_storyboard_artist)

**上游依赖**:
- 需要 `storyboard` (locked)

**下游交接**:
- 触发 HR-07 (sound_designer → ai_production_supervisor)

**停止边界**:
- 声音设计方案锁定

**Human 升级条件**:
- 音频资源超出项目预算
- 需要定制音乐或音效

---

### IR-14: 制作可行性

**用户触发条件**:
- "评估制作可行性"
- "检查能否用AI实现"
- "做技术可行性分析"

**主 Agent**: `ai_production_supervisor`  
**主 Skill**: `ai-production-feasibility-review`  
**Topic Responsible**: `ai_production_supervisor`

**输入 Artifacts**:
- `storyboard` (必需)
- `character-design` (可选)
- `scene-design` (可选)
- `sound-design-plan` (可选)

**输出 Artifacts**:
- `production-feasibility-report` (artifact_type)
- Handoff document: `PRODUCTION-FEASIBILITY-REPORT.md`

**可协作 Agents**:
- `director_storyboard_artist` (分镜调整)
- `visual_development_designer` (设计调整)

**可协作 Skills**:
- `ai-video-shot-prompt` (生成提示词)

**Message 类型**: `request`

**上游依赖**:
- 需要 HR-06, HR-07 完成（设计和声音方案）

**下游交接**:
- 触发 HR-08 (ai_production_supervisor → quality_control_editor)

**停止边界**:
- 可行性报告交付
- AI 生成提示词准备完毕

**Human 升级条件**:
- 发现无法用现有技术实现的需求
- 需要额外预算或资源

---

### IR-15: 成片审查

**用户触发条件**:
- "审查成片质量"
- "做最终质量检查"
- "检查是否达到交付标准"

**主 Agent**: `quality_control_editor`  
**主 Skill**: `final-film-quality-review`  
**Topic Responsible**: `quality_control_editor`

**输入 Artifacts**:
- `production-feasibility-report` (必需)
- 所有其他 Artifacts (script, storyboard, designs, sound-design-plan)

**输出 Artifacts**:
- `final-qc-report` (artifact_type)
- Handoff document: `FINAL-QC-REPORT.md`

**可协作 Agents**:
- `film_project_lead` (最终收口)

**可协作 Skills**:
- `continuity-check` (连续性)
- `script-review` (剧本对照)

**Message 类型**: `completion_notification` (从 ai_production_supervisor)

**上游依赖**:
- 需要 HR-08 完成（制作可行性确认）

**下游交接**:
- 触发 HR-09 (quality_control_editor → film_project_lead)

**停止边界**:
- 最终审查报告交付
- 项目进入收口阶段

**Human 升级条件**:
- 发现重大质量问题
- 交付物不符合要求
- 需要对外发布决策

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

**Message 类型**: `request`

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
- `script` (artifact_type)
- artifact_id: `ARTIFACT-XXX-tvc-script-v{version}`
- Handoff document: `SCRIPT-HANDOFF.md`
- Status: draft → locked (after review)

**触发条件**:
- tvc_creative_director 完成创意 brief 并锁定
- 发送 `request` 消息给 narrative_screenwriter

**Message 类型**: `request`

**停止边界**:
- narrative_screenwriter 完成 TVC 脚本并经过 script-review
- 脚本状态改为 locked
- 发送 `completion_notification` 给 film_project_lead

**升级条件**:
- 脚本内容与品牌定位冲突 → 回复 tvc_creative_director 协商
- 创意执行超出预算或技术能力 → 升级到 film_project_lead

**Human 介入条件**:
- 品牌核心信息需要调整
- 创意方向需要客户确认

**串行/并行规则**:
- 串行：必须等待 tvc-creative-brief locked 后才能开始
- 下游串行：脚本必须 locked 后才能交给 director_storyboard_artist

---

### HR-03: narrative_screenwriter → director_storyboard_artist

**Handoff ID**: HR-03  
**From Agent**: `narrative_screenwriter`  
**To Agent**: `director_storyboard_artist`

**输入 Artifacts**:
- `script` (artifact_type)
- artifact_id: `ARTIFACT-XXX-script-v{version}`
- Handoff document: `SCRIPT-HANDOFF.md`
- Status: locked

**输出 Artifacts**:
- `storyboard` (artifact_type)
- artifact_id: `ARTIFACT-XXX-storyboard-v{version}`
- Handoff document: `STORYBOARD-HANDOFF.md`
- Status: draft → locked

**触发条件**:
- narrative_screenwriter 完成剧本并锁定
- 发送 `completion_notification` 给 director_storyboard_artist

**Message 类型**: `completion_notification`

**停止边界**:
- director_storyboard_artist 完成分镜并锁定
- 发送 `completion_notification` 给下游 Agents (visual/sound)

**升级条件**:
- 剧本可视化存在技术难度 → 回复 narrative_screenwriter 协商
- 分镜数量或复杂度超出预算 → 升级到 film_project_lead

**Human 介入条件**:
- 关键镜头设计需要用户确认
- 视觉风格选择

**串行/并行规则**:
- 串行：必须等待 script locked 后才能开始
- 下游并行：可同时触发 HR-04 和 HR-05

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
- `character-design` (artifact_type) 和/或 `scene-design` (artifact_type)
- artifact_id: `ARTIFACT-XXX-character-design-v{version}`, `ARTIFACT-XXX-scene-design-v{version}`
- Handoff documents: `CHARACTER-DESIGN.md`, `SCENE-DESIGN.md`
- Status: draft → locked

**触发条件**:
- director_storyboard_artist 完成分镜并锁定
- 发送 `request` 消息给 visual_development_designer

**Message 类型**: `request`

**停止边界**:
- visual_development_designer 完成角色和场景设计并锁定
- 发送 `completion_notification` 给 ai_production_supervisor

**升级条件**:
- 设计需求与分镜不匹配 → 回复 director_storyboard_artist 协商
- 设计复杂度超出制作能力 → 升级到 film_project_lead

**Human 介入条件**:
- 角色或场景视觉风格需要用户选择
- 设计与品牌形象冲突

**串行/并行规则**:
- 串行：必须等待 storyboard locked 后才能开始
- 下游串行：设计必须 locked 后才能交给 ai_production_supervisor
- 可与 HR-05 并行

---

### HR-05: director_storyboard_artist → sound_designer

**Handoff ID**: HR-05  
**From Agent**: `director_storyboard_artist`  
**To Agent**: `sound_designer`

**输入 Artifacts**:
- `storyboard` (artifact_type)
- artifact_id: `ARTIFACT-XXX-storyboard-v{version}`
- Handoff document: `STORYBOARD-HANDOFF.md`
- Status: locked

**输出 Artifacts**:
- `sound-design-plan` (artifact_type)
- artifact_id: `ARTIFACT-XXX-sound-design-plan-v{version}`
- Handoff document: `SOUND-HANDOFF.md`
- Status: draft → locked

**触发条件**:
- director_storyboard_artist 完成分镜并锁定
- 发送 `request` 消息给 sound_designer

**Message 类型**: `request`

**停止边界**:
- sound_designer 完成声音设计方案并锁定
- 发送 `completion_notification` 给 ai_production_supervisor

**升级条件**:
- 声音需求与分镜不匹配 → 回复 director_storyboard_artist 协商
- 音频资源超出预算 → 升级到 film_project_lead

**Human 介入条件**:
- 音乐风格需要用户选择
- 需要定制音效或音乐

**串行/并行规则**:
- 串行：必须等待 storyboard locked 后才能开始
- 下游串行：声音方案必须 locked 后才能交给 ai_production_supervisor
- 可与 HR-04 并行

---

### HR-06: visual_development_designer → ai_production_supervisor

**Handoff ID**: HR-06  
**From Agent**: `visual_development_designer`  
**To Agent**: `ai_production_supervisor`

**输入 Artifacts**:
- `character-design` (artifact_type) 和/或 `scene-design` (artifact_type)
- artifact_id: `ARTIFACT-XXX-character-design-v{version}`, `ARTIFACT-XXX-scene-design-v{version}`
- Handoff documents: `CHARACTER-DESIGN.md`, `SCENE-DESIGN.md`
- Status: locked

**输出 Artifacts**:
- `production-feasibility-report` (artifact_type, partial)
- artifact_id: `ARTIFACT-XXX-production-feasibility-v{version}`
- Status: draft (等待 sound-design-plan 后 locked)

**触发条件**:
- visual_development_designer 完成设计并锁定
- 发送 `completion_notification` 给 ai_production_supervisor

**Message 类型**: `completion_notification`

**停止边界**:
- ai_production_supervisor 接收设计并开始可行性评估
- 等待 HR-07 完成后综合评估

**升级条件**:
- 设计无法用现有 AI 技术实现 → 回复 visual_development_designer 协商
- 需要额外预算或技术资源 → 升级到 film_project_lead

**Human 介入条件**:
- 发现技术限制需要用户决策
- 需要调整设计以适应制作能力

**串行/并行规则**:
- 串行：必须等待 character/scene-design locked 后才能开始
- 下游等待：需等待 HR-07 (sound) 完成后才能 locked 并交给 quality_control_editor

---

### HR-07: sound_designer → ai_production_supervisor

**Handoff ID**: HR-07  
**From Agent**: `sound_designer`  
**To Agent**: `ai_production_supervisor`

**输入 Artifacts**:
- `sound-design-plan` (artifact_type)
- artifact_id: `ARTIFACT-XXX-sound-design-plan-v{version}`
- Handoff document: `SOUND-HANDOFF.md`
- Status: locked

**输出 Artifacts**:
- `production-feasibility-report` (artifact_type, 完整版)
- artifact_id: `ARTIFACT-XXX-production-feasibility-v{version}`
- Handoff document: `PRODUCTION-FEASIBILITY-REPORT.md`
- Status: locked

**触发条件**:
- sound_designer 完成声音方案并锁定
- 发送 `completion_notification` 给 ai_production_supervisor

**Message 类型**: `completion_notification`

**停止边界**:
- ai_production_supervisor 完成综合可行性评估并锁定
- 发送 `completion_notification` 给 quality_control_editor

**升级条件**:
- 声音方案无法实现 → 回复 sound_designer 协商
- 需要额外音频资源 → 升级到 film_project_lead

**Human 介入条件**:
- 发现技术限制需要用户决策
- 需要调整声音方案以适应制作能力

**串行/并行规则**:
- 串行：必须等待 sound-design-plan locked 后才能开始
- 合并点：与 HR-06 合并，形成完整的可行性报告
- 下游串行：报告 locked 后触发 HR-08

---

### HR-08: ai_production_supervisor → quality_control_editor

**Handoff ID**: HR-08  
**From Agent**: `ai_production_supervisor`  
**To Agent**: `quality_control_editor`

**输入 Artifacts**:
- `production-feasibility-report` (artifact_type)
- artifact_id: `ARTIFACT-XXX-production-feasibility-v{version}`
- Handoff document: `PRODUCTION-FEASIBILITY-REPORT.md`
- Status: locked

**输出 Artifacts**:
- `final-qc-report` (artifact_type)
- artifact_id: `ARTIFACT-XXX-final-qc-report-v{version}`
- Handoff document: `FINAL-QC-REPORT.md`
- Status: locked

**触发条件**:
- ai_production_supervisor 完成可行性报告并锁定
- 发送 `completion_notification` 给 quality_control_editor

**Message 类型**: `completion_notification`

**停止边界**:
- quality_control_editor 完成最终质量审查并锁定
- 发送 `completion_notification` 给 film_project_lead

**升级条件**:
- 发现重大质量问题 → 返回相关 Agent 修复
- 交付物不符合要求 → 升级到 film_project_lead

**Human 介入条件**:
- 发现无法解决的质量问题
- 需要对外发布决策

**串行/并行规则**:
- 串行：必须等待 production-feasibility-report locked 后才能开始
- 下游串行：报告 locked 后触发 HR-09

---

### HR-09: quality_control_editor → film_project_lead

**Handoff ID**: HR-09  
**From Agent**: `quality_control_editor`  
**To Agent**: `film_project_lead`

**输入 Artifacts**:
- `final-qc-report` (artifact_type)
- artifact_id: `ARTIFACT-XXX-final-qc-report-v{version}`
- Handoff document: `FINAL-QC-REPORT.md`
- Status: locked

**输出 Artifacts**:
- `project-completion-report` (artifact_type)
- artifact_id: `ARTIFACT-XXX-project-completion-v{version}`
- Handoff document: `PROJECT-COMPLETION.md`
- Status: locked

**触发条件**:
- quality_control_editor 完成最终审查并锁定
- 发送 `completion_notification` 给 film_project_lead

**Message 类型**: `completion_notification`

**停止边界**:
- film_project_lead 确认项目完成
- 生成项目总结报告
- 项目归档

**升级条件**:
- 需要重大调整 → 返回相关 Agent
- 无法满足交付标准 → 升级到 Human

**Human 介入条件**:
- 项目最终验收
- 对外发布决策

**串行/并行规则**:
- 串行：必须等待 final-qc-report locked 后才能开始
- 终点：项目主流程结束

---

### HR-10: film_project_lead → 任意 Agent (项目启动)

**Handoff ID**: HR-10  
**From Agent**: `film_project_lead`  
**To Agent**: `short_drama_planner` 或 `tvc_creative_director` 或 `narrative_screenwriter`

**输入 Artifacts**:
- `project-requirements` (artifact_type)
- artifact_id: `ARTIFACT-XXX-project-requirements-v{version}`
- Handoff document: `PROJECT-KICKOFF.md`
- Status: locked

**输出 Artifacts**:
- 依接收 Agent 而定（planning, creative-brief, 或 script）

**触发条件**:
- 项目启动，用户需求已确认
- film_project_lead 分配任务给合适的 Agent

**Message 类型**: `request`

**停止边界**:
- 接收 Agent 确认并开始工作

**升级条件**:
- 需求不明确 → 回复 film_project_lead 或升级到 Human
- 项目约束冲突 → 升级到 Human

**Human 介入条件**:
- 项目需求需要用户澄清
- 资源或预算限制

**串行/并行规则**:
- 起点：项目主流程开始
- 下游分支：根据项目类型选择不同路径

---

### HR-11: 任意 Agent → film_project_lead (项目收口)

**Handoff ID**: HR-11  
**From Agent**: 任意 Agent  
**To Agent**: `film_project_lead`

**输入 Artifacts**:
- 任意 Artifact (report, issue, question)
- Handoff document: `ESCALATION.md` 或 `NEEDS-YOU.md`

**输出 Artifacts**:
- `decision-record` (artifact_type)
- artifact_id: `ARTIFACT-XXX-decision-v{version}`
- Status: locked

**触发条件**:
- Agent 遇到超出职责范围的问题
- 需要协调多个 Agent
- 需要 Human 决策

**Message 类型**: `notification` 或 `request`

**停止边界**:
- film_project_lead 做出决策并回复
- 或升级到 Human

**升级条件**:
- 决策需要用户参与 → 升级到 Human
- 涉及项目范围或预算变更 → 升级到 Human

**Human 介入条件**:
- 重大决策
- 项目方向调整

**串行/并行规则**:
- 随时可触发：任何 Agent 任何时候都可以升级
- 阻塞：触发此路由的 Agent 暂停工作，等待决策

---

## 5. 路由统计

### 5.1 Intent Routes 统计

| Route | 主 Agent | 主 Skill | Artifact 输出 |
|---|---|---|---|
| IR-01 | narrative_screenwriter | screenwriter | script |
| IR-02 | tvc_creative_director | tvc | tvc-creative-brief |
| IR-03 | short_drama_planner | short-drama-planning | short-drama-plan |
| IR-04 | director_storyboard_artist | director-storyboard | storyboard |
| IR-05 | narrative_screenwriter | screenwriter | script (单集) |
| IR-06 | quality_control_editor | script-review | script-review-report |
| IR-07 | director_storyboard_artist | director-storyboard | storyboard |
| IR-08 | director_storyboard_artist | ai-video-shot-prompt | shot-prompts |
| IR-09 | visual_development_designer | character-visual-design | character-design |
| IR-10 | visual_development_designer | scene-asset-design | scene-design |
| IR-11 | narrative_screenwriter / visual_development_designer | worldbuilding-management | worldbuilding-bible |
| IR-12 | quality_control_editor | continuity-check | continuity-report |
| IR-13 | sound_designer | sound-design | sound-design-plan |
| IR-14 | ai_production_supervisor | ai-production-feasibility-review | production-feasibility-report |
| IR-15 | quality_control_editor | final-film-quality-review | final-qc-report |

**总计**: 15 条 Intent Routes

### 5.2 Handoff Routes 统计

| Route | From Agent | To Agent | 串行/并行 |
|---|---|---|---|
| HR-01 | short_drama_planner | narrative_screenwriter | 串行 |
| HR-02 | tvc_creative_director | narrative_screenwriter | 串行 |
| HR-03 | narrative_screenwriter | director_storyboard_artist | 串行 |


## V1.3 路由补充：ACTING / CINEDANCE / LIRA

V1.3 不新增 Agent，也不修改 15 条核心 Intent Route 和 11 条 Handoff Route 的责任拓扑；新增 Skill 作为现有 Domain 内的专业子路由。Task/RoutingDecision 必须记录实际选中的 Skill 与版本。

### IR-V13-01：角色表演设计

- **主 Agent**：master profile → `narrative_screenwriter`；逐场表演/调度 → `director_storyboard_artist`
- **主 Skill**：`character-acting-system`
- **输入**：character causal/relationship evidence、scene objective、blocking、voice profile
- **输出**：`character-acting-profile` / `scene-acting-adaptation`
- **下游**：`director-storyboard`、`ai-video-shot-prompt`、`cinedance-video-director`
- **边界**：只把人物心理事实编译成可观察行为，不改剧情动机、镜头或外貌。

### IR-V13-02：图像资产 Prompt 编译

- **主 Agent**：`visual_development_designer`
- **主 Skill**：`lira-image-prompts`
- **输入**：LOCKED/REVIEW Asset Bible、Scene Bible、Reference Lock、edit/view-change 请求
- **输出**：`image-prompt-pack`
- **下游**：若实际生成，进入 generation Attempt/Result/QC；通过后才能升级为 reference asset
- **边界**：供应商/model profile 是项目方法资料，不是当前 runtime 能力证明。

### IR-V13-03：Seedance / Higgsfield 专业镜头编译

- **主 Agent**：`ai_production_supervisor`
- **总入口 Skill**：`ai-video-shot-prompt`
- **专业 Skill**：目标 profile 为 Seedance/Higgsfield 时调用 `cinedance-video-director`；人物 performance layer 消费 `character-acting-system`
- **输入**：Shot Contract、active references、Scene Bible、Reference Lock、scene acting adaptation、声音/对白规则
- **输出**：canonical `prompt-manifest` / `ai-video-prompts`
- **边界**：不改变导演意图；Prompt 完成不等于模型执行或媒体完成。

## V1.2 路由补充：类型、具身与生成链

### IR-V12-01：故事类型选择

- **主 Agent**：`short_drama_planner`
- **主 Skill**：`story-type-engine`
- **输入**：PROJECT-BRIEF、已确认 Canon、媒介体量、未决项
- **输出**：`story-type-profile` Artifact；若主角、主题或结局会被选型改变，创建 `human-decision` / NEEDS-YOU 条目
- **下游**：`short-drama-planning`（分集压力）、`screenwriter`（人物选择与后果）、`script-review`（类型审查基准）

### IR-V12-02：角色具身与声音对齐

- **主 Agent**：`visual_development_designer`
- **协作 Agent**：`sound_designer`、`narrative_screenwriter`
- **Skills**：`character-visual-design` + `sound-design` + `screenwriter`
- **输出**：`character-embodiment-bible`，引用角色因果/对白行为；性格或声线不得被路由为外形刻板印象
- **下游**：`director-storyboard`、`ai-video-shot-prompt`、`continuity-check`

### IR-V12-03：导演镜头到生成闭环

- **主 Agent**：`director_storyboard_artist` → `ai_production_supervisor`
- **Skills**：`director-storyboard` → `ai-video-shot-prompt` → `ai-production-feasibility-review`
- **输出**：`shot-decision-sheet` → `ai-video-prompts` → `generation-attempt` / `generation-result` / `generation-qc-report`
- **限制**：没有可访问媒体时 Result 为 UNKNOWN，终审不得 PASS；`continuity-check` 只能检查结构化连续性。

所有 V1.2 路由必须由 `film_project_lead` 创建 Task 与 RoutingDecision，记录 Agent、Skill、Skill 版本、输入版本、理由、边界和替代方案。路由记录不是 runtime 执行证明。
| HR-04 | director_storyboard_artist | visual_development_designer | 串行，可与 HR-05 并行 |
| HR-05 | director_storyboard_artist | sound_designer | 串行，可与 HR-04 并行 |
| HR-06 | visual_development_designer | ai_production_supervisor | 串行，等待 HR-07 |
| HR-07 | sound_designer | ai_production_supervisor | 串行，与 HR-06 合并 |
| HR-08 | ai_production_supervisor | quality_control_editor | 串行 |
| HR-09 | quality_control_editor | film_project_lead | 串行 |
| HR-10 | film_project_lead | 任意 Agent | 项目启动 |
| HR-11 | 任意 Agent | film_project_lead | 随时升级 |

**总计**: 11 条 Handoff Routes

---

## V1.3.1 内容冲突硬化

- ACTING：`screenwriter` 仍拥有人物目标/关系/台词，`character-visual-design` 拥有 physique/外貌，`sound-design` 拥有永久 Voice Identity；`character-acting-system` 只读这些事实并输出 behavior adaptation。
- LIRA：路由到 `lira-image-prompts` 前必须绑定 Visual Bible/Project Style Profile；模型 profile 或 source 方法不能覆盖二维/赛璐珞等已锁定风格。
- CINEDANCE：`dynamic_negative_constraints` 由 continuity/reference/geometry 生成并留在数据层；最终 Prompt 使用 positive-first/local-lock 编译，仅必要时保留最小 emitted negative constraint。
- 冲突裁定统一使用 `contracts/SKILL-AUTHORITY-PRECEDENCE-V1.3.1.md`。
