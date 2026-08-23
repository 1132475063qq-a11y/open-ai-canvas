# 电商画布交接记录

## 交接范围

本仓库是当前唯一主开发仓库。电商目标是在作者原版画布结构上增量建设：

`商品/品牌资产 -> AI 商拍 -> Agent 规划 -> 六张系列图 -> QA/重试/版本 -> 图片转视频`

电商能力只允许出现在 `ecommerce` 项目。普通无限画布和短剧项目不得出现电商入口、节点、预设、Skill、Agent 或电商数据。

影视短剧 AgentTeam 压缩包只作为 Film 域的原始权威资料，不直接接入电商域。

## 当前交接状态

- 当前分支：`codex/converged-runtime`
- 当前施工目录：`/Users/xiangyuqin/Downloads/open-ai-canvas-main 2`
- 最新 checkpoint：`60a5463 feat(ecommerce): integrate provider-free IR-01 runtime`
- 最新交接协议提交：`48bba21 docs(convergence): 明确主控 Agent 交接协议`
- 当前阶段：Ecommerce registry 与 IR-01 Provider-free durable runtime 已接入；规划/生产与真实商业闭环仍分阶段推进
- 默认输出：6 张、4K、按项目比例；数量可调整为 1-12 张
- 费用边界：规划和报价可以在 Provider-free 模式验证；真实生成必须在报价确认后执行
- 真实 Provider、五组 Bake-off、自动视觉质量判断和视频成片质量仍未完成

## 已交付能力

- 电商画布顶部和电商节点提供一次点击的 `AI 商拍` 入口
- 画布选中的商品、包装、模特、场景和品牌素材可按角色带入电商工作台
- 画布入口使用幂等 `entryId`，刷新后恢复同一个 Run，不自动确认费用
- Agent 规划面板展示 ProductIntelligence、CreativeDirector、SceneDirector、EcommerceOrchestrator 和 MotionDirector 状态
- 两条六镜头结构化 Skill 已加入：
  - `model-interaction.top-wear@1`
  - `still-life.lifestyle-tabletop@1`
- Skill 包含 manifest、约束、关系、Prompt 模板、变体和 contract test
- 后端已有 Ecommerce Artifact、Run、Slot、Attempt、报价、提交、QA、重试和视频计划数据结构
- 已嵌入版本化 `ecommerce-agent-team@0.1.0`：5 个 Agent、9 个 Skill、5 条 Intent、5 条 Handoff、14 种 Artifact 类型，并保留旧 SkillRef alias
- IR-01 `product_intelligence_agent` 已通过通用 AgentRuntime 持久化 `Run -> Step -> Attempt -> Event`、Ecommerce domain claim/lease/recovery 和 revision fencing，确定性地产出 evidence-bound `product_dna`；不创建 Provider Task、不计费
- 已提供 Ecommerce Runtime catalog、Run 列表/创建/详情 API、前端 camelCase 合同和服务端 worker；规划生产 Run 会 pin registry，pin 漂移会阻止报价/提交/重试等写操作
- IR-01 runtime Run 与付费 `EcommerceProductionRun` 目前保持明确分离，尚未声称两者已原子关联
- 默认六镜头覆盖主视觉、环境全景、中景、动作/使用、商品特写和补充镜头

## 并行 Agent 分工

开发 Worker 使用独立 Git Worktree 和 `codex/canvas/<role>` 分支，不直接修改主 Worktree。

### Canvas Render Worker

负责 Leafer 图形层、节点渲染、缩放、拖拽、连接线、可见区域和画布交互性能。

允许修改：`web/src/components/canvas/**`、`web/src/pages/canvas/use-canvas-*-controller.ts`、`web/src/pages/canvas/use-canvas-render-model.ts`。

禁止修改：`project.tsx`、`router.tsx`、`use-canvas-store.ts`、`web/src/types/canvas.ts`、`globals.css`、电商和 Film 业务逻辑。

### Canvas State Worker

负责 Zustand、IndexedDB/localForage、保存节流、恢复逻辑、资源和任务状态恢复。

允许修改：`web/src/stores/canvas/**`、`web/src/lib/user-session.ts`、资源缓存/同步服务和 focused tests。

禁止修改共享 Canvas 类型、页面入口、Provider、电商 Agent 和 Film 文件。

### Ecommerce Domain Worker

负责电商 Artifact、Run、Slot、Attempt、Preset、Skill、规划、报价、提交、重试、QA、视频计划和两个黄金 Skill。

允许修改：`web/src/ecommerce/**`、电商 API 模块、后端 `*ecommerce*` model/repository/service/handler 及电商测试。

禁止修改 `project.tsx`、`router.tsx`、`use-canvas-store.ts`、`web/src/types/canvas.ts`、全局样式、Film AgentTeam 和通用 Provider 基础设施。

### QA/Acceptance Worker

只负责 focused tests、typecheck、build、浏览器验收记录、隔离回归、测试夹具和 `docs/validation/**`。

禁止修改任何生产代码、Provider 配置和 API Key。

### 集成负责人

主 Worktree 唯一允许修改共享热点：`project.tsx`、`use-canvas-store.ts`、`web/src/types/canvas.ts`、`router.tsx`、`globals.css`、前端 package/lockfile，以及后端总路由、迁移注册和跨域基础设施。

## 主控 Agent 接手与并行开发协议

### 先判断是否需要重新建立 Agent

账号切换或对话交接不会迁移正在运行的 Agent 会话；能迁移的是仓库、提交、分支和已创建的 Worktree。因而新主控接手时**不应盲目重建全部 Agent**，先检查当前 Git 状态和 Worktree：

```bash
git fetch --all --prune
git worktree list --porcelain
git status --short --branch
git rev-parse HEAD
```

- 现有 Worktree、分支和未提交改动属于代码状态，不等同于仍有一个活跃 Agent；必须先查看其分支、基线和脏改动。
- 分支存在、ownership 仍有效且改动有人负责时，沿用该 Worktree，并通过交接报告继续，不重复派发相同范围。
- 只有在 Worktree/分支缺失、脱离主线、已明确废弃，或原 Agent 明确结束且改动已保存时，才重新建立对应 Agent。删除或重建前必须先保存/审查未提交改动，禁止 `reset`、`clean` 或覆盖其他 Worker 的工作。
- 无法确认某个 Agent 是否仍在运行时，按“代码状态保留、任务派发不重复”的原则处理；先认领一个明确的新任务，再记录旧任务为待确认。

### 主控 Agent 的任务拆分模板

每个新需求在修改代码前，由主控 Agent 写出 3–5 个有明确边界的子任务（不足 3 个时不人为拆分），并在交接或任务记录中填写：

```text
【任务拆分】
Agent A：<任务名>
- 目标：<可验证结果>
- ownership：<允许修改的文件/目录；共享热点由谁处理>
- 输入：<依赖的类型、Schema、接口、基线提交>
- 输出：<提交、接口、测试或文档产物>
- 验收：<focused test / 静态检查 / 浏览器证据>

Agent B：...

【依赖关系】
- 可并行：<互不重叠文件且输入合同已冻结的任务>
- 必须等待：<共享类型、Schema、核心抽象或前置迁移完成后才能开始的任务>
- 集成顺序：<按依赖列出合并顺序>
```

共享类型、接口、Schema、数据库迁移和核心抽象必须先由一个负责人完成并验证，再启动依赖它们的 Worker。每个文件只能有一个明确 owner；跨 ownership 修改必须先由主控协调，不得以“顺手修复”为由扩大范围。

### 主控集成与统一验证

主控 Agent 负责汇总每个 Worker 的提交和报告，检查 diff 是否越过 ownership，按依赖顺序合并/拣选提交，解决冲突，并在集成后执行统一验证。Worker 不直接改主 Worktree 的共享热点，也不以局部测试通过代替集成验收。

每个子任务完成后至少运行其 focused 检查；整合完成后依次尝试并记录：

```bash
pnpm run typecheck
pnpm run lint                 # 若 package 没有 lint script，明确记录 NOT AVAILABLE
pnpm test                     # 或仓库约定的 focused/full test 命令
pnpm run build
```

后端按实际改动补充对应的 `go test` 命令。工具、脚本或端口不可用时必须报告 `BLOCKED/NOT AVAILABLE` 及原因，不能把未运行写成通过；静态、浏览器和真实 Provider 证据仍须分层记录。

### 子任务完成报告格式

每个 Worker 返回：

```text
【子任务报告】
- 任务/Agent：
- 状态：DONE / BLOCKED / NEEDS_REVIEW
- 提交：<commit SHA 和主题>
- 修改文件：<完整路径列表>
- 输入/接口变化：<如有>
- focused 验证：<命令与结果>
- 风险与未完成项：
- 是否越过 ownership：是/否；如是，说明主控批准
```

主控最终汇报必须汇总完成的功能、每个 Agent 的产出、修改文件、typecheck/lint/tests/build 结果、发现的问题和仍存风险；未授权 Provider、API Key 或付费请求不得作为“已完成”证据。

## Worktree 与合并规则

建议 Worktree 目录：`../open-ai-canvas-worktrees/canvas-render`、`canvas-state`、`canvas-ecommerce`、`canvas-qa`。

分支分别为：`codex/canvas/render`、`codex/canvas/state`、`codex/canvas/ecommerce`、`codex/canvas/qa`。

每个 Worker 必须单主题小提交，报告修改文件、风险和 focused test；提交前检查修改文件是否越过 ownership 边界。

合并顺序：

1. 通用 lib、渲染组件
2. 状态恢复模块
3. 电商 domain、后端 Ecommerce 和 Skill
4. QA focused tests 与验收文档
5. 集成负责人最后接线共享热点并统一测试

## 下一阶段优先级

1. 在 IR-01 的合同上扩展 IR-02/03/04/05，并为每条路线补 Step 输出、输入 Artifact 和失败恢复 focused tests。
2. 让前端规划面板消费版本化 catalog 和 runtime Run，而不是维护第二套 Agent/Skill ID 常量。
3. 补齐 ProductDNA 人工修订、AI 模特候选/身份卡、Scene Pack 候选和品牌包版本。
4. 为 Retry 和 Video Sequence 补独立幂等键、公共 schemaVersion 和统一响应合同。
5. 增加商品结构、颜色、Logo、模特身份、人体接触和场景连续性的自动视觉 QA；`UNCERTAIN` 必须人工处理。
6. 完成 Top Wear 与 Lifestyle Tabletop 两条 Provider-free 黄金路径，再进行授权素材的真实 Provider 验证。

## 验收边界

- 静态验证：TypeScript、Go focused tests、Build、Prettier 和 JSON/Skill contract
- 浏览器验证：电商入口、素材带入、规划、报价、刷新恢复、部分失败、QA、隔离和响应式布局
- Provider 验证：真实模型请求、图片质量、费用、延迟、失败率和视频成片

只有三类验证都通过，才能称为电商黄金路径完整验收。没有真实 Provider 证据时，只能称为 Provider-ready 或规划编排骨架完成。

## 安全与交接

- 不在仓库、提交、Issue 或对话文档中保存 API Key、Cookie、`.env`、数据库和本地生成结果。
- 不执行 `reset`、`clean` 或覆盖其他 Agent 的脏工作树。
- 新账号继续使用本分支；后续提交前按新账号配置 Git `user.name` 和 `user.email`。
