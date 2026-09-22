# Phase 5.0.5 — Fault-injection and large-publication checkpoint

Date: 2026-09-22. Baseline `dfd551fdb5ce0ac3273861f1442729870aaf0aa8`
was accepted as the real-integration checkpoint. This follow-up is validation-only:
no production implementation or frozen contract changes. Final phase acceptance,
ADR 0023 promotion, downstream work and release remain manager gates.

## Isolation and reproducibility

The existing disposable harness gains `faults` (Ubuntu) and `faults-windows`
(native Windows against its own new WSL PostgreSQL cluster) modes. Both use the
frozen migrations, a disposable least-capability CI login, real runtime admission,
and test-only PostgreSQL observer connections. No personal database is used.
Run from the repository root through Ubuntu-24.04 as the postgres OS user:

```text
bash backend/internal/integration/dependency/tests/validate_disposable.sh faults
bash backend/internal/integration/dependency/tests/validate_disposable.sh faults-windows
```

Filesystem capacity testing uses a different harness, run as root only to create
and mount a new private 16-MiB ext4 image. Go tests run as postgres. No host disk is
filled. The mount uses nosuid,nodev,noexec; exit cleanup unmounts it and removes
only the validated generated directory. This is genuine filesystem-capacity
exhaustion, not simulated writer failure, but it is not physical hardware failure.

```text
bash backend/internal/integration/dependency/tests/validate_filesystem_full.sh
```

## Fault semantics and oracles

### Filesystem and OS failures

- Spill writes exhaust the isolated filesystem before the configured 64-MiB
  payload maximum; the integration returns a safe failure and refuses to seal.
- An owned descriptor is deliberately closed by the test before spool Close;
  actual OS closed-descriptor failure is surfaced, not a mocked success.
- Unix directory write permission is removed after sealing; unlink is denied,
  surfaced, and succeeds after permission restoration. Windows ACL removal
  injection is not claimed by this Unix-specific case.
- Test cleanup restores deliberately invalidated descriptors/permissions before
  removing test resources; this is not a claim that production magically repairs
  arbitrary OS faults or that synchronous filesystem calls have a hard timeout.

### Abrupt termination and cancellation

The boundary inventory is explicit: before encoding, before Begin (after sealed
encoding), after Begin, before Stage, during Stage stream consumption, after Stage,
before Publish, and after successful Publish but before its reply reaches the bridge.
For each boundary a parent test waits for the child's marker and kills only that
child; no child deferred cleanup is allowed to manufacture a successful outcome.
Independent parent reads require absence/non-publication before commit and exact
durable publication after commit. Parent-owned spool directories collect orphans.

Each boundary also has a cancellation case. Precommit cancellation must never
produce a successful visible publication; postcommit cancellation must reconcile
the exact durable publication. These are public capability/stream boundaries,
not hooks inside every SQL statement of the frozen adapter or power-loss tests.

### Real payload corruption

After successful large publication, the test observer commits one fault at a time:
flip a chunk byte, truncate a chunk, delete a chunk, or swap the contents of two
ordered chunks. Real export must fail with **zero bytes written to the caller**.
Original chunks are restored and final export must pass. No constraints, triggers
or released database behavior are disabled to force a pass.

### Storage metadata coverage

The matrix covers the publication contract's identities and metadata, not unrelated
audit timestamps or every possible corrupt database bit pattern:

| Fields | Enforcement tested |
|---|---|
| Artifact UUID, name/version, stable-ID scheme, codec name/version/media type, producer name/version | Committed mutation rejected by real bridge reconciliation |
| Source revision and analysis-profile digest | Committed mutation rejected by reconciliation |
| Repository security scope | Committed scope move becomes inaccessible to original scope |
| Payload size/digest, scan UUID, repository UUID invalid reference | PostgreSQL FK rejection, exact SQLSTATE 23503; no claim that a forbidden row existed |
| Stored publication certificate | Test observer detects altered certificate; production read port still has no certificate getter |

Constraint-blocked mutations and observer-only checks are not mislabeled as
production tamper detection. This preserves the already approved read-port limit.

## Large fixture and memory methodology

The fixture contains 20,000 package nodes, distinct source IDs and 4-KiB evidence
values. Its canonical payload is **91,400,628 bytes**, exceeding the default
64-MiB memory-to-disk threshold and spanning multiple 4-MiB PostgreSQL chunks.
SHA-256: `b235618afb7c7d6d61e9cc634a205a7f22f386558618cfcf4bda04080516602f`.

The test measures prerequisite construction separately, forces GC before measuring
publication, samples Go HeapAlloc every 5 ms, and records operation TotalAlloc delta.
Publish timing includes encoding/sealing, stage and atomic publication; export
timing includes verification into spool and caller streaming. The test observes
the actual spool file size immediately before staging, then compares exported
SHA-256 and byte count against the published receipt. Exact frozen codec oracles
remain the independent small-fixture proof; the large case does not introduce a
second artifact-sized reference serialization or a decoder.

These are warm workstation/WSL characterizations, not new performance gates,
maximum-RSS measurements, 4-GiB qualification, cold-cache measurements or a hard
process-memory bound. Sampled Go heap can miss peaks between samples. The tests
do not close DIE-HARDEN-001 or DIE-HARDEN-002.

## Measurements and environment

Final characterization (same payload hash and 91,400,628-byte spool on both):

| Platform | Prerequisite | Publish including encode/seal | Verified export | Sampled Go heap | Operation allocations |
|---|---:|---:|---:|---:|---:|
| Ubuntu | 0.565 s | 1.708 s | 0.662 s | 338,568,432 B | 739,455,064 B |
| Windows | 0.318 s | 2.594 s | 3.349 s | 489,082,480 B | 739,515,904 B |

Environment: existing Go 1.26.2, PostgreSQL 18.4, Atlas Community 1.2.3,
Ubuntu-24.04 WSL kernel 6.18.33.2-microsoft-standard-WSL2, native Windows amd64,
GOMAXPROCS=4, default 64-MiB spool threshold. Current WSL visible memory was
3,953,799,168 B plus 1,073,741,824 B swap. CPU remains Intel i5-12450H. Database
fsync is enabled; test clusters use loopback trust and default server settings.
No TLS/production-role qualification is inferred. Filesystem image tests explicitly
confirmed ENOSPC; 10,485,760 bytes had been accepted before exhaustion. Cleanup
inspection found no remaining harness mounts or cluster directories.

## Quality and retained development failure

Windows and Ubuntu `go test ./... -timeout=120s` and `go vet ./...` passed on the
final source. Targeted `go test -race ./internal/integration/dependency -count=3
-shuffle=on -timeout=120s` passed on both, with no race reports. Live fault tests
are separately opt-in and ran normally, not under these targeted unit race runs.
Full-backend race evidence remains in the prior checkpoint; it was not rerun here.
No production source changed, so no new production coverage percentage is asserted.

The initial reorder injection failed before changing storage: PostgreSQL inferred
CASE parameters as text and rejected assignment to bytea (SQLSTATE 42804). Explicit
bytea casts corrected the **test SQL**, after which the reorder case passed. This
was not a production correctness defect. The initial run is not counted as a pass.

## Governance

See the accompanying machine-readable results for actual executed platform checks
and measurements. This commit submits evidence for manager review, not self-issued
engineering acceptance. TLS/production role qualification remains unclaimed.
ADR 0023 remains Design Approved; candidate version remains 0.1.0; Phase 5.0.6 and
release remain unauthorized. No tags or production changes are part of this work.
