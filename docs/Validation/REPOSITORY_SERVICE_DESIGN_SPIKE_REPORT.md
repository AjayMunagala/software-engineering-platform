# Phase 4.0.1 Repository Service Design Spike Report

## Status

- Milestone: Phase 4.0.1
- Status: Accepted on 2026-07-27
- Date: 2026-07-27
- ADR: 0016 Accepted
- Production service implementation: not started
- Phase 4.0.2: authorized

## Executive result

The design spike validates the risky assumptions required by ADR 0016 without
creating a production Repository Service implementation. All authorized gates
pass within the documented spike scope:

- deterministic artifact identity and a frozen canonical byte preimage;
- exact-byte, file-backed, sealed materialization;
- independent SHA-256 and byte-count verification during simulated staging;
- bounded heap behavior for a 64-MiB payload;
- source-root redaction and post-serialization leak rejection;
- released RIE and Go LIE composition through fresh per-run artifacts;
- cancellation-aware keyed in-process single-flight;
- committed-but-response-lost publication reconciliation;
- Windows and Ubuntu regression, vet, shuffle, and race validation.

No REST/gRPC/HTTP transport, authentication, UI, queue, worker, database
adapter, SQL, credential, remote fetch, repository mutation, or production
service package was introduced.

## Experimental implementation

The isolated package is:

```text
backend/internal/experiments/repositoryservice/
```

Its package comment and README identify it as spike-only evidence. Released
engines, Persistence Port, PostgreSQL Adapter, and Runtime Infrastructure do
not import it.

## Finding 1 — Canonical artifact identity

`repository-service-artifact-id/v1` is validated with a fixed byte contract:

```text
ASCII "repository-service-artifact-id/v1" followed by one NUL byte
uint32-be length + exact UTF-8 repository ID
uint32-be length + exact UTF-8 scan ID
uint32-be length + exact UTF-8 artifact name
uint32-be length + exact UTF-8 artifact version
uint32-be length + exact UTF-8 stable-ID scheme
```

Every field is non-empty, already trimmed, valid UTF-8, no more than 1,024
bytes, and contains no ASCII control characters. The output is `rsaid1_` plus
lowercase hexadecimal SHA-256.

Frozen golden vector:

```text
repository ID: repo-001
scan ID: scan-01
artifact: go-semantic-inventory
artifact version: 1.0.0
stable-ID scheme: go-semantic-id/v1
artifact ID: rsaid1_3c55ac33a130d92a42bd4f782ad7868d9310b94e3fbb91cc3ba9abb85df8fce8
```

Golden preimage bytes, input validation, material-change behavior, and example
output are executable tests. A future incompatible representation requires a
new identity-scheme version.

## Finding 2 — Exact-byte materialization

The spike materializer:

1. creates a permission-restricted temporary file outside the repository;
2. invokes the encoder exactly once;
3. counts bytes and computes SHA-256 while writing;
4. rejects the write immediately above the configured limit;
5. syncs and closes the file before exposing it;
6. scans the sealed bytes for forbidden source-root variants;
7. exposes reopenable readers without exposing the spool path;
8. independently re-hashes and counts during simulated staging;
9. detects post-seal tampering;
10. removes the spool idempotently on success, failure, or cancellation.

A real 64-MiB exact-byte write and verification allocated 69,840–70,200 bytes
on the Go heap across three Windows samples. This excludes the existing source
artifact and operating-system page cache and remains far below the candidate
64-MiB working-memory ceiling.

The 4-GiB limit is enforced arithmetically before a write crosses the bound.
The spike did not create a disposable 4-GiB file because the accepted
persistence benchmark already validates multi-gigabyte storage behavior.

## Finding 3 — Durable codec views

Released analysis stages compose without API changes, but not every engine
artifact should be serialized directly:

- the RIE report contains scan ID, wall-clock times, duration, throughput, and
  local root paths, so the spike defines an explicit deterministic durable view
  that removes those values;
- `GoLanguageInventory` intentionally stores private state and has no JSON
  method, so the spike builds a detached view from its frozen defensive
  accessors;
- Go Package Identity and Go Semantic Inventory already expose detached JSON
  views and serialize deterministically.

This supports the design decision that versioned codecs belong between engines
and persistence. Persistence remains unable to interpret engine artifacts.
Full production codec coverage for every frozen RIE artifact remains a later
gated milestone.

## Finding 4 — Released engine composition

The spike executed this sequence on a disposable Go module:

```text
RIE 1.0.0 pipeline
  -> GoLanguageInventory 1.0.0
  -> GoPackageIdentityInventory 1.0.0
  -> GoSemanticInventory 1.0.0
```

Two fresh runs produced identical semantic JSON and identical redacted durable
RIE JSON. The encoded payload contained no absolute repository root. A separate
non-Go fixture completed after RIE and intentionally produced no Go artifacts.

## Finding 5 — Single-flight and cancellation

One hundred concurrent callers using the same key executed the supplied
analysis function exactly once and received the same result. Additional tests
prove:

- one canceled waiter does not cancel a remaining waiter;
- when every current waiter leaves, shared execution is canceled;
- the canceled key is released so a later request can start fresh;
- invalid context, key, and function inputs fail explicitly;
- the implementation passes Windows and Linux race detection.

This evidence covers only one process. Durable distributed leases and worker
recovery remain explicitly deferred.

## Finding 6 — Publication reconciliation

The spike injects a raw persistence error containing sensitive driver-like text
after modeled commit. When durable state reports `succeeded`, the operation is
returned as a reconciled success. Running, unavailable, or missing proof returns
only `publication outcome is ambiguous`; the raw error is not exposed.

PostgreSQL publication itself was not reimplemented. Its already-released
atomic contract remains behind Persistence Port `1.0.0`.

## Reference environments

### Windows

- Microsoft Windows 11 Home Single Language `10.0.26200`;
- 12th Gen Intel Core i5-12450H, 12 logical processors;
- 7.72 GiB visible RAM;
- Go `1.26.2 windows/amd64`;
- MSYS2 UCRT64 GCC `16.1.0` for race builds.

### Ubuntu WSL

- Ubuntu `24.04.4 LTS` on WSL2;
- kernel `6.18.33.2-microsoft-standard-WSL2`;
- 12 logical processors and approximately 3.68 GiB visible RAM;
- SHA-256-verified official disposable Go `1.26.2 linux/amd64`;
- GCC `13.3.0`.

The Linux bootstrap downloaded the official archive, verified its checksum
against current Go release metadata, ran validation, and removed the toolchain.

## Automated validation

| Gate | Windows | Ubuntu |
|---|---|---|
| Targeted tests | PASS | PASS |
| Three shuffled targeted runs | PASS | PASS |
| Full backend regression | PASS | PASS |
| `go vet ./...` | PASS | PASS |
| Targeted race | PASS, zero races | PASS, zero races |
| Full backend race | PASS, zero races | PASS, zero races |
| Source-root leak rejection | PASS | PASS |
| Tamper detection | PASS | PASS |
| 100-caller single-flight | PASS | PASS |
| Statement coverage | 86.8% | Same source/test suite |

The package also passes repeat execution, malformed identity/configuration,
limit, cancellation, cleanup, exact-byte, redaction-boundary, non-Go profile,
and publication-failure tests.

## Benchmarks

Benchmarks use warm operating-system cache, five Windows repetitions and three
Ubuntu repetitions. Engine, database, and transport time are not included.

| Benchmark | Windows | Ubuntu | Allocations |
|---|---:|---:|---:|
| One artifact ID | 399.3–426.1 ns | 389.2–431.8 ns | 384 B, 4 allocs |
| Twenty artifact IDs | 8.259–9.233 µs | 8.301–8.522 µs | 8,000 B, 100 allocs |
| 16-MiB materialize + verify | 66.68–67.87 ms | 30.96–32.09 ms | ~68 KB, 28–29 allocs |
| 100-caller single-flight harness | 96.90–100.89 µs | 107.47–114.52 µs | platform-dependent |

Observed exact-byte throughput:

- Windows: 247.20–251.60 MB/s;
- Ubuntu WSL: 522.80–541.96 MB/s.

The 20-artifact identity work is more than three orders of magnitude below the
candidate 25-ms service-overhead ceiling. Materializer allocation remains
independent of payload size in the measured fixtures.

## Defects found and corrected during the spike

1. JSON escaped Windows path separators were initially absent from the
   forbidden-value scan. The scanner now checks native, slash-normalized, and
   JSON-escaped root representations with a boundary-crossing test.
2. A single-flight key initially remained occupied briefly after every waiter
   canceled. The final implementation removes the key before canceling the
   abandoned execution, allowing a later request to start cleanly.
3. The first Windows race invocation supplied an absolute compiler path without
   adding the UCRT64 runtime to `PATH`. The documented MSYS2 environment was
   restored; targeted and full race suites then passed.

## Known limitations

- This is an experimental internal package, not a stable public service API.
- Publication reconciliation uses a neutral fake state reader; PostgreSQL
  integration belongs to later gated implementation.
- Full per-artifact RIE codec and dependency-manifest coverage is not yet built.
- JSON codec versions are not frozen by this spike; only the artifact-ID
  canonical representation is frozen as candidate evidence.
- Single-flight is process-local and provides no restart recovery.
- The root leak guard complements explicit durable views; it is not a general
  secret-detection system.
- The large heap proof uses 64 MiB, not the four-GiB operational maximum.

## Exit-gate assessment

| Requirement | Result |
|---|---|
| Released RIE/Go LIE composition | PASS |
| Canonical deterministic artifact ID | PASS; candidate bytes frozen |
| Exact-byte materialization and independent verification | PASS |
| Bounded heap and cleanup | PASS |
| Path redaction and leak rejection | PASS |
| 100-request single-flight | PASS |
| Cancellation policy | PASS |
| Publication ambiguity reconciliation | PASS |
| Windows and Ubuntu race validation | PASS |
| Production boundary preserved | PASS |

## Governance decision

Engineering accepted Phase 4.0.1 evidence on 2026-07-27. The spike package
remains explicitly experimental, and the candidate contract incorporates the
frozen ID algorithm while remaining at `0.1.0`. Only Phase 4.0.2 — Neutral
Service Contract and Conformance Harness is authorized. Phase 4.0.3 and later,
REST/gRPC, UI, authentication, AI, queues, workers, cloning, and remote fetch
remain unauthorized.
