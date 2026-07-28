# ADR 0017 — Repository Service Persistence & Runtime Integration

## Status

Accepted with recommendations

## Date

2026-07-28

## Context

Repository Service Phase 4.0.5 now produces deterministic, sealed, exact-byte
artifacts through an engine-neutral scan coordinator. Persistence Port,
PostgreSQL Adapter, and Runtime Infrastructure are independently frozen at
`1.0.0`, but no production package connects those released contracts.

Direct integration reveals real contract-shape differences:

- PostgreSQL physical identifiers are UUIDs while the candidate service
  initially accepted broader machine IDs;
- public artifact IDs are versioned SHA-256 strings, not UUIDs;
- persistence transactions own durable timestamps;
- persistence publication needs request, source, and dependency information
  that the fake-oriented internal scan commands did not expose;
- Runtime Infrastructure owns admission and capability-specific persistence
  views, while the service must not own pools or shutdown.

These differences must be deliberate architectural decisions, not hidden
implementation conversions.

## Decision

1. Introduce one internal Phase 4.0.6 integration package that implements the
   lifecycle and scan store contracts using only Persistence Port and Runtime
   capability interfaces.
2. Keep intelligence engines, persistence, PostgreSQL Adapter, and Runtime
   Infrastructure unchanged and independently owned.
3. Tighten candidate service `ScopeID`, `RepositoryID`, and `ScanID` to
   canonical lowercase UUID strings and update all conformance fixtures.
4. Keep public artifact IDs unchanged and derive internal RFC 9562 version-8
   UUIDs through `repository-service-storage-artifact-id/v1`.
5. Reconstruct public artifact IDs on reads and verify the deterministic UUID
   mapping against every stored record.
6. Treat persistence-returned timestamps as authoritative and validate
   ordering invariants rather than cross-clock nanosecond equality.
7. Add source fingerprint, request ID, and dependency accessors to the
   unreleased internal scan command models.
8. Verify the scan source fingerprint against the registered repository before
   beginning a durable scan.
9. Stage each exact payload in its own persistence transaction, then publish
   all artifact envelopes and dependency edges in one atomic publication.
10. Freeze the proposed `repository-service-manifest/v1` canonical preimage
    before production implementation.
11. Borrow runtime admission, ingest, and read capabilities. Never construct
    pools, load secrets, run migrations, or close the runtime in the service.
12. Reconcile ambiguous publication using complete durable scan and artifact
    state. Never mark a possibly published scan failed.
13. Keep projections, diagnostics, statistics, retention scheduling,
    transports, authentication, UI, and AI outside Phase 4.0.6.
14. Freeze UUID validation, physical artifact UUID mapping, and the canonical
    manifest as normative golden vectors before implementation begins.

## Rationale

An internal integration adapter is the narrowest place that can understand
both service-owned orchestration models and released infrastructure contracts.
Changing the engines or persistence port would reverse dependency direction.
Letting the scan coordinator import runtime or PostgreSQL would couple business
policy to deployment and physical storage.

UUID restrictions are accepted at the still-unreleased service boundary
because repository and scan IDs must be listed and therefore cannot use a
one-way physical mapping without a new schema mapping table. Public artifact
IDs can use a one-way physical mapping because they are deterministically
reconstructed from the artifact contract fields already stored in each
envelope.

Persistence-authoritative timestamps reflect the real transaction commit and
avoid invalid assumptions about independent process and database clocks.

## Alternatives

### Add service ID mapping tables

Rejected for this milestone. It would change the frozen physical schema and
migration contract solely to preserve a candidate ID grammar that has not been
released.

### Store public artifact IDs directly

Rejected. The accepted schema requires UUID artifact IDs and the PostgreSQL
Adapter validates that contract.

### Modify Persistence Port 1.0.0

Rejected. Existing capabilities are sufficient. Application translation does
not justify a breaking persistence release.

### Let the integration package construct pools

Rejected. Runtime Infrastructure exclusively owns TLS, credentials,
compatibility proof, pools, health, drain, and shutdown.

### Compare service and database timestamps exactly

Rejected. It is not reproducible across independent clocks and says nothing
about semantic integrity.

### Publish artifacts one by one

Rejected. It violates the accepted atomic scan-visibility contract.

### Run migrations at service startup

Rejected. Deployment preflight owns migrations; runtime only proves schema
compatibility.

## Consequences

- Candidate public UUID validation becomes stricter before Repository Service
  `1.0.0`.
- Existing Phase 4 conformance fixtures must migrate to UUID IDs.
- Internal scan command models receive additive immutable accessors.
- A deterministic physical artifact ID mapping and manifest codec require
  golden vectors.
- PostgreSQL timestamps appear in service results.
- Staged payloads may remain unreferenced after pre-publication failure and are
  reclaimed later by the accepted retention subsystem.
- Phase 4.0.6 gains end-to-end persistence without changing any frozen
  foundational contract.

## Security consequences

- Scope isolation is applied on every persistence call.
- Physical UUIDs never cross the service boundary.
- Source handles and local paths never enter persistence.
- Credentials, SQL, pgx errors, and pool objects remain below the integration
  boundary.
- Audit actors are derived from the already-authorized principal.
- Cross-scope read, list, export, stage, and mutation tests are mandatory.

## Acceptance gate

Engineering accepted this ADR with recommendations on 2026-07-28 after the
required golden-vector contract was recorded in
`docs/API/REPOSITORY_SERVICE_INTEGRATION_GOLDEN_VECTORS.md`. That acceptance
authorizes Phase 4.0.6 implementation only. Phase 4.0.7 remains separately
gated.
