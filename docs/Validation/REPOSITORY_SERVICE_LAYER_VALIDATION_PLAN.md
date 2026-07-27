# Repository Service Layer Validation Plan

## Status

- Phase: 4.0
- Version: `0.1.0`
- Status: Accepted design-spike baseline on 2026-07-27
- Phase 4.0.1 design spike: accepted
- Phase 4.0.2 neutral contract: accepted
- Phase 4.0.3 repository lifecycle: authorized
- Production implementation: not started
- Date: 2026-07-27

## Purpose

Define reproducible evidence required to accept each Repository Service Layer
milestone and eventually freeze the service contract at `1.0.0`.

## Validation principles

- Conformance precedes adapter-specific integration testing.
- Unknown, ambiguous, canceled, and failed states are explicit.
- The same authorized input and profile must produce the same durable result.
- No test may require network access, package download, or repository mutation.
- Exact artifact bytes and SHA-256 digests are verified at every boundary.
- Scope isolation applies to reads, writes, listing, cancellation, and export.
- Windows and Ubuntu evidence are recorded separately.

## Test environments

Every report records:

- operating system and version;
- CPU model and logical core count;
- RAM;
- filesystem and storage type;
- Go version;
- PostgreSQL version and configuration;
- frozen component and migration versions;
- worker limits and relevant service configuration;
- warm/cold cache conditions where meaningful.

Disposable databases and test repositories are used. No production, shared,
or personal credentials are permitted.

## Design-spike gates

### Artifact identity

- identical scan/profile/artifact inputs produce identical IDs;
- reordered candidate discovery produces the same final order and IDs;
- a material change to identity input changes the ID;
- unsupported identity scheme fails closed;
- algorithm and canonical byte format are documented.

### Exact-byte materialization

- serialize each candidate exactly once;
- declared byte count equals staged/exported byte count;
- declared SHA-256 equals independently calculated SHA-256;
- reopening the sealed artifact produces identical bytes;
- mid-write cancellation removes the partial spool;
- mutation between measurement and staging is impossible or detected;
- artifacts beyond 4 GiB fail before staging;
- representative large artifacts do not create an artifact-sized memory copy.

### Source privacy

- absolute Windows and Unix roots do not occur in durable payload views;
- source handles and paths do not occur in results, errors, logs, or metrics;
- spools are created outside the analyzed repository with restrictive access;
- traversal, symlink, and boundary failures remain safely redacted.

### Single-flight and cancellation

- 100 identical concurrent requests execute analysis once;
- every waiter receives an equivalent detached result;
- conflicting request reuse fails with `idempotency_conflict`;
- canceling one waiter does not cancel remaining waiters;
- leader cancellation follows the documented all-interested-callers policy;
- all leases, source handles, readers, and spools close exactly once.

### Publication ambiguity

Inject failure after commit but before the adapter response. The service must
query durable state and return success for a published scan. It must never
finalize that scan as failed.

## Neutral contract and conformance gates

Local Phase 4.0.2 evidence is recorded in
`REPOSITORY_SERVICE_CONTRACT_VALIDATION_REPORT.md`. Both coverage gates pass;
Windows and Ubuntu regression, vet, shuffle, race, benchmark, fuzz, scope, and
dependency evidence pass. Phase 4.0.3 is authorized; Phase 4.0.4 remains gated.

- constructor validation for every ID, enum, profile, timestamp, and limit;
- deep-copy and detached-view validation;
- stable error classification and `errors.Is`/`errors.As` behavior;
- context cancellation and timeout mapping;
- pagination and deterministic ordering;
- all public operations require a valid scope;
- repository A cannot read, list, cancel, export, or mutate repository B;
- no SQL, pgx, filesystem path, engine type, runtime pool, or transport type in
  the public package;
- fuzz invalid requests and configuration without panic;
- race test the conformance harness.

## Repository lifecycle gates

- register is idempotent for the same normalized request;
- conflicting request reuse is rejected;
- source proof, not local path, becomes durable identity;
- get/list return detached safe views;
- archive is idempotent and prevents unsupported new scans;
- cross-scope repository IDs remain indistinguishable from not found;
- partial persistence failure does not create visible incomplete state.

## Scan execution gates

- every scan begins and reaches exactly one documented terminal outcome;
- a successful return always has a visible atomic publication;
- failure before publication exposes no artifact envelopes;
- cancellation uses bounded detached finalization;
- an orphaned durable running scan is reported, not resumed or overwritten;
- no prior mutable engine/artifact-store state is reused;
- publication happens at most once;
- duplicate artifact candidates and missing dependencies fail closed;
- scan result and artifact order are deterministic.

## Intelligence integration gates

For each validation repository record:

- RIE artifact names and versions;
- Go artifacts present or intentionally absent;
- exact artifact sizes and SHA-256 values;
- profile name, version, and digest;
- dependency manifest and stable ordinals;
- engine, materialization, staging, and publication durations;
- warnings, diagnostics, omissions, and terminal state.

Required behavior:

- non-Go repository succeeds with the approved RIE-only artifact set;
- Go repository requires syntax, package identity, and semantic artifacts;
- an upstream stage failure prevents downstream stages and publication;
- repeated runs and one-worker/eight-worker runs are deterministic;
- source files, ASTs, and raw repository content are never persisted unless a
  future artifact contract explicitly authorizes them.

## PostgreSQL and runtime integration gates

- run neutral conformance before PostgreSQL integration tests;
- use migrations only through deployment preflight, never service startup;
- prove exact four-MiB chunk storage through the accepted adapter;
- prove atomic publication and rollback under injected failures;
- verify SHA-256 on stage and export;
- verify runtime admission rejects new work during drain;
- verify graceful shutdown waits for admitted work or cancels at deadline;
- repeat start, scan, publish, export, drain, and stop cycles without leaks;
- validate runtime compatibility and least-privilege capability wiring.

## Security and dependency gates

- secret scan passes;
- listener audit confirms no HTTP/gRPC/WebSocket server;
- network/command-execution audit passes;
- public dependency audit confirms frozen boundary direction;
- no source handle, path, credential, payload, SQL, authenticated URL, or raw
  driver error is emitted;
- temporary artifacts are removed after success, failure, and cancellation.

## Required quality commands

From `backend/`:

```text
go test ./...
go test -shuffle=on ./...
go vet ./...
go test -race ./...
```

Targeted packages additionally run repeat counts, coverage, benchmarks, fuzz
tests, and failure-injection suites. Linux race evidence is collected in Ubuntu;
Windows race evidence uses the installed MSYS2 toolchain when required.

## Candidate performance gates

These are design targets and must be confirmed or revised using spike evidence:

- p95 service orchestration overhead below 25 ms for a 20-artifact scan,
  excluding engine, codec, and database time;
- materializer working memory below 64 MiB above the artifact object and OS
  cache, independent of payload size;
- zero extra complete payload-sized copies;
- 100 identical concurrent requests invoke analysis once;
- artifact export memory remains bounded by streaming buffers;
- 1,000 fake-adapter lifecycle executions have no resource leak;
- PostgreSQL publication remains within the accepted persistence target.

Engine performance is reported separately so service overhead cannot hide an
analysis regression.

## Coverage gates

- candidate neutral service package: at least 85% statement coverage;
- conformance harness: at least 85%;
- engine/materialization adapters: at least 85%;
- persistence/runtime integration adapters: at least 85%;
- all security, lifecycle, cancellation, and publication branches require
  explicit tests even when aggregate coverage passes.

## Real-repository matrix

- documentation/configuration-only repository;
- small Go CLI;
- medium Go service;
- generics-heavy library;
- multi-module/workspace repository;
- unavailable external-dependency fixture;
- malformed source fixture;
- stale source fixture;
- large repository already pinned by the Go semantic validation package.

All public repositories are pinned to immutable revisions. Manual verification
records expected artifact presence and representative facts.

## Final `1.0.0` exit gate

- no crashes, panics, data races, scope escapes, secret/path leaks, or partial
  publication;
- deterministic IDs, manifests, bytes, digests, diagnostics, and results;
- all critical/high defects resolved;
- performance targets passed or deliberately revised with accepted evidence;
- public API, profile, identifier, codec, error, and compatibility policies
  documented and frozen;
- full regression, vet, shuffled, Windows race, and Ubuntu race gates pass;
- validation report, benchmark summary, changelog, release notes, known
  limitations, supported-feature matrix, and architecture overview complete;
- explicit engineering acceptance before commit, push, and release tags.
