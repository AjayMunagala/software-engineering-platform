# Persistence & Runtime Integration Validation Plan

## Status

- Phase: 4.0.6 design
- Status: Proposed
- Implementation: Unauthorized
- Date: 2026-07-28

## Goal

Prove that Repository Service can use Runtime Infrastructure and Persistence
Port end to end without weakening determinism, scope isolation, exact-byte
integrity, atomic publication, resource ownership, or frozen platform
boundaries.

## Required environments

- Windows with the pinned Go toolchain and MSYS2 race support;
- Ubuntu WSL with the pinned Go toolchain;
- disposable PostgreSQL 18.x initialized only through accepted migrations;
- loopback/local CI runtime profile;
- verify-full TLS integration profile where already supported by runtime tests.

No personal, shared, staging, or production credentials may be used. Test
secrets are generated for disposable instances and are not committed.

## Validation order

1. Repository Service conformance suite.
2. Persistence conformance suite against the PostgreSQL adapter.
3. Integration adapter unit tests with strict fakes.
4. Disposable PostgreSQL integration tests.
5. Runtime-backed lifecycle tests.
6. Full backend regression, vet, race, fuzz, and benchmarks.

Adapter-specific success cannot compensate for a neutral conformance failure.

## Contract-refinement tests

- accept canonical lowercase UUID scope/repository/scan IDs;
- reject uppercase, braced, truncated, malformed, or non-UUID values;
- keep RequestID and PrincipalID behavior unchanged;
- migrate all service conformance fixtures to UUIDs;
- prove fake and production constructors enforce identical ID rules;
- verify publication/finalization request IDs survive translation;
- verify begin carries mutation and source fingerprints independently;
- prove dependency accessors are immutable and ordered.

## Physical artifact identity tests

- golden vectors for `repository-service-storage-artifact-id/v1`;
- deterministic output on Windows and Ubuntu;
- RFC 9562 version and variant bits;
- different public IDs produce different fixture UUIDs;
- list/get/export reconstruct the exact public artifact ID;
- stored UUID mismatch fails with `integrity_failure`;
- injected collision/mismatch fails closed;
- no physical UUID appears in service models or safe errors.

## Manifest tests

- golden vectors for `repository-service-manifest/v1`;
- identical manifests and digests on Windows and Ubuntu;
- every field boundary and raw digest byte is covered;
- artifact ordering is canonical;
- dependency ordinals are preserved;
- missing, duplicate, self, cyclic, reordered, or trailing data is rejected;
- timestamps, paths, source handles, principals, and request IDs do not affect
  the manifest;
- one-bit metadata, payload, profile, revision, or dependency changes alter the
  digest.

## Repository lifecycle tests

- register, idempotent retry, conflict, get, list, archive;
- persistence timestamps returned unchanged and valid;
- path-free source proof persisted exactly;
- source handle never persisted, formatted, logged, or returned;
- scope isolation for every operation;
- persistence error translation contains no raw driver data;
- source resolution closes exactly once on every branch.

## Scan lifecycle tests

- source fingerprint matches registered repository;
- mismatched source fails before `BeginScan`;
- begin returns persistence-authoritative running timestamps;
- fresh Phase 4.0.5 analysis executes once;
- every payload is staged sequentially and exactly once;
- staging receipts match declared digest and size;
- ordered artifact and dependency submissions are complete;
- publication sets the scan current atomically;
- returned scan and artifact timestamps satisfy ordering invariants;
- failed and canceled scans finalize with safe codes;
- orphaned running scans remain explicit;
- staged payloads stay invisible before publication.

## Publication ambiguity and failure injection

Inject failure:

- before runtime admission;
- after admission;
- before/after source resolution;
- before/after begin;
- during each payload stream;
- after an individual stage commit;
- before publish;
- before publish commit;
- after publish commit but before response;
- during post-publication scan read;
- during post-publication artifact pagination;
- during reconciliation;
- during detached finalization and cleanup.

For committed-but-response-lost publication, reconciliation must return success
only for the complete matching durable result. Partial, running, missing, or
mismatched results fail explicitly and are never finalized as failed.

## Runtime integration tests

- one leader acquires one runtime work item;
- 99 joined callers acquire no additional work;
- independent waiter cancellation does not leak work;
- all-waiter cancellation reaches the leader;
- drain cancellation reaches the leader;
- work `Done` executes exactly once;
- runtime rejection maps to a stable service error;
- integration never calls runtime shutdown or closes pools;
- 100 start-ready-scan-drain-stop cycles against disposable PostgreSQL;
- runtime readiness changes do not bypass already-acquired work semantics.

## Exact-byte and query tests

- 7-artifact non-Go and 10-artifact Go profiles;
- exact stage/export round trip for every artifact;
- SHA-256 and byte count match Phase 4.0.5 spools;
- get/list/export return service IDs, never physical UUIDs;
- export cancellation is bounded;
- wrong stored digest or size is `integrity_failure`;
- metadata reads never read payload chunks;
- list pagination is deterministic and bounded.

## Scope isolation matrix

For two scopes using the same visible request values, verify isolation for:

- repository register/get/list/archive;
- scan begin/get/list/fail/cancel;
- payload stage;
- publication and reconciliation;
- artifact get/list/export;
- current-scan updates.

Every cross-scope access must be `not_found` or the approved neutral equivalent,
without confirming that another scope's object exists.

## Determinism

Repeat the same fixture with one and eight analysis workers and across Windows
and Ubuntu. Compare:

- Phase 4.0.5 payload digests and sizes;
- public artifact IDs;
- physical artifact UUIDs;
- dependency order;
- manifest bytes and digest;
- terminal metadata excluding persistence-owned timestamps.

## Race, fuzz, and regression gates

Required:

- full backend tests;
- `go vet ./...`;
- at least five shuffled target runs;
- target and full Windows race tests;
- target and full Ubuntu race tests;
- zero data races;
- fuzz ID translation, manifest construction, record translation, error
  translation, and hostile cursor/record inputs;
- statement coverage at least 85% for the integration package and changed
  coordinator packages;
- dependency, secret, path, listener, network, and command audits.

## Performance and memory

Record separately:

- pure model-translation time;
- manifest construction time;
- payload stage time and throughput;
- atomic publication time;
- metadata get/list time;
- exact export throughput;
- service overhead excluding analysis/serialization/database;
- peak Go heap and process working set;
- database pool statistics before and after;
- WAL and database growth for representative artifacts.

Candidate gates:

- p95 integration overhead below 25 ms for ten artifacts;
- p95 manifest construction below 5 ms for ten artifacts;
- no artifact-sized Go heap copy during stage/export;
- 100 identical callers produce one stage/publication sequence;
- zero connection, goroutine, reader, spool, or work-item leaks across 1,000
  small lifecycles;
- PostgreSQL publication remains below the accepted 500 ms p95 gate excluding
  payload staging.

## Required validation report

The Phase 4.0.6 report must record:

- commit and exact toolchain versions;
- OS, CPU, RAM, storage, PostgreSQL version/configuration, and client settings;
- migration revision and runtime compatibility proof;
- commands and pass/fail results;
- coverage, race, fuzz, benchmark, memory, pool, and database metrics;
- golden vectors;
- every defect found, root cause, fix, and verification;
- known limitations and deferred work;
- confirmation that frozen contracts were not modified;
- explicit recommendation on Phase 4.0.7 authorization.

## Exit gate

Phase 4.0.6 is complete only when all mandatory gates pass and engineering
accepts the report. Until then, Phase 4.0.7 remains unauthorized.

