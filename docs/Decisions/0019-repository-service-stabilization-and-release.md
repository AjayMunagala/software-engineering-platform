# ADR 0019 - Repository Service Stabilization and Release

## Status

Proposed

## Date

2026-07-30

## Context

Phases 4.0.0 through 4.0.6 produced a neutral Repository Service candidate and
integrated it with released intelligence, persistence, and runtime contracts.
Phase 4.0.7 validated the production composition on a pinned corpus and was
accepted with one open larger-host Kubernetes qualification.

The service remains version `0.1.0`. Promoting it without a dedicated API,
compatibility, resource, security, and release audit could freeze accidental
behavior. Adding functionality during stabilization would make the evidence
ambiguous.

## Proposed decision

1. Make Phase 4.0.8 stabilization-only; add no product capability.
2. Review and propose the exact existing neutral service surface for `1.0.0`.
3. Freeze the existing versioned artifact, storage-artifact, profile, manifest,
   and codec algorithms through committed golden vectors.
4. Preserve synchronous scans, immutable values, explicit scope, deterministic
   ordering, single-flight, cancellation, idempotency, atomic publication,
   reconciliation, and exact-byte export as conformance requirements.
5. Permit only compatible defect fixes, output-preserving internal
   optimizations, tests, audits, and release documentation before promotion.
6. Forbid adding methods to frozen capability interfaces during `1.x`; add new
   optional behavior through new narrow interfaces and separate versioned
   contracts.
7. Keep service, artifact, codec, stable-ID, database schema, adapter, runtime,
   and future transport versions independent.
8. Carry the incomplete Kubernetes host matrix as an explicit release
   qualification. Allow append-only evidence from a larger host and require a
   final engineering disposition before release.
9. Promote versions and create annotated namespaced tags only after the final
   validation package is committed, pushed, and explicitly accepted.

## Rationale

A stabilization-only milestone preserves traceability between validated
behavior and the first stable contract. Narrow compatibility rules protect
consumers without forcing unrelated subsystem versions to move together.
Explicit qualification is more honest than either claiming uncollected
evidence or redesigning correct software around one limited workstation.

## Alternatives

### Add transport APIs before freezing the service

Rejected. A transport would couple wire design to an unfrozen application
contract and make later corrections more expensive.

### Treat the candidate as 1.0 immediately

Rejected. Candidate coverage is strong, but API, compatibility, resource,
security, and release-package audits are distinct release gates.

### Block all stabilization until the full Kubernetes matrix completes

Rejected as the default policy. Engineering accepted Phase 4.0.7 with an
environmental qualification and no observed correctness defect. The final
release review still decides whether that qualification is acceptable.

### Change behavior to fit the current workstation

Rejected. Host protection limits must not silently become product semantics.
Compatible, output-preserving optimizations remain allowed when evidence
supports them.

## Consequences

- Phase 4.0.8 cannot become a feature milestone.
- API mistakes discovered during design review are inexpensive to correct
  because `1.0.0` has not been promoted.
- Later `1.x` work must preserve the accepted public behavior.
- Transport, UI, authentication, and AI remain independently gated.
- The release package visibly carries any accepted qualification until
  append-only evidence closes it.

## Acceptance gate

This ADR may become `Accepted` only after engineering reviews this design
package. That acceptance authorizes stabilization implementation, not version
promotion. Promotion to `1.0.0` and release tags require a second explicit
engineering acceptance of the completed Phase 4.0.8 evidence.
