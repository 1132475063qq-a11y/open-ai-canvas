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
validation, atomic completion, and evidence-fenced project closeout are
implemented and tested.

## Gate 2: Complete AgentTeam Registration

- Exactly 9 Agent profiles, 17 Skills, 15 intent routes, and 11 handoff routes
  load from versioned runtime assets.
- Every route references registered inputs and outputs.
- The original AgentTeam validation suite still passes.
- Runtime contract tests prove each registry entry is executable, not merely
  discoverable.

Current status: **registration complete, route execution contract passed; external-provider breadth remains open**. Exact
counts, references, versioned source digests, startup failure behavior, and
compilation of all 15 intent routes into Agent-owned Skill Steps pass. A
deterministic executor contract now runs every Intent route through Claim ->
Attempt -> validated output Artifact and persists terminal evidence, including
explicit Agent selection for the disambiguated route. The Film workspace now
defaults to deterministic Registry routing, exposes the IR-11 candidate-Agent
choice, and carries current locked Artifact revisions into the selected
Route's filtered input contract. HR-01 through HR-08 have
structured automatic scheduling contracts; HR-09/10/11 are kept out of the
Agent worker by design and now have tested orchestration paths: HR-10 records
project-start authority, while HR-09/HR-11 validate final QC, write an
immutable project summary, and archive atomically. No Gate 2 pass is claimed
until the full route set is also exercised against the intended real Provider
path and its outputs are reviewed.

## Gate 3: Film Golden Path

One authorized short-drama fixture completes:

`brief -> planning -> script -> review -> visual assets -> storyboard -> prompts -> generation attempt -> result -> QC -> accept/retry`

- Refresh/restart restores the exact active step.
- At least one human decision pauses and resumes correctly.
- A failed generation retries as a new Attempt without overwriting history.
- Canvas shows only key artifacts and attention items.

Current status: **in progress**. The tested backend slice completes
`HR-10 -> script -> REVIEW -> human LOCKED -> HR-03 -> storyboard -> human
LOCKED -> HR-04 + HR-05`, including durable Trigger recovery and idempotent
Handoff Run creation. A provider-backed single-Shot slice now also completes
`locked storyboard/prompt/feasibility -> cost quote -> paid Attempt -> persisted
image Result -> system hold -> human PASS/FAIL -> paid retry`, including quote
expiry, price drift, idempotency, cancellation/refund, and no-route-fallback
tests. A separate paid visual-semantic QC slice now sends the persisted result
as the first image to an image-capable text model, validates eight ordered
dimensions, persists model evidence as `hold`, and still requires a later human
decision. The video slice now enforces `human-accepted image -> locked per-Shot
video Prompt -> Sequence/Slot -> quote -> paid Attempt -> persisted video
Result -> technical media QC -> human accept/retry`, with a structured
Continuity Ledger and slot-scoped Rework Event for evidence-backed technical
failures. Completed Sequences can be imported idempotently into the existing
TimelineProject and exported by its FFmpeg runtime. A deterministic two-Shot
Provider fixture now also proves partial image failure isolation, refunded
failure, append-only image retry, two video Slots, per-Shot QC, continuity PASS,
and RootRun-scoped refresh recovery. A fixture can also prove the final
`QC -> cross-shot continuity review -> HR-09 -> project-summary -> HR-11 ->
archived` transaction, but this is control-plane evidence rather than a real
generated film. The configured commercial Provider path, real media semantic
QC, and final-film acceptance remain open.

## Gate 4: Film User Closure

- Image and video generation paths produce real persisted Results.
- Automatic and human QC are distinct evidence records.
- Character, scene, prop, costume, spatial, and temporal continuity checks run
  across scenes.
- Accepted shots can be sequenced, pass a current cross-shot continuity review,
  and export with existing timeline/FFmpeg capabilities.
- Version rollback changes the selected revision without deleting history.

Current status: **production control path implemented, commercial acceptance
open**. Real image and video Tasks produce persisted Results; technical system
QC is distinct from human acceptance; Sequence/Slot state survives refresh;
and only fully accepted Sequences can enter the existing timeline/FFmpeg export
path. The Continuity Ledger records per-Shot read-in/write-out, Reference Lock,
and unresolved semantic dimensions without pretending they were visually
verified. Single-image model-backed semantic QC is integrated with independent
quote, cost, Attempt, strict output, and human-acceptance fencing. Real
multi-scene visual continuity, configured commercial-model acceptance,
audio/subtitle finishing, and final-user golden-path evidence remain open.
Artifact rollback is append-only and exposed through the Film version-history
UI, but still requires user-workflow acceptance with real production revisions.

## Gate 5: Ecommerce Migration

- Ecommerce models, services, API, and UI are ported behind domain gates.
- Existing preset, Run/Slot/Attempt, quote, retry, QA, version, and video-plan
  tests pass against the upstream mainline.
- No Film AgentTeam import is present in Ecommerce runtime packages.

Current status: **engineering migration implemented, commercial acceptance
open**. Ecommerce presets, Run/Slot/Attempt, route-and-cost-frozen quotes,
generation, retry, human QA, revision history, and video planning are present
behind Ecommerce project routes. Browser and automated isolation checks keep
Film production UI and AgentTeam semantics out of Ecommerce. The generic
`/canvas` library also derives listing, counts, recent-open, bulk operations,
exports, and previews from freeform-only documents, excluding project-linked
and legacy Film/Ecommerce semantic canvases. Gate 5 remains open until the
migrated paths complete the required recovery and golden-path acceptance
against configured Providers.

## Gate 6: Ecommerce Commercial Paths

- `Top Wear` model interaction and `Lifestyle Tabletop` both complete real
  generation, refresh recovery, partial failure, paid single-slot retry, QA,
  revision, and cost history.
- Default six-image output contains at least five commercially usable images.
- Five authorized product packs complete the same provider bake-off rubric.
- Accepted images contain no critical product structure, color, logo, identity,
  anatomy, or contact errors.

Current status: **Provider evaluation contract ready; commercial evidence
open**. Ecommerce now has durable plan, Attempt, score, blinded preference, and
human GO/MODIFY/STOP records with project/user isolation and idempotency. This
only makes the bake-off resumable and auditable; it does not count as a model
pass. Five authorized product packs and both golden paths still need real
Provider results and human acceptance.

## Gate 7: Product Release

- Infinite, Film, and Ecommerce workspace menus, nodes, Skills, Agents, routes,
  caches, and APIs pass explicit isolation tests.
- Desktop, tablet, and phone Playwright checks show no blank canvas, overlap,
  clipping, or inaccessible primary action.
- Upgrade from the pinned upstream baseline is documented and replayable.
- A release candidate passes build, unit, integration, recovery, and golden-path
  suites with no unclassified regression.
