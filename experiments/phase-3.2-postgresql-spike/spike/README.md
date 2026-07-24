# Phase 3.2 PostgreSQL Spike

This package is the accepted isolated evidence harness for the PostgreSQL
physical schema. It is not a production persistence adapter or migration
framework.

It performs two bounded tasks:

1. Generate exact JSON fixture payloads from the released RIE/Go artifacts.
2. Compare single-`bytea` and 256-KiB, 1-MiB, and 4-MiB chunk storage in a
   disposable PostgreSQL 18 database.

The benchmark verifies exact SHA-256 round trips, idempotent staging, atomic
publication, rollback, same-scan dependencies, projection provenance,
referenced-payload protection, unreferenced garbage collection, metadata-only
query behavior, and backup/restore.

After the payload run, `sql/metadata_scale.sql` adds one million synthetic
dependency rows to the disposable schema. The approved metadata query is then
measured ten times to prove that it does not fetch payload chunks or degrade as
dependency evidence grows.

The database name must begin with `platform_bench_`. The harness refuses any
other target. Peer-authenticated local execution requires no credential value.

All generated fixture payloads, database files, dumps, and raw measurements are
local evidence. They must not be committed. Only an accepted sanitized report
may enter project history.
