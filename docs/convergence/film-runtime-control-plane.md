# Film Runtime Control Plane

## Implemented Scope

The Film AgentTeam v1.3.1 control plane is backend-owned and isolated to
`short-drama` projects. It currently provides:

- Versioned, startup-validated definitions for 9 Agents, 17 Skills, 15 intent
  routes, 11 handoff routes, and 46 canonical Artifact types.
- Durable `Run`, `Step`, append-only `Attempt`, `RoutingDecision`,
  `HumanDecision`, ordered `Event`, stable `ProductionArtifact`, and immutable
  `ProductionArtifactRevision` records.
- Optimistic revision fencing for concurrent state changes.
- Atomic root Run creation with locked `project-requirements`, `task`, and
  `routing-decision` Artifact revisions plus the HR-10 completion Event.
- Deterministic explicit-route or exact-trigger routing and Agent ownership
  checks for every compiled Skill Step.
- Human review pause/resume, failed-Step retry as a new Attempt, restart
  restoration, archived-project history reads, and Film/domain isolation.
- A provider-neutral text Executor that resolves the pinned logical model,
  creates or resumes one durable Task per Attempt, calls the existing provider
  queue, and validates a strict JSON Artifact contract before commit.
- Short execution leases with recovery fencing. A restarted worker reuses the
  same Attempt and Task identity instead of issuing a second paid request.
- Atomic successful completion across Attempt, Step, Run, Event, immutable
  Artifact revisions, and newly unblocked Steps. Agent output is always
  `REVIEW`; an Agent cannot assert `LOCKED` in model output.
- Human Artifact locking as a new immutable `LOCKED` revision. The lock Event
  and durable `AgentHandoffTrigger` outbox fact commit in the same transaction.
- Root Run lineage and deterministic Handoff Run idempotency derived from the
  route plus immutable input revision IDs. Static Agent routes HR-01 through
  HR-08 are scheduled automatically; HR-09, HR-10, and HR-11 remain explicit
  orchestration boundaries.
- Handoff input resolution with OR semantics inside a required group and AND
  semantics between groups, restricted to current locked Artifacts from one
  user/project/domain/root lineage.
- Explicit HR-09/HR-11 closeout. A read-only preview returns blockers and a
  SHA-256 fingerprint over project, Registry, Run, Step, Attempt, routing,
  human-decision, Handoff Trigger, and current Artifact facts. User confirmation
  rechecks that evidence under transaction locks, then atomically creates one
  locked `project-summary`, appends ordered HR-09/HR-11 Events, and archives the
  project. Replays return the original Summary.
- Browser-safe catalog output that omits executable Agent developer
  instructions and Skill instruction bodies.
- A backend-owned single-shot image production path that accepts only the
  current locked `storyboard`, `image-prompt-pack`/`prompt-manifest`, and
  `production-feasibility-report` revisions. Prompt text is compiled from the
  structured Prompt Artifact for the exact Shot; clients cannot inject a
  replacement runtime prompt.
- Immutable `FilmProductionQuote`, append-only `FilmProductionAttempt`, and
  append-only `FilmProductionQCReport` facts. A quote freezes the logical
  model revision, physical route, channel capability/price versions, request
  fingerprint, and estimated charge for ten minutes.
- Atomic quote submission across credit reservation, Task, Attempt,
  `generation-attempt` Artifact, and ordered Film Event. Paid retries always
  create a new quote, Task, Attempt, and billing order and may only reference
  the latest retryable Attempt.
- Film Worker completion that requires exactly one accessible persisted image,
  then atomically writes the Task result, explicit `film_generation_result`,
  locked `generation-result`, system `NOT_ASSESSABLE/hold` QC, locked
  `generation-qc-report`, Attempt terminal state, and Event. A missing media
  URL cannot become success.
- Human `PASS/accept`, `UNCERTAIN/hold`, and `FAIL/retry` QC with full-request
  idempotency comparison. Film Tasks cannot use generic Task retry or automatic
  provider-route fallback, so neither cost nor the quoted provider can change
  silently.
- Independent image visual-semantic QC quotes and append-only Attempts over an
  image-capable text logical model. The generated Result is always image one;
  later images are references. Output is strict JSON over identity, anatomy,
  contact, acting, props, scene, composition, and visual continuity. Every
  model verdict is persisted as `hold`; even model `PASS` cannot replace a
  human `PASS/accept` fact.
- A backend-owned image-to-video path over human-accepted Film images. It pins
  the current locked `ai-video-prompts`/`prompt-manifest`, persists
  Sequence/Slot/Quote/Attempt/Result/QC and billing facts, and applies the same
  no-silent-retry and no-route-fallback rules as image production.
- Sequence creation atomically compiles a `continuity-ledger` Artifact plus
  per-Shot read-in/write-out, nine continuity dimensions, immutable source image
  Reference Locks, and unresolved semantic issues. Its `structured_only` media
  state explicitly prevents text/Prompt checks from being reported as visual
  continuity evidence.
- Video completion runs evidence-bounded technical media QC over managed media,
  MIME, dimensions/aspect ratio, and duration when available. A proven hard
  mismatch creates one append-only, slot-scoped `rework-event` and blocks human
  acceptance; unassessed identity, acting, physics, lip-sync, sound, and visual
  continuity remain `NOT_ASSESSABLE` until a real media review.
- Independent paid video visual-semantic QC freezes the current video Result
  revision, Prompt/source-image revisions, and 3-8 ordered managed frame
  resources captured by the browser. Strict JSON covers identity, anatomy,
  contact, acting, props, scene, composition, motion, temporal continuity, and
  lip-sync/audio. The backend records that it did not natively decode the video;
  model verdicts remain advisory `hold` evidence, malformed paid output becomes
  `uncertain`, and a newer video version invalidates the old report.
- Independent paid whole-Sequence visual continuity QC freezes the Sequence,
  Ledger, Prompt, every current video Result and source-image revision, and an
  ordered browser-captured start/end frame pair for every Slot. Strict JSON
  covers identity, costume, props, scene, screen direction/axis, action,
  lighting/color, motion transition, and lip-sync/audio continuity, plus one
  verdict for every adjacent Shot transition. No audio is attached, so the
  lip-sync/audio dimension must remain `UNCERTAIN`. The model report is an
  append-only `source=model/action=hold` fact; only a current human whole-
  Sequence `PASS/accept` can make the Ledger ready and Sequence completed.
- The Film canvas restores continuity/QC/rework state and can import only a
  fully human-accepted Sequence with a current PASS cross-shot review into the
  existing project `TimelineProject` as direct managed media. Re-import replaces
  the same Sequence in place; export continues through the existing FFmpeg
  runtime rather than a parallel Film editor.

## HTTP API

All endpoints require a valid session and a Film (`short-drama`) project:

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/projects/:id/film/agent-runtime/catalog` | Safe AgentTeam and route catalog |
| `GET` | `/api/projects/:id/film/agent-runs?limit=50` | Film-only Run list |
| `POST` | `/api/projects/:id/film/agent-runs` | Create or idempotently restore a Run |
| `GET` | `/api/projects/:id/film/agent-runs/:runId` | Full durable Run evidence |
| `POST` | `/api/projects/:id/film/agent-runs/:runId/decisions/:decisionId/resolve` | Approve or cancel a human gate |
| `POST` | `/api/projects/:id/film/agent-runs/:runId/steps/:stepId/retry` | Append a retry Attempt for a failed Step |
| `POST` | `/api/projects/:id/film/agent-runs/:runId/artifacts/:artifactId/lock` | Append a user-approved locked revision and Handoff Trigger |
| `GET` | `/api/projects/:id/film/agent-runs/:runId/closeout` | Preview final deliverables, QC evidence, blockers, revisions, and evidence fingerprint |
| `POST` | `/api/projects/:id/film/agent-runs/:runId/closeout` | Explicitly confirm evidence-fenced HR-09/HR-11 closeout and archive |
| `POST` | `/api/projects/:id/film/production/image-quotes` | Freeze one Shot image request, route, capability version, and cost |
| `POST` | `/api/projects/:id/film/production/image-quotes/:quoteId/submit` | Confirm a quote and atomically create the paid Task/Attempt |
| `GET` | `/api/projects/:id/film/production/attempts?rootRunId=&shotId=&limit=50` | Restore Film Attempts, Tasks, Results, QC history, and cost |
| `POST` | `/api/projects/:id/film/production/attempts/:attemptId/qc` | Append an idempotent human QC decision |
| `POST` | `/api/projects/:id/film/production/visual-qc-quotes` | Freeze one image visual-semantic QC request, multimodal route, evidence order, and cost |
| `POST` | `/api/projects/:id/film/production/visual-qc-quotes/:quoteId/submit` | Confirm cost and create a separate visual QC Task/Attempt |
| `POST` | `/api/projects/:id/film/production/video-sequences` | Create a Sequence and its structured Continuity Ledger from accepted images |
| `GET` | `/api/projects/:id/film/production/video-sequences?rootRunId=&limit=20` | Restore Sequence, Slots, Attempts, Results, QC, Continuity, Rework, and costs |
| `POST` | `/api/projects/:id/film/production/video-sequences/:sequenceId/review` | Append a whole-sequence continuity and final acceptance review |
| `POST` | `/api/projects/:id/film/production/video-quotes` | Freeze one Slot image-to-video request and cost |
| `POST` | `/api/projects/:id/film/production/video-quotes/:quoteId/submit` | Confirm cost and create the paid video Task/Attempt |
| `POST` | `/api/projects/:id/film/production/video-attempts/:attemptId/qc` | Append human video QC; technical blockers cannot be accepted |
| `POST` | `/api/projects/:id/film/production/video-visual-qc-quotes` | Freeze ordered sampled frames, current video evidence, multimodal route, and cost |
| `POST` | `/api/projects/:id/film/production/video-visual-qc-quotes/:quoteId/submit` | Confirm cost and create a separate paid video visual QC Task/Attempt |
| `POST` | `/api/projects/:id/film/production/video-sequence-visual-qc-quotes` | Freeze ordered cross-shot frame evidence, current Sequence/Ledger revisions, multimodal route, and cost |
| `POST` | `/api/projects/:id/film/production/video-sequence-visual-qc-quotes/:quoteId/submit` | Confirm cost and create a separate paid whole-Sequence visual QC Task/Attempt |

Run creation requires `X-Idempotency-Key` with 8-128 safe characters. The same
user/key/request returns the original Run; reusing the key for changed input is
a conflict. Run input is capped at 256 KiB and rejects inline media and common
credential fields.

Film quote submission and human QC also require `X-Idempotency-Key`. A quote
expires after ten minutes. Submission rechecks the current project, root Run,
locked Artifact revisions and digests, Registry identity, logical model,
physical route, capability version, price version, and billing snapshot under
transaction locks before reserving any credit.

## Production Persistence Contract

The canvas stores references only. The backend is authoritative for:

- `film_production_quotes`: immutable request, route, capability, price, and
  billing snapshots plus pending/consumed/expired state.
- `film_production_attempts`: one append-only paid execution identity per Shot
  attempt, including source Artifact revisions and Result links.
- `film_production_qc_reports`: append-only system and human QC evidence.
- `film_visual_qc_quotes` and `film_visual_qc_attempts`: frozen multimodal
  model-review evidence, independent cost, and append-only execution history.
- `film_video_sequences`, `film_video_slots`, `film_video_quotes`, and
  `film_video_attempts`: ordered video plans and append-only paid executions.
- `film_video_qc_reports`: technical system media QC and human media review as
  separate evidence facts.
- `film_video_visual_qc_quotes` and `film_video_visual_qc_attempts`: frozen
  browser-sampled frame evidence, independent cost, append-only model review,
  and explicit provenance limitation.
- `film_video_sequence_visual_qc_quotes` and
  `film_video_sequence_visual_qc_attempts`: frozen ordered cross-shot evidence,
  source revisions, route/cost snapshot, and append-only paid model review.
- `film_video_sequence_reviews`: append-only whole-sequence continuity verdicts
  with scope fingerprints and explicit human/model source separation; an old
  report becomes invalid after any slot revision.
- `film_continuity_ledgers`, `film_continuity_shot_states`, and
  `film_continuity_issues`: immutable structured continuity preflight and
  Reference Lock facts for one Sequence.
- `film_rework_events`: the smallest evidence-backed retry/revision scope and
  its recheck gate.
- `results`: the explicit available media fact linked to the Attempt and
  `generation-result` Artifact revision.
- `tasks`, billing orders, Artifact revisions, and ordered Run Events, written
  in the same domain transactions where consistency requires it.

## Evidence Commands

From `backend/` with Go 1.25, the convergence suites pass with:

```bash
go vet ./...
go test ./internal/agentruntime ./internal/database ./internal/repository ./internal/handler ./cmd/server -count=1
go test ./internal/service -skip 'Test(ChannelFromRequestStoresAndClearsHeaders|RuntimeConcurrencyUsesEnvironmentFallback|QiniuKodoSettingAllowsMissingCDNBaseURL|OnlyResumableNewAPIChannel2VideoDeadlinesStayRunning|ResumableVideoDeadlineUsesResolvedSystemChannelProtocol)$' -count=1
go test -race ./internal/repository ./internal/service -run 'Test(FilmAgentCloseout|AgentRuntimeCloseout|CreateAgentRuntimeBundleRejectsArchived)' -count=1
```

Focused runtime suites:

```bash
go test ./internal/agentruntime ./internal/database ./internal/repository ./internal/handler -count=1
go test ./internal/service -run 'TestFilmAgent|TestCreateFilmAgent|TestArchivedFilm|TestRetryFilmAgent|TestProcessNextFilmAgent|TestLockedFilmArtifacts|TestFilmProduction|TestFilmVisualQC|TestFilmVideoVisualQC|TestParseFilmVideoVisualQC|TestFilmVideoSequenceVisualQC|TestParseFilmVideoSequenceVisualQC|TestFilmMultiShotGoldenPath' -count=1
```

The read-only AgentTeam authority package is validated separately from its own
directory:

```bash
python3 scripts/validate_goal_acceptance.py
```

An unfiltered `go test ./... -count=1` currently reproduces five upstream
environment-sensitive failures listed in the convergence baseline. They are
outside the Film runtime and remain classified separately; no production
security behavior was weakened to make live-DNS fixtures pass.

## Not Yet Execution Evidence

The following are intentionally not claimed as complete:

- Only the IR-01 provider path has an HTTP-backed provider integration test;
  every Intent and automatic Handoff route has not yet completed against a real
  configured provider.
- HR-09, HR-10, and HR-11 now have durable orchestration evidence, but the final
  closeout fixture does not claim that the missing real media stages ran.
- Deterministic multimodal-provider fixtures now evaluate eight image dimensions,
  ten ordered-frame single-video dimensions, and nine whole-Sequence continuity
  dimensions plus every adjacent transition through separate paid visual QC
  paths. No configured commercial vision model has completed the same acceptance
  runs. Browser samples are not server-native decode proof, no audio evidence is
  attached, and real-media human acceptance remains mandatory.
- Sequence import reaches the existing TimelineProject/FFmpeg export runtime
  only after the current whole-sequence PASS review. A deterministic local
  Provider fixture now covers two-Shot retry, video, continuity review, and
  refresh recovery; a real configured multi-Shot Provider fixture with
  audio/subtitles and final-film QC has not yet completed the end-user
  acceptance gate.

The tested Film slice now reaches HR-10 project start, script execution,
immutable human lock, durable HR-03 scheduling, storyboard execution, parallel
HR-04/HR-05 Run creation, one provider-backed image Attempt through human QC and
paid retry, single-image, single-video, and whole-Sequence model visual QC with
human fencing, a deterministic two-Shot image/video Sequence/Slot production,
structured continuity, technical media QC, timeline import, and an
evidence-fenced HR-09/HR-11 closeout fixture. Gate 2 and the complete Film
golden path still require all Agent routes, commercial-model real-media
cross-scene continuity evidence, and real configured multi-Shot export evidence.
