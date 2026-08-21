# Convergence Acceptance Gates

No phase advances on file presence alone. Each gate requires executable tests
and persisted runtime evidence.

## Gate 0: Upstream Baseline

- Branch and immutable baseline tag exist.
- Source and migration matrix are recorded.
- Web production build passes.
- Canvas Agent Node/tsx suite passes.
- Upstream-only failures are recorded separately from convergence regressions.
- Donor and AgentTeam sources remain unchanged.

## Gate 1: Shared Runtime

- Registry rejects duplicate IDs, missing versions, invalid Agent/Skill links,
  unknown artifact types, and invalid routes.
- A run persists before execution and survives process restart.
- Steps support `planned`, `ready`, `running`, `awaiting_human`, `completed`,
  `failed`, and `cancelled` without illegal regressions.
- Invocation attempts are append-only and idempotent.
- Structured output is schema-validated before creating an artifact revision.
- Human gates pause and resume from an explicit decision record.
- Provider credentials never enter prompts, logs, artifacts, or canvas data.

Current status: **passed for the Film shared runtime**. Durable
Run/Step/Attempt/Event state, optimistic revision fencing, immutable Artifact
revisions, idempotent creation, human pause/resume, restart restoration, secret
rejection, provider-neutral execution, lease recovery, strict structured output
validation, and atomic completion are implemented and tested.

## Gate 2: Complete AgentTeam Registration

- Exactly 9 Agent profiles, 17 Skills, 15 intent routes, and 11 handoff routes
  load from versioned runtime assets.
- Every route references registered inputs and outputs.
- The original AgentTeam validation suite still passes.
- Runtime contract tests prove each registry entry is executable, not merely
  discoverable.

Current status: **registration complete, execution proof incomplete**. Exact
counts, references, versioned source digests, startup failure behavior, and
compilation of all 15 intent routes into Agent-owned Skill Steps pass. HR-01
through HR-08 have structured automatic scheduling contracts; HR-09/10/11 are
kept out of the Agent worker by design. No Gate 2 pass is claimed until every
route completes through its intended real Executor or orchestration path and
persists validated output Artifacts.

## Gate 3: Film Golden Path

One authorized short-drama fixture completes:

`brief -> planning -> script -> review -> visual assets -> storyboard -> prompts -> generation attempt -> result -> QC -> accept/retry`

- Refresh/restart restores the exact active step.
- At least one human decision pauses and resumes correctly.
- A failed generation retries as a new Attempt without overwriting history.
- Canvas shows only key artifacts and attention items.

Current status: **in progress**. The tested backend slice completes
`script -> REVIEW -> human LOCKED -> HR-03 -> storyboard -> human LOCKED ->
HR-04 + HR-05`, including durable Trigger recovery and idempotent Handoff Run
creation. Planning, full visual/sound/production/QC progression, media
generation, and canvas projection remain open.

## Gate 4: Film User Closure

- Image and video generation paths produce real persisted Results.
- Automatic and human QC are distinct evidence records.
- Character, scene, prop, costume, spatial, and temporal continuity checks run
  across scenes.
- Accepted shots can be sequenced and exported with existing timeline/FFmpeg
  capabilities.
- Version rollback changes the selected revision without deleting history.

## Gate 5: Ecommerce Migration

- Ecommerce models, services, API, and UI are ported behind domain gates.
- Existing preset, Run/Slot/Attempt, quote, retry, QA, version, and video-plan
  tests pass against the upstream mainline.
- No Film AgentTeam import is present in Ecommerce runtime packages.

## Gate 6: Ecommerce Commercial Paths

- `Top Wear` model interaction and `Lifestyle Tabletop` both complete real
  generation, refresh recovery, partial failure, paid single-slot retry, QA,
  revision, and cost history.
- Default six-image output contains at least five commercially usable images.
- Five authorized product packs complete the same provider bake-off rubric.
- Accepted images contain no critical product structure, color, logo, identity,
  anatomy, or contact errors.

## Gate 7: Product Release

- Infinite, Film, and Ecommerce workspace menus, nodes, Skills, Agents, routes,
  caches, and APIs pass explicit isolation tests.
- Desktop, tablet, and phone Playwright checks show no blank canvas, overlap,
  clipping, or inaccessible primary action.
- Upgrade from the pinned upstream baseline is documented and replayable.
- A release candidate passes build, unit, integration, recovery, and golden-path
  suites with no unclassified regression.
