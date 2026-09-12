# ADR 0023 — Opt-in Dependency Artifact Publication

## Status

**Design Approved**, 2026-09-12, at reviewed design commit `1b9f4d6`.
Phase 5.0.5 implementation is authorized, starting with independent vectors.
Production encoding waits for review of the vector commit. PostgreSQL/runtime
integration testing remains unauthorized. Full ADR acceptance still requires
implementation evidence and engineering acceptance. Phase 5.0.4
and ADR 0022 remain accepted at canonical governance commit `fca668d`.
Candidate version remains 0.1.0. DIE-HARDEN-001/002 remain open.

## Context

Accepted DIE produces immutable inventories, with optional neutral graph analysis.
The platform already has stable persistence and runtime capabilities. Integration
must not move those responsibilities into DIE or alter released contracts.

Actual Repository Service 1.0.0 construction fixes its profile registry and engine
adapter. Although lower-level profile/store constructors exist, arbitrary profiles
cannot be passed through its current validated execution request constructor.
The released materializer also does not expose its typed engine store publicly.
An apparently simple profile addition would therefore change frozen behavior.

Accepted Go dependency source references are names/versions, not cryptographic
links to stored source payloads. Persistence edges require a same-publication DAG.
Joining an unrelated stored scan by names would invent provenance.

## Proposed decision

1. Integrate through a separate opt-in artifact publication capability consuming
   a caller-supplied accepted immutable inventory. No engine execution in Publish.
2. Add a distinct candidate canonical codec, seal exact bytes, stage through the
   frozen port, atomically publish one envelope, and verify export/recovery.
3. Borrow runtime admission and ingest/read capabilities; keep TLS, pools,
   migrations, configuration loading and shutdown in their released owners.
4. Use dedicated integration repository records and never set MakeCurrent, so
   unknown integration profiles do not enter frozen Repository Service histories.
5. Preserve logical upstream references unchanged, with no invented digest or
   cross-scan persistence edge. Upstream closure persistence is explicitly deferred.
6. Defer optional Repository Service profile/capability extension and typed graph
   reload. Neither is silently treated as supported by this milestone.
7. Freeze independent codec/profile/UUID/manifest/request vectors after separate
   implementation authorization and before their production encoder logic.
8. Preserve uncertainty, immutable facts, existing ID/cursor vectors, and all
   existing open hardening limitations. No release or downstream authorization.

## Alternatives

Changing the released default profile or forging its private requests is rejected.
Adding new factory methods/profile support could be a future additive service
release, but it needs separately approved compatibility and historical-read rules;
it is not assumed to be a bug fix in 1.0.x. A breaking extension needs a major
version. Reaching into private materializer state or rerunning analysis and claiming
identical provenance is rejected. Persistence in DIE itself violates ownership.

Persisting an entire upstream closure would improve provenance navigation, but
requires reviewed source codecs/verified bindings and same-scan dependency rules.
Persisting only the inventory is narrower and honest about current evidence.
This trade-off and the dedicated repository routing are mandatory review topics,
not previously accepted architecture facts.

## Consequences and limitations

The proposed path validates real durability without changing existing engines or
services. It does not automatically enrich `repository-go/v1` scans, persist source
artifact FKs, or enable graph queries after restart from serialized private values.
Consumers get exact inventory bytes and metadata, not a proof of source freshness.
Host orchestration must supply typed inputs and route repository records explicitly.
Spooling bounds encoded-byte retention, not all graph heap or cancellation latency.

## Gates

Review [architecture](../Architecture/DEPENDENCY_PLATFORM_INTEGRATION.md),
[codec](../Architecture/DEPENDENCY_PLATFORM_CODEC.md),
[candidate API](../API/DEPENDENCY_PLATFORM_INTEGRATION_CANDIDATE_API.md), and
[validation plan](../Validation/DEPENDENCY_PLATFORM_INTEGRATION_VALIDATION_PLAN.md)
together. Record design approval separately. Wait for explicit implementation
authorization, freeze vectors first, implement, validate, and submit committed
evidence. Promote this ADR to Accepted only on engineering acceptance of that
implementation. Phase 5.0.6 and 5.0.7 remain unauthorized.
