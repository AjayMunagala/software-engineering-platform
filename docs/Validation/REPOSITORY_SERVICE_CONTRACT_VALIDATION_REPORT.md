# Repository Service Contract Validation Report

## Status

- Phase: 4.0.2 — Neutral Service Contract and Conformance Harness
- Contract version: `0.1.0`
- Local implementation: complete
- Local validation: pass
- Engineering acceptance: accepted on 2026-07-27
- Phase 4.0.3 Repository Lifecycle: authorized
- Date: 2026-07-27

## Scope

Phase 4.0.2 implements only the transport-, runtime-, engine-, and
storage-neutral Repository Service contract plus reusable validation
infrastructure.

Implemented:

- three narrow public capability interfaces and one composed convenience
  interface;
- constructor-validated immutable requests, queries, results, pages, and
  receipts;
- exact SHA-256 digest values and the frozen
  `repository-service-artifact-id/v1` algorithm;
- a bounded neutral configuration;
- an immutable, detached analysis-profile registry;
- stable redacted error kinds and safe cancellation/timeout unwrapping;
- redacted opaque source handles and request-level formatting protection;
- a reusable adapter-independent conformance suite;
- a thread-safe in-memory fake used only by conformance and benchmarks;
- Windows and checksum-bootstrapped Ubuntu validation.

Not implemented:

- production repository lifecycle or scan orchestration;
- intelligence-engine execution or artifact materialization;
- Persistence Port, PostgreSQL, SQL, pgx, migrations, or pools;
- filesystem/source resolving, command execution, or network access;
- runtime startup wiring, APIs, HTTP/gRPC, authentication, UI, queues, workers,
  AI, or repository mutation.

## Candidate contract

The public package is `backend/service/repository` and remains at `0.1.0`.
It defines:

- `RepositoryLifecycleService`;
- `ScanExecutionService`;
- `ArtifactQueryService`;
- the composed `Service` interface;
- immutable neutral models and constructors;
- the initial `repository-go` version `1` profile;
- the stable service error taxonomy.

The reusable conformance package is
`backend/service/repository/conformance`. Adapters provide a fresh seeded
fixture. The suite owns only observable contract assertions, not adapter
implementation details.

## Defect found during validation

### Request formatting could expose an opaque source handle

The original `SourceHandle.String` and `GoString` methods correctly redacted
the handle when it was formatted directly. Formatting the containing request
with `%+v` or `%#v`, however, traversed its private fields and revealed the
stored handle.

The contract now defines redacted `String` and `GoString` methods on both
source-bearing request types. Regression tests format the handle and the full
request and reject the original secret value. No source handle is exposed in
the safe representation.

### Fake payload lookup omitted repository scope

The fake adapter originally indexed artifact metadata by full scope but
indexed exact payload bytes only by artifact ID. Two scopes using the same
repository, scan, and artifact IDs could therefore collide in validation
infrastructure.

Payloads are now indexed by scope, repository, scan, and artifact ID. The
conformance suite creates the same IDs in two independent scopes with distinct
payloads and proves that each exact export remains isolated. This defect did
not affect a production adapter because Phase 4.0.3 has not started.

## Constructor and immutability validation

Tests cover:

- invalid and over-limit configuration;
- IDs, scopes, profiles, digests, source handles, names, versions, media types,
  enums, cursors, and timestamps;
- coherent requested/running/terminal scan timestamps;
- invalid zero values inside detached pages;
- profile and artifact-profile duplicate rejection;
- exact profile resolution by name, version, and digest;
- response slices detached on input and output;
- nil contract, registry, and error receiver safety;
- source-handle redaction at both direct and containing-request boundaries.

## Conformance validation

The reusable suite validates:

- seeded repository, scan, and artifact reads;
- deterministic pagination and detached pages;
- exact streaming artifact export and receipt integrity;
- scope isolation for get, list, register, archive, execute, cancel, and export;
- independent same-ID registration across scopes without cross-scope mutation;
- idempotent registration and explicit conflicting request reuse;
- archive behavior;
- synchronous fake scan creation and retry disposition;
- running-scan cancellation and published-scan rejection;
- context cancellation mapping;
- idempotent fixture cleanup.

The in-memory service is explicitly a fake adapter. It does not constitute the
Phase 4.0.3 repository lifecycle implementation.

## Validation environments

### Windows

- Microsoft Windows 11 Home Single Language `10.0.26200`;
- 12th Gen Intel Core i5-12450H, 12 logical processors;
- 7.7 GiB visible RAM;
- Go `1.26.2 windows/amd64`;
- MSYS2 UCRT64 GCC race-capable toolchain.

### Ubuntu

- Ubuntu 24.04 under WSL2;
- same host CPU and memory;
- disposable official Go `1.26.2 linux/amd64` downloaded by the project
  bootstrap and verified against the current official SHA-256 metadata;
- CGO-enabled Linux race validation.

Ubuntu's installed system Go remains unchanged. The disposable toolchain is
removed after validation.

## Validation results

| Gate | Result |
|---|---:|
| Target package tests | PASS |
| Target `go vet` | PASS |
| Five shuffled target repetitions | PASS |
| Full backend regression | PASS |
| Full backend `go vet` | PASS |
| Three shuffled full-backend repetitions | PASS |
| Windows targeted race | PASS, zero races |
| Windows full-backend race | PASS, zero races |
| Ubuntu targeted regression/shuffle/vet | PASS |
| Ubuntu targeted race | PASS, zero races |
| Ubuntu full-backend regression/race | PASS, zero races |
| Neutral contract coverage | 95.3% |
| Conformance harness coverage | 85.4% |
| Artifact-identity fuzz campaign | 91,944 executions after 98 cached corpus cases, PASS |
| Source-handle fuzz campaign | 1,085,033 executions after 102 cached corpus cases, PASS |
| Combined final fuzz executions | 1,176,977, no panic |
| Forbidden dependency/source audit | PASS |

## Windows benchmark summary

12th Gen Intel Core i5-12450H, Go 1.26.2, five runs for the core contract and
three runs for the conformance fake:

| Benchmark | Range | Memory |
|---|---:|---:|
| Artifact identity | 446.3–491.9 ns/op | 544 B/op, 6 allocs/op |
| Register request validation | 130.6–141.3 ns/op | 0 B/op, 0 allocs/op |
| Profile resolution | 159.5–173.6 ns/op | 480 B/op, 1 alloc/op |
| Fake repository read | 54.42–60.11 ns/op | 0 B/op, 0 allocs/op |
| Fake 16-byte export | 334.1–654.6 ns/op | 368 B/op, 5 allocs/op |

These measurements cover contract construction and fake-adapter overhead only.
They do not claim production lifecycle, engine, persistence, or database
performance.

## Ubuntu benchmark summary

Same host, disposable official Go 1.26.2, three runs:

| Benchmark | Range | Memory |
|---|---:|---:|
| Artifact identity | 537.8–590.5 ns/op | 544 B/op, 6 allocs/op |
| Register request validation | 141.2–147.0 ns/op | 0 B/op, 0 allocs/op |
| Profile resolution | 201.3–208.4 ns/op | 480 B/op, 1 alloc/op |
| Fake repository read | 57.32–59.93 ns/op | 0 B/op, 0 allocs/op |
| Fake 16-byte export | 330.8–367.5 ns/op | 368 B/op, 5 allocs/op |

## Dependency and security audit

The production-neutral package imports no:

- RIE or LIE package;
- persistence or PostgreSQL adapter package;
- runtime package;
- database driver or SQL package;
- filesystem, command-execution, network, or transport package.

No database URL, credential, listener, repository root, or production secret
was introduced. A deliberately unsafe raw test error proves that safe errors
do not expose their raw cause.

## Reproducibility

From `backend/`:

```text
go test ./service/repository/...
go vet ./service/repository/...
go test -shuffle=on -count=5 ./service/repository/...
go test -race ./service/repository/...
go test -run '^$' -bench . -benchmem -count=5 ./service/repository/...
```

Ubuntu uses:

```text
bash backend/service/repository/tests/bootstrap_linux.sh
```

## Exit-gate assessment

The Phase 4.0.2 exit gate is satisfied and its engineering evidence was
accepted on 2026-07-27. The candidate contract and conformance harness may be
committed and pushed. Phase 4.0.3 Repository Lifecycle is authorized; scan
execution and later milestones remain gated.
