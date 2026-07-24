# PostgreSQL Payload Benchmark Report

## Status

- Phase: 3.2 — PostgreSQL Payload Benchmark Spike
- Execution date: 2026-07-24
- Status: Accepted
- Authorization: accepted Phase 3.2 benchmark plan
- Accepted decision: fixed four-MiB ordered chunks and a four-GiB operational limit
- Phase 3.3: authorized

## Decision Summary

The isolated benchmark passed every correctness gate. Exact payload bytes and
SHA-256 digests survived staging, reads, publication, rollback, garbage
collection, backup, and restore. Readers never observed a partial publication.

The benchmark does **not** support the original one-MiB chunk choice as the
release default. On the 1,556,379,091-byte Kubernetes fixture, its slowest warm
stage rate was 42.14 MiB/s, below the proposed 50 MiB/s gate. Four-MiB chunks
maintained at least 52.42 MiB/s, at least 499.64 MiB/s verified-read throughput,
and no more than 46.91 MiB of measured client RSS above baseline.

The accepted physical contract is therefore:

- four-MiB ordered `bytea` chunks;
- a four-GiB initial operational artifact limit;
- an unchanged eight-GiB schema ceiling;
- application-side streaming and digest verification;
- no single-value `bytea` path for authoritative artifacts.

## Approval and Isolation

The run used a disposable local PostgreSQL database whose name began with
`platform_bench_`. It used Unix-socket peer authentication. No password,
credential value, environment file, shared database, production database, or
personal data was used. The benchmark DDL is experimental evidence and is not
a Phase 3.3 migration.

## Environment

| Item | Recorded value |
|---|---|
| Host OS | Microsoft Windows 11 Home Single Language 10.0.26200 (build 26200) |
| Guest OS | Ubuntu 24.04.4 LTS under WSL2 |
| Kernel | Linux 6.18.33.2-microsoft-standard-WSL2, x86-64 |
| CPU | 12th Gen Intel Core i5-12450H, 8 cores / 12 logical processors |
| Host RAM | 8,284,266,496 bytes (7.72 GiB) |
| WSL RAM / swap | 3,861,140 KiB RAM / 1 GiB swap |
| Host storage | SSSTC CL4-8D512, NVMe SSD, 512,110,190,592 bytes |
| Measured filesystem | WSL ext4 virtual disk backed by the host NVMe SSD |
| PostgreSQL | 18.4, Ubuntu PGDG build, 64-bit |
| Go client | Go 1.26.2, linux/amd64, pgx v5.10.0 |
| Client workers / connections | one payload writer; one independent publication reader |
| Database | UTF8, `C` collation, data checksums enabled, UTC |
| PostgreSQL memory | `shared_buffers=128MB`, `max_connections=100` |
| WAL | replica level, full-page writes on, WAL compression off |
| Durability | synchronous commit on |
| TOAST compression | pglz |
| Filesystem state | fixtures pre-copied to ext4; measured repetitions used a warm OS cache |
| Repetitions | 10 per fixture/representation; recovery cases at least 3 where applicable |

The benchmark client baseline RSS was 11,874,304 bytes. Fixture generation,
copying, and initial manifest verification were outside the database timing
region.

## Immutable Fixtures

| Fixture | Commit | Artifact | Bytes | SHA-256 |
|---|---|---:|---:|---|
| Project | `2aefd88` | RIE report 1.0.0 | 6,417 | `e1cb6c89fdf26704babbb2573b14cdc6a566e646a4130c97428e8e576777a40d` |
| Project | `2aefd88` | Go Language Inventory 1.0.0 | 746,081 | `435d62b9099c602fe296dde01058cb72a2c436dd3f7d1debfef5268c04eeb461` |
| Project | `2aefd88` | Go Package Identity Inventory 1.0.0 | 199,502 | `ceebac3118f0941bf21dc287b3de36a43f8a348e813ea6601bc114e3760d6a64` |
| Project | `2aefd88` | Go Semantic Inventory 1.0.0 | 34,180,600 | `1d8901628d2f9cc4b6586d2da90ffa7513b5658efbf713bbf6012b3a7be70c79` |
| OpenTelemetry Go | `d98b5f42f2550ba2b851d9d95f4ddb6c4f98c4b0` | Go Semantic Inventory 1.0.0 | 635,557,163 | `b67e613b846b746ad45ae7fad16ad96056eaf66f42c92e8f8f0b03d68414d826` |
| Kubernetes | `1056cbb4511a20ae5e4d0173813cd3403c6d836a` | Go Semantic Inventory 1.0.0 | 1,556,379,091 | `a349696887dd3fd318f2fbebe43ad715a804c1f5b4c6d1d313b0c7e146689dba` |

The raw JSON report is retained outside Git. Its SHA-256 is
`2432ddbc48f0e7b4c381100b742111a7340205e553ca43c85b1ab206e84f52ad`.
Artifact payload bytes are not committed.

## Representation Results

The following figures are the large-fixture results that determine the
physical choice. Throughput minima are the slowest of ten warm repetitions.
RSS delta is the maximum measured client RSS minus the 11.32-MiB baseline.

| Fixture | Representation | Chunks | Min stage MiB/s | Min read MiB/s | WAL/input | Storage/input | Max RSS delta MiB | Gate |
|---|---|---:|---:|---:|---:|---:|---:|---|
| OpenTelemetry | 256 KiB | 2,425 | 48.43 | 217.09 | 0.090x | 0.089x | 8.07 | stage fail |
| OpenTelemetry | 1 MiB | 607 | 52.85 | 310.73 | 0.088x | 0.086x | 13.68 | pass |
| OpenTelemetry | 4 MiB | 152 | 86.73 | 642.57 | 0.088x | 0.085x | 48.18 | pass |
| Kubernetes | 256 KiB | 5,938 | 32.80 | 280.65 | 0.086x | 0.084x | 7.76 | stage fail |
| Kubernetes | 1 MiB | 1,485 | 42.14 | 447.13 | 0.084x | 0.081x | 17.84 | stage fail |
| Kubernetes | 4 MiB | 372 | 52.42 | 499.64 | 0.083x | 0.081x | 46.91 | pass |

The low WAL and relation amplification values are caused by pglz compression of
the JSON fixtures. They are valid for these released payloads but must not be
treated as a guarantee for incompressible future codecs.

Selected four-MiB latency details:

| Fixture | Stage p50 / p95 / max | Read p50 / p95 / max |
|---|---|---|
| Project semantic, 32.60 MiB | 325.66 / 428.02 / 428.02 ms | 62.65 / 78.86 / 78.86 ms |
| OpenTelemetry, 606.11 MiB | 5,496.57 / 6,988.13 / 6,988.13 ms | 835.12 / 943.27 / 943.27 ms |
| Kubernetes, 1,484.28 MiB | 25,404.90 / 28,317.06 / 28,317.06 ms | 2,328.74 / 2,970.69 / 2,970.69 ms |

With ten samples, the nearest-rank p95 is the maximum. Standard deviations for
four-MiB Kubernetes staging and reads were 3,063.58 ms and 265.10 ms,
respectively.

## Single-Value Failure Evidence

One `bytea` worked for the 34,180,600-byte project semantic artifact but was
not operationally safe for the 635,557,163-byte OpenTelemetry artifact. During
that insert, Linux killed the PostgreSQL backend for out-of-memory pressure.
At the kernel snapshot, the client held approximately 1.40 GiB RSS and the
backend approximately 2.00 GiB RSS on the 3.7-GiB WSL runner. PostgreSQL then
performed crash recovery successfully.

The probe was not repeated. OpenTelemetry and Kubernetes single-value rows are
recorded as unsupported. This is both a client/materialization failure and a
confirmation that Kubernetes exceeds PostgreSQL's approximate one-GiB field
limit. The failure does not affect the chunked results.

## Correctness Results

| Gate | Result |
|---|---|
| Exact byte and SHA-256 round trips | PASS |
| Duplicate stage is idempotent | PASS |
| Rollback exposes no scan/publication | PASS |
| Publication is atomically visible | PASS |
| Cross-scan dependency is rejected | PASS |
| Wrong projection digest is rejected | PASS |
| Referenced payload is protected | PASS |
| Unreferenced payload is collected | PASS |
| Metadata query avoids payload chunks | PASS |
| Backup/restore verifies every payload | PASS |
| Truncation or implicit reserialization | none observed |

The independent reader saw no publication before commit and the complete
publication after commit. Ten publication runs had p50 5.32 ms, p95/max
130.15 ms, no partial visibility, and complete visibility in every run. The
publication p95 gate of 500 ms passed.

## One-Million-Dependency Metadata Scale

The audited harness added exactly 1,000,000 synthetic same-scan dependency
rows after the payload run. Including prior correctness fixtures, the relation
contained 1,000,050 rows and occupied 157 MiB; the disposable database occupied
341 MiB after cleanup of representation measurements.

The approved metadata-only repository/scan artifact query was executed ten
times with `EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON)`:

- p50: 0.103 ms;
- p95/max: 1.643 ms;
- shared payload-chunk blocks: zero;
- temporary blocks: zero;
- p95 gate: less than 100 ms — PASS.

The query touched publication and artifact-envelope metadata only. The large
dependency relation did not force a dependency or payload scan.

## Backup and Restore

The logical backup contained all six content-addressed core payloads and their
publication/dependency graph:

- dump size: 150,187,276 bytes;
- dump time: 33,990.94 ms;
- isolated restore time: 30,347.36 ms;
- exact payloads digest-verified after restore: 6/6.

## Operational Payload Limit

The largest released exact payload is 1,556,379,091 bytes. Applying the
approved formula:

1. double for headroom: 3,112,758,182 bytes;
2. round upward to a power of two: 4,294,967,296 bytes;
3. compare with the schema ceiling: four GiB is below eight GiB.

The initial operational limit is therefore **4 GiB**. The adapter must reject a
larger envelope before staging. It must not truncate, silently change chunk
size, or partially publish.

## Defects and Resolutions

1. The first smoke fixture contained one publication while the cross-scan test
   correctly required two. The smoke run was corrected to two publications.
2. The initial full run attempted a 635,557,163-byte single-value insert and
   triggered the documented OOM. The harness now records this measured failure
   and does not repeat that destructive candidate for equal or larger payloads.
3. The first payload run did not populate the one-million-dependency metadata
   condition. A reproducible SQL scale fixture was added and the missing gate
   was measured separately before this report was written.
4. The raw machine report retained the pre-benchmark one-MiB selection field.
   This report does not silently rewrite that evidence; it records the measured
   four-MiB recommendation and requires ADR/schema acceptance to change it.

## Residual Risks

- Results describe one 3.7-GiB WSL runner and one NVMe-backed ext4 virtual
  disk; production hardware requires its own capacity baseline.
- pglz compressibility materially reduced storage and WAL. Add an
  incompressible-codec fixture if a future artifact codec changes this profile.
- The benchmark used one payload writer. Multi-writer ingestion contention is
  a Phase 3.4 adapter concern and requires a separate approved test.
- Backup/restore was logical and local. Production recovery-time and recovery-
  point objectives still require deployment-specific exercises.
- The four-GiB operational limit is a rejection boundary, not proof that a
  four-GiB fixture has already been generated and round-tripped.

## Engineering Acceptance

Engineering accepted the Phase 3.2 benchmark evidence on 2026-07-24. ADR 0011
and the physical schema now use fixed four-MiB chunks, a four-GiB operational
limit, and an eight-GiB schema ceiling.

Phase 3.3 — Migration Framework is authorized. Database credentials,
production connections, API work, UI work, and the Go persistence adapter
remain unauthorized by this acceptance.
