# Repository Lifecycle Validation Report

## Status

- Phase: 4.0.3 — Repository Lifecycle
- Contract version: `0.1.0`
- Local implementation: complete
- Local validation: pass
- Engineering acceptance: accepted on 2026-07-27
- Phase 4.0.4 Scan Execution Core: authorized
- Date: 2026-07-27

## Scope

Phase 4.0.3 implements production repository lifecycle coordination behind the
accepted neutral Repository Service contract.

Implemented:

- repository registration, lookup, deterministic listing, and archival;
- a narrow atomic `Store` abstraction for scope, idempotency, mutation, and
  pagination semantics;
- opaque source-handle resolution into immutable, durable, path-free proof;
- deterministic registration and archival mutation fingerprints;
- bounded source-resolution cleanup before durable registration;
- stable neutral error translation without raw dependency leakage;
- additive lifecycle-only conformance support;
- Windows and checksum-bootstrapped Ubuntu validation.

Not implemented:

- scan execution, engine orchestration, or artifact materialization;
- PostgreSQL, SQL, pgx, Persistence Port integration, or runtime wiring;
- filesystem traversal, command execution, network access, or remote fetch;
- REST/gRPC, HTTP, authentication, authorization, UI, queues, workers, or AI.

PostgreSQL-specific integration remains deferred because the accepted Phase
4.0.3 authorization requires a repository persistence abstraction and
explicitly excludes a PostgreSQL implementation. Persistence and runtime
integration remains Phase 4.0.6 work.

## Architecture and behavior

The production package is `backend/service/repository/lifecycle`. It depends
only on the neutral `backend/service/repository` contract plus three narrow
capabilities:

- `Store`, which owns atomic mutation, idempotency, repository scope, and
  pagination persistence semantics;
- `SourceProofResolver`, which consumes an opaque process-local source handle
  and returns path-free identity evidence;
- `Clock`, which makes timestamps deterministic under test.

The lifecycle service never calls `SourceHandle.Reveal`, never persists the
handle, and never places it in a store command. A registration becomes durable
only after the resolver has returned a valid proof and its bounded cleanup has
completed successfully.

The store receives immutable `RegisterCommand` and `ArchiveCommand` values.
Their SHA-256 mutation fingerprints use versioned domains so implementations
can distinguish an identical retry from conflicting request reuse atomically.

## Conformance validation

The reusable conformance harness now exposes an additive lifecycle-only
factory and suite. It validates:

- seeded repository get/list behavior;
- deterministic pages and detached result slices;
- idempotent registration and explicit conflicting request reuse;
- idempotent archival;
- same repository IDs in independent scopes;
- cross-scope get, list, and archive isolation;
- no cross-scope mutation;
- cancellation behavior;
- idempotent fixture cleanup.

The complete Phase 4.0.2 fake remains available for full neutral-contract
tests. The new production lifecycle service passes the lifecycle-only suite
through an independent atomic memory store used only for tests.

## Targeted lifecycle validation

Tests additionally cover:

- path-free proof materialization and source cleanup;
- source handles absent from durable models and fingerprints;
- invalid dependencies, configuration, clocks, proofs, and store results;
- redaction of unknown dependency errors;
- deterministic fingerprints and revision-sensitive registration identity;
- 100 concurrent identical registration calls;
- duplicate repository conflicts;
- pagination across independently registered repositories;
- immutable command and list accessors;
- more than one million malformed source-proof fuzz inputs without panic.

## Validation environments

### Windows

- Microsoft Windows 11 `10.0.26200`;
- 12th Gen Intel Core i5-12450H, 12 logical processors;
- Go `1.26.2 windows/amd64`;
- MSYS2 UCRT64 GCC race-capable toolchain.

### Ubuntu

- Ubuntu 24.04 under WSL2;
- same host CPU and storage;
- disposable official Go `1.26.2 linux/amd64`, verified against official
  download metadata and SHA-256 before use;
- CGO-enabled targeted and full-backend race validation.

The installed Ubuntu system Go was not modified. The verified disposable
toolchain was removed after the run.

## Validation results

| Gate | Result |
|---|---:|
| Lifecycle unit and conformance tests | PASS |
| Target `go vet` | PASS |
| Three shuffled full-backend repetitions | PASS |
| Full backend regression | PASS |
| Full backend `go vet` | PASS |
| Windows targeted race | PASS, zero races |
| Windows full-backend race | PASS, zero races |
| Ubuntu target tests and five shuffled repetitions | PASS |
| Ubuntu full-backend regression and vet | PASS |
| Ubuntu targeted race | PASS, zero races |
| Ubuntu full-backend race | PASS, zero races |
| Lifecycle statement coverage | 85.4% |
| Conformance statement coverage | 85.3% |
| Source-proof fuzz campaign | 1,063,068 executions, PASS |
| Forbidden production dependency audit | PASS |
| Production source-handle reveal audit | PASS |

## Windows benchmark summary

12th Gen Intel Core i5-12450H, Go 1.26.2, five runs:

| Benchmark | Range | Memory |
|---|---:|---:|
| Lifecycle repository get | 120.8–138.1 ns/op | 48 B/op, 1 alloc/op |
| Registration fingerprint | 605.5–693.2 ns/op | 608 B/op, 6 allocs/op |
| Independent registration | 2.965–3.704 µs/op | 2,467–2,469 B/op, 27 allocs/op |

The lifecycle benchmarks use an in-memory atomic test store and do not claim
database, filesystem, engine, or transport performance.

## Ubuntu benchmark summary

Same host, disposable official Go 1.26.2, three runs:

| Benchmark | Range | Memory |
|---|---:|---:|
| Lifecycle repository get | 98.12–111.0 ns/op | 48 B/op, 1 alloc/op |
| Registration fingerprint | 547.0–701.8 ns/op | 608 B/op, 6 allocs/op |
| Independent registration | 2.576–3.468 µs/op | 2,468–2,469 B/op, 27–28 allocs/op |

## Dependency and security audit

Production lifecycle Go files import no RIE, LIE, persistence, PostgreSQL
adapter, runtime, database driver, SQL, HTTP, filesystem, command-execution, or
network package. They contain no call to `SourceHandle.Reveal`.

No database URL, credential, source path, repository root, listener, or secret
was introduced. Unknown raw dependency failures are converted to stable safe
service errors without exposing their text.

## Reproducibility

From `backend/` on Windows:

```text
go test ./service/repository/lifecycle ./service/repository/conformance
go test -cover ./service/repository/lifecycle ./service/repository/conformance
go test -shuffle=on -count=3 ./...
go vet ./...
go test -race ./...
go test ./service/repository/lifecycle -run '^$' -fuzz '^FuzzSourceProofNeverPanics$' -fuzztime=5s
go test -run '^$' -bench . -benchmem -count=5 ./service/repository/lifecycle ./service/repository/conformance
```

Ubuntu uses the already accepted Repository Service bootstrap:

```text
bash backend/service/repository/tests/bootstrap_linux.sh
```

## Exit-gate assessment

Phase 4.0.3 reached its documented implementation and quality gate. Engineering
accepted the evidence on 2026-07-27. The implementation may be committed and
pushed. Phase 4.0.4 is authorized; every later milestone remains unauthorized.
