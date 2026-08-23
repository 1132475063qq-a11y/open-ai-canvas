# Ecommerce Provider Evaluation

This layer records a controlled Provider bake-off without coupling the
Ecommerce Agents or Skills to a provider implementation. Credentials remain in
the channel configuration and are never accepted by these APIs.

## Durable Contract

The following records are scoped by authenticated user and Ecommerce project:

- `EcommerceProviderEvaluationPlan`: frozen Skill reference, mode, settings,
  fixture cases, candidates, request fingerprint, and final human decision.
- `EcommerceProviderEvaluationAttempt`: one candidate/case/variant execution,
  fingerprints, provider job reference, result references, latency, usage,
  cost, failure and evidence level.
- `EcommerceProviderEvaluationScore`: immutable eight-dimension score tied to a
  succeeded Attempt.
- `EcommerceProviderEvaluationPreference`: immutable blinded pairwise result
  preference tied to succeeded results from one case.

Every create/record endpoint requires an idempotency key. Plan, Attempt, score
and blind-preference replays return the existing projection; a plan key reused
with a different request fingerprint is rejected. A completed `go`, `modify`
or `stop` decision closes the plan to further evidence writes.

## API

```text
GET  /projects/:id/ecommerce/provider-evaluations
POST /projects/:id/ecommerce/provider-evaluations
GET  /projects/:id/ecommerce/provider-evaluations/:planId
POST /projects/:id/ecommerce/provider-evaluations/:planId/attempts
POST /projects/:id/ecommerce/provider-evaluations/:planId/scores
POST /projects/:id/ecommerce/provider-evaluations/:planId/blind-preferences
POST /projects/:id/ecommerce/provider-evaluations/:planId/decision
```

## Acceptance Boundary

The persistence and lifecycle contract is covered by the focused service test.
It does not claim that a model has passed the bake-off. Five authorized product
fixtures, real Provider outputs, human scoring, and the final decision remain a
commercial acceptance task. A plan with missing Provider evidence must remain
`pending`/unknown and cannot be promoted merely because the API records exist.
