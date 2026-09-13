# Phase 5.0.5 — Codec and fake-capability implementation checkpoint

Date: 2026-09-13. Candidate 0.1.0. **Submitted for engineering review, not phase
acceptance.** Baseline/design `1b9f4d6`; independent vectors `3d5afe7` were approved
before this production implementation. ADR 0023 remains Design Approved.

## Scope and evidence

New packages are `backend/die/codec` and
`backend/internal/integration/dependency`. No released contract, existing graph
algorithm, vector literal, migration or Repository Service implementation changed.
The bridge accepts immutable inventories and borrowed capabilities. It never
executes analysis or constructs runtime/pools. Tests use fake persistence/admission
and a fake public runtime facade, not a live runtime.

Implemented: ordered canonical encoding, exact final-LF hashing, identity frames,
quota-limited memory/disk spool, sealed reads, publication/reconciliation, verified
export, scoped allowlisting and safe errors. One artifact, no projections or
invented upstream edges, MakeCurrent=false. Typed reload/profile extension remain
deferred. Dedicated repository registration is a host prerequisite.

## Recorded local checks

| Check | Result |
|---|---|
| Independent Node verifier | PASS: six payloads, 30 child IDs, five identity mutations, manifest byte mutations, two existing digest anchors |
| Codec unit coverage | PASS: 91.2% |
| Bridge unit/fake capability coverage | PASS: 85.6% |
| Full backend ordinary regression | PASS (includes cached packages; live opt-ins disabled) |
| Final `go vet ./...` | PASS |
| New-package Windows race/shuffle, three repeats | PASS; no data races reported |
| Codec bounded fuzz | PASS: 208,009 executions |
| Request bounded fuzz | PASS: 45,242 executions |
| Initial full backend Windows race run | FAIL: unchanged runtime shutdown timing assertion; no race detector report |
| Isolated shutdown race rerun, three repeats | PASS (4.457 s package result) |
| Final full backend Windows race rerun | PASS; no data races reported; live integration disabled |

The initial full-race failure was
`TestBlockedResourceCloseIsBoundedAndNotReportedStopped`: the bounded-close
assertion observed approximately 491.78 seconds and state `stopping`. No cause has
been established; a host scheduling explanation is not asserted. No frozen runtime
code was changed. The subsequent full-race rerun passed; a retry does not erase
this failure.

Commands were run from backend: `go test ./...`, `go vet ./...`,
`go test ./die/codec ./internal/integration/dependency -cover -timeout=30s`,
`go test -race ./die/codec ./internal/integration/dependency -shuffle=on -count=3 -timeout=60s`,
and `go test -race ./... -timeout=120s` for the final retry. Isolated command:
`go test -race ./internal/runtime/app -run=^TestBlockedResourceCloseIsBoundedAndNotReportedStopped$ -count=3 -timeout=30s`.
Fuzz targets: `FuzzRecordEncoding` and `FuzzRequestValidation`, five-second budgets,
bounded worker count. These are short local fuzz characterizations, not exhaustive
failure-sequence exploration.

Live database/runtime/repository integration switches and large-fixture switches
were disabled; POSTGRES_TEST_URL and SEMANTIC_VALIDATION_ROOT were empty.
Existing fake-backed runtime unit tests are not live runtime integration tests.

## Oracles and failure cases

- Public Encode is checked against frozen empty and analyzed-empty literal bytes.
  All six vectors additionally exercise record encoding, including Unicode,
  escaping and uint64 beyond JavaScript's safe range. Wire-only fixtures are not
  claimed to be valid normalized inventories. No production decoder was added.
- All six artifact UUID/manifest sets and 30 operation identities match frozen
  literal values. The independent Node verifier remains unchanged.
- Fake publication/export, lost publication reply, idempotent retry, incorrect
  digest and incomplete stream consumption, stage failure, scope rejection,
  archival, unknown/running/terminal reconciliation and metadata mismatches pass.
- Fourteen envelope fields are independently corrupted. Unverified export bytes
  never reach the caller. Caller writer failure returns no success receipt.
- Memory/spill/quota, pre-seal/reopen/close, cancellation, owned-file cleanup,
  active-reader protection and concurrent reads are covered.

## Corrections and API refinements

1. Cleanup now tracks successful exclusive creation; failed creation cannot cause
   removal of a pre-existing file. A preservation regression test covers this.
2. `Uncertain` retains a detached expected publication for later reconciliation,
   without exposing revision/cause in error formatting.
3. Receipt/reconciliation formatting was explicitly redacted after nested private
   revision leakage was identified during review.
4. Revision maximum is 512 UTF-8 bytes, matching the frozen persistence port rather
   than allowing a larger value that the port would later reject. Paths/controls
   are rejected. Zero spool limits select approved defaults.

Successful existing-scan retries fully encode and compare expected metadata but
do not restage a succeeded scan: the frozen stager requires a running scan. Every
actual StagePayload call must consume the complete stream and EOF. Manifest
reconciliation reconstructs from metadata; it does not claim direct stored
certificate access unavailable through the frozen port.

## Characterization and environment

Windows amd64, Go 1.26.2, Git 2.54.0.windows.1, Node 24.14.1/OpenSSL 3.5.5.
CPU reported by Go benchmark: 12th Gen Intel Core i5-12450H. Windows race uses
MSYS2 UCRT64 GCC with CGO_ENABLED=1, GOMAXPROCS=4. No PostgreSQL configuration,
TLS/pool settings or Linux measurements apply to this checkpoint.

`go test ./die/codec -run=^$ -bench=BenchmarkEmpty -benchmem -count=1`:
330,001 iterations; 3,407 ns/op; 1,921 B/op; 44 allocs/op. This is one warm local
empty-inventory measurement, not a cold-storage, large-payload or performance gate.
No peak heap/RSS claim is made. Spool limits bound encoded storage, not defensive
clones, individual records, process memory or uninterrupted filesystem calls.

## Explicitly unexecuted / remaining

PostgreSQL/runtime integration, crash/restart/backup restore, retention, actual
database concurrency and certificate checks are **not authorized and not run**.
Ubuntu validation, the full mandatory failure matrix (including physical disk-full
and OS close/remove fault injection), nonempty accepted-inventory end-to-end
cross-platform cases, large-fixture characterization and requalification of the
unchanged Core 30-second/2-GiB gate remain outstanding. This checkpoint does not
claim completion of the entire Phase 5.0.5 validation plan.

Host spool privacy/ACLs, disk capacity and crash-orphan recovery are host obligations.
Dependency calls must honor cancellation; no hard interruptibility claim is made.
DIE-HARDEN-001/002 remain open. No engineering acceptance, ADR promotion, Phase
5.0.6 work, release/version promotion or release tag is performed by this commit.
