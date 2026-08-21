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
- Atomic Run creation with a locked `project-requirements` Artifact revision.
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
- Browser-safe catalog output that omits executable Agent developer
  instructions and Skill instruction bodies.

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

Run creation requires `X-Idempotency-Key` with 8-128 safe characters. The same
user/key/request returns the original Run; reusing the key for changed input is
a conflict. Run input is capped at 256 KiB and rejects inline media and common
credential fields.

## Evidence Commands

From `backend/` with Go 1.25, the convergence suites pass with:

```bash
go vet ./...
go test ./internal/agentruntime ./internal/database ./internal/repository ./internal/handler ./cmd/server -count=1
go test ./internal/service -skip 'TestChannelFromRequestStoresAndClearsHeaders|TestRuntimeConcurrencyUsesEnvironmentFallback|TestQiniuKodoSettingAllowsMissingCDNBaseURL|TestOnlyResumableNewAPIChannel2VideoDeadlinesStayRunning|TestResumableVideoDeadlineUsesResolvedSystemChannelProtocol' -count=1
```

Focused runtime suites:

```bash
go test ./internal/agentruntime ./internal/database ./internal/repository ./internal/handler -count=1
go test ./internal/service -run 'TestFilmAgent|TestCreateFilmAgent|TestArchivedFilm|TestRetryFilmAgent|TestProcessNextFilmAgent|TestLockedFilmArtifacts' -count=1
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
- HR-09 project closeout, HR-10 project-start dispatch, and HR-11 project
  completion remain orchestration work rather than automatic Agent Runs.
- No image/video Task, provider Result, automatic visual QC, continuity pass,
  cost confirmation, or final export is produced by this runtime yet.
- The Film workspace does not yet project Run attention items and key Artifacts
  into the upstream canvas UI.

The tested Film slice now reaches script execution, immutable human lock,
durable HR-03 scheduling, storyboard execution, and parallel HR-04/HR-05 Run
creation. Gate 2 and the complete Film golden path still require all route and
media stages to produce persisted runtime evidence.
