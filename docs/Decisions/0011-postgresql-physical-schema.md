# ADR 0011: PostgreSQL Physical Artifact Schema

- Status: Accepted; benchmark validation pending
- Date: 2026-07-23
- Accepted: 2026-07-24
- Prerequisite: accepted ADR 0010
- Decision owners: Phase 3.2 schema review
- Implementation authorization: isolated disposable payload benchmark only

## Context

ADR 0010 defines persistence as a downstream, artifact-oriented boundary. A
physical PostgreSQL contract is required before migrations or adapter code, but
it must preserve exact artifact bytes, atomic publication, storage neutrality,
and independent version evolution.

## Proposed Decision

1. Support PostgreSQL 18 at its current minor release initially.
2. Require no PostgreSQL extensions.
3. Store all application relations in a migration-owned `platform` schema.
4. Use application-generated UUIDs and application-computed SHA-256 digests.
5. Store authoritative payload bytes as content-addressed metadata plus ordered
   one-MiB `bytea` chunks; never use `jsonb` as the artifact source of truth.
6. Enforce Repository → Scan → Envelope → Payload relationships with foreign
   keys and enforce both dependency endpoints within the same scan through
   composite foreign keys.
7. Bind every projection to both an artifact envelope and its exact payload
   digest. Projections remain bounded and rebuildable.
8. Use an immutable publication certificate so consumers cannot observe a
   partial successful scan.
9. Use explicit text/check lifecycle states rather than PostgreSQL enums.
10. Start without table partitioning, broad JSON indexes, full-text indexes,
    vector indexes, or language-specific source-of-truth tables.
11. Separate migration ownership, ingestion, query, exact-artifact read,
    retention, audit, and backup privileges.
12. Preserve audit subject identifiers without foreign keys so authorized
    operational purges do not erase their historical identity.

The complete proposed contract is
[POSTGRESQL_SCHEMA_SPECIFICATION.md](../Database/POSTGRESQL_SCHEMA_SPECIFICATION.md).

## Why Chunked `bytea`

PostgreSQL TOAST permits large variable-length values but one field remains
limited to about one GiB and client operations commonly materialize a complete
field. Ordered fixed-size rows provide bounded streaming, normal transactions,
ordinary foreign-key ownership, content-addressed deduplication, and explicit
garbage collection. They avoid the separate lifecycle and privileges of
PostgreSQL Large Objects.

Physical chunking is not semantic fragmentation. The artifact is the ordered
concatenation verified by one size and SHA-256 digest.

## Consequences

### Positive

- exact artifacts remain independently verifiable;
- large reads/writes can use bounded client buffers;
- shared payload bytes are stored once by digest;
- same-scan dependencies and projection provenance are database-enforced;
- metadata queries avoid loading payload bytes;
- extensions and database-generated identity are unnecessary;
- future engines fit without new source-of-truth table families.

### Costs

- staging and reading require ordered chunk orchestration;
- aggregate size/digest integrity spans rows and therefore needs application
  verification in addition to row constraints;
- a crashed process may leave complete unreferenced payloads for audited GC;
- exact payload export requires an explicitly privileged reader;
- projection changes create/rebuild rows rather than updating artifact bytes.

## Alternatives Rejected

### Full artifact in `jsonb`

Rejected because normalization does not preserve the authoritative exact bytes
and a full semantic JSON projection would duplicate very large artifacts.

### One unrestricted `bytea` column

Retained only as a benchmark reference. Its one-field limit and full-value
client behavior conflict with the bounded-memory direction for Kubernetes-scale
artifacts.

### PostgreSQL Large Objects

Rejected for v1 because their separate object namespace/lifecycle complicates
ordinary foreign keys, privilege isolation, garbage collection, and backups.

### Language-specific relational tables

Rejected because they duplicate released artifact facts and couple migrations
to a language implementation.

### Initial partitioning

Rejected until measured query, vacuum, retention, or recovery evidence shows a
specific benefit.

## Validation Required Before Acceptance

- approve the complete physical schema and privilege matrix;
- approve execution of the isolated payload benchmark plan;
- validate one-MiB chunks against all released fixtures;
- freeze the operational payload maximum from measured evidence;
- validate exact round trips, atomic visibility, dependency constraints,
  retention safety, backup/restore, WAL, latency, and client memory;
- update this ADR with the accepted results.

Engineering accepted this ADR on 2026-07-24 and authorized the isolated payload
benchmark. Acceptance of the resulting benchmark report authorizes only Phase
3.3 migration implementation. It does not authorize APIs, UI, production
connections, or production credentials.
