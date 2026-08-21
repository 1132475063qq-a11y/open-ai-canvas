# Preserve, Rewrite, and Migrate Matrix

| Capability | Decision | Source of truth | Implementation rule |
|---|---|---|---|
| Canvas shell, viewport, selection, shortcuts | Preserve | upstream v1.1.4 | Extend through registries and hooks only |
| Node and tool registries | Preserve | upstream v1.1.4 | Register domain-owned definitions; no shared switch expansion |
| Local Runtime signing, origin checks, sessions | Preserve | upstream v1.1.4 | Add scoped modules/contracts without weakening security |
| Provider and logical model registry | Preserve | upstream v1.1.4 | Domain code selects capabilities, never embeds provider branches |
| Generation task durability and recovery | Preserve | upstream v1.1.4 | Reuse task IDs and provider evidence in production Attempts |
| Billing, quote, and retry confirmation | Preserve and extend | upstream plus donor contracts | Backend owns all paid facts and idempotency |
| Film project models and artifact revisions | Selectively migrate | donor Film backend | Port models and tests by responsibility, then adapt to upstream contracts |
| Film semantic nodes and Inspector | Selectively migrate | donor `web/src/film/**` | Keep behind Film workspace capability gates |
| Donor shared canvas page and global CSS | Reject | donor | Reimplement only missing extension hooks upstream |
| AgentTeam 9 Agent profiles | Compile | AgentTeam `.codex/agents/*.toml` | Typed registry data plus immutable source/version metadata |
| AgentTeam 17 Skills | Compile | AgentTeam `.agents/skills/**/SKILL.md` | Lazy-loaded runtime instructions with input/output contracts |
| 15 intent routes and 11 handoffs | Compile | AgentTeam routing contracts | Deterministic routing table with validation and audit records |
| Agent execution | Rewrite | new shared runtime | Durable run/step/invocation state machine; no file-mode claims |
| Human decisions | Rewrite | new shared runtime plus AgentTeam contract | Explicit pause/resume gate with versioned user answer |
| Ecommerce backend production models | Defer, then migrate | donor Ecommerce backend | Port only after Film gate; keep domain package isolated |
| Ecommerce workspace UI | Defer, then migrate | donor `web/src/ecommerce/**` | Mount only in Ecommerce workspace |
| Short-drama AgentTeam ZIP in ecommerce | Reject | product boundary | No imports, routes, Skills, or data exposure |

## Target Ownership

### Shared runtime

- Typed registry and startup validation.
- Durable run, step, invocation, transition, decision, and audit contracts.
- Pause, resume, cancel, retry, idempotency, leases, and recovery.
- Provider-neutral invocation and structured-output validation.

### Film domain

- Film Agent/Skill definitions and routing policy.
- Canon, script, character, scene, shot, prompt, continuity, and QC artifacts.
- Film workspace projection and Inspector panels.
- Film-specific acceptance rules and golden fixtures.

### Ecommerce domain

- ProductDNA, model, scene pack, preset, Run/Slot/Attempt, visual QA, and video
  sequence contracts.
- Ecommerce Agent/Skill definitions and preset compiler.
- Ecommerce-only workspace projection and controls.

### Canvas document

The canvas stores references, layout, and user presentation state. Authoritative
runtime status, costs, approvals, artifact revisions, Attempts, Results, and QC
facts remain backend-owned.
