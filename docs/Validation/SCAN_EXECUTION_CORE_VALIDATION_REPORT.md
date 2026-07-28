# Scan Execution Core Validation Report

## Status

- Phase: 4.0.4 — Scan Execution Core
- Contract version: `0.1.0`
- Local implementation: complete
- Local validation: pass
- Engineering acceptance: accepted on 2026-07-28
- Phase 4.0.5 Intelligence & Materialization Adapters: authorized
- Date: 2026-07-27

## Scope

Phase 4.0.4 implements the transport-, persistence-, runtime-, and
engine-neutral synchronous scan coordinator.

Implemented:

- explicit running, succeeded, failed, and canceled state transitions;
- one admission lease for each newly created in-process execution flight;
- keyed single-flight with 100-caller validation;
- independent waiter cancellation and all-interested-caller cancellation;
- admission-drain and explicit `CancelScan` coordination;
- store-owned atomic begin, publication, terminal finalization, cancellation,
  scope isolation, and idempotency;
- explicit durable orphan detection;
- committed-but-response-lost publication reconciliation;
- deterministic artifact metadata from fake, already-prepared analysis input;
- scan and artifact get/list plus exact fake-payload export orchestration;
- stable redacted errors and bounded detached cleanup;
- additive reusable scan-and-artifact conformance.

Not implemented:

- real RIE or Go LIE orchestration;
- artifact codecs, durable views, or sealed materialization;
- Persistence Port, PostgreSQL, SQL, pgx, or migrations;
- Runtime Infrastructure integration or runtime package imports;
- filesystem traversal, source-root access, network, or commands;
- REST/gRPC, HTTP, authentication, authorization, UI, queues, or AI.

## Architecture

The production package is `backend/service/repository/scan`. It implements the
neutral `ScanExecutionService` and `ArtifactQueryService` contracts through
four narrow dependencies:

- `AdmissionController`, returning one cancellable `WorkLease`;
- `AnalysisPreparer`, returning one isolated, path-free `AnalysisSession`;
- `Store`, owning all atomic lifecycle and publication semantics;
- `Clock`, making state transitions deterministic under test.

The analysis session returns path-free source proof and immutable fake
artifact candidates. It does not expose a repository root or engine artifact.
Exact fake payloads are already prepared and reopenable; creating their real
durable representation remains Phase 4.0.5 work.

## Single-flight and cancellation

Flights are keyed by scope, principal, repository ID, and scan ID. Identical
requests join. Reusing the same request ID with different input produces
`idempotency_conflict`; another request targeting a running scan produces
`scan_already_running`.

The first caller creates a detached execution context. Canceling one waiter
does not cancel other interested callers. When every interested caller leaves,
the execution context is canceled. Runtime-drain cancellation is modeled only
through the neutral lease context. Every acquired lease and prepared analysis
session is released once.

## Durable behavior

`Begin` atomically distinguishes:

- a new running scan;
- an already-published idempotent retry;
- a previously failed/canceled request;
- a durable running scan without an in-process leader (`orphaned_scan`).

Before publication, cancellation finalizes the running scan as canceled and
analysis/metadata failures finalize it as failed. Publication itself belongs
to the store. If the publication response is lost, the coordinator queries
durable state:

- succeeded with exact expected scan/artifact metadata returns success;
- canceled or failed returns its explicit terminal result;
- running, missing, or unavailable returns retryable
  `persistence_unavailable (publication-ambiguous)`;
- a published metadata mismatch returns `integrity_failure`.

An ambiguous scan is never finalized as failed.

## Validation defects corrected

### Publication verification was initially too narrow

The first implementation checked only the returned scan state and ID after a
successful publication. That could accept a malicious or defective store
returning different artifact metadata.

Publication and reconciliation now compare the complete scan contract and
every ordered artifact metadata field against the deterministic expected
publication. Any mismatch fails with `integrity_failure`.

### Admission cancellation initially depended only on an asynchronous callback

The lease context was connected through `context.AfterFunc`, but an already
canceled lease could race with the next startup checkpoint. The coordinator
now checks the lease context synchronously immediately after acquisition and
also retains the callback for later drain cancellation.

## Conformance validation

The reusable conformance package now exposes `ScanFactory` and `RunScan`. The
suite requires only `ScanExecutionService` and `ArtifactQueryService` and
validates:

- seeded scan/artifact reads and exact export;
- deterministic scan and artifact listing;
- repository-scope isolation for scans, artifacts, lists, and export;
- successful synchronous execution and durable retry disposition;
- idempotent cancellation of a running scan;
- context cancellation and idempotent fixture cleanup.

Both the original Phase 4.0.2 memory fake and the new production coordinator
pass the additive suite.

## Failure-injection and concurrency evidence

Tests cover:

- 100 concurrent identical callers: one admission, begin, analysis, and
  publication; one `created` and 99 `joined` results;
- canceled waiter while the leader continues successfully;
- cancellation after all callers leave;
- admission-drain cancellation;
- explicit `CancelScan` racing with an active analysis;
- analysis failure and bounded failed finalization;
- terminal finalization failure;
- durable orphan detection;
- publication commit followed by response loss;
- publication failure before commit and unavailable reconciliation;
- reconciled succeeded, failed, and canceled states;
- conflicting in-process requests;
- path-free source proof, scope isolation, deterministic ordering, pagination,
  query/export cancellation, invalid dependencies, and safe error redaction.

## Validation environments

### Windows

- Microsoft Windows 11 `10.0.26200`;
- 12th Gen Intel Core i5-12450H, 12 logical processors;
- Go `1.26.2 windows/amd64`;
- MSYS2 UCRT64 GCC race-capable toolchain.

### Ubuntu

- Ubuntu 24.04 under WSL2;
- disposable official Go `1.26.2 linux/amd64`;
- toolchain archive verified against official download metadata and SHA-256;
- CGO-enabled targeted and full-backend race validation.

The Ubuntu system Go was not modified, and the disposable toolchain was
removed after validation.

## Validation results

| Gate | Result |
|---|---:|
| Scan and conformance tests | PASS |
| Five shuffled target repetitions | PASS |
| Full backend regression | PASS |
| Full backend `go vet` | PASS |
| Three shuffled full-backend repetitions | PASS |
| Windows targeted race | PASS, zero races |
| Windows full-backend race | PASS, zero races |
| Ubuntu target tests and five shuffled repetitions | PASS |
| Ubuntu full-backend regression and vet | PASS |
| Ubuntu targeted race | PASS, zero races |
| Ubuntu full-backend race | PASS, zero races |
| Scan statement coverage | 88.4% |
| Conformance statement coverage | 85.0% |
| Artifact-candidate fuzz campaign | 913,409 executions, PASS |
| Forbidden production dependency audit | PASS |
| Production source-handle reveal audit | PASS |

## Windows benchmark summary

12th Gen Intel Core i5-12450H, Go 1.26.2, five runs:

| Benchmark | Range | Memory |
|---|---:|---:|
| Scan get | 157.6–178.3 ns/op | 160 B/op, 3 allocs/op |
| Independent fake scan execution | 13.002–15.044 µs/op | 9,677–9,731 B/op, 73 allocs/op |
| Execute mutation fingerprint | 640.6–762.9 ns/op | 736 B/op, 8 allocs/op |

## Ubuntu benchmark summary

Same host, disposable official Go 1.26.2, three runs:

| Benchmark | Range | Memory |
|---|---:|---:|
| Scan get | 158.8–180.3 ns/op | 160 B/op, 3 allocs/op |
| Independent fake scan execution | 21.231–21.566 µs/op | 9,613–9,671 B/op, 73 allocs/op |
| Execute mutation fingerprint | 559.3–598.0 ns/op | 736 B/op, 8 allocs/op |

These benchmarks measure orchestration with a memory store and fake prepared
analysis only. They make no claim about real engines, materialization,
PostgreSQL, runtime, network, or transport performance.

## Dependency and security audit

Production scan Go files import no RIE, LIE, Persistence Port, PostgreSQL
adapter, Runtime Infrastructure, database driver, SQL, HTTP, filesystem,
command-execution, or network package. They contain no call to
`SourceHandle.Reveal`.

No repository path, database URL, credential, listener, or secret was
introduced. Unknown dependency errors are translated into stable service
errors without raw text.

## Reproducibility

From `backend/` on Windows:

```text
go test ./service/repository/scan ./service/repository/conformance
go test -cover ./service/repository/scan ./service/repository/conformance
go test -shuffle=on -count=5 ./service/repository/scan ./service/repository/conformance
go test ./...
go test -shuffle=on -count=3 ./...
go vet ./...
go test -race ./...
go test ./service/repository/scan -run '^$' -fuzz '^FuzzArtifactCandidateNeverPanics$' -fuzztime=5s
go test -run '^$' -bench . -benchmem -count=5 ./service/repository/scan
```

Ubuntu uses:

```text
bash backend/service/repository/tests/bootstrap_linux.sh
```

## Exit-gate assessment

Phase 4.0.4 reached its documented implementation and quality gate and was
accepted by engineering on 2026-07-28. Phase 4.0.5 is authorized. Phase 4.0.6
and every later milestone remain unauthorized.
