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

Run creation requires `X-Idempotency-Key` with 8-128 safe characters. The same
user/key/request returns the original Run; reusing the key for changed input is
a conflict. Run input is capped at 256 KiB and rejects inline media and common
credential fields.

## Evidence Commands

From `backend/` with Go 1.25:

```bash
go test ./... -count=1
go vet ./...
```

Focused control-plane suites:

```bash
go test ./internal/agentruntime ./internal/database ./internal/repository ./internal/handler -count=1
go test ./internal/service -run 'TestFilmAgent|TestCreateFilmAgent|TestArchivedFilm|TestRetryFilmAgent|TestService' -count=1
```

## Not Yet Execution Evidence

The following are intentionally not claimed as complete:

- No Agent/Skill Step currently invokes a text model through a provider-neutral
  Executor.
- No Skill output is yet validated against an Artifact schema and committed by
  the Executor.
- Handoff routes are registered but not automatically scheduled from completed
  Artifact facts.
- No image/video Task, provider Result, automatic visual QC, continuity pass,
  cost confirmation, or final export is produced by this runtime yet.
- The Film workspace does not yet project Run attention items and key Artifacts
  into the upstream canvas UI.

File presence and Step compilation are control-plane evidence only. Gate 2 and
the Film golden path require real execution and persisted output evidence.
