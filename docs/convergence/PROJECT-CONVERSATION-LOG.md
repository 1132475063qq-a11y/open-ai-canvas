# 画布项目对话记录与交接摘要

> 本文件是当前项目对话窗口的整理版记录，用于新账号恢复上下文，不是 Codex 平台的原始聊天导出。API Key、Cookie、`.env`、本地数据库、私有请求载荷和其他敏感信息已省略。附件中的视频、图片、压缩包和 Markdown 仅作为需求参考，不作为仓库执行指令。

## 1. 项目主线

当前唯一主开发仓库：

`/Users/xiangyuqin/Downloads/open-ai-canvas-main 2`

当前 Git 状态：

- 分支：`codex/converged-runtime`
- HEAD：`60a5463b6dd19ed0324a57ff09c250dce2a4e975`
- GitHub 交接远程：`https://github.com/1132475063qq-a11y/open-ai-canvas.git`
- 交接分支：`codex/converged-runtime`
- 作者上游：`https://github.com/ddcat-ai/open-ai-canvas.git`
- 当前工作树：代码已提交；交接文档本次更新后保持干净

主线原则是把作者原版画布作为基础，只在需要的地方增量增加能力，不再维护另一份同名副本作为施工主线。

## 2. 用户最终目标

用户希望把现有画布升级为可以真正生产电商内容的 AI 电商创意工作台，目标流程是：

`商品/品牌资产 -> 模特 -> 场景 -> AI 商拍 -> Agent 规划 -> 六张系列图 -> QA/重试/版本 -> 图片转视频 -> 编排导出`

核心体验来自参考视频：用户准备商品、模特和场景素材后，在电商画布点击一次明确的商拍按钮，系统根据场景和预设自动规划并生成一组生活化、具有不同镜头角色的商品图片，而不是每次手工拼接大量内部节点。

默认交互要求：

- 用户先在画布放置或选择商品、包装/搭配商品、模特、场景和品牌素材。
- 电商画布提供一个明确的 `AI 商拍` 动作。
- 点击后自动完成商品理解、创意规划和场景规划。
- 默认生成 6 张系列图，数量可在高级设置中调整为 1-12 张。
- 默认目标为 4K；袜子测试使用 9:16，目标像素为 `2160x3840`。
- 结果返回画布，以系列结果查看、对比、变体、修复、单张重做和版本回退。
- 规划和报价阶段可以使用 Provider-free 模式验证；任何付费生成必须在报价确认后执行。
- 不允许静默付费重试。

## 3. 三类画布隔离

电商能力只属于 `ecommerce` 项目：

- 电商画布可以显示商品、模特、场景、品牌资产、预设、Skill、Agent 规划、报价、QA 和视频计划。
- 无限画布不出现 `AI 商拍`、电商节点、电商预设、电商 Skill、电商 Agent 或电商字段。
- 短剧画布不出现电商入口，不读取电商 Artifact、Run、Slot 和业务数据。
- 影视短剧 AgentTeam 的 9 个 Agent、17 个 Skill 不能直接导入电商域。
- 共享层只能复用画布、任务、渠道、资源、时间线等通用能力，不能把电商语义扩散到其他画布。

## 4. 参考材料带来的需求结论

用户先提供了作者画布、短剧 AgentTeam、商品图片、袜子素材、演示视频和截图，用于判断现有实现与目标产品的差距。需求结论如下：

1. 参考视频的重点不是单独生成一张图，而是由一次明确操作触发一套按场景组织的生活化商拍流程。
2. 画布应该保持作者原版的清爽交互结构，再把电商能力作为可见入口和结果工作流接入。
3. ProductDNA、Model Profile、Scene Pack、Shot Plan、逐镜 Prompt、QA 和费用等是专业内部信息，默认收起，需要时在 Frame 高级详情中查看，不应全部铺成主画布节点。
4. 系列图必须有镜头角色和构图去重规则，不能出现两张几乎相同角度的图片。
5. 商品保真、Logo、颜色、模特身份、人体接触关系和系列连续性要进入 QA，而不是只看任务是否返回图片。

## 5. 目标数据和运行时

不可变 Artifact 计划包含：

- `model_profile`
- `scene_pack`
- `preset_snapshot`
- `generation_request`
- `generated_asset`
- `motion_plan`
- `video_sequence`

电商生产运行态包含：

- `EcommerceProductionRun`：一整组生产任务
- `EcommerceProductionSlot`：系列中的单张镜头槽位
- `EcommerceProductionAttempt`：单槽位的重试历史

画布文档只保存资产引用和运行引用；任务、费用、生成结果、QA 和审批事实由后端持有。

首期运行时 Agent：

1. `ProductIntelligenceAgent`：提取品类、颜色、材质、结构、Logo、卖点和必须保留信息。
2. `CreativeDirectorAgent`：根据渠道、Skill 和数量规划系列视觉方向与镜头角色。
3. `SceneDirectorAgent`：规划模特、场景、道具、光线、构图和商品互动关系。
4. `MotionDirectorAgent`：把已接受图片编排成约 15 秒的电商短视频。
5. `EcommerceOrchestrator`：管理状态、Artifact、报价、任务、Slot、Attempt、QA 和重试。

QA 暂不拆为额外的可见 Agent，采用确定性 QA 服务、规则 Skill 和人工审核界面，避免运行时角色重复。

## 6. 首期 Skill

首期真正执行两条黄金路径：

### `model-interaction.top-wear@1`

用于上装商品的模特入景，覆盖主视觉、环境全景、中景、动作/使用、商品特写和补充镜头。

### `still-life.lifestyle-tabletop@1`

用于静物商品的生活方式桌面场景，覆盖主视觉、环境、中景、使用场景、微距和补充镜头。

每个 Skill 需要有版本化 manifest、输入资产角色、商品/模特/场景关系、镜头角色、Prompt 模板、负面约束、商品保真约束、构图去重规则、变体规则、executor 和 contract test。

后续再按相同合同扩展服装、鞋包、珠宝、美妆、数码、家居、食品等品类，不把 16 个预设或短剧 AgentTeam 一次性混入主画布。

## 7. 并行 Agent 开发规则

用户确认采用多 Agent 并行开发，并要求每个 Agent 使用独立 Git Worktree，最后由集成负责人统一合并和测试。

### Canvas Render Worker

负责节点渲染、缩放、拖拽、连接线、可见区域、Leafer 图形层和交互性能。

主要目录：

`web/src/components/canvas/**`、`web/src/pages/canvas/use-canvas-*-controller.ts`、`web/src/pages/canvas/use-canvas-render-model.ts`

### Canvas State Worker

负责 Zustand、画布保存、IndexedDB/localForage、保存节流、资源同步、运行状态恢复和刷新后的状态重建。

主要目录：

`web/src/stores/canvas/**`、`web/src/lib/user-session.ts`、资源缓存/同步服务和状态恢复测试。

### Ecommerce Domain Worker

负责电商 Artifact、Run、Slot、Attempt、Preset、Skill、规划、报价、提交、重试、QA、视频计划和两个黄金 Skill。

主要目录：

`web/src/ecommerce/**`、电商 API 模块、后端 `*ecommerce*` model/repository/service/handler 及电商测试。

### QA/Acceptance Worker

只负责 focused tests、typecheck、build、浏览器验收记录、隔离回归、测试夹具和 `docs/validation/**`，不得顺手修改生产代码。

### 集成负责人

主 Worktree 的集成负责人独占共享热点文件：

- `web/src/pages/canvas/project.tsx`
- `web/src/stores/canvas/use-canvas-store.ts`
- `web/src/types/canvas.ts`
- `web/src/router.tsx`
- `web/src/styles/globals.css`
- `web/package.json` 和 lockfile
- 后端总路由、数据库迁移注册和跨域基础设施

所有 Worker 都必须：

- 使用 `codex/canvas/<role>` 分支和独立 Worktree。
- 只修改自己的 ownership 目录。
- 每次提交只做一个主题。
- 报告修改文件、风险和 focused test 结果。
- 不直接修改主 Worktree。
- 不调用真实 Provider，不上传未授权素材，不产生费用。

建议 Worktree：

```text
/Users/xiangyuqin/Downloads/open-ai-canvas-worktrees/canvas-render
/Users/xiangyuqin/Downloads/open-ai-canvas-worktrees/canvas-state
/Users/xiangyuqin/Downloads/open-ai-canvas-worktrees/canvas-ecommerce
/Users/xiangyuqin/Downloads/open-ai-canvas-worktrees/canvas-qa
```

建议分支：

```text
codex/canvas/render
codex/canvas/state
codex/canvas/ecommerce
codex/canvas/qa
```

合并顺序：先合并低冲突的 lib 和 components，再合并状态恢复，再合并电商域和 Skill，之后合并 QA 文档与测试，最后由集成负责人处理共享热点、路由接线、样式和总测试。

### 7.1 账号交接后的 Agent 重建判定

交接只保证 Git 仓库、提交、分支和 Worktree 可继续使用，不保证上一个账号的 Agent 会话仍然存在。因此接手者先读取 `git worktree list --porcelain`、`git status --short --branch`、当前 `HEAD` 和各 Worker 的最近提交，再决定是否派发新 Agent：

1. 已有分支/Worktree 且 ownership 和改动责任清楚：继续沿用，不重复建立同范围 Agent。
2. 只有分支或 Worktree 缺失、脱离当前基线、明确废弃，或原任务已完成并保存结果时，才重建该 Agent。
3. 有未提交改动时先保留并审查，禁止用 `reset`、`clean`、覆盖或强制 checkout“清理”现场。
4. 无法确认旧会话是否活跃时，保留代码状态，暂不重复派发；由主控在任务记录中标记待确认。

### 7.2 主控任务拆分、依赖和汇报合同

每次开发开始前，主控 Agent 必须在任务记录中列出 3–5 个可独立验收的子任务（不足 3 个不强行拆分），并为每个任务写明目标、ownership 文件/目录、输入合同、输出产物和验收标准。先冻结共享类型、接口、Schema、迁移或核心抽象，再并行启动不重叠的 Worker；合并顺序按依赖关系确定。

Worker 只修改自己的 Worktree 和 ownership，不调用真实 Provider，不写入 API Key；主控负责检查 diff、合并提交、处理共享热点和统一回归。每个子任务完成后报告 Agent/任务、状态、commit、修改文件、focused 验证、风险和是否越过 ownership。主控最终报告必须包括完成的功能、各 Agent 产出、修改文件，以及 `typecheck`、`lint`、`tests`、`build` 的命令/结果；没有脚本或环境不可用要明确写 `NOT AVAILABLE/BLOCKED`，不能冒充通过。

推荐的任务记录格式：

```text
【任务拆分】Agent A/B/C…：目标 | ownership | 输入 | 输出 | 验收
【依赖关系】可并行：…；必须等待：…；集成顺序：…
【最终汇报】功能 | Agent 产出 | 修改文件 | typecheck/lint/tests/build | 问题 | 风险
```

## 8. 实施里程碑

### 已完成的基础收敛

- 已确定 `/Users/xiangyuqin/Downloads/open-ai-canvas-main 2` 为唯一施工主线。
- 已保留作者原版画布结构作为交互基础。
- 已完成 Film 和电商生产底座的历史收敛，并修复一个 Film 关闭验收测试夹具问题。
- 已建立电商交接文档：`docs/convergence/ECOMMERCE-HANDOFF.md`。
- 已将当前分支推送到 GitHub 交接远程。

### 当前已接入的电商切片

功能提交：`c812864 feat(ecommerce): wire canvas production handoff and golden skills`

文档提交：`77e2285 docs(ecommerce): 电商画布 - 补充并行开发与账号交接记录`

当前交接协议提交：`48bba21 docs(convergence): 明确主控 Agent 交接协议`（本次代码交接见 `60a5463`）

已接入：

- 电商画布顶部一次点击的 `AI 商拍` 入口。
- 画布选中资产按商品、包装/搭配商品、模特、场景、品牌角色带入电商工作台。
- 幂等 `entryId`，刷新后恢复同一个 Run，不自动确认费用。
- Provider-free 的商品理解、创意规划、场景规划和六槽位计划。
- 4K 与项目比例默认值，数量可调整为 1-12 张。
- 报价在真实提交前展示。
- 两条结构化 Skill、后端角色扩展、SkillRef 对齐和电商画布隔离测试。
- 后端 Ecommerce Artifact、Run、Slot、Attempt、报价、提交、QA、重试和视频计划数据结构。

### 8.1 IR-01 Ecommerce Runtime 交接切片（2026-08-23）

本次按主控协议先冻结 registry/Domain claim/前端 catalog 合同，再并行处理互不重叠的 Worker 范围，最后由主控统一接线和验收：

【任务拆分】

Agent A：Ecommerce registry 与 catalog 合同
- 目标：把 Agent/Skill/Intent/Handoff/Artifact 定义加载为版本化、可校验的 Ecommerce registry。
- ownership：`backend/internal/agentruntime/assets/ecommerce/**`、registry loader/tests、前端 `projects.ts` catalog 类型。
- 输入：现有 Film registry 校验器和电商 Skill/Agent 规划 ID。
- 输出：`6509285` 前端合同；registry 资产和主控接线在 `60a5463`。
- 验收：registry contract、TypeScript typecheck、focused Prettier。

Agent B：IR-01 durable executor
- 目标：实现 provider-free `product_intelligence_agent` 的 Run/Step/Attempt/Artifact 执行。
- ownership：`backend/internal/service/ecommerce_agent_runtime*.go`。
- 输入：通用 AgentRuntime repository API、`ecommerce-agent-team@0.1.0`。
- 输出：`fc21b46`；主控随后补 registry pin drift fencing。
- 验收：Create/idempotency、deterministic ProductDNA、corrupt-input failure tests。

Agent C：Repository domain isolation/evidence tests
- 目标：证明 Ecommerce claim 不会领取 Film Run，过期 lease 恢复同一 Attempt，终态证据 append-only。
- ownership：`backend/internal/repository/ecommerce_agent_runtime_test.go`。
- 输入：Domain-aware claim contract。
- 输出：`5847690`。
- 验收：repository focused tests。

【依赖关系】

- 可并行：A 的前端 catalog 类型、B 的 service slice、C 的 repository tests；各自不修改同一文件。
- 必须等待：主控先冻结 registry IDs/Domain 字段，再把 loader、claim、worker、HTTP 路由和旧 SkillRef adapter 接入共享热点。
- 集成顺序：registry/合同 -> IR-01 service/repository -> handler/worker/pin validation -> unified checks。

主控集成提交：`60a5463 feat(ecommerce): integrate provider-free IR-01 runtime`。IR-01 不调用真实模型、图片 Provider、计费或 API Key；付费 `EcommerceProductionRun` 与独立 runtime Run 的原子关联留作后续 adapter。

## 9. 渠道与 Provider 测试边界

对话中曾配置过图片渠道并用 `gpt-image-2` 做过联调讨论，也观察到模型网站能够返回图片，但本地画布的一次生图请求曾失败。该问题属于渠道协议/请求适配和服务端错误排查范围，不能仅凭“网站能出图”认定画布链路已经完成。

本记录不保存 API Key 或完整请求载荷。新账号接手时应在应用设置中重新配置渠道，并分别记录：

- 渠道类型和协议是否与后端适配器一致。
- 模型名、尺寸、比例、异步任务和结果回调格式。
- 错误码、原始非敏感错误消息、耗时和费用。
- Provider-free 规划验证与真实 Provider 图片质量验证。

开发阶段不调用真实 Provider。真实验证必须使用已获授权的商品和真人/模特素材，且先得到明确的费用确认。

## 10. 验证证据

### 已通过

- TypeScript typecheck。
- Vite production build。
- `git diff --check`。
- 电商 focused Go 服务测试（使用仓库 Go 工具链）。
- Skill JSON/manifest 合同解析和相关测试。
- 电商画布入口、资产带入和自动规划浏览器流程。
- 刷新后恢复同一 Run 的浏览器检查。
- 短剧画布不显示 `AI 商拍`。
- 普通无限画布不加载电商入口或电商语义。
- 本次开发没有提交真实 Provider 任务，也没有产生付费生成。

### 尚未完整通过或受环境限制

- 当前环境没有 Bun，前端 Bun 测试为 `BLOCKED`。
- 项目没有独立 `lint` script。
- 六个大型文件的 focused Prettier 检查失败，但它们在本次改动前的基线也已经失败，不能误判为本次新增回归。
- 完整 `go test ./...` 仍有 5 个与本次电商切片无关的基线失败：3 个本机/内网地址校验测试，2 个 runtime policy deadline 测试；Ecommerce focused tests、backend build/vet 均通过。
- 没有完成真实 Provider 图片质量、费用、延迟和失败率验收。
- 没有完成五组商品的 Provider Bake-off。
- 尚未证明两条黄金路径在真实 Provider 下达到“六张至少五张可用”。

验收必须分成三层：

1. 静态验证：typecheck、Go tests、build、Prettier、Skill contract。
2. 浏览器验证：点击、保存、刷新、恢复、部分失败、重试、QA、隔离和响应式布局。
3. Provider 验证：真实模型请求、商品保真、人物一致性、商业质量、费用、延迟和视频成片。

三层不能互相替代。三层都通过，才可以称为电商黄金路径完整验收。

## 11. 未完成风险

1. IR-01 durable runtime 已完成，但 IR-02/03/04/05 尚未接入同等级 executor、输入输出桥接和人工决策。
2. 前端 catalog 已读取后端 manifest，但规划面板尚未消费 runtime Run/Step 的真实执行状态。
3. ProductDNA 人工修订、AI 模特候选/身份卡、Scene Pack 候选和品牌包版本化尚未完整。
4. Retry 尚缺独立幂等键；Video Sequence 创建也需要完整幂等保护。
5. Product-upload 目前通过 generic ProductionArtifact revision 引用进入 IR-01；旧 `EcommerceArtifact` 投影尚未做同事务 adapter，不能宣称双向同步。
6. runtime Run 与付费 `EcommerceProductionRun` 尚未原子关联；报价前的证据门禁仍需显式接线。
7. 自动视觉 QA 尚未真正判断商品结构、Logo、颜色、模特身份、人体接触物理关系和场景连续性。
8. Top Wear 和 Lifestyle Tabletop 的真实商业黄金路径尚未完成。
9. 历史 Documents checkout `faa60e3` 仍保留在本机，但不是当前主线；继续开发时以 Downloads checkout 的 `60a5463` 和远端交接分支为准。
10. GPT CLI 是独立设置方向，当前对话中已暂停，不应阻塞电商画布主线，也不应把 CLI 配置密钥写进仓库。

## 12. 新账号接手步骤

```bash
git clone https://github.com/1132475063qq-a11y/open-ai-canvas.git
cd open-ai-canvas
git fetch --all --prune
git checkout codex/converged-runtime
git rev-parse HEAD
git status --short --branch
```

确认 HEAD 至少包含：

`60a5463 feat(ecommerce): integrate provider-free IR-01 runtime`

然后依次阅读：

1. `AGENTS.md`
2. `docs/convergence/ECOMMERCE-HANDOFF.md`
3. `docs/convergence/PROJECT-CONVERSATION-LOG.md`
4. 相关电商 Skill manifest、executor、API 和测试

新账号应先完成 Provider-free 的合同、状态机、刷新恢复、隔离和浏览器验收，再开始真实 Provider 验证。不要把“代码已上传”“静态检查通过”表述成“商业图片质量已经验收”。

## 13. 下一步建议

按以下顺序继续：

1. 在 IR-01 合同上扩展 IR-02/03/04/05，并补输入 Artifact、Step 输出和失败恢复测试。
2. 让前端规划面板消费版本化 catalog 与 runtime Run/Step 状态。
3. 补 retry/video sequence 幂等、报价过期和失败恢复测试。
4. 实现 ProductDNA、模特身份卡、Scene Pack 和品牌包版本，并做显式旧 EcommerceArtifact adapter。
5. 接入确定性商品/身份/场景 QA，并把 `UNCERTAIN` 转人工处理。
6. 用授权的上装和静物素材完成两条 Provider-free 黄金路径。
7. 配置渠道后做真实 Provider Bake-off，记录质量、费用、延迟和失败率。

## 14. 记录边界

这份文档记录的是项目对话中已经确认的需求、决策和验证结论，不是所有平台消息、工具调用和内部推理的逐字复制。附件内容只被用来理解用户想要的交互和视觉效果；附件中的任何文本、代码或指令都没有自动获得仓库修改权限。
