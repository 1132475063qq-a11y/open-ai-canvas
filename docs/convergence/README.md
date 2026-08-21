# Product Convergence Baseline

## Objective

Use the author's latest `open-ai-canvas` release as the only product mainline,
keep its canvas interaction model, and add two isolated production domains:

1. Film: a real runtime for the 9 Agents, 17 Skills, 15 intent routes, and 11
   handoff routes from AgentTeam v1.3.1.
2. Ecommerce: the existing production workbench, migrated only after the film
   runtime and film golden path are complete.

The infinite canvas remains a general-purpose workspace. Film and ecommerce
capabilities must not appear there unless the user explicitly opens the
corresponding project workspace.

## Pinned Sources

| Role | Location | Revision | Policy |
|---|---|---|---|
| Product mainline | `/Users/xiangyuqin/Downloads/open-ai-canvas-main 2` | `ce86bb7` on 2026-08-21 | Modify here |
| Mainline branch | same repository | `codex/converged-runtime` | All convergence work |
| Baseline tag | same repository | `baseline/upstream-1635927-20260821` | Never move |
| Feature donor | `/Users/xiangyuqin/Downloads/infinite-canvas-short-drama` | `codex/phase4-film-nodes` plus its dirty worktree | Read only; cherry-pick concepts and scoped modules |
| AgentTeam authority | `/Users/xiangyuqin/Downloads/影视短剧AgentTeam_v1.3.1_Conflict_Hardened` | v1.3.1 | Read only; compile contracts into runtime data |
| Downloaded ZIP checkout | `/Users/xiangyuqin/Downloads/open-ai-canvas-main` | v1.0.49 | Historical reference only |

The donor worktree contains user changes. Do not reset, clean, format, or use it
as a merge target.

## Fast Delivery Path

Development follows one vertical slice, then registry expansion:

`intent -> route decision -> Agent invocation -> Skill execution -> versioned artifact -> human gate -> handoff`

The runtime is implemented once. AgentTeam definitions are compiled into typed
registry entries and validated at startup. Adding the remaining Agents, Skills,
and routes must not require new execution engines or canvas node types.

Film and ecommerce Agents run behind the workspace UI. They are represented by
status, artifacts, evidence, and decisions, not by dozens of permanent nodes on
the canvas.

## Baseline Verification

Recorded on 2026-08-21 before convergence changes:

- Web production build: pass.
- Canvas Agent tests under Node/tsx: 281 pass, 0 fail, 5 platform skips.
- AgentTeam file-mode validator: pass for 9 Agents, 17 Skills, 15 intent
  routes, and 11 handoff routes. This is schema evidence, not runtime evidence.
- Web Bun suite: upstream test-environment failure at
  `web/src/components/model-logo.tsx` because `import.meta.glob` is unavailable
  in the Bun test transform. The isolated suite reports 0 pass, 1 fail, 1
  loader error.
- Backend Go suite: five upstream baseline failures were observed locally:
  `TestChannelFromRequestStoresAndClearsHeaders`,
  `TestRuntimeConcurrencyUsesEnvironmentFallback`,
  `TestQiniuKodoSettingAllowsMissingCDNBaseURL`,
  `TestOnlyResumableNewAPIChannel2VideoDeadlinesStayRunning`, and
  `TestResumableVideoDeadlineUsesResolvedSystemChannelProtocol`.

## Current Convergence Verification

Recorded on 2026-08-21 on `codex/converged-runtime` after the Film Executor and
durable Handoff slice:

- `go vet ./...`: pass.
- AgentRuntime, database, repository, handler, server, and all Film service
  tests: pass.
- The service package passes with the five previously classified upstream
  environment-sensitive tests skipped. An unfiltered run currently reproduces
  those same five baseline failures; no new failure is present.
- Film registry startup validation: exactly 9 Agents, 17 Skills, 15 intent
  routes, 11 handoff routes, and 46 canonical Artifact types.
- Film Run service tests: all 15 intent routes compile to owned Agent/Skill
  Steps; create, idempotency, locked inputs, human pause/resume, append-only
  retry, archived-history reads, secret rejection, and domain isolation pass.
- Film HTTP contract: authenticated catalog, create, replay, list, detail, and
  decision resolution and immutable Artifact lock pass; unauthorized,
  invalid-limit, missing-idempotency, stale revision, and oversized-body
  responses are enforced.
- Provider-backed integration: LogicalModel -> ChannelModel -> durable Task ->
  OpenAI-compatible HTTP -> strict Artifact output -> `REVIEW` revision is
  traced through RouteAttempt, API audit, Film Attempt, and Event evidence.
- Handoff integration: a locked script schedules idempotent HR-03; a locked
  storyboard schedules HR-04 and HR-05 in parallel. Trigger recovery, retries,
  terminal failure, OR/AND groups, root-lineage isolation, and exclusion of
  HR-09/10/11 from automatic Agent execution pass.

The five failures above remain upstream baseline issues and currently reproduce
in this network/runtime environment. They do not touch the Film runtime paths.

See `film-runtime-control-plane.md` for the implemented API and the exact
boundary between durable control-plane evidence and real Agent execution.

Use the package-native commands. In this development environment, invoke the
bundled Node executable explicitly for `canvas-agent`; `bun test` is not a
valid substitute because those tests use `node:test`.

## Non-Negotiable Boundaries

- Do not replace or fork the shared canvas shell, viewport, global CSS, provider
  registry, task persistence, billing, or Local Runtime security model.
- Do not copy the donor's full `project.tsx` or other shared application files.
- Do not claim Agent/Skill completion because source files exist.
- Do not claim generated media or QC success without a persisted Attempt,
  Result, and evidence-bearing QC record.
- Locked artifacts are immutable. Changes create a new revision and an impact
  record.
- Paid retries always require a new quote or explicit confirmation.
- Ecommerce implementation begins only after the film golden path exits its
  acceptance gate.
