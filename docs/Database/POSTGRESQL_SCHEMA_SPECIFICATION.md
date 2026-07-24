# PostgreSQL Schema Specification

## Status

- Phase: 3.2 — PostgreSQL Schema Specification
- Status: Accepted and frozen for Phase 3.3 migration implementation
- Date: 2026-07-23
- Accepted: 2026-07-24
- Architecture prerequisite: accepted ADR 0010
- Benchmark accepted: 2026-07-24
- Implementation authorization: Phase 3.3 migration framework only
- Production/shared connections, migration implementation, credentials,
  environment variables, and Go persistence code remain unauthorized
- Accepted benchmark result: fixed four-MiB chunks and a four-GiB operational
  artifact limit; the absolute schema ceiling remains eight GiB

## Purpose

Define the physical PostgreSQL contract that can durably preserve immutable
artifact bytes, scan history, dependency evidence, bounded query projections,
and audit events without making an intelligence engine depend on a database.

This document specifies relations, columns, types, nullability, keys,
constraints, indexes, lifecycle rules, roles, retention order, and scale gates.
It is a schema specification, not a migration.

## Responsibility

The schema stores completed platform artifacts and their lifecycle evidence. It
does not interpret Go, reconstruct semantic facts, execute engines, resolve
dependencies, or redefine released artifact contracts.

## Out of Scope

- executable SQL or migration files;
- database installation or a live connection;
- credentials or environment-variable names;
- a PostgreSQL driver or Go storage package;
- REST, gRPC, UI, search, vector, or LLM integration;
- language-specific source-of-truth tables;
- source code blobs;
- multi-tenant row-level security;
- production backup credentials or deployment topology.

## PostgreSQL Baseline

- Supported major version: PostgreSQL 18.
- Required deployment level: the current PostgreSQL 18 minor release.
- Recorded current minor on 2026-07-23: 18.4.
- Required extensions: none.
- Database encoding: UTF-8.
- Machine-key text uses deterministic `C` collation where database ordering is
  required. Artifact ordering still comes from the artifact contract, never
  from an unspecified database order.
- Application and migration sessions operate with UTC as their display time
  zone. All durable instants use `timestamp with time zone`.

PostgreSQL supports a major version for five years and recommends the current
minor release. PostgreSQL 18 is supported through November 2030. Selecting one
major version keeps the initial compatibility matrix bounded. Support for a
later major version requires migration and conformance evidence; an untested
major version is not implicitly supported.

UUIDs and SHA-256 digests are produced and verified by the application. This
avoids requiring `uuid-ossp`, `pgcrypto`, or another extension. Database
constraints still verify UUID relationships, digest lengths, declared sizes,
and lifecycle structure.

## Schema Namespace and Ownership

All application relations live in one non-public schema named `platform`.
`public` is not used for application tables. The migration-owner role owns the
schema and every object; runtime roles own none of them.

One namespace keeps the first migration set understandable. Projection and
audit access are separated by table privileges rather than additional schemas.
Additional schemas require an accepted ADR.

## Common Physical Conventions

| Concern | Physical rule |
|---|---|
| Durable identity | application-generated `uuid` |
| Digest | raw 32-byte SHA-256 in `bytea` |
| Size/count | non-negative `bigint` unless explicitly bounded to `integer` |
| Time | `timestamp with time zone`, supplied by the persistence boundary |
| Machine names/versions | non-empty `text` with an octet-length bound and `C` collation |
| State | `text` plus a closed check constraint, not a PostgreSQL enum |
| Flexible bounded details | `jsonb` only when the data is a rebuildable projection or audit detail |
| Exact artifact value | ordered binary chunks, never `jsonb` |
| Mutation | only repository and scan lifecycle columns may change |

Database enums are intentionally avoided because adding or removing states
would couple operational migrations to a PostgreSQL enum type. Closed text
checks preserve integrity while remaining migration-friendly.

## Relation Graph

```text
repositories
    │
    ├──< repository_scans
    │        │
    │        ├── 0..1 scan_publications
    │        └──< artifact_envelopes >── artifact_payloads
    │                    │                      │
    │                    │                      └──< artifact_payload_chunks
    │                    │
    │                    ├──< artifact_dependencies >── artifact_envelopes
    │                    └──< artifact_projections >── artifact_payloads
    │                              │
    │                              ├──< projected_diagnostics
    │                              └──< projected_statistics
    │
    └── current_scan_id ──> scan_publications

audit_events (append-only; durable subject identifiers intentionally outlive
              purged operational rows)
```

The following requested relationships are enforced explicitly:

1. Repository → Scan by a foreign key.
2. Scan → Artifact Envelope by a foreign key.
3. Artifact Envelope → Payload by a foreign key.
4. Artifact Envelope → Dependency Edges through composite same-scan foreign
   keys for both endpoints.
5. Projection → Payload Digest by both a payload foreign key and an
   `(artifact_id, payload_digest)` foreign key to the exact envelope.

## Table: `repositories`

One row is a durable logical repository identity. It contains no source code or
absolute workstation path.

| Column | Type | Null | Rule |
|---|---|---:|---|
| `repository_id` | `uuid` | no | primary key; application generated |
| `security_scope_id` | `uuid` | no | opaque authorization scope owned by the application |
| `idempotency_key` | `text` | no | 1–256 UTF-8 octets |
| `registration_digest` | `bytea` | no | exactly 32 octets |
| `display_name` | `text` | no | 1–512 UTF-8 octets |
| `source_kind` | `text` | no | 1–64 machine-key octets |
| `source_fingerprint_scheme` | `text` | no | versioned normalization/hash scheme, 1–128 octets |
| `source_fingerprint` | `bytea` | no | exactly 32 octets; no raw local path required |
| `lifecycle_state` | `text` | no | `active`, `archived`, or `purge_pending` |
| `current_scan_id` | `uuid` | yes | must identify a publication for this repository |
| `created_at` | `timestamptz` | no | immutable |
| `updated_at` | `timestamptz` | no | not earlier than `created_at` |
| `archived_at` | `timestamptz` | yes | present only for archived/purge-pending state |

Keys and constraints:

- primary key: `repository_id`;
- unique: `(security_scope_id, idempotency_key)`;
- unique: `(security_scope_id, source_kind, source_fingerprint_scheme,
  source_fingerprint)`;
- check: every digest is 32 octets;
- check: state and `archived_at` agree;
- composite foreign key `(repository_id, current_scan_id)` references
  `(repository_id, scan_id)` in `scan_publications` and is deferrable;
- no delete cascade.

The source fingerprint is an application-defined digest of a normalized source
identity. The normalization algorithm must be versioned by the future port. A
raw credential-bearing URL or absolute local path is not stored here.

## Table: `repository_scans`

One row is one analysis attempt. Terminal rows never return to a non-terminal
state; rescanning creates a new row.

| Column | Type | Null | Rule |
|---|---|---:|---|
| `scan_id` | `uuid` | no | primary key; application generated |
| `repository_id` | `uuid` | no | foreign key to `repositories` |
| `idempotency_key` | `text` | no | 1–256 octets within the repository |
| `request_digest` | `bytea` | no | exactly 32 octets |
| `analysis_profile_digest` | `bytea` | no | exactly 32 octets |
| `source_revision` | `text` | yes | bounded opaque revision, at most 512 octets |
| `lifecycle_state` | `text` | no | `requested`, `running`, `succeeded`, `failed`, or `cancelled` |
| `requested_at` | `timestamptz` | no | immutable |
| `started_at` | `timestamptz` | yes | required for running/succeeded |
| `finished_at` | `timestamptz` | yes | required for every terminal state |
| `published_at` | `timestamptz` | yes | required only for succeeded |
| `failure_code` | `text` | yes | 1–128 machine-key octets for failed/cancelled only |
| `failure_summary` | `text` | yes | safe bounded text, at most 4096 octets |

Keys and constraints:

- primary key: `scan_id`;
- unique: `(repository_id, scan_id)` for ownership-preserving references;
- unique: `(repository_id, scan_id, lifecycle_state)` for a publication proof;
- unique: `(repository_id, idempotency_key)`;
- foreign key `repository_id` references `repositories`, `ON DELETE RESTRICT`;
- check: all digests are 32 octets;
- check: timestamp order is monotonic when values are present;
- check: requested has no start, finish, publication, or failure fields;
- check: running has a start and no finish/publication/failure fields;
- check: succeeded has start, finish, and publication fields and no failure;
- check: failed/cancelled has a finish, no publication, and a failure code;
- lifecycle updates use compare-and-set semantics so only one terminal
  transition wins.

`scan_publications` provides structural proof that a succeeded scan was
published. The final publication transaction inserts it while changing the
scan state to `succeeded`.

## Table: `scan_publications`

One row is the immutable publication certificate for a completed scan.

| Column | Type | Null | Rule |
|---|---|---:|---|
| `scan_id` | `uuid` | no | primary key |
| `repository_id` | `uuid` | no | ownership component |
| `lifecycle_state` | `text` | no | fixed to `succeeded` |
| `manifest_scheme` | `text` | no | versioned ordered-manifest algorithm, 1–128 octets |
| `artifact_set_digest` | `bytea` | no | SHA-256 of the ordered envelope/dependency manifest |
| `artifact_count` | `integer` | no | 1–256 |
| `published_at` | `timestamptz` | no | immutable |

Keys and constraints:

- primary key: `scan_id`;
- unique: `(repository_id, scan_id)`;
- composite foreign key `(repository_id, scan_id, lifecycle_state)` references
  the same columns in `repository_scans`, `ON DELETE RESTRICT`, deferrable and
  initially deferred for the final publication transaction;
- check: lifecycle state is exactly `succeeded`;
- check: set digest is 32 octets and artifact count is bounded;
- a deferred finalization check in the future persistence transaction verifies
  that `artifact_count` equals the attached envelopes and that the scan state
  is `succeeded` before commit.

Consumers identify published data by joining through this relation. They never
treat the presence of staged payload bytes as publication.

## Table: `artifact_payloads`

One row describes one complete, content-addressed exact payload. The row and
its chunks become immutable after their staging transaction commits.

| Column | Type | Null | Rule |
|---|---|---:|---|
| `payload_digest` | `bytea` | no | primary key; SHA-256 of concatenated exact bytes |
| `payload_size` | `bigint` | no | 0 through 8 GiB |
| `chunk_size` | `integer` | no | fixed at 4,194,304 bytes |
| `chunk_count` | `integer` | no | 0 for empty payload; otherwise 1–2048 |
| `created_at` | `timestamptz` | no | immutable |

Keys and constraints:

- primary key: `payload_digest`;
- unique: `(payload_digest, payload_size)` for envelope integrity;
- check: digest is exactly 32 octets;
- check: size is between 0 and 8,589,934,592 bytes;
- check: chunk size is exactly 4 MiB;
- check: empty size means zero chunks and non-empty size means the mathematically
  expected chunk count;
- no update is granted to runtime roles.

Eight GiB is the absolute schema safety ceiling, not the operational default.
The accepted initial operational limit is four GiB, derived from the largest
released fixture and the approved headroom formula. A payload over that limit
is rejected before staging; the system never truncates it. Raising the
operational limit within the eight-GiB ceiling requires new benchmark evidence.
Raising the schema ceiling requires an accepted migration and architecture
review.

## Table: `artifact_payload_chunks`

Physical chunking supports bounded client memory and avoids relying on one
near-one-GiB PostgreSQL field. It does not split the logical artifact contract:
the ordered concatenation is the one exact payload covered by SHA-256.

| Column | Type | Null | Rule |
|---|---|---:|---|
| `payload_digest` | `bytea` | no | parent payload |
| `chunk_ordinal` | `integer` | no | zero-based, 0–2047 |
| `chunk_bytes` | `bytea` | no | 1–4,194,304 octets |

Keys and constraints:

- primary key: `(payload_digest, chunk_ordinal)`;
- foreign key `payload_digest` references `artifact_payloads`,
  `ON DELETE CASCADE` only for explicit unreferenced-payload garbage
  collection;
- check: ordinal and byte length are bounded;
- all chunks and the parent row are staged in one transaction;
- the application verifies contiguous ordinals, declared total size, and the
  SHA-256 of the ordered stream before commit;
- readers re-verify size and digest before returning a trusted payload.

PostgreSQL `bytea` is used because it stores arbitrary octets. Large Objects
are rejected for v1 because their separate object lifecycle weakens ordinary
foreign-key ownership and complicates backup, privilege, and garbage-collection
behavior. One unchunked `bytea` value remains a benchmark reference only.

## Table: `artifact_envelopes`

One row identifies one released artifact inside one scan.

| Column | Type | Null | Rule |
|---|---|---:|---|
| `artifact_id` | `uuid` | no | primary key; application generated |
| `scan_id` | `uuid` | no | owning scan |
| `artifact_name` | `text` | no | 1–128 machine-key octets |
| `artifact_version` | `text` | no | exact validated version, 1–64 octets |
| `stable_id_scheme` | `text` | yes | exact scheme such as `go-semantic-id/v1`, at most 128 octets |
| `codec_name` | `text` | no | 1–64 machine-key octets |
| `codec_version` | `text` | no | exact version, 1–64 octets |
| `media_type` | `text` | no | 1–128 octets |
| `producer_name` | `text` | no | 1–128 machine-key octets |
| `producer_version` | `text` | no | exact version, 1–64 octets |
| `payload_digest` | `bytea` | no | exact payload |
| `payload_size` | `bigint` | no | must match payload metadata |
| `created_at` | `timestamptz` | no | artifact creation instant |

Keys and constraints:

- primary key: `artifact_id`;
- unique: `(scan_id, artifact_name)`;
- unique: `(scan_id, artifact_id)` for same-scan dependencies;
- unique: `(artifact_id, payload_digest)` for projection proof;
- foreign key `scan_id` references `repository_scans`, `ON DELETE RESTRICT`;
- foreign key `scan_id` also references `scan_publications`,
  `ON DELETE RESTRICT`, deferrable and initially deferred; this prevents an
  envelope from existing for a failed, cancelled, or incomplete scan;
- composite foreign key `(payload_digest, payload_size)` references the same
  columns in `artifact_payloads`, `ON DELETE RESTRICT`;
- bounded non-empty machine fields;
- no update or delete granted to ingestion/query roles.

An envelope is visible to consumers only when its scan has a
`scan_publications` row.

## Table: `artifact_dependencies`

One row is an ordered, directed edge from a consuming artifact to an exact
source artifact in the same scan.

| Column | Type | Null | Rule |
|---|---|---:|---|
| `scan_id` | `uuid` | no | same-scan ownership proof |
| `consumer_artifact_id` | `uuid` | no | edge origin |
| `dependency_ordinal` | `integer` | no | zero-based declared order |
| `source_artifact_id` | `uuid` | no | edge target |
| `declared_name` | `text` | no | exact required source artifact name |
| `declared_version` | `text` | no | exact required source version |

Keys and constraints:

- primary key: `(consumer_artifact_id, dependency_ordinal)`;
- unique: `(consumer_artifact_id, source_artifact_id)`;
- composite foreign key `(scan_id, consumer_artifact_id)` references
  `(scan_id, artifact_id)` in `artifact_envelopes`, `ON DELETE RESTRICT`;
- composite foreign key `(scan_id, source_artifact_id)` references the same
  target, `ON DELETE RESTRICT`;
- check: consumer and source differ;
- check: ordinal is 0–4095;
- final publication verifies declared name/version against the source envelope.

These composite keys make a dependency across scans structurally impossible.

## Table: `artifact_projections`

One row is one bounded, rebuildable query projection for an exact artifact
payload. It never replaces the payload.

| Column | Type | Null | Rule |
|---|---|---:|---|
| `projection_id` | `uuid` | no | primary key |
| `artifact_id` | `uuid` | no | source envelope |
| `source_payload_digest` | `bytea` | no | exact source payload |
| `projector_name` | `text` | no | 1–128 machine-key octets |
| `projector_version` | `text` | no | exact version, 1–64 octets |
| `projection_schema_version` | `text` | no | exact version, 1–64 octets |
| `projection_digest_scheme` | `text` | no | versioned canonicalization/hash scheme, 1–128 octets |
| `projection_digest` | `bytea` | no | SHA-256 of canonical projection bytes |
| `document` | `jsonb` | no | bounded query document, never the authoritative artifact |
| `document_size` | `integer` | no | canonical input size, 0–8 MiB |
| `record_count` | `integer` | no | non-negative |
| `created_at` | `timestamptz` | no | immutable |

Keys and constraints:

- primary key: `projection_id`;
- unique: `(artifact_id, projector_name, projector_version,
  projection_schema_version)`;
- foreign key `source_payload_digest` references `artifact_payloads`,
  `ON DELETE RESTRICT`;
- composite foreign key `(artifact_id, source_payload_digest)` references
  `(artifact_id, payload_digest)` in `artifact_envelopes`,
  `ON DELETE CASCADE` because projections are rebuildable dependents;
- check: both digests are exactly 32 octets;
- check: `document_size` is at most 8 MiB;
- no broad GIN index in v1.

A new projector or projection-schema version creates a new row. It never
updates a prior projection in place. Obsolete projection rows may be removed by
an audited rebuild/retention operation without touching payloads.

## Table: `projected_diagnostics`

This is a bounded query index owned by one projection.

| Column | Type | Null | Rule |
|---|---|---:|---|
| `projection_id` | `uuid` | no | projection owner |
| `diagnostic_ordinal` | `integer` | no | deterministic zero-based order |
| `severity` | `text` | no | `info`, `warning`, or `error` |
| `code` | `text` | no | 1–128 machine-key octets |
| `engine_name` | `text` | no | 1–128 machine-key octets |
| `relative_path` | `text` | yes | at most 4096 octets; never absolute |
| `line_number` | `integer` | yes | positive when present |
| `column_number` | `integer` | yes | positive when present |
| `message` | `text` | no | sanitized, at most 4096 octets |

Keys and constraints:

- primary key: `(projection_id, diagnostic_ordinal)`;
- foreign key `projection_id` references `artifact_projections`,
  `ON DELETE CASCADE`;
- bounded severity, locations, message, and ordinal;
- no credentials, payload excerpts, or absolute local paths.

## Table: `projected_statistics`

One row is one typed query statistic owned by one projection.

| Column | Type | Null | Rule |
|---|---|---:|---|
| `projection_id` | `uuid` | no | projection owner |
| `metric_key` | `text` | no | 1–128 machine-key octets |
| `value_kind` | `text` | no | `integer`, `decimal`, `boolean`, or `text` |
| `integer_value` | `bigint` | yes | set only for integer kind |
| `decimal_value` | `numeric` | yes | set only for decimal kind |
| `boolean_value` | `boolean` | yes | set only for boolean kind |
| `text_value` | `text` | yes | at most 4096 octets; set only for text kind |
| `unit` | `text` | yes | at most 64 machine-key octets |

Keys and constraints:

- primary key: `(projection_id, metric_key)`;
- foreign key `projection_id` references `artifact_projections`,
  `ON DELETE CASCADE`;
- check: exactly the value column named by `value_kind` is non-null.

Floating-point storage is intentionally avoided for deterministic counters and
percentages. Decimal values use exact `numeric` representation.

## Table: `audit_events`

Audit rows are append-only and intentionally survive operational retention.

| Column | Type | Null | Rule |
|---|---|---:|---|
| `audit_event_id` | `bigint` identity | no | primary key |
| `occurred_at` | `timestamptz` | no | immutable |
| `event_type` | `text` | no | 1–128 machine-key octets |
| `outcome` | `text` | no | `succeeded`, `failed`, or `denied` |
| `actor_kind` | `text` | no | bounded machine key |
| `actor_id` | `text` | no | opaque, at most 256 octets |
| `correlation_id` | `uuid` | no | request/operation correlation |
| `security_scope_id` | `uuid` | no | authorization scope at event time |
| `repository_id` | `uuid` | yes | durable subject identifier |
| `scan_id` | `uuid` | yes | durable subject identifier |
| `artifact_id` | `uuid` | yes | durable subject identifier |
| `safe_details` | `jsonb` | no | sanitized object, canonical input at most 64 KiB |

Audit subject identifiers do not have foreign keys by design: an audit event
must retain the original identifiers after an authorized purge. The absence of
those foreign keys is an explicit retention exception, not accidental missing
integrity. Runtime audit writers receive insert only; update/delete are denied.

## Publication Transaction

Payload staging occurs before publication:

1. Validate the operational payload limit, declared size, codec support, and
   application-computed SHA-256.
2. In one staging transaction, insert the immutable payload metadata and all
   ordered chunks, or confirm that an identical digest/size already exists.
3. Re-read the ordered stream and verify byte count and digest before treating
   it as staged.

The final publication uses one transaction:

1. lock the running scan row;
2. reject a non-running or already-published scan;
3. validate the complete artifact set and exact dependency names/versions;
4. insert envelopes, dependency edges, and bounded projections;
5. insert the publication certificate;
6. transition the scan to succeeded and set finish/publication times;
7. optionally change the repository's current scan pointer;
8. append the publication audit event;
9. commit.

Consumers query through the publication certificate. A rollback exposes none
of the envelopes as a successful scan. The already-staged content-addressed
payloads remain unreferenced and are eligible for later garbage collection.

## Approved Initial Queries and Indexes

Only indexes supporting approved lifecycle and exact-lookup queries are in v1.

| Query | Index beyond primary/unique constraints |
|---|---|
| list repositories in a security scope | `(security_scope_id, lifecycle_state, created_at DESC, repository_id)` |
| get repository by normalized source | unique `(security_scope_id, source_kind, source_fingerprint_scheme, source_fingerprint)` |
| list scans for repository | `(repository_id, requested_at DESC, scan_id)` |
| list scans by lifecycle state | `(repository_id, lifecycle_state, requested_at DESC, scan_id)` |
| find running scans for recovery | partial `(started_at, scan_id)` where state is `running` |
| get artifact by scan/name | unique `(scan_id, artifact_name)` |
| list artifact versions by name | `(artifact_name, artifact_version, created_at DESC)` |
| find payload references | `(payload_digest, artifact_id)` |
| traverse dependency targets | `(source_artifact_id, consumer_artifact_id)` |
| list projector outputs | unique projection key described above |
| filter diagnostics | `(severity, code, projection_id, diagnostic_ordinal)` |
| find audit events by subject/time | separate indexes on repository, scan, and correlation identifiers with `occurred_at DESC` |

There is no initial GIN index on projection JSON, no full-text index, no vector
index, and no language-specific index. New indexes require a named query,
representative `EXPLAIN (ANALYZE, BUFFERS)` evidence, measured write cost, and
an accepted migration.

## Partitioning Decision

Initial v1 tables are not partitioned.

There is no measured production volume or retention workload showing that
partitioning improves the approved queries. PostgreSQL documentation warns that
index and partition choices should follow the actual access pattern. Premature
partitioning would complicate composite uniqueness, foreign keys, migrations,
and retention without evidence.

Reconsider partitioning only when the disposable benchmark or production
telemetry demonstrates a specific failure in query latency, vacuum duration,
index size, retention duration, or backup/restore objectives. The review must
identify the relation and partition key; time partitioning is not assumed.

## Role and Privilege Matrix

Roles are capability roles. Login/user binding is deployment configuration and
is deferred.

| Role | Purpose | Allowed | Explicitly denied |
|---|---|---|---|
| `platform_owner` | non-login object owner | own schema/objects | application login |
| `platform_migrator` | Phase 3.3 migrations | assume owner during approved migration; read schema history | runtime use |
| `platform_ingestor` | persistence writer | select repository/scan state; insert staged payloads/chunks/envelopes/dependencies/projections; perform bounded lifecycle updates; insert audit | DDL, truncate, arbitrary delete, role changes |
| `platform_query_reader` | API metadata reads | select published repositories, scans, envelopes, dependencies, and projections through approved views | raw payload chunks, audit, writes |
| `platform_artifact_reader` | authorized exact artifact export | query-reader rights plus exact payload/chunk reads | writes, audit |
| `platform_retention_worker` | approved purge/GC | bounded deletes in documented order; insert audit | DDL, role changes, unaudited broad delete |
| `platform_audit_writer` | append audit | insert audit rows | select/update/delete audit |
| `platform_auditor` | compliance review | select audit and safe metadata | payload bytes, writes |
| `platform_backup` | backup/restore workflow | read required schema data and perform separately authorized restore | application DML, DDL outside restore procedure |

`PUBLIC` receives no privileges on the `platform` schema. Runtime roles cannot
disable constraints, own objects, create functions, alter search paths, or
apply migrations. Direct end-user database access is unsupported. Application
authorization must filter by `security_scope_id`; multi-tenant RLS requires a
future accepted security design.

## Retention and Purge Rules

No time-based retention duration is approved yet. Durations are deployment
policy, not schema semantics. The schema enforces safe dependency order:

1. record an immutable purge-request audit event;
2. move a repository to `purge_pending` and prevent new scans;
3. clear or move its `current_scan_id` within an authorized transaction;
4. remove rebuildable diagnostic/statistic rows and projection headers;
5. remove dependency edges;
6. remove artifact envelopes;
7. remove publication certificates;
8. remove scan rows;
9. remove the repository row when policy permits;
10. garbage-collect a payload only when no envelope references its digest and
    its staging safety interval has elapsed;
11. retain audit identifiers according to the audit policy.

Core historical foreign keys use `RESTRICT`; deletion order must be explicit.
`CASCADE` is limited to physical payload chunks and rebuildable children whose
parent is already explicitly selected. Every batch is bounded, retryable, and
audited. Backups and legal holds are evaluated before deletion.

## Storage Limits

| Item | Absolute schema ceiling | Operational behavior |
|---|---:|---|
| exact artifact payload | 8 GiB | initial runtime maximum is 4 GiB |
| payload chunk | 4 MiB | fixed physical chunk size |
| artifacts per scan | 256 | reject publication above limit |
| dependencies per artifact | 4096 | reject publication above limit |
| projection JSON document | 8 MiB | projection fails without affecting exact payload |
| audit safe details | 64 KiB | reject; never truncate secrets/content into it |
| diagnostic message | 4 KiB | projector emits bounded safe message |
| relative path | 4 KiB | no absolute path |

The accepted initial operational payload limit is four GiB. The benchmark
doubled the 1,556,379,091-byte Kubernetes fixture and rounded upward to the next
power of two. The application rejects larger artifacts before staging. Future
increases require equivalent benchmark and engineering evidence.

## Integrity Verification

- On staging: verify chunk order, total bytes, and SHA-256.
- On exact read: stream, count, and verify SHA-256 before trust.
- On publication: verify envelope size/digest and all same-scan dependency
  names and versions.
- On projection use: verify its recorded source digest still equals the
  envelope digest; a mismatch is stale/corrupt and never silently repaired.
- On backup restore: verify every payload for the first release; future sampled
  verification requires an accepted risk decision.
- On garbage collection: prove zero envelope references in the deleting
  transaction.

Database constraints detect relationship and length violations. Application
verification detects properties that ordinary row constraints cannot prove,
including a SHA-256 over an ordered set of chunk rows.

## Migration Ownership

Phase 3.3 chooses the migration tool. Regardless of tool:

- `platform_owner` owns objects;
- only the migrator records schema history;
- migrations are ordered, immutable after release, and checksum verified;
- the application refuses a newer unsupported schema;
- migration startup is explicit, never an implicit engine side effect;
- exact payload bytes are never rewritten by a projection/schema migration.

No migration filename or executable DDL is frozen by this document.

## Error Mapping Requirements

The future adapter maps constraint outcomes to storage-neutral errors:

- duplicate repository/scan request with equal digest → idempotent success;
- reused idempotency key with different digest → idempotency conflict;
- duplicate artifact name in a scan → duplicate publication;
- missing/cross-scan dependency → invalid dependency;
- envelope/payload size mismatch → integrity failure;
- invalid lifecycle transition → lifecycle conflict;
- operational/schema payload limit exceeded → payload too large;
- projection source mismatch → stale projection;
- authorization-scope mismatch → authorization denied.

Database object names, SQL text, connection details, and payload bytes are not
included in public errors.

## Accepted Benchmark Validation

1. PostgreSQL 18.4 with no extensions passed the isolated run.
2. Exact bytes and SHA-256 digests survived stage, read, publication, backup,
   and restore for all six released fixtures.
3. Four-MiB chunks passed the accepted stage, read, WAL, and client-memory
   gates; one-MiB and 256-KiB candidates missed the Kubernetes stage floor.
4. The 1,556,379,091-byte Kubernetes artifact established a four-GiB
   operational limit through the approved headroom formula.
5. Dependency, projection, lifecycle, rollback, retention, and atomic
   publication constraints passed.
6. Metadata access p95 was 1.643 ms with 1,000,050 dependency rows and no
   payload-chunk reads.
7. Publication p95 was 130.15 ms and no reader observed a partial publication.
8. Logical backup and isolated restore digest-verified all six payloads.
9. ADR 0011 records the accepted measured contract and remaining boundaries.

Phase 3.2 and its measured four-MiB refinement were accepted on 2026-07-24.
Phase 3.3 migration implementation is authorized. Later persistence adapter,
API, UI, and production-connection work remains gated by the roadmap.

## Primary References

- [PostgreSQL versioning policy](https://www.postgresql.org/support/versioning/)
- [PostgreSQL 18 binary data types](https://www.postgresql.org/docs/18/datatype-binary.html)
- [PostgreSQL 18 TOAST storage](https://www.postgresql.org/docs/18/storage-toast.html)
- [PostgreSQL 18 constraints](https://www.postgresql.org/docs/18/ddl-constraints.html)
- [PostgreSQL 18 indexes](https://www.postgresql.org/docs/18/indexes.html)
- [PostgreSQL 18 roles and privileges](https://www.postgresql.org/docs/18/user-manag.html)
