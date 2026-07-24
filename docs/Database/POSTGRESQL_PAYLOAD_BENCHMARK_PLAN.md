# PostgreSQL Payload Benchmark Plan

## Status

- Phase: 3.2 — PostgreSQL Schema Specification
- Status: Executed and accepted
- Date: 2026-07-23
- Authorized: 2026-07-24
- Database installation and connection: disposable local benchmark instance only
- Credentials: not required

The accepted evidence and four-MiB schema refinement are recorded in
[POSTGRESQL_PAYLOAD_BENCHMARK_REPORT.md](../Validation/POSTGRESQL_PAYLOAD_BENCHMARK_REPORT.md).
Phase 3.3 migration implementation is authorized.

## Purpose

Measure whether the proposed PostgreSQL 18 schema can stage, publish, read,
verify, retain, back up, and restore released immutable artifacts with bounded
memory and acceptable amplification. The results freeze the operational payload
limit and release performance gates; they do not change artifact semantics.

## Authorization Gate

Engineering authorized this disposable isolated PostgreSQL spike on
2026-07-24. The spike must not use user, shared, staging, or production
credentials or data. Its transient benchmark DDL and harness are evidence, not
Phase 3.3 migration implementation.

## Reference Environment

Record before every run:

- host OS, CPU, memory, filesystem, and storage medium;
- container/VM/WSL boundary when present;
- PostgreSQL exact version and server configuration;
- client runtime and exact commit;
- cold/warm filesystem state;
- database encoding, collation, compression, and WAL settings;
- fixture repository commit and artifact SHA-256;
- worker and connection counts.

These environment details are mandatory report evidence. A measurement missing
the exact CPU, RAM, storage type, OS, PostgreSQL configuration, and client
configuration cannot set or revise a performance or payload-size gate.

Use PostgreSQL 18 at its then-current minor release. Use only core PostgreSQL
features. The first acceptable runner may be a disposable Ubuntu environment;
the result is not tied to the user's normal credentials.

## Required Released Fixtures

For each fixture, preserve the exact serialized artifact bytes and digests:

1. this project's RIE artifact set;
2. this project's `GoLanguageInventory 1.0.0`;
3. this project's `GoPackageIdentityInventory 1.0.0`;
4. this project's `GoSemanticInventory 1.0.0`;
5. pinned OpenTelemetry semantic inventory;
6. pinned Kubernetes semantic inventory at the one-million relationship limit.

The fixture manifest records repository URL, immutable commit, artifact name,
artifact version, codec version, exact byte count, and SHA-256. No live network
resolution occurs inside a measured run.

## Representations Under Test

Compare, without changing the authoritative bytes:

| Candidate | Purpose |
|---|---|
| one `bytea` value | reference for small/medium payload latency and TOAST behavior |
| ordered 256-KiB chunks | low-memory/high-row-count comparison |
| ordered 1-MiB chunks | proposed schema representation |
| ordered 4-MiB chunks | lower-row-count comparison |

PostgreSQL Large Objects are not a release candidate. They may be measured only
if every chunked candidate fails an approved gate and a new architecture review
is opened.

## Operations

For every fixture/candidate combination measure:

1. first content-addressed stage;
2. duplicate idempotent stage;
3. final atomic publication of the full artifact set;
4. exact sequential read with SHA-256 verification;
5. metadata-only list/read without fetching payload bytes;
6. bounded projection write and query;
7. concurrent metadata readers during an unrelated publication;
8. rollback immediately before publication commit;
9. unreferenced-payload garbage collection;
10. logical backup, isolated restore, and full digest verification.

Run each non-destructive case at least ten times after warm-up. Record cold and
warm results separately. Destructive recovery cases run at least three times.

## Metrics

- exact payload and total database bytes;
- main-table, index, and TOAST bytes per relation;
- WAL bytes per first stage, duplicate stage, and publication;
- stage/read throughput;
- p50, p95, maximum, and standard deviation for latency;
- client process peak resident memory and allocation profile;
- server process and shared-buffer observations;
- rows/chunks written and read;
- transaction duration and lock-wait duration;
- metadata query plans and buffer reads;
- backup size/time and restore size/time;
- digest failures, retries, errors, and orphan rows.

Network, serialization, filesystem, and database time are recorded separately
where the harness can distinguish them.

## Correctness Gates

All are mandatory:

- exact restored bytes equal the fixture bytes;
- all staged/read/restored SHA-256 values equal the fixture digest;
- duplicate staging creates no duplicate payload/chunk set;
- rollback exposes no succeeded scan or partial artifact set;
- cross-scan dependency and wrong-digest projection writes fail;
- metadata-only queries do not fetch payload chunks;
- garbage collection never deletes a referenced payload;
- backup/restore preserves the publication and dependency graph;
- no truncation, implicit reserialization, or artifact mutation occurs.

## Candidate Performance Gates

These are review candidates, not accepted measurements:

- payload streaming adds no more than 128 MiB peak client memory above the
  already-materialized input or output buffer; the final adapter should avoid
  materializing the entire payload merely for database transport;
- metadata-only list/get p95 remains below 100 ms on the reference runner with
  one million stored dependency relationships;
- final publication transaction p95 remains below 500 ms when payload staging
  is excluded;
- duplicate stage reads/verifies metadata without rewriting chunk bytes;
- first-stage WAL amplification remains below 2.5 times exact input bytes;
- exact warm read throughput is not less than 100 MiB/s on the reference local
  SSD runner;
- exact warm stage throughput is not less than 50 MiB/s on that runner;
- concurrent readers observe either the prior publication or the complete new
  publication, never a partial state.

If a candidate misses a target, record the evidence and revise the target or
design through engineering review. Do not quietly tune the acceptance gate to
the observed result.

## Operational Payload Limit

After measuring all released fixtures:

1. identify the largest exact payload;
2. multiply its byte size by two for release headroom;
3. round upward to the next power of two;
4. use 64 MiB if that result is smaller;
5. fail the design review if the result exceeds the eight-GiB schema ceiling;
6. freeze the result as the initial operational maximum;
7. record the fixture digest and measurement that justified it.

The application rejects a larger payload before staging. It never truncates or
silently changes chunk size. Future increases require benchmark evidence.

## Required Report

Execution produces `POSTGRESQL_PAYLOAD_BENCHMARK_REPORT.md` containing:

- approval reference and environment identity;
- exact CPU, RAM, storage type, OS, PostgreSQL settings, and client settings;
- immutable fixture manifest;
- raw and summarized measurements;
- correctness results;
- representation comparison;
- selected chunk size and operational payload limit;
- accepted/revised performance gates;
- query plans for approved indexes;
- defects and resolutions;
- residual risks;
- explicit recommendation to accept or revise Phase 3.2.

The report contains no credentials, connection strings, absolute personal
paths, source code, or artifact payload bytes.
