# ADR 0012: Atlas Versioned Migration Framework

- Status: Accepted
- Date: 2026-07-24
- Accepted: 2026-07-24
- Prerequisites: accepted ADR 0010 and ADR 0011
- Decision owners: Phase 3.3 migration-framework review
- Implementation authorization: Phase 3.3 complete; Phase 3.4 authorized

## Context

The accepted PostgreSQL persistence contract requires ordered, immutable,
checksum-verified migrations; schema-version tracking; serialized concurrent
execution; transactional failure behavior; and reproducible installation and
upgrade validation. The solution must not introduce a PostgreSQL dependency
into any intelligence engine or begin the Phase 3.4 storage adapter.

## Decision

1. Use Atlas Community CLI v1.2.3 for PostgreSQL versioned migrations.
2. Commit plain SQL migration files and the generated `atlas.sum` integrity
   manifest. Do not commit connection URLs or credentials.
3. Use Atlas's default linear execution order. Missing or out-of-order
   migrations fail rather than being silently skipped.
   A project wrapper additionally rejects incomplete, unknown, or newer
   database revisions before Atlas status or apply.
4. Apply each migration file in its own PostgreSQL transaction. A migration
   must not opt out of transactional execution without a later accepted ADR.
5. Use Atlas's PostgreSQL advisory lock to serialize concurrent migration
   attempts.
6. Store Atlas's revision ledger in its dedicated revisions schema. It is
   migration metadata, not an application relation or artifact fact.
7. Use sequential, timestamp-qualified migration versions. Released migration
   files and their checksums are immutable.
8. Prefer roll-forward repair. A released or applied migration is never edited
   or silently rolled back. A correction is a new migration.
9. Keep `platform_owner` as the non-login owner of the `platform` schema and
   application objects. Runtime roles own no schema object and receive only
   explicit privileges.
   Atlas records statement progress within each owner-scoped transaction, so
   this non-login role receives bounded DML access to the Atlas revision table;
   no runtime role receives revision-schema access.
   Migration 001 is the explicit cluster bootstrap requiring role-creation
   authority. All later migrations must pass under a non-superuser principal
   granted only `platform_migrator`.
10. Keep all database URLs ephemeral and supplied directly by an authorized
    deployment/test process. Environment-file support remains deferred to
    Phase 3.5.

## Why Atlas

Atlas provides the required migration-directory checksum (`atlas.sum`), linear
history validation, a database revision ledger, PostgreSQL advisory locking,
transaction-per-file execution, status inspection, and dry runs in one
versioned tool. Goose and golang-migrate provide ordered execution but would
require a second project-specific checksum and history-integrity layer for the
accepted immutability gate. A custom runner would duplicate mature migration
coordination before the persistence adapter exists.

## Rollback and Forward Repair

Production rollback is not a routine `down` operation. PostgreSQL DDL is
transactional, so a failed migration file rolls back with no successful
revision entry. If a released migration has already succeeded, the remedy is a
new reviewed migration that restores compatibility or repairs the schema.

Destructive reversal is allowed only for a disposable database or under a
separate incident plan with backup verification, impact review, and explicit
approval. Exact artifact payload bytes are never rewritten as part of schema
repair.

## Lock and Backfill Policy

- DDL migrations set a five-second PostgreSQL lock timeout and a five-minute
  statement timeout unless measured evidence approves another bound.
- Schema changes that may rewrite a populated relation must be split into
  additive schema, bounded resumable backfill, verification, and later
  constraint-enforcement migrations.
- A backfill must be restartable, observable, and performed by a separately
  authorized operational process. Atlas migration transactions do not contain
  unbounded data backfills.
- Non-transactional DDL, including concurrent index creation, requires a
  separate migration and accepted exception before use.

## Consequences

### Positive

- migration bytes are independently checksum-verified before execution;
- concurrent deployment attempts serialize;
- schema history is explicit and inspectable;
- failed PostgreSQL DDL is atomically rolled back;
- the migration tool remains outside intelligence-engine packages;
- connections and secrets remain deployment concerns.

### Costs

- operators and CI must install the pinned Atlas CLI;
- the Atlas revision schema is an additional operational schema;
- roll-forward repair may require a compatibility migration rather than a
  convenient destructive rollback;
- Atlas version upgrades require their own compatibility validation.

## Alternatives Rejected

### Goose

Rejected for v1 because its normal revision table does not by itself satisfy
the accepted immutable file-checksum contract.

### golang-migrate

Rejected for v1 for the same reason: ordered versions and dirty-state tracking
would still need a project-owned checksum verifier.

### Custom Go Runner

Rejected because advisory locking, transaction orchestration, revision state,
and SQL-file integrity are mature migration-tool responsibilities and are not
platform intelligence.

### Implicit Application Startup Migration

Rejected. Migration is an explicit operational command and never an engine or
application startup side effect.

## Accepted Validation Evidence

The Phase 3.3 validation report proved:

- checksum verification rejects changed migration bytes;
- empty PostgreSQL 18 installation succeeds;
- a partial installation upgrades to the latest version;
- two concurrent apply attempts serialize safely;
- a failed transactional migration leaves neither schema residue nor a
  successful revision;
- runtime roles cannot perform DDL or assume ownership;
- all objects have the intended owner and privileges;
- migration status and repeat application are deterministic.

Engineering accepted this ADR and the Phase 3.3 evidence on 2026-07-24.
Phase 3.4 storage-adapter implementation is authorized. Production
credentials, environment configuration, APIs, and UI remain unauthorized.

## Primary References

- [Atlas migration directory integrity](https://atlasgo.io/concepts/migration-directory-integrity)
- [Atlas versioned migration apply, revisions, locks, and transactions](https://atlasgo.io/versioned/apply)
- [Atlas migration troubleshooting and transactional failure behavior](https://atlasgo.io/versioned/troubleshoot)
