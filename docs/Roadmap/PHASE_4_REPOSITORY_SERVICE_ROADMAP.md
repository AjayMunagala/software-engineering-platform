# Phase 4 — Repository Service Layer Roadmap

## Status

- Phase 4.0 design: accepted on 2026-07-27
- Phase 4.0.1 design spike: accepted on 2026-07-27
- Phase 4.0.2 neutral service contract and conformance harness: accepted on 2026-07-27
- Phase 4.0.3 repository lifecycle: accepted on 2026-07-27
- Phase 4.0.4 scan execution core: accepted on 2026-07-28
- Phase 4.0.5 intelligence and materialization adapters: accepted on 2026-07-28
- Phase 4.0.6 persistence and runtime integration design: accepted with recommendations on 2026-07-28
- Phase 4.0.6 persistence and runtime integration: accepted and frozen on 2026-07-29; implementation commit `abdb395` pushed
- Phase 4.0.7 real-repository validation: accepted on 2026-07-30 with one open larger-host Kubernetes qualification
- Phase 4.0.8 stabilization and Repository Service 1.0.0: accepted on 2026-07-30
- Release tag: `repository-service/v1.0.0`
- Phase 4.1 transport design: eligible for separate review; not started
- Date: 2026-07-30

## Goal

Provide one transport-neutral application service that coordinates authorized
local repository analysis and exact, atomic artifact publication using the
released platform foundation.

## Frozen prerequisites

- Repository Intelligence Engine `1.0.0`
- Go Language Intelligence contracts `1.0.0`
- Persistence Port `1.0.0`
- PostgreSQL Adapter `1.0.0`
- Runtime Infrastructure `1.0.0`
- accepted migrations and PostgreSQL physical persistence contract

No Phase 4 milestone may change these contracts without a separate compatible
release and explicit governance decision.

## Phase 4.0.0 — Architecture and candidate contract ✅

Deliverables:

- Repository Service Layer architecture;
- candidate Go service API;
- ADR 0016;
- this staged roadmap;
- validation plan.

Exit gate:

- all five documents reviewed together;
- responsibilities and exclusions approved;
- artifact profile and dependency order approved;
- source, idempotency, cancellation, transaction, and failure policies approved;
- authorization granted only for Phase 4.0.1.

## Phase 4.0.1 — Design spike ✅

Purpose: validate the risky integration assumptions without creating the
production service implementation.

Required evidence:

- execute the released RIE and Go LIE sequence through a spike-only adapter;
- prove deterministic `repository-service-artifact-id/v1` IDs;
- serialize representative small and large immutable artifacts once;
- verify digest and exact bytes through a fake storage-neutral sink;
- prove path redaction in durable artifact views and error output;
- exercise 100 identical concurrent requests through keyed single-flight;
- model cancellation and bounded spool cleanup;
- model committed-but-response-lost publication reconciliation;
- measure service overhead and peak materializer memory;
- document every assumption changed by evidence.

Exit gate:

- spike report accepted;
- ADR 0016 promoted from Proposed to Accepted;
- candidate API updated where evidence requires it;
- explicit authorization for Phase 4.0.2.

Spike code is experimental and must not become a production dependency by
accident.

Local evidence is recorded in
`docs/Validation/REPOSITORY_SERVICE_DESIGN_SPIKE_REPORT.md` and was accepted on
2026-07-27.

## Phase 4.0.2 — Neutral service contract and conformance harness ✅

Implement only:

- immutable service requests and results;
- capability interfaces;
- stable service errors;
- configuration and profile registry;
- fake adapters;
- adapter-independent conformance harness;
- defensive-copy, validation, fuzz, benchmark, and race tests.

Do not implement released engines, PostgreSQL, pgx, SQL, HTTP, or runtime pool
wiring in this package.

Exit gate: public candidate behavior passes conformance and is accepted before
repository lifecycle implementation begins.

Local evidence is recorded in
`docs/Validation/REPOSITORY_SERVICE_CONTRACT_VALIDATION_REPORT.md`. The evidence
was accepted on 2026-07-27 and authorized Phase 4.0.3 only.

## Phase 4.0.3 — Repository lifecycle ✅

Implement:

- register, get, list, and archive coordination;
- opaque source proof handling;
- idempotent mutation behavior;
- repository scope isolation;
- persistence model translation behind the internal service store.

Exit gate: neutral lifecycle conformance, scope isolation, idempotency,
regression, Windows and Ubuntu race, coverage, benchmark, source privacy, and
dependency evidence accepted. PostgreSQL-specific implementation remains
deferred to Phase 4.0.6 by the accepted Phase 4.0.3 authorization.

Local evidence is recorded in
`docs/Validation/REPOSITORY_LIFECYCLE_VALIDATION_REPORT.md`. Engineering
accepted the evidence on 2026-07-27 and authorized Phase 4.0.4 only.

## Phase 4.0.4 — Scan execution core ✅

Implement:

- synchronous scan state machine;
- admission lease ownership;
- keyed in-process single-flight;
- cancellation and cleanup coordination;
- terminal failure finalization;
- publication ambiguity reconciliation;
- artifact metadata/list/export orchestration using fake analysis inputs.

Exit gate: concurrency, cancellation, orphan, exact-once publication, and
failure-injection suites accepted.

Evidence is recorded in
`docs/Validation/SCAN_EXECUTION_CORE_VALIDATION_REPORT.md`. Engineering
accepted the evidence on 2026-07-28 and authorized Phase 4.0.5 only.

## Phase 4.0.5 — Intelligence and materialization adapters ✅

Implement:

- the frozen `repository-go/v1` profile;
- fresh RIE and Go LIE orchestration;
- versioned durable artifact codecs;
- deterministic manifest/dependency construction;
- sealed streaming materialization;
- source/path redaction proof.

Exit gate: identical input produces identical artifact bytes, IDs, dependency
order, and terminal result on Windows and Ubuntu.

Evidence is recorded in
`docs/Validation/INTELLIGENCE_MATERIALIZATION_ADAPTERS_VALIDATION_REPORT.md`.
Engineering accepted the implementation and validation evidence on 2026-07-28
and authorized Phase 4.0.6 design only.

## Phase 4.0.6 — Persistence and runtime integration ✅

Design package:

- `docs/Architecture/REPOSITORY_SERVICE_PERSISTENCE_RUNTIME_INTEGRATION.md`;
- `docs/API/REPOSITORY_SERVICE_INTEGRATION_CANDIDATE_API.md`;
- `docs/API/REPOSITORY_SERVICE_INTEGRATION_GOLDEN_VECTORS.md`;
- `docs/Decisions/0017-repository-service-persistence-runtime-integration.md`;
- `docs/Validation/PERSISTENCE_RUNTIME_INTEGRATION_VALIDATION_PLAN.md`.

Engineering accepted the design with recommendations on 2026-07-28. UUID
validation, physical artifact mapping, and manifest golden vectors are frozen
as mandatory pre-implementation contracts. Production implementation is
authorized only within this accepted scope.

After design acceptance, implement only:

- adapters to Persistence Port `1.0.0`;
- Runtime Infrastructure admission integration;
- end-to-end stage and atomic publication;
- exact artifact export and integrity verification;
- disposable PostgreSQL lifecycle validation.

No migrations run at service startup. Pool construction remains runtime-owned.

Exit gate: conformance runs before adapter-specific tests; all integration,
race, rollback, scope-isolation, and leak tests pass.

Local evidence is recorded in
`docs/Validation/PERSISTENCE_RUNTIME_INTEGRATION_VALIDATION_REPORT.md`. The
implementation reached its exit gate in review candidate `abdb395`, which is
pushed to `main`. Engineering accepted and froze Phase 4.0.6 on 2026-07-29.
Phase 4.0.7 design was accepted and its validation execution is complete.

## Phase 4.0.7 — Real repository validation ✅

Design package:

- `docs/Architecture/REPOSITORY_SERVICE_REAL_REPOSITORY_VALIDATION.md`;
- `docs/Validation/REPOSITORY_SERVICE_REAL_REPOSITORY_FIXTURE_MANIFEST.md`;
- `docs/Validation/REPOSITORY_SERVICE_REAL_REPOSITORY_VALIDATION_PLAN.md`;
- `docs/Decisions/0018-pinned-real-repository-service-validation.md`.

Validate representative inputs:

- small non-Go repository;
- small Go CLI;
- medium Go service;
- generics-heavy Go library;
- multi-module Go repository;
- malformed and stale source fixtures;
- large released validation repository.

Record files, engine timings, artifact names/versions/sizes/digests, stage and
publication timing, service overhead, peak memory, diagnostics, failures,
cancellation, and deterministic hashes.

Exit gate: no crashes, scope escapes, path leaks, partial publication, digest
mismatch, or unexplained nondeterminism; critical defects resolved.

Governance gate: engineering accepted the design with recommendations on
2026-07-29. The harness and available-host evidence are complete. Interrupted
PostgreSQL recovery, exact Git-version evidence, and explicit Kubernetes
classification are recorded. All ordinary matrices pass, while Kubernetes
one-worker Windows and Ubuntu runs exceed safe memory ceilings. Engineering
accepted that resource finding as an environmental release qualification on
2026-07-30. Phase 4.0.8 and the larger-host qualification are accepted.
Repository Service is frozen at `1.0.0`.

## Phase 4.0.8 — Stabilization and Service Contract 1.0.0 ✅

The design package was approved with recommendations on 2026-07-30:

- `docs/Architecture/REPOSITORY_SERVICE_STABILIZATION.md`;
- `docs/API/REPOSITORY_SERVICE_V1_RELEASE_CANDIDATE.md`;
- `docs/Decisions/0019-repository-service-stabilization-and-release.md`;
- `docs/Validation/REPOSITORY_SERVICE_STABILIZATION_VALIDATION_PLAN.md`.

ADR 0019, stabilization evidence, qualification disposition, final versioned
validation, and release are accepted. The implementation is frozen at `1.0.0`
and the annotated `repository-service/v1.0.0` tag identifies the release.

Perform:

- API and compatibility review;
- identifier and codec freeze;
- memory/CPU profiling;
- dependency, security, and documentation audits;
- Windows and Ubuntu race validation;
- full backend regression;
- release notes, changelog, known limitations, and operator guidance;
- annotated namespaced release tags after acceptance.

Only compatible defect fixes are allowed on the future `1.0.x` line.

## Phase 4.1 — Transport APIs (design eligible; not started)

REST/gRPC design may now be proposed through a separate review. Transport work
must not change frozen service semantics. No implementation is authorized yet.

## Explicitly deferred

- authentication and authorization policy;
- HTTP health endpoints;
- UI and IDE integrations;
- queues, distributed workers, leases, retries, and schedulers;
- Git clone or remote repository acquisition;
- dependency and architecture intelligence;
- AI reasoning, patch generation, and validation execution.
