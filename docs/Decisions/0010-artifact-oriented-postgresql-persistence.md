# ADR 0010: Artifact-Oriented PostgreSQL Persistence Boundary

- Status: Accepted
- Date: 2026-07-23
- Accepted: 2026-07-23
- Decision owners: Phase 3.1 architecture review
- Supersedes: ADR 0004
- Implementation authorization: Phase 3.2 schema specification documents only

## Context

RIE, `GoLanguageInventory 1.0.0`, `GoPackageIdentityInventory 1.0.0`, and
`GoSemanticInventory 1.0.0` publish immutable versioned artifacts. Durable
repository history and later APIs require persistence, but coupling engine
models to relational tables would make database migrations control analysis
contracts and would duplicate language-specific facts.

PostgreSQL was selected provisionally in ADR 0004, but the ownership boundary,
source of truth, transaction model, and evolution rules were not defined.

## Decision

If this ADR is accepted:

1. Persistence is a downstream consumer of completed immutable artifacts.
   Intelligence engines do not import, configure, or call PostgreSQL.
2. The application orchestration layer submits storage-neutral repository,
   scan, artifact-envelope, dependency, and exact-payload values through a
   persistence port.
3. Exact serialized artifact bytes are authoritative and content-addressed by
   SHA-256. The artifact name/version, stable-ID scheme, codec version, payload
   size, producer metadata, and dependency references are stored explicitly.
4. Relational metadata, `JSONB`, diagnostics, statistics, and future search
   structures are rebuildable projections, not competing sources of truth.
5. A scan becomes visible only through one atomic publication transition after
   all required artifacts and exact-version dependency edges validate.
6. Artifact version, stable-ID scheme, payload codec, projector, and database
   schema versions evolve independently and are never silently coerced.
7. PostgreSQL is the first adapter. Its row models and driver errors do not
   cross the storage-neutral port.
8. No language-specific fact tables are part of the initial schema. They may be
   introduced only as measured rebuildable query projections.

## Rationale

- Preserves the released artifact contracts as the platform source of facts.
- Keeps analysis deterministic and usable without a database.
- Allows PostgreSQL schemas and query projections to evolve without changing
  engines or rewriting artifact history.
- Supports exact replay, integrity verification, auditing, and explicit major
  version compatibility.
- Keeps future TypeScript, SQL, Python, and other language artifacts within the
  same persistence model.
- Makes a later external object store possible without changing engine APIs.

## Important Payload Decision

PostgreSQL `JSONB` does not preserve the exact input byte representation.
Therefore it cannot be the sole authoritative representation when the platform
promises digest verification and exact replay. Phase 3.2 must design and
benchmark bounded storage for exact payload bytes; any `JSONB` document is a
projection tied to a projector version and source digest.

Large payload behavior must be validated against the released Kubernetes
semantic fixture. The implementation may reject an artifact above an approved
limit, but it may not truncate, silently split, or alter the payload.

## Transaction Decision

Large immutable payloads may be staged idempotently by digest. One final short
transaction validates the complete artifact set and dependency graph, attaches
the envelopes, changes the scan to `succeeded`, and records the audit event.
Readers see only succeeded scans. Failed or cancelled attempts cannot appear as
complete repository intelligence.

## Security Decision

- Credentials are external secrets and never artifact data.
- Migration owner, application writer, query reader, and backup/restore roles
  are separate.
- Repository artifacts and relative paths are sensitive metadata.
- Authorization precedes all repository/scan/artifact access.
- SQL is parameterized; application roles do not own schemas.
- Production requires encrypted connections, encrypted backups, and verified
  restore procedures.

## Rejected Alternatives

### Normalize Every Go Fact Into Tables

Rejected as the source of truth because it duplicates the semantic artifact,
couples storage to one language, and makes artifact-version changes into
destructive schema migrations. Measured query projections remain possible.

### Store Only `JSONB`

Rejected because PostgreSQL normalizes representation and cannot guarantee an
exact byte-for-byte artifact replay without an independently specified
canonical codec.

### Let Engines Persist Their Own Results

Rejected because it introduces side effects, database availability, retries,
credentials, and transaction behavior into deterministic analysis engines.

### Add PostgreSQL Types to Artifact Models

Rejected because driver types and row layouts would leak infrastructure into
stable engine contracts.

### Start With Filesystem or Object Storage

Rejected for the first adapter because current requirements emphasize
transactional repository/scan lifecycle, auditability, and structured queries.
External object storage remains a measured future extension for payload scale.

### Add Qdrant During Persistence

Rejected because retrieval indexes are not durable truth and there is no
approved embedding/search requirement yet.

## Consequences

### Positive

- Engines remain independently testable and database-free.
- Artifact history is immutable, verifiable, and language-neutral.
- Consumers never observe partial scans.
- Query projections can be rebuilt after bugs or schema changes.
- Database and artifact migrations have clear independent ownership.

### Costs

- Exact bytes plus query projections may duplicate storage.
- Large payload staging/publication requires explicit limits and benchmarks.
- Projection rebuilds and garbage collection need audited operational jobs.
- The application layer must own lifecycle and idempotency rather than relying
  on engine execution order.

## Validation Required Before Acceptance

- Review the complete boundary in
  [PERSISTENCE_BOUNDARY.md](../Architecture/PERSISTENCE_BOUNDARY.md).
- Confirm released artifacts can supply deterministic exact bytes and metadata.
- Approve the conceptual entity and transaction model.
- Approve security, retention, backup, restore, and deletion boundaries.
- Approve Phase 3.2 as schema design only.

## Acceptance Effect

Acceptance authorizes only Phase 3.2 — PostgreSQL Schema Design. It does not
authorize SQL files, migration tools, database installation/connection,
credentials, Go persistence code, APIs, or UI work.
