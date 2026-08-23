# Continue Here

## Canonical Checkout

- Working tree: `/Users/xiangyuqin/Documents/ChatGPT/无限画布-短剧/open-ai-canvas`
- Branch: `codex/converged-runtime`
- Writable handoff remote: `https://github.com/1132475063qq-a11y/open-ai-canvas.git`
- Handoff branch: `handoff/codex/converged-runtime`
- Author upstream: `https://github.com/ddcat-ai/open-ai-canvas.git`

The checkout under `/Users/xiangyuqin/Downloads/open-ai-canvas-main 2` is the
source snapshot used to seed this writable checkout. Continue in the canonical
checkout above so code, commits, and account handoff remain in one place.

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
- The currently observed local 4173/8080 processes belong to a separate
  `/Users/xiangyuqin/Downloads/open-ai-canvas-main 2` checkout at the earlier
  `1406b5f` commit. Their page or API behavior is not evidence for this
  `open-ai-canvas` checkout; start the runtime from this branch before browser
  acceptance.
- Service and HTTP fixtures now use a complete synthetic `LogicalModel ->
Revision -> Route -> ChannelModel -> SystemChannel` text path. Focused tests
  cover missing, unknown, archived, disabled, restored, and legacy model states.
- The web type check, focused Node smoke, Prettier check, and production build
  pass. Bun and Go 1.25 were unavailable in the current shell, so the Bun suite
  and newly changed backend suites were not executed here.
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

## Verification Commands

```bash
cd /Users/xiangyuqin/Documents/ChatGPT/无限画布-短剧/open-ai-canvas/web
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
