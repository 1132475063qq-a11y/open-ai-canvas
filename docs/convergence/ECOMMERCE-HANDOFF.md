# 电商画布交接记录

## 交接范围

本仓库是当前唯一主开发仓库。电商目标是在作者原版画布结构上增量建设：

`商品/品牌资产 -> AI 商拍 -> Agent 规划 -> 六张系列图 -> QA/重试/版本 -> 图片转视频`

电商能力只允许出现在 `ecommerce` 项目。普通无限画布和短剧项目不得出现电商入口、节点、预设、Skill、Agent 或电商数据。

影视短剧 AgentTeam 压缩包只作为 Film 域的原始权威资料，不直接接入电商域。

## 当前交接状态

- 当前分支：`codex/converged-runtime`
- 当前施工目录：`/Users/xiangyuqin/Downloads/open-ai-canvas-main 2`
- 最新 checkpoint：`feat(ecommerce): wire canvas production handoff and golden skills`
- 最新交接文档提交：`f9b0f9b docs(ecommerce): 保存项目对话交接记录`
- 当前阶段：电商规划与生产编排骨架已接入，尚未完成完整商业闭环
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

1. 建立与 Film 同等级的 Ecommerce Agent Runtime 注册、Step、Attempt、事件和 Worker 合同。
2. 消除前端 Skill 与后端私有 Skill 的两套真相，统一 manifest 和 executor 来源。
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
