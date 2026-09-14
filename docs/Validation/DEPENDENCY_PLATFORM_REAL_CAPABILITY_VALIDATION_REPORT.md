# Phase 5.0.5 — Real PostgreSQL/runtime validation checkpoint

Date: 2026-09-14. Baseline: `38dc5e83014a95e34a8cf7df509ce11fb2741322`.
The manager accepted that implementation checkpoint and authorized real integration
validation. **This report is submitted for review, not final Phase 5.0.5 acceptance.**
ADR 0023 remains Design Approved. Candidate stays 0.1.0; hardening 001/002 stay open.

## Execution and isolation

The new `backend/internal/integration/dependency/tests/validate_disposable.sh`
creates only disposable clusters and databases. No personal database, credential,
frozen contract, migration or released Repository Service implementation changed.
Each platform's final run used a distinct new cluster. Runtime was constructed
through the released default starter and closed after each test process. All
publication calls used borrowed ingest/read capabilities and real runtime admission.

## Passing real-capability cases (Windows and Ubuntu)

- Initial exact publication and verified export.
- Lost Publish reply injected after the real transaction succeeded; exact recovery.
- Same request/content idempotent retry.
- Eight concurrent independent bridge instances against one real scan.
- Already-canceled request rejection; zero remaining runtime admission leases.
- Wrong expected digest rejected; unallowlisted scope rejected without exposure.
- Stored publication digest/scheme/count compared with reconstructed manifest using
  a test-only observer connection (not a new production capability).
- Immediate PostgreSQL stop after publication, restart, and a fresh Go process
  reconstructing its expectation independently without a retained spool/result.
- `pg_dump` custom archive restored into a new disposable database, then another
  fresh process verifies both payloads and stored certificates.
- Archived repository rejects publication. Released retention capability marks,
  purges and garbage-collects the restored fixture; purged publication is absent.
- Runtime shutdown reports resources closed. Harness cluster/data cleanup completed.

| Fixture | Bytes | SHA-256 (same on both platforms and recovery passes) |
|---|---:|---|
| Empty inventory | 614 | `2341cdbfd54d1819086af76f07ebb9bf8c4c55dbd6a7e73b395a3114db122088` |
| Nonempty Unicode package | 988 | `b66500731e027bd4ce83ece85444db13f255c446e668ae568bd6b9fa5d5e5c4d` |

Expected bytes use accepted immutable View JSON plus LF independently of the new
encoder. This validates a small nonempty normalized fixture, not upstream Go/RIE
execution, a large graph, typed reload or persisted upstream closure.

## Defect found and corrected

The first Ubuntu real-concurrency run failed: two callers received `scope_not_found`
while the final scan was correctly published. A concurrent instance can finish
between Begin and Stage; Stage then rejects a scan that is no longer running.
The bridge now attempts exact publication reconciliation for stage Missing/Conflict
errors and accepts only an exact durable match. Otherwise it preserves the error.
No persistence/runtime behavior changed. Repeated subsequent real matrix runs on
both platforms passed. The initial failure is retained as evidence.

A preliminary Ubuntu regression build also failed to import newly added test
imports while test source was being edited. This was an invalid concurrent-edit
validation attempt; it is retained, not counted as a pass. Regression/vet on stable
source subsequently passed. No frozen package was changed to address it.

## Environment / methodology

- CPU: Intel Core i5-12450H; WSL reports 12 logical CPUs; Linux tests GOMAXPROCS=4.
- Ubuntu 24.04, kernel `6.18.33.2-microsoft-standard-WSL2`, Go 1.26.2 linux/amd64,
  CGO enabled; Git 2.43.0. Native Windows Go 1.26.2 amd64.
- WSL visible RAM 3,953,807,360 B; swap 1,073,741,824 B. Measured free/available
  memory varies; this is environment description, not an allocation guarantee.
- PostgreSQL `18.4 (Ubuntu 18.4-1.pgdg24.04+1)`, Atlas Community 1.2.3.
- Cluster in ext4 `/tmp`; repository source on Windows drive exposed via WSL 9p.
  Reported ext4 available space 995,223,532 KiB at preflight; not a physical disk
  capacity/performance guarantee (virtual filesystem).
- Fresh UTF-8/no-locale initdb, local trust, loopback TCP only, private socket path,
  high PID-derived port. fsync remains enabled; no `-F` optimization. Other server
  settings are initdb defaults. No production secrets or TLS qualification.
- Released runtime CI defaults with only host/port/database/user overrides; host
  owns lifecycle and combined capability login. Spool threshold 32 B, maximum
  1 MiB forces disk spill for both small fixtures. No pool policy changes.

Each final platform matrix ran initial publication, crash recovery and restored
recovery in separate processes (three processes per platform). No benchmark gate
is inferred from their elapsed times. Exact test commands are in the committed
harness. Unit commands: Ubuntu `go test ./... -timeout=120s`, `go vet ./...`, and
new-package `go test -race ... -count=3 -shuffle=on -timeout=120s`.

## Remaining mandatory evidence — phase remains open

The unchanged Core gate was requalified with DIE_CORE_SCALE=1, GOMAXPROCS=4,
`go test ./die -run ^TestScaleGate100KNodes1MEdges$ -count=1 -timeout=90s -v`.
Ubuntu: 4.978260343 s, 1,041,487,208 B sampled peak live heap. Windows: 9.9567845 s,
1,047,335,432 B. Both pass the existing 30-second/2-GiB gate; neither measures
large publication, disk-spool or database throughput. New-package race/shuffle
three-repeat checks passed on both platforms. Windows targeted vet passed.
Ubuntu full-backend `go test -race ./... -timeout=120s` also passed on stable
source, with no data races reported and live integration opt-ins disabled.

This checkpoint does **not** finish the entire authorized validation plan. Still
outstanding: physical disk-full and OS close/remove fault injection; forced
process termination at every precommit boundary; cancellation at all commit
checkpoints; real database chunk corruption/truncation/reordering matrix; exhaustive
metadata mutation against real storage; large publication/spool/heap characterization;
and any full-backend cross-platform race evidence not explicitly recorded in the
machine-readable results. TLS/least-privilege profile expansion is not claimed by
this combined-login CI fixture. Counts are not a process-memory bound.

The prior fake report remains historical evidence; it is not rewritten to pretend
these later runs occurred earlier. No ADR promotion, final phase acceptance,
Phase 5.0.6, release tag, typed reload, profile extension or upstream closure is
authorized by a passing checkpoint. Stop after the review commit as requested.
