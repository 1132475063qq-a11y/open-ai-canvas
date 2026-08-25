# 账号交接记录

这份文档是账号切换时的当前工作树快照。用户本轮要求是：继续处理“返图到画布”，并把当前工作上传到 GitHub。仓库中的 `AGENTS.md`、设计文档和旧的收敛报告属于开发约定或历史背景，不等同于本轮新增需求；继续开发时应同时遵守它们和本交接记录。

## 先看结论

- 当前可继续开发的 Git 仓库是 `open-ai-canvas/`，不是旁边的下载副本。
- 当前工作分支是 `codex/upstream-plugin-integration`，基线提交为 `eafc699`（`feat: integrate plugin-based protocols and runtime fixes`）。
- `docs/convergence/CONTINUE-HERE.md` 记录的是另一条 `codex/converged-runtime` 收敛线；它的历史结论不能直接当作当前分支状态。
- “创作页生成结果 -> 添加到画布”的通用链路已经在当前代码中实现，并有专项静态/单元测试；尚未在本次交接前完成真实浏览器点击和真实 Provider 生成验收。
- 当前未提交的提示词 skill 修改需要保留，已与本交接文档一起准备上传；不要删除或覆盖。

## 仓库和分支

| 项目       | 当前值                                                                 |
| ---------- | ---------------------------------------------------------------------- |
| 工作树     | `/Users/xiangyuqin/Documents/ChatGPT/无限画布-短剧/open-ai-canvas`     |
| 当前分支   | `codex/upstream-plugin-integration`                                    |
| 当前 HEAD  | `eafc699c7d6ea1a23e6b39011fd7a2d14f3e9f9f`                             |
| 可写远端   | `handoff` -> `https://github.com/1132475063qq-a11y/open-ai-canvas.git` |
| 原作者远端 | `origin` -> `https://github.com/ddcat-ai/open-ai-canvas.git`           |
| 参考副本   | `/Users/xiangyuqin/Downloads/open-ai-canvas-main 2`                    |

参考副本有自己的工作树和数据库，不能把它正在运行的页面、端口或数据当作当前分支证据。外层目录中的 `root-control-*.patch` 是针对参考副本的未应用补丁，不属于本次上传内容。

### 当前工作树中必须保留的改动

```text
M  backend/internal/agentruntime/assets/film/v1.3.1/source/skills/ai-video-shot-prompt/SKILL.md
?? backend/internal/agentruntime/assets/film/v1.3.1/source/skills/ai-video-shot-prompt/references/storyboard-prompt-standard.md
```

改动含义：短剧视频提示词 skill 强制读取新的 `storyboard-prompt-standard.md`，并按固定字段顺序输出分镜决策、独立视频 Prompt、连续性和验收条件。它不是返图到画布的实现，但属于当前账号已经开始的工作，交接时不要回滚。

## “返图到画布”当前实现

当前确认的入口是创作页结果卡片中的“添加到画布”。核心代码如下：

1. `web/src/pages/create/index.tsx`
   - 生成结果完成后，从素材库按 `messageId`、`taskIds`、`outputIndex/resultIndex` 和结果 URL 找出本次结果对应的资源。
   - 只有资源数量与结果数量完整匹配时，才生成 `mode=handoff&asset=...` URL；否则按钮退化为“打开画布”，不会伪装成已返图。
2. `web/src/lib/canvas/canvas-asset-handoff.ts`
   - `creationResultAssetIds`：按结果顺序映射图片/视频 Asset ID，优先精确匹配 URL，完整资源集才允许顺序兜底。
   - `canvasAssetHandoffAttempt`：读取 URL 中的 Asset ID；只要有一个资源尚未物化，就整体等待，不做部分插入。
   - `uninsertedCanvasAssetHandoffPayloads`：用节点 metadata 中的 `assetId` 防止刷新或重试重复建节点。
   - `finalizeCanvasAssetHandoff`：先持久化节点并 flush，再移除 URL 中的 handoff 参数；持久化失败时保留参数以便重试。
3. `web/src/pages/canvas/index.tsx`
   - `mode=handoff` 自动新建一个自由画布并把查询参数转发到具体项目页。
4. `web/src/pages/canvas/project.tsx`
   - 等待项目和素材库 hydrate；资源齐全后调用 `handleProjectAssetsInsert` 创建图片/视频节点。
   - 持久化成功后才清理 handoff 参数。
5. 生成任务资源化链路
   - `web/src/services/project-asset-sync.ts` 的 `materializeGenerationTaskAssets` 将任务输出落入统一 Asset store。
   - `web/src/services/canvas-generation-consumer.ts` 与 `web/src/lib/canvas/canvas-generation-task-sync.ts` 负责已有生成节点的结果回填、幂等 effect key 和持久化确认。

### 已覆盖的失败边界

- 结果资源还没有 materialize：不创建空节点，保留 handoff URL 等待下一次 hydrate。
- 只找到部分结果：不生成“添加到画布”路径，避免漏图。
- 刷新/重复 effect：通过 Asset ID 和 generation effect key 去重。
- 画布持久化失败：不清除 handoff 参数，允许重新进入后重试。
- 图片和视频都走同一套 Asset ID 传递，但节点 payload 仍保留各自的尺寸、MIME、时长和 storage key。

### 当前未证明的事项

- 尚未用当前分支启动全新前后端并完成一次真实浏览器点击“生成 -> 添加到画布 -> 刷新 -> 节点仍在”。
- 尚未用真实 Provider 生成并验证远端结果 URL、服务器资源和浏览器 Asset store 三者的一致性。
- Film/Ecommerce 生产面板的结果不是同一个通用 handoff 入口：Film 视频序列是从生产面板导入时间线，Ecommerce 是 linked canvas projection。若用户说的“返图”指这两个入口，应分别沿对应域的 API/投影链路排查，不要只改 `canvas-asset-handoff.ts`。
- 当前分支的 `CONTINUE-HERE.md` 生产控制面结论来自 `codex/converged-runtime`，不能据此声称 Film/Ecommerce 已完成真实 Provider 验收。

## 本地启动

先确认没有复用参考副本的进程或数据目录。开发数据必须放在 Git 忽略目录，不要使用 `backend/data`。

```bash
cd /Users/xiangyuqin/Documents/ChatGPT/无限画布-短剧/open-ai-canvas
mkdir -p .local/project-workbench-debug .local/cache/go-build .local/cache/go-mod
```

后端终端：

```bash
cd /Users/xiangyuqin/Documents/ChatGPT/无限画布-短剧/open-ai-canvas/backend
CANVAS_BACKEND_DATA_DIR=../.local/project-workbench-debug go run ./cmd/server
```

前端终端：

```bash
cd /Users/xiangyuqin/Documents/ChatGPT/无限画布-短剧/open-ai-canvas/web
bun install
bun run dev
```

默认前端为 `http://localhost:3000`，Vite 将 `/api` 代理到 `http://127.0.0.1:8080`。端口冲突时先处理旧进程，或使用项目现有的 Vite/Compose 端口变量；不要把参考副本的 4173/8080 页面当作当前分支验收。

Canvas Agent 只有在需要本地 Agent/MCP 时才启动：

```bash
cd /Users/xiangyuqin/Documents/ChatGPT/无限画布-短剧/open-ai-canvas/canvas-agent
npm install
npm run build
node dist/index.js
```

## 验证顺序

先跑返图专项测试，再跑前端类型和构建；测试失败要记录真实输出，不能把静态源码检查写成浏览器验收：

```bash
cd /Users/xiangyuqin/Documents/ChatGPT/无限画布-短剧/open-ai-canvas/web
bun test test/create-canvas-handoff.test.ts
bun run typecheck
bun run build
```

返图专项测试覆盖 URL 去重、资源顺序、任务 metadata 关联、缺资源等待、重复插入去重、持久化成功后清理参数和持久化失败保留参数。完整前端测试命令见 `web/package.json`，后端验证按 `AGENTS.md` 使用 `go test ./...` 或对应专项包。

## 账号切换后的接续步骤

```bash
cd /Users/xiangyuqin/Documents/ChatGPT/无限画布-短剧/open-ai-canvas
git fetch handoff
git checkout codex/upstream-plugin-integration
git pull --ff-only handoff codex/upstream-plugin-integration
git status --short
```

继续返图问题时，建议先复现并记录以下事实：

- 创作结果消息的 `messageId`、关联 `taskIds` 和 `resultUrls`；
- Asset store 中是否存在同一批资源，以及 metadata 的 `source/messageId/taskId/outputIndex`；
- 进入 `/canvas?mode=handoff&asset=...` 后，`assetsHydrated`、`projectLoaded` 和 `canvasAssetHandoffAttempt` 的结果；
- `handleProjectAssetsInsert` 是否返回节点；
- `flushCanvasStorePersistence()` 是否成功，以及刷新后节点 metadata 是否仍有 `assetId`。

若要继续合并 `codex/converged-runtime` 的生产控制面改动，先在新分支做显式比较或 cherry-pick；不要直接把两条线强行覆盖。所有密钥、Cookie、数据库文件和真实 Provider 返回内容都不要写入提交或此文档。

## GitHub 上传记录

本次上传目标是 `handoff/codex/upstream-plugin-integration`。提交完成后，以以下命令确认新账号可以接手：

```bash
git log --oneline -3
git status --short
git ls-remote --heads handoff codex/upstream-plugin-integration
```

如果当前账号没有该远端的写权限，保留本地提交并把完整错误交给用户，不要改写远端 URL，也不要把 Token 写进 remote URL、日志或文档。
