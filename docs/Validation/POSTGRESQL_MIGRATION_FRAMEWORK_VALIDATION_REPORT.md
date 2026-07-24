# PostgreSQL Migration Framework Validation Report

## Status

- Phase: 3.3 — Migration Framework
- Evidence status: accepted
- Engineering acceptance: accepted on 2026-07-24
- Validation date: 2026-07-24
- Tool candidate: Atlas Community CLI v1.2.3
- Database: PostgreSQL 18.4
- ADR 0012: Accepted
- Phase 3.4: authorized

## Scope

This report validates only the migration framework and the SQL representation
of the frozen Phase 3.2 physical schema. It does not validate an artifact
storage adapter, application connection settings, APIs, UI, production
deployment, or artifact ingestion.

## Implementation Delivered

- seven ordered forward-only SQL migrations;
- a committed Atlas `atlas.sum` checksum manifest;
- strict linear migration history;
- one PostgreSQL transaction per migration file;
- advisory-lock serialization for concurrent apply attempts;
- Atlas schema-revision tracking;
- PostgreSQL 18 schema with 11 platform tables;
- non-login `platform_owner` plus eight non-login capability roles;
- explicit runtime grants and `PUBLIC` revocation;
- five-second DDL lock timeout and five-minute statement timeout;
- forward-repair, rollback, lock-duration, and backfill policies;
- automated disposable-cluster validation.

## Reference Environment

| Item | Value |
|---|---|
| Host OS | Windows with Ubuntu 24.04 WSL2 |
| Linux kernel | 6.18.33.2-microsoft-standard-WSL2 |
| CPU | Intel Core i5-12450H, 12 logical CPUs available to WSL |
| WSL memory | 3.7 GiB total, 1.0 GiB swap |
| Workspace storage | D: drive through `/mnt/d`, 342 GiB total |
| PostgreSQL | 18.4 (Ubuntu package) |
| Atlas | Community v1.2.3 |
| Required PostgreSQL extensions | none |

The automated suite initialized a fresh PostgreSQL cluster under a unique
temporary directory with local socket access only and host authentication
disabled. It did not use the installed default cluster or any password.

## Validation Results

| Gate | Result | Evidence |
|---|---|---|
| Committed checksum manifest | PASS | `atlas migrate validate` returned success |
| Changed migration bytes | PASS | intentional edit was rejected before SQL execution |
| Empty-cluster role bootstrap | PASS | all nine roles were created from no prior role state |
| Empty-database install | PASS | seven revisions and 73 SQL statements applied |
| Least-privilege migration | PASS | versions 2–7 applied by a non-superuser login granted only `platform_migrator` |
| Schema assertions | PASS | 11 tables, frozen constraints, publication FKs, 4-MiB chunk check |
| Object ownership | PASS | schema, tables, indexes, and identity sequence owned by `platform_owner` |
| Partial upgrade | PASS | first three revisions followed by four pending revisions |
| Repeat apply | PASS | completed as a deterministic no-op |
| Newer database compatibility | PASS | revision 7 database rejected by version-6 directory |
| Failed migration rollback | PASS | test table absent and failing revision unrecorded |
| Concurrent apply | PASS | two processes serialized; exactly eight temporary revisions |
| Runtime DDL denial | PASS | `platform_ingestor` could not create a table |
| Exact-payload isolation | PASS | query reader denied; artifact reader allowed |
| Audit isolation | PASS | audit writer insert-only; auditor read-only without payload access |
| Elevated role audit | PASS | capability roles had no login, superuser, createdb, or createrole |
| Cleanup | PASS | temporary server stopped and data directory removed |
| Full backend regression | PASS | `go test ./...` |
| Static analysis | PASS | `go vet ./...` |
| Targeted race tests | PASS | package-identity and semantic packages |
| Full backend race test | PASS | `go test -race ./...`; zero reported races |
| Documentation links | PASS | all local links in changed documents resolve |
| Secret scan | PASS | no credential value, private key, or authenticated URL found |

## Measured Metrics

The accepted candidate run reported:

```text
atlas_version=v1.2.3
postgres_version=18.4 (Ubuntu 18.4-1.pgdg24.04+1)
migration_files=7
schema_tables=11
schema_constraints=185
schema_indexes=34
capability_roles=9
empty_install_ms=1460
noop_apply_ms=482
```

The complete validation suite, including fresh-cluster initialization,
checksum-negative tests, four databases, a one-second lock probe, assertions,
and cleanup, completed in approximately 15.6 seconds.

## Defects Found and Corrected

### Owner-scoped revision write

Initial apply created the first migration, then correctly failed when Atlas
attempted to record progress while `SET LOCAL ROLE platform_owner` was active.
The non-login owner could not access Atlas's dedicated revision schema.

Correction: migration 001 grants only the non-login `platform_owner` bounded
`USAGE` plus revision-table DML/sequence access. Runtime roles receive no
revision-schema access. Owner-scoped application DDL and Atlas's transactional
revision writes then passed together.

### PUBLIC privilege assertion

The first test implementation treated `PUBLIC` as a normal PostgreSQL role.
PostgreSQL represents it as ACL grantee zero. The test was corrected to inspect
the exploded schema ACL. This was a test-harness defect, not a schema defect.

## Checksum and Immutability Evidence

The test copied the released migration directory, changed one byte sequence in
the first SQL file, retained the committed `atlas.sum`, and required
`atlas migrate validate` to fail. No database was contacted for this negative
gate. A released migration must therefore be corrected by appending a new
version, never by regenerating the checksum after an unreviewed edit.

## Upgrade and Compatibility Evidence

The suite installed versions `202607240001` through `202607240003`, verified
exactly three revision rows, then applied versions `202607240004` through
`202607240007`. The resulting database passed the same schema assertions as a
single empty installation. Reapplying the complete directory produced no
additional revision.

## Failure and Repair Evidence

A temporary eighth migration created a table and then divided by zero. Atlas's
per-file PostgreSQL transaction rolled back both the table and revision row.
This validates automatic rollback for a failed, still-running migration.

An already successful released migration is not edited or destructively
rolled back. Its repair path is a new migration after engineering review, as
defined by ADR 0012.

## Concurrency Evidence

Two Atlas processes targeted the same empty database and migration directory.
A temporary eighth migration held the advisory lock for one second. Both
processes completed successfully, the lock-probe table existed once, and the
revision ledger contained exactly eight versions. No duplicate or partial
revision was observed.

## Security and Cleanup

- no password, application credential, `.env`, or connection file was used;
- the migration wrapper rejects password-bearing URLs during Phase 3.3;
- only migration 001 used the isolated-cluster administrator; migrations
  002–007 used an ephemeral non-superuser login with `platform_migrator`;
- the suite listened only on a temporary Unix socket;
- all test databases existed only inside the temporary cluster;
- the server was stopped before recursive temporary-directory removal;
- an earlier manual test database and its nine no-login roles were removed
  from the installed Ubuntu cluster after exact-target verification;
- the installed Ubuntu cluster contains zero `aegis_phase33_%` databases and
  zero `platform_%` roles from this validation;
- Windows PostgreSQL was not accessed.
- the first Windows race command lacked the MSYS2 toolchain on `PATH`; after
  adding `C:\msys64\ucrt64\bin` and enabling CGO, targeted and full race suites
  passed. This was an invocation-environment correction, not a code defect.

## Known Boundaries

- Atlas Community v1.2.3 is pinned; another Atlas version requires repeat
  migration validation.
- Only PostgreSQL 18.4 was validated.
- The suite validates schema migrations, not artifact round trips; those belong
  to Phase 3.4.
- Deployment login binding and persistent connection configuration belong to
  Phase 3.5.
- No non-transactional DDL or production backfill is authorized.
- No production, shared, or personal database was tested.

## Exit-Gate Assessment

The Phase 3.3 local exit criteria are satisfied:

- disposable empty installation: passed;
- supported partial upgrade: passed;
- concurrent execution: passed;
- least privilege and ownership: passed;
- checksum immutability: passed;
- rollback/failure recovery: passed;
- schema revision tracking: passed;
- documentation and policy: complete.

Engineering accepted the evidence, ADR 0012, and the Phase 3.3 implementation
on 2026-07-24. Phase 3.4 storage-adapter implementation is authorized.
Production credentials, environment configuration, APIs, and UI remain gated.
