# Repository Service Changelog

## 1.0.0 - Proposed

### Candidate capabilities

- Added the transport-neutral Repository Service contract.
- Added repository lifecycle and synchronous scan coordination.
- Added released RIE/Go LIE intelligence and canonical materialization adapters.
- Added Persistence Port, PostgreSQL Adapter, and Runtime Infrastructure
  integration without changing those frozen contracts.
- Added exact-byte export, deterministic manifests, atomic publication,
  reconciliation, single-flight, cancellation, idempotency, and scope isolation.
- Added pinned real-repository and stabilization validation evidence.

### Compatibility

- Proposed `1.x` interfaces receive no breaking changes or added methods.
- New optional capabilities require new narrow interfaces.
- Profile, codec, stable-ID, service, persistence schema, adapter, runtime, and
  future transport versions evolve independently.

### Release qualification

- Kubernetes one-worker Windows and Ubuntu eight-worker completion remains
  pending larger race-capable hardware. Completed evidence shows no software
  defect. Final engineering disposition is required before promotion.
