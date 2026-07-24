# PostgreSQL Migration Framework

## Status

- Phase: 3.3 — Migration Framework
- Status: Accepted and frozen
- Date: 2026-07-24
- Tool: Atlas Community CLI v1.2.3
- ADR: accepted ADR 0012
- Accepted: 2026-07-24
- Authorized next scope: Phase 3.4 storage adapter only
- Storage adapter, APIs, UI, and production connections: unauthorized

## Purpose

Install and evolve the accepted PostgreSQL physical schema through explicit,
ordered, immutable, checksum-verified migrations without coupling any
intelligence engine to PostgreSQL.

## Responsibilities

- validate migration-directory integrity before execution;
- apply pending migrations in strict linear order;
- record schema revisions;
- serialize concurrent migration attempts;
- apply each migration atomically;
- create the accepted ownership and least-privilege role structure;
- expose deterministic migration status for operators and CI;
- support reproducible empty-install and upgrade validation.

## Non-Responsibilities

- artifact persistence or retrieval;
- database connection configuration for application runtime;
- engine startup migration;
- API or UI behavior;
- data projection or semantic interpretation;
- production credential management;
- arbitrary destructive downgrade automation;
- unbounded data backfills.

## Inputs

1. The committed migration directory and `atlas.sum` file.
2. An explicitly supplied PostgreSQL 18 connection URL for an authorized
   disposable, development, staging, or deployment database.
3. A migration principal able to create the initial non-login capability roles
   for migration 001. Later migrations use a non-superuser deployment login
   granted only the `platform_migrator` capability, which may assume the
   non-login `platform_owner` during approved DDL.

No connection URL, password, or environment file belongs in Git.

## Outputs

- the `platform` schema at a known migration version;
- Atlas's database revision ledger;
- non-login ownership and runtime capability roles;
- deterministic status and validation results;
- no artifact data.

## Migration Layout

```text
backend/persistence/postgres/migrations/
    202607240001_bootstrap_roles_and_schema.sql
    202607240002_create_repositories_and_scans.sql
    202607240003_create_artifact_payloads.sql
    202607240004_create_artifact_envelopes.sql
    202607240005_create_query_projections.sql
    202607240006_create_audit_and_indexes.sql
    202607240007_apply_runtime_privileges.sql
    atlas.sum
```

One migration owns one coherent dependency layer. Later migrations may depend
only on earlier versions.

## Public Operational Commands

Commands are run from Ubuntu/WSL or an equivalent supported environment. The
connection URL is represented here only by a placeholder.

```text
bash backend/persistence/postgres/migrate.sh validate
bash backend/persistence/postgres/migrate.sh status <authorized-url>
bash backend/persistence/postgres/migrate.sh apply <authorized-url>
```

The wrapper validates migration checksums and rejects incomplete, unknown, or
newer database revisions before status/apply. Phase 3.3 also rejects a URL with
an embedded password; secure deployment-secret delivery remains a Phase 3.5
responsibility. The URL is supplied by the operator or test harness and must
not be printed into logs. Atlas v1.2.3 is the accepted tool version candidate;
a tool upgrade requires repeat validation.

## Error Handling

- checksum mismatch: stop before SQL execution;
- out-of-order history: stop and require engineering review;
- lock timeout: roll back the file and retry only under the deployment policy;
- SQL/constraint failure: roll back the file and leave no successful revision;
- newer database revision than supported directory: refuse operation;
- missing ownership privilege: stop; never fall back to superuser runtime;
- partially applied non-transactional migration: unsupported in v1.

Migration errors may include version and filename but must not include a
credential-bearing URL or artifact bytes.

## Logging and Audit

Atlas records migration version, description, execution time, checksum, and
result in its revision ledger. Deployment logs record the tool version,
database identity by a safe opaque label, starting/ending schema version,
duration, and result. Passwords and full URLs are redacted.

## Configuration

- execution order: linear;
- transaction mode: one transaction per file;
- advisory lock: enabled;
- DDL lock timeout: five seconds;
- DDL statement timeout: five minutes;
- migration directory integrity: required;
- environment-variable substitution in SQL: prohibited;
- automatic application-startup migration: prohibited.

## Testing Strategy

The disposable PostgreSQL suite validates:

1. migration-directory checksum;
2. empty installation;
3. partial installation followed by upgrade;
4. repeat apply/no-op behavior;
5. concurrent migration serialization;
6. transactional rollback on injected failure;
7. migration-byte tamper rejection;
8. table, key, foreign-key, check, and index inventory;
9. object ownership;
10. runtime DDL denial and required privilege grants;
11. PostgreSQL 18 baseline and no required extensions.

## Performance Targets

- empty schema installation: under 30 seconds on the reference local runner;
- no-op status/apply: under five seconds;
- concurrent apply: no corruption, duplicate revision, or partial schema;
- lock waits are bounded by the migration/session policy;
- migration runner memory remains independent of artifact payload size because
  Phase 3.3 stores no artifact data.

Measured values belong in the Phase 3.3 validation report.

## Security and Privacy

- `platform_owner` is non-login and owns every application object;
- `platform_owner` has bounded revision-ledger DML only because Atlas records
  progress inside owner-scoped migration transactions;
- runtime roles are non-login capability roles and own no objects;
- `PUBLIC` has no privileges on the `platform` schema;
- migration credentials are not committed, logged, or accepted as engine input;
- migration SQL contains no artifact or repository source data;
- exact payload bytes are never rewritten by migration tooling.

## Upgrade, Rollback, and Forward Repair

Released files are immutable. An upgrade appends a version. Failed
transactional files roll back automatically. A defect discovered after a
successful release is corrected by a new forward migration. Destructive
rollback is restricted to disposable databases or a separately approved
incident plan.

## Future Extensions

- CI installation and migration checks;
- in Phase 3.5, CI verification of the pinned Atlas version, committed
  migration checksum, and expected database schema version before deployment;
- a Phase 3.5 one-command disposable environment;
- separately approved online-index or bounded-backfill procedures;
- support for a later PostgreSQL major version after conformance validation.
