---
name: cinedance-video-director
description: Seedance/Higgsfield 镜头需要强化首帧、空间调度、镜头光学、物理、灯光、对白和参考控制时使用；作为 ai-video-shot-prompt 的专业编译层。
---

# CINEDANCE 视频导演层

## 适用场景

用于已经有明确镜头意图、空间关系和版本化资产后，把镜头编译为 Seedance 2.0 / Higgsfield Seedance 风格的生产提示词；尤其适合多人站位、首帧、视线、左右关系、镜头侧位、焦段、物理、灯光和对白时序容易漂移的镜头。它是 `ai-video-shot-prompt` 的专业编译层，不取代导演分镜。

## 不适用场景

不用于改剧本、重做分镜、设计角色/场景、判断真实模型当前价格/能力，也不用于把一段模糊概念直接包装成“可生产镜头”。非 Seedance/Higgsfield 模型默认回到 `ai-video-shot-prompt` 的通用模型 profile；本 Skill 中任何平台能力只按项目提供的 source profile 使用，未通过真实运行验证时必须标为未验证。

## 必需输入

版本化 `shot-contract` / storyboard、可见角色与状态、场景/地标、道具、Reference Lock、起始帧要求、时长/格式模式、声音/对白规则；如有人物表演，读取 `scene-acting-adaptation` 或等价行为证据。

## 可选输入

目标 Seedance/Higgsfield profile、active @tags、location reference、允许的 UI 参数、历史 generation-qc-report、已批准的降级方案。参考图只作为身份/空间/材质锚点，不默认继承构图。

## 输入不足处理

先标出缺失的首帧、站位、视线、地标、动作时序、灯光方向或引用角色；能从 LOCKED Artifact 回读的直接回读，不能回读的保持 UNKNOWN。涉及导演意图、角色关系、范围或锁定事实时写入 `NEEDS-YOU.md` 或回请求对应 Responsible，禁止凭经验补齐。

## 锁定事实与禁止修改项

角色身份、服装/伤势状态、地标位置、screen direction、action axis、台词、镜头目的和 Reference Lock 都来自上游版本化 Artifact。Skill 只能把这些事实编译成模型可执行语言，不能借“优化提示词”改变它们。不得使用过期 @tag、前镜残留、scene number 或“same as before / continues from”等跨镜污染。

## 完整分阶段工作流

1. **Deconstruct**：只提取当前镜头的 active characters、references、location、props、dialogue、duration、first frame、blocking、movement、lighting 与 forbidden carryover。
2. **Diagnose**：检查空首帧、角色延迟出现、左右翻转、视线反转、地标距离、镜头侧位、焦段漂移、平光、道具错手、漂浮物理、对白错时和 reference 误继承构图风险。
3. **Develop**：按 `SCENE CONTEXT → ACTIVE REFERENCES → LOCATION MAP → FIRST FRAME AND SPATIAL BLOCKING → FORMAT MODE → OPTICS → CAMERA → ACTION TIMING → PHYSICS → LIGHTING → AUDIO → POSITIVE CONSTRAINTS` 编译；先把上游 `dynamic_negative_constraints` 当作内部保护台账，逐条转换为 `positive_locks` 或局部 failure-prevention lock，只有已知失败模式无法正向表达时才产生最小 `emitted_negative_constraints`，禁止机械复制大段 NOT-stack。
4. **Deliver**：默认输出最终英文模型提示词和结构化 prompt-manifest 字段；用户明确要求 QA/解释时才附分析。文件模式下同时保留来源、版本和 acceptance checks。
5. **Evidence**：创建 Prompt 只表示计划；只有真实 Attempt/Result Event 与可访问 media_ref 才能进入媒体 QC，不得把“提示词已完成”写成“视频已生成”。

## 阶段门禁

- Gate A：Shot Contract、active references 和当前角色状态可定位。
- Gate B：首帧、blocking、gaze、body orientation、camera side、action timing 与 lighting direction 无冲突。
- Gate C：Seedance/Higgsfield 专属语法只在目标 profile 已选定时使用；否则回退通用 prompt。
- Gate D：Prompt 通过局部失败风险 QA 后才交生成；任何改变锁定镜头的建议先返回导演/Human。

## 输出契约

输出仍使用 canonical `prompt-manifest` / `ai-video-prompts`，至少包含：`shot_id`、`source_shot_contract_ref`、`active_reference_roles`、`location_map`、`first_frame_blocking`、`format_mode`、`optics`、`camera`、`timed_action`、`physics`、`lighting`、`audio`、`dynamic_negative_constraints`（内部台账）、`positive_locks`、`local_failure_prevention_locks`、`emitted_negative_constraints`、`acceptance_checks`、`generation_chain_ref`。模型提示词本体使用清晰直接的 cinematic English；内部 negative ledger 不默认逐字进入提示词。

## V1.3.1 Constraint Compilation Override

本 Skill 遵守 [`SKILL-AUTHORITY-PRECEDENCE-V1.3.1.md`](../../../contracts/SKILL-AUTHORITY-PRECEDENCE-V1.3.1.md)。`dynamic_negative_constraints` 属于数据/QC 层保护台账；最终 Seedance/Higgsfield Prompt 优先使用 positive/local locks。只有具体失败模式无法正向表达时，才保留最小 emitted negative constraint。

## Production System V2 集成

只从通过的 Shot Package/Shot Contract 编译 Prompt Manifest；Reference Lock、screen direction、动作轴、角色状态与空间禁变项继续作为 Source of Truth。Prompt 不得覆盖上游事实。详见 [`PRODUCTION-SYSTEM-V2.md`](../../../contracts/PRODUCTION-SYSTEM-V2.md)。

## Creative Depth V1.3 集成

把 Observable Image Specification 转成具体可见的空间、动作、光线、遮挡和声音，不以“电影感、压迫、史诗”替代这些事实；表演层优先消费 `character-acting-system` 的场景适配。详见 [`CREATIVE-DEPTH-V1.3.md`](../../../contracts/CREATIVE-DEPTH-V1.3.md)。

## 质量检查表

- 首帧立即包含所有必须出现的主体；无无用 establishing opening。
- 每个重要人物的 screen/world position、身体朝向、视线和运动方向明确。
- 地标距离可测量或有物理接触锚点；不只写 near/beside。
- 光学由内容需要决定并有抗漂移描述；多镜头切换保持空间、手持物、灯光、伤势和方向连续。
- 物理有重量、接触、惯性、液体/衣物/道具反应；对白只由说话者发声且时序明确。
- 不含未使用 @tag、旧场景残留、随机插入镜头、模型 UI 已控制的冗余参数。

## 错误与异常处理

目标模型/profile 未确认时不伪装成 Seedance 专属最佳实践；reference 不可用时改为 descriptor-only 并标风险；空间冲突返回 `director_storyboard_artist`；角色状态冲突返回 continuity；表演冲突返回 `character-acting-system`/编剧；媒体不存在时不做成片 PASS。

## 与其他Skill的边界

`director-storyboard` 拥有镜头意图；`character-acting-system` 拥有可观察表演行为；`ai-video-shot-prompt` 是通用提示词总入口；本 Skill 只负责 Seedance/Higgsfield 专业编译；`ai-production-feasibility-review` 负责技术风险和降级，`continuity-check` 负责跨镜一致性。

## 上游输入和下游交接

上游通常为 `director_storyboard_artist` 的 Shot Contract + `visual_development_designer` 的资产/场景锁 + `character-acting-system` 的 scene adaptation。下游返回 `ai_production_supervisor` 的 Prompt Manifest；如接入真实模型，再进入 Attempt → Result → QC → Retry 证据链。

## Human介入条件

需要改变导演意图、角色数量/关系、对白、场景地理、核心资产、预算/范围、LOCKED 事实或接受高风险降级时升级 Human。平台当前能力与项目假设冲突且无法证实时，也应保持未验证并升级。

## 示例

输入：“单镜 8 秒，两人在诊所操作台旁对话，必须第一帧同时出现，A 在画面左、B 在右，B 手持手机。”输出应先绑定 active references，再写 location map、首帧站位、视线/身体方向、镜头光学、0–8 秒行为、手机物理、单一灯光逻辑和对白；不能写“沿用上一镜站位”。

## 最小测试用例

见 [`evals/cases/cinedance-video-director/`](../../../evals/cases/cinedance-video-director/)：3 正向、2 不触发、2 边界、1 输入不足、1 相邻冲突；验证的是文件契约，不是 Seedance/Higgsfield 实际生成成功率。

## 版本信息

- schema_version：`3.2`
- content_version：`3.2.1`
- 状态：`ACTIVE`

## 参考资料

- [CINEDANCE V4 原始用户资料](references/source-CINEDANCE-V4.md)
- [CINEDANCE Prompt Skeleton](references/prompt-skeleton.md)
- [CINEDANCE Spatial / Optics / Physics](references/spatial-optics-physics.md)
