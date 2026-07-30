# Repository Service Stabilization Validation Plan

## Status

- Phase: 4.0.8 design
- Status: Approved with recommendations on 2026-07-30
- Stabilization execution: authorized
- Version promotion and release tags: unauthorized
- Date: 2026-07-30

## Purpose

Define the evidence required to promote the existing Repository Service
candidate from `0.1.0` to `1.0.0` without introducing new behavior.

## Review package

Engineering reviews these documents together:

- `docs/Architecture/REPOSITORY_SERVICE_STABILIZATION.md`;
- `docs/API/REPOSITORY_SERVICE_V1_RELEASE_CANDIDATE.md`;
- `docs/Decisions/0019-repository-service-stabilization-and-release.md`;
- this validation plan.

The design package is accepted. Stabilization execution is authorized. Version
promotion and release tags remain unauthorized until final evidence review.

## Mandatory versus optional evidence

Every API, golden-vector, functional/integration, quality, determinism,
security/boundary, documentation, and release-package gate below is mandatory.
Critical/high defects must be resolved. A mandatory gate may be qualified only
through an explicit final engineering decision.

Extra fixtures, performance improvements below approved targets, and
larger-host evidence that closes an already recorded environmental
qualification are optional additions. Optional work must not change the public
contract or add product behavior.

## Mandatory validation order

1. Verify a clean working tree and exact reviewed commit.
2. Verify candidate API inventory and golden-vector inventory.
3. Run neutral Repository Service conformance first.
4. Run lifecycle, scan, intelligence/materialization, persistence/runtime, and
   real-repository integration regressions.
5. Run full backend tests, shuffled tests, vet, and coverage.
6. Run targeted and full race validation on Windows and Ubuntu.
7. Run fuzz, deterministic-output, security, dependency, and privacy audits.
8. Run performance and memory comparison against the accepted baseline.
9. Verify documentation, examples, compatibility policy, changelog, release
   notes, known limitations, operator checklist, and qualification register.
10. Commit and push the complete evidence package for engineering review.
11. Only after explicit acceptance, promote versions, rerun versioned gates,
    commit and push, then create annotated tags.

## API freeze gates

- Enumerate every exported identifier in `backend/service/repository` and its
  supported public subpackages.
- Compile representative consumers against each narrow capability interface.
- Confirm no SQL, pgx, runtime-pool, HTTP, filesystem path, engine AST, or
  transport type appears in the neutral API.
- Confirm immutable constructors, defensive copies, zero-value behavior,
  validation limits, pagination, ordering, cancellation, and retry semantics.
- Confirm every error kind, reason-code policy, retry flag, and `errors.Is`
  behavior.
- Record an API snapshot and fail validation on unreviewed drift.

## Golden-vector gates

Validate committed positive and negative vectors for:

- `repository-service-artifact-id/v1`;
- `repository-service-storage-artifact-id/v1`;
- `repository-service-manifest/v1`;
- `repository-service-profile/v1`;
- `repository-go/v1` profile bytes and digest;
- `canonical-json/1.0.0` for every released artifact in the profile.

Run vectors across Windows and Ubuntu. Inputs, canonical preimages, encoded
bytes, hashes, prefixes, UUIDs, ordering, and rejection cases must match
exactly.

## Functional and integration gates

- repository registration, lookup, list, archival, idempotency, and scope
  isolation;
- scan state transitions, keyed single-flight, independent waiter
  cancellation, all-waiter cancellation, and terminal-state behavior;
- deterministic manifest and dependency construction;
- sealed exact-byte materialization and independent staging verification;
- atomic publication, response-lost reconciliation, rollback, restart
  recovery, retention safety, and exact-byte export;
- runtime admission, startup compatibility, readiness behavior, drain, and
  repeated start/stop cycles;
- production composition against disposable migration-created PostgreSQL.

The conformance suite is always the first adapter/service behavioral gate.

## Quality gates

- `go test ./...` passes;
- at least three shuffled full-backend runs pass;
- `go vet ./...` passes;
- targeted and full `go test -race` pass on Windows and Ubuntu;
- zero data races;
- changed production packages retain at least 85% statement coverage;
- deterministic fuzz targets execute a recorded bounded corpus with zero
  panics, leaks, or contract violations;
- benchmarks are repeatable and record CPU, memory, allocations, environment,
  cache state, worker count, and fixture revision.

## Determinism gates

For equivalent canonical source manifests, compare repeated clean processes,
one/eight-worker runs where the host safely permits them, and Windows/Ubuntu:

- exact authoritative artifact bytes and SHA-256 digests;
- artifact order and dependency edges;
- diagnostics, statistics, and omission counts;
- normalized service result and manifest views;
- stable IDs when canonical identity inputs are identical.

Different scan UUIDs and explicitly documented timestamps may be normalized
only according to the accepted Phase 4.0.7 rules.

## Resource gates

Compare service overhead, CPU, wall time, peak heap, process working set,
allocations, PostgreSQL stage/publication/export throughput, and cleanup against
the Phase 4.0.6 and 4.0.7 baselines. Any regression above a documented target
requires investigation and engineering disposition.

Safety memory ceilings remain enabled. A ceiling termination is recorded as a
resource qualification, never converted into a correctness pass.

## Security and boundary audits

Prove that production composition performs no:

- network fetch or repository command execution;
- repository mutation;
- source-handle, absolute-path, credential, SQL, payload, or raw-driver-error
  disclosure;
- cross-scope read, write, list, export, cancellation, or idempotency reuse;
- migration execution at runtime;
- HTTP listener, transport, authentication, UI, or AI behavior.

Run secret, dependency, listener, command-execution, source-path, and generated
artifact audits on the final commit.

## Open Kubernetes qualification

The Phase 4.0.7 result remains open for:

- Kubernetes Windows one-worker on a larger race-capable host;
- Kubernetes Ubuntu matrix on a larger race-capable host.

The final report must show completed evidence, omitted evidence, safety ceiling,
host resources, and exact outcome separately. Larger-host evidence is
append-only. Final engineering acceptance must explicitly choose one of:

1. qualification closed by passing evidence;
2. qualification accepted for `1.0.0` and retained in release notes;
3. release blocked pending further evidence or a compatible defect fix.

## Release package

The review commit must contain:

- stabilization validation report and machine-readable results;
- public API snapshot and compatibility policy;
- changelog and `1.0.0` release notes;
- supported feature matrix;
- benchmark and memory summary;
- known limitations and qualification register;
- architecture and artifact dependency overview;
- operator pre-release checklist;
- planned annotated tag names and tag verification procedure.

## Exit criteria

Phase 4.0.8 passes only when every mandatory gate passes or has an explicit,
reviewed qualification; no critical defect remains; the public API is approved;
documentation matches the code; and engineering explicitly authorizes version
promotion.

The design commit itself must leave `ContractVersion` at `0.1.0`, make no
production changes, and create no release tag.
