# ADR 0013: Storage-Neutral Persistence Port

- Status: Accepted
- Date: 2026-07-24
- Accepted: 2026-07-24
- Decision owners: Phase 3.4.1 architecture review
- Depends on: ADR 0010, ADR 0011, ADR 0012
- Implementation authorization: Phase 3.4.2 neutral port and conformance only

## Context

The persistence boundary, PostgreSQL physical schema, measured payload
contract, and migration framework are accepted. Phase 3.4 must now connect
application orchestration to durable storage without making released
intelligence engines depend on PostgreSQL or importing language-specific
artifact types into the storage layer.

A broad persistence interface or leaked transaction abstraction would allow
callers to create partial lifecycle states. Accepting engine artifacts directly
would couple the port to every artifact package and serialization contract.
Exposing PostgreSQL rows or driver errors would prevent future adapters and
make database changes part of the public application API.

## Decision

The project adopts the following decisions:

1. Intelligence engines remain completely persistence-independent. Application
   orchestration serializes released artifacts and submits detached exact bytes
   plus neutral metadata.
2. The public persistence contract is split into repository, scan, staging,
   publication, reading, verification, and retention capabilities. A composed
   `Port` is optional convenience.
3. The port accepts streams, fixed SHA-256 digests, byte counts, opaque IDs,
   versioned names, bounded metadata, and explicit repository scope. It does
   not accept RIE/LIE models, PostgreSQL values, SQL, or driver types.
4. The port exposes no generic transaction manager. Repository lifecycle,
   one-payload staging, atomic publication, and bounded retention operations own
   their transaction boundaries.
5. Exact bytes are fully consumed and verified even for idempotent staging.
   The accepted 4 GiB operational maximum is neutral policy; PostgreSQL's
   ordered 4 MiB chunks remain an adapter detail.
6. Publication is one atomic manifest operation. A succeeded scan and its
   artifacts become visible together or not at all.
7. Stable storage-neutral error kinds classify validation, integrity,
   lifecycle, idempotency, authorization, cancellation, timeout, availability,
   and internal failures without leaking database details.
8. A reusable conformance suite defines mandatory observable behavior for
   PostgreSQL and future adapters.
9. The candidate port begins at `0.1.0` and freezes at `1.0.0` only after
   implementation and PostgreSQL conformance evidence are accepted.

## Rationale

- Preserves deterministic, database-free engine execution.
- Keeps serialization ownership with artifact codecs and orchestration.
- Prevents SQL transaction semantics from spreading into application logic.
- Makes exact-byte integrity and publication atomicity mandatory for every
  adapter rather than incidental PostgreSQL behavior.
- Supports future storage adapters without weakening the accepted artifact
  lifecycle.
- Enables focused unit and conformance testing at capability boundaries.

## Rejected Alternatives

### Semantic Engine Depends on the Port

Rejected because persistence availability, retries, credentials, and side
effects would enter deterministic analysis.

### Port Accepts Released Engine Artifact Interfaces

Rejected because the persistence package would import every engine/artifact
package and own serialization decisions. Detached bytes and neutral metadata
are the correct boundary.

### One Large Repository Interface

Rejected because callers would depend on capabilities they do not use and
tests/future adapters would require unnecessary methods. Small capability
interfaces remain composable.

### Public TransactionManager or WithTransaction

Rejected because callers could create long transactions or publish only part
of a scan. Transactions implement lifecycle guarantees and remain internal.

### Trust an Existing Digest Without Reading Retry Input

Rejected because an idempotent retry could report success for corrupt or
different submitted bytes. Successful staging always verifies the submitted
stream.

### Expose PostgreSQL Errors and Row Models

Rejected because it leaks schema/driver details, weakens security, and prevents
adapter substitution.

### Add Connection Pool and Environment Configuration Now

Rejected because Phase 3.4.1 is a design gate and Phase 3.5 owns local runtime
configuration and secrets. The adapter receives an approved database execution
capability in its later implementation phase.

## Consequences

### Positive

- Engines and released artifacts remain unchanged.
- Every adapter has testable lifecycle, integrity, and idempotency obligations.
- Application services can request narrow capabilities.
- Database failures have a stable safe application representation.
- Large payloads remain streamed and bounded.

### Costs

- Application orchestration must serialize and validate artifact metadata
  before calling persistence.
- Adapter implementations must translate native errors and implement all
  conformance behavior.
- Exact-output streaming requires callers to withhold partially written output
  until success.
- A staged design/implementation/adapter sequence adds governance gates.

## Acceptance Evidence

Engineering reviewed and accepted together:

- `docs/Architecture/STORAGE_NEUTRAL_PERSISTENCE_PORT.md`;
- `docs/API/PERSISTENCE_PORT_V1.md`;
- the updated Phase 3 roadmap;
- dependency direction and forbidden imports;
- lifecycle and transaction ownership;
- idempotency, cancellation, ambiguous-commit, and safe-error rules;
- conformance and benchmark strategy.

## Acceptance Effect

Acceptance authorizes only Phase 3.4.2: implementation of the neutral
`backend/persistence` contract and its adapter-independent conformance harness.
It does not authorize PostgreSQL adapter code, database configuration,
credentials, APIs, UI, authentication, connection pooling, or business logic.

The conformance suite must verify scope isolation for every public operation,
including repository/scan lists, metadata reads, exact payload reads,
verification, lifecycle writes, publication, retention, and garbage
collection. Scope isolation is not limited to write methods.
