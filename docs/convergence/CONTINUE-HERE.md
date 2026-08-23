# Continue Here

## Canonical Checkout

- Working tree: `/Users/xiangyuqin/Downloads/open-ai-canvas-main 2`
- Branch: `codex/converged-runtime`
- Writable handoff remote: `https://github.com/1132475063qq-a11y/open-ai-canvas.git`
- Handoff branch: `handoff/codex/converged-runtime`
- Author upstream: `https://github.com/ddcat-ai/open-ai-canvas.git`

This is the writable handoff checkout and the current source of truth for code,
commits, and account handoff. Do not continue from the historical checkout at
`/Users/xiangyuqin/Documents/ChatGPT/无限画布-短剧/open-ai-canvas` unless it has
first been fast-forwarded and independently checked.

## Agent Handoff Protocol

The handoff does not transfer live Agent sessions. Do not recreate every Agent
just because the account changed: inspect the current branches, Worktrees, HEAD,
and uncommitted ownership first. Reuse a valid Worker when its scope and saved
changes are clear; recreate only a missing, detached, abandoned, or explicitly
finished Worker, preserving any uncommitted changes before rebuilding. The
canonical task-splitting template, dependency rules, ownership boundaries,
integration responsibilities, and final `typecheck`/`lint`/`tests`/`build`
report format are recorded in
`docs/convergence/ECOMMERCE-HANDOFF.md` §“主控 Agent 接手与并行开发协议” and
`docs/convergence/PROJECT-CONVERSATION-LOG.md` §7.1–7.2.

At the beginning of each new task, the primary Agent must record 3–5 bounded
subtasks (unless fewer are genuinely needed), mark parallel versus serial
dependencies, and assign one owner per file. Shared contracts come first; the
primary Agent integrates Worker commits and reports unavailable checks as
`NOT AVAILABLE/BLOCKED` rather than claiming success. No real Provider or
credential may be used during this handoff-only phase.

## Current Product Boundary

The durable Film and Ecommerce control planes are implemented, but the final
goal is not complete. Do not infer commercial acceptance from type checks,
fixtures, or the existence of quote and QC tables.

Open gates:

1. Film needs an authorized real logical text model for the backend Agent
   Worker. The browser-side GPT CLI adapter is currently a normal local text
   generator; it is not a durable Film Run/Step/Attempt executor.
2. Film still needs real multi-shot image and video generation, model and human
   QC, cross-shot continuity review, and one final FFmpeg export.
3. All 15 Intent routes and 11 Handoff routes still need approved real-provider
   execution evidence.
4. Ecommerce still needs five authorized Provider bake-off packs and the Top
   Wear and Lifestyle Tabletop commercial golden paths.
5. Desktop, tablet, and phone release-candidate browser acceptance remains
   open.

## Local Runtime Evidence

The existing local app uses:

- Frontend: `http://localhost:4173`
- Backend: `http://127.0.0.1:8080`
- Data: `/Users/xiangyuqin/Downloads/open-ai-canvas-main 2/.local/project-workbench-debug`

Runtime configuration in that database is not Git data. On 2026-08-23 the
administrator published one available logical image model named
`GPT Image 2 · Film`, routed to the configured and priced `gpt-image-2` system
channel. The Film panel then discovered the model and exposed a vertical image
quote path. No Film quote, paid Task, Provider request, Result, or QC fact was
created by that check.

The existing Film acceptance project is:

- Project: `db7f3a196f85c140e7778a390f109ad8`
- Root Run: `25b725c79f764bfbf2dd78ad531fa590`
- State: `awaiting_human`
- Problem: the Run predates logical text-model publication and has no pinned
  `logicalModelId`; approving it would fail in the backend Agent Worker.

The UI and backend now both block approval of that unbound Run, allow
cancellation, and block creation of a replacement until an available logical
text model is selected. Approval and failed-Step retry also re-resolve the
pinned text-model route so an archived model or disabled route cannot resume
execution.

## Latest Engineering Slice

- Film vertical image selection targets a real `2160x3840` size when the route
  declares it, instead of treating the abstract `9:16` ratio as a 4K size.
- Provider `quality=high` is displayed as `High quality`, not mislabeled as
  `4K`; pixel size and provider quality remain separate request facts.
- A 4K target never silently escalates to an available 8K size.
- Model selection and legacy Run-envelope behavior have focused tests, and the
  test is included in the web test command.
- The Film Agent API now requires an available logical text model when creating
  a Run. Human approval and failed-Step retry recheck availability; an unbound
  legacy Run remains readable and cancellable. Automatic Handoff scheduling
  also rechecks the inherited model before creating a child Run.
- The Film panel now projects pinned text-model readiness as missing,
  unavailable, or available. It hides approval for unavailable Runs, disables
  impossible retries, clears stale creation selections, and surfaces backend
  decision errors in the panel.
- Film video-sequence planning now preserves the user-selected shot order and
  per-shot 1-30 second durations instead of re-sorting every sequence by the
  source storyboard position. An owned, ready audio resource can be attached
  as a frozen sequence input and is imported once into the existing audio
  timeline track when the accepted sequence is opened.
- Created Film video sequences can now be edited and restored through a
  revision-fenced backend API. Saving order, duration, aspect ratio, title, or
  music appends new `video-sequence` and `continuity-ledger` Artifact revisions;
  duration or aspect-ratio changes reset only the affected Slot result links,
  while prior paid Attempts remain readable history.
- Ecommerce now has an embedded `ecommerce-agent-team@0.1.0` registry with 5
  Agents, 9 Skills, 5 Intent routes, 5 Handoff routes and 14 canonical
  Artifact types. Legacy frontend Skill references are resolved through
  declared aliases, while persisted responsibility and runtime Step IDs use
  canonical registry IDs.
- Ecommerce IR-01 (`product_intelligence_agent`) is a provider-free durable
  slice: one idempotent Run creates one ready Step, the domain-scoped worker
  claims/recoveries one fenced Attempt, and a deterministic executor writes a
  review-stage `product_dna` Artifact revision without a Provider Task or
  billing fact.
- Ecommerce catalog and Run control-plane endpoints plus frontend types are
  available. Paid `EcommerceProductionRun` planning pins the registry and
  rejects drift before quote/submit/retry/video writes. The planning Run and
  the standalone IR-01 runtime Run are intentionally not claimed to be an
  atomic pair yet.
- Ecommerce IR-01 input normalization also rejects inline media Data URLs and
  credential-shaped keys before durable InputJSON is written.
- The currently observed local 4173/8080 processes were started from this
  checkout at the earlier `1406b5f` commit. Their page or API behavior is not
  evidence for the current `60a5463` tree; restart the runtime from this branch
  before browser acceptance.
- Service and HTTP fixtures now use a complete synthetic `LogicalModel ->
Revision -> Route -> ChannelModel -> SystemChannel` text path. Focused tests
  cover missing, unknown, archived, disabled, restored, and legacy model states.
- The web type check, focused Prettier check, and production build pass. The
  Ecommerce registry, repository fencing, IR-01 service, pin contract and
  backend build/vet checks pass with the bundled Go 1.25 toolchain. Bun is not
  installed, and the web package has no `lint` script, so those checks are
  recorded as `BLOCKED`/`NOT AVAILABLE` rather than inferred green.
- The current sandbox denied opening a second localhost port, so the new UI
  build did not receive a fresh Playwright screenshot. The existing app did
  prove the backend logical-model publication and discovery path.

## Next Execution Order

1. Configure an authorized backend logical text model, or implement a durable
   client-Worker lease and evidence contract before allowing local GPT CLI to
   execute Film Steps. Do not call the browser-only adapter from the backend or
   bypass Run/Step/Attempt persistence.
2. Cancel the unbound legacy Run and create a new Film Run with the logical text
   model pinned. Complete human review and lock storyboard, image Prompt, and
   feasibility Artifacts.
3. Create a `GPT Image 2 · Film` quote and verify the frozen request contains
   `size=2160x3840` and `quality=high`. Ask for confirmation immediately before
   the paid submit.
4. Persist the real Result, run model visual QC, record human QC, and prove a
   paid retry as a new quote and Attempt when needed.
5. Publish a real image-to-video logical model, complete two shots, use and
   persist the sequence ordering/duration/music controls, then prove refresh
   recovery, continuity review, timeline import, and FFmpeg export.
6. Continue Ecommerce bake-off and golden paths only with authorized packs and
   persisted human scoring.
7. Extend the provider-free Ecommerce runtime from IR-01 to the remaining
   declared routes, then connect accepted ProductDNA revisions to the paid
   planning adapter in an explicit transaction.

## Verification Commands

```bash
cd "/Users/xiangyuqin/Downloads/open-ai-canvas-main 2/web"
pnpm run typecheck
pnpm run build
bun test test/film-production-model.test.ts test/film-workspace-isolation.test.ts

cd ../backend
go test ./internal/service -run 'TestFilmProduction|TestFilmVisualQC|TestFilmVideo|TestFilmMultiShot' -count=1
go test ./internal/service -run 'TestFilmAgent|TestCreateFilmAgent|TestArchivedFilm|TestRetryFilmAgent|TestProcessNextFilmAgent|TestLockedFilmArtifacts' -count=1
go test ./internal/repository ./internal/handler ./internal/database ./internal/agentruntime -count=1
```

Never write API keys into this document, commits, logs, prompts, or Artifact
payloads. Rotate any credential that was pasted into chat before a release.
