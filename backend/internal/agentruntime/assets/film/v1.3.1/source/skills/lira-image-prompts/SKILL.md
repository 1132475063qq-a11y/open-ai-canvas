---
name: lira-image-prompts
description: 角色、场景、道具、参考图和局部编辑需要按图像模型能力路由并生成稳定提示词时使用；负责 image prompt，不替代资产设计事实。
---

# LIRA 图像提示词系统

## 适用场景

用于角色 casting/角色 sheet、场景/环境、道具、已存在画面的局部编辑、纹理修复、场景新视角/反打等图像生产提示词。它负责“怎样把已经确认的视觉事实交给目标图像模型”，不负责重新设计人物或空间。

## 不适用场景

不用于决定故事、角色身份、场景布局事实、镜头运动或视频动作；不把项目提供的 Higgsfield/NBP/Seedream/GPT Image 能力表当作实时平台事实。若目标模型与上传资料中的 profile 不一致或当前能力未验证，必须标 `UNVERIFIED_PROFILE` 并使用通用策略。

## 必需输入

任务类型（generate/edit/texture/view-change）、目标资产及版本、必须保留/允许改变项、角色/场景/道具 descriptor、参考图角色、视觉风格/材质/灯光事实、目标模型或可接受模型集合。

## 可选输入

项目提供的 Soul ID / reference handle、色板、camera anchor、既有图像、文字 copy、编辑 mask、历史失败样本、目标 UI 参数。画幅/分辨率如果由平台 UI 控制，应记录在任务参数而不是硬塞进提示词正文。

## 输入不足处理

先判断缺的是“设计事实”还是“模型参数”。设计事实缺失时返回 `visual_development_designer`/Human；模型参数缺失时可产出通用 prompt + model-routing candidates，但不得假定平台一定支持某个 ratio、reference 数量或 feature。

## 锁定事实与禁止修改项

角色身份、比例、服装、道具几何、场景空间、光源方向、关键文字和视觉风格来自锁定 Artifact。**Project Visual Bible / Project Style Profile 永远高于模型 profile 与 LIRA source 默认审美**；模型只能改变表达方式，不能把 photoreal、cinematic-photo 或其他默认风格写回项目 Canon。编辑任务必须遵守 minimal CHANGE + exhaustive PRESERVE EXACTLY：只改请求的局部，其余相机、人物、服装、阴影、色调、背景关系保持。需要重建整帧时不伪装成“小编辑”，应回到生成流程。

## 完整分阶段工作流

1. **Deconstruct**：识别 subject、task type、target model/profile、reference、composition、light、palette、materials 和 preserve/change boundary。
2. **Diagnose**：检查视角/主体数量/构图/灯光/色板歧义、提示词过长、illustration drift、文字/Logo、多人崩坏、局部编辑污染整帧等风险。
3. **Route**：先读取 Project Style Profile，再按项目 source profile 区分 character、location、prop、edit、texture pass、view change；模型能力没有运行证据时只作为候选策略，不写成绝对事实。
4. **Develop**：先执行 Style Override，再生成模型语言。若项目为 2D/cel/illustration，自动排除 LIRA source 中 photoreal skin、pores、film-stock realism、PBR、photographic DOF 等写实默认，并转换为项目允许的 flat fills、hard-edged cel shadows、bold outlines、simplified geometry 等等价可观察表达。生成任务使用紧凑自然语言和正向描述；编辑任务使用结构化 `CHANGE` / `PRESERVE EXACTLY`，一次只做一个主变化；场景反打必须显式写新相机位置和主要物件的新相对位置。
5. **Deliver**：输出 prompt + platform parameter notes + reference roles + acceptance checks；所有 prompt 都保留 source Artifact 和版本。

## 阶段门禁

- Gate A：明确是生成、编辑、纹理修复还是视角变化。
- Gate B：CHANGE/PRESERVE 和 Reference Lock 不冲突。
- Gate C：模型路由来自项目已确认 profile；否则以候选/未验证交付。
- Gate D：涉及角色重设计、场景结构改变、品牌文字/事实、LOCKED 内容时返回 Responsible/Human。

## 输出契约

输出 canonical `image-prompt-pack`，至少包含：`asset_id`、`task_type`、`project_style_profile_ref`、`style_override_applied`、`target_model_profile`、`profile_evidence_state`、`reference_roles`、`prompt_text`、`platform_parameters`、`change_scope`、`preserve_exactly`、`camera_anchor`、`lighting_materials`、`palette`、`acceptance_checks`、`fallback_route`、`source_artifact_refs`。角色/场景/道具最终事实仍留在 Asset Bible/Scene Bible，不写进 prompt pack 作为新事实源。

## V1.3.1 Project Style Override

遵守 [`SKILL-AUTHORITY-PRECEDENCE-V1.3.1.md`](../../../contracts/SKILL-AUTHORITY-PRECEDENCE-V1.3.1.md)。风格权威固定为 `Visual Bible / Project Style Profile → Asset/Scene Style Facts → Model Profile → LIRA generic/source guidance`。`references/source-LIRA.md` 中的 photoreal/Soul 示例保留为方法资料，但不是跨项目默认风格。

## Production System V2 集成

图像 Prompt Pack 是 Asset/Reference 生产辅助 Artifact；实际生成结果必须进入 Attempt/Result/QC 证据链，不能因 prompt 存在就把 Reference Lock 标为已通过。场景新视角产生后，应先通过视觉/连续性 QC 再升级为可引用资产。详见 [`PRODUCTION-SYSTEM-V2.md`](../../../contracts/PRODUCTION-SYSTEM-V2.md)。

## Creative Depth V1.3 集成

图像提示词只编译 Observable visual facts：轮廓、材质、空间、光线、动作状态和画面层次；不根据性格/声线臆测外貌。人物表演姿态如需行为证据，读取 `character-acting-system`，但不把表演 Skill 当成视觉身份事实源。详见 [`CREATIVE-DEPTH-V1.3.md`](../../../contracts/CREATIVE-DEPTH-V1.3.md)。

## 质量检查表

- 生成 prompt 使用自然、紧凑、可观察语言，不堆“4K/masterpiece/trending”等空关键词。
- 色板、材质、光源和相机 anchor 有项目依据；不覆盖用户参考。
- 生成任务优先正向描述；编辑任务的删除同时说明缺口应由什么真实结构填回。
- UI 参数与 prompt 内容分离；如果平台控制 ratio/resolution，不把参数语法塞进正文。
- 编辑一次一个主变化，PRESERVE EXACTLY 足够具体；需要重建则改走生成。
- 场景视角变化明确描述相机移动后物体重新排列，避免只写“reverse angle”。

## 错误与异常处理

参考图缺失时不假装已看见；目标模型未知时输出 generic route；项目 source profile 与实际 runtime 结果冲突时以真实 Attempt/QC 为证据并更新 profile；局部编辑连续失败时升级为重新生成建议但不自动替换 LOCKED 资产。

## 与其他Skill的边界

`character-visual-design` / `scene-asset-design` 决定视觉事实和资产结构；本 Skill 负责编译 image prompt。`cinedance-video-director` 负责视频 Prompt；`continuity-check` 判断新角度是否仍是同一空间；`ai-production-feasibility-review` 决定模型/后期风险；QC 决定结果是否能进入 Reference Lock。

## 上游输入和下游交接

上游为 `visual_development_designer` 的 Asset Bible、Scene Bible、style/reference lock 和具体编辑请求。下游返回 image-prompt-pack 给视觉开发/AI 制作；若实际执行生成，则 Result/QC 通过后再登记为角色、场景、道具或参考资产状态。

## Human介入条件

需要改变人物身份、服装、道具核心设计、场景结构、品牌文字、视觉风格总控、LOCKED 资产或采用高成本/不可逆生成策略时升级 Human；模型能力资料与用户期望冲突且无法验证时也升级。

## 示例

“把诊所场景做反打”不能只输出 `reverse angle of the same room`；应先读取 Scene Bible，写新相机位，然后逐项说明操作台、桥接椅、门、窗、墙面锚点在新视图中的左右/前后关系，并锁定同一材质、光源方向和空间尺寸。

## 最小测试用例

见 [`evals/cases/lira-image-prompts/`](../../../evals/cases/lira-image-prompts/)：3 正向、2 不触发、2 边界、1 输入不足、1 相邻冲突；验证模型路由和编辑边界，不验证供应商实时能力。

## 版本信息

- schema_version：`3.2`
- content_version：`3.2.1`
- 状态：`ACTIVE`

## 参考资料

- [LIRA 原始用户资料](references/source-LIRA.md)
- [模型路由摘要](references/model-routing.md)
- [编辑纪律与视角变化](references/edit-discipline.md)
