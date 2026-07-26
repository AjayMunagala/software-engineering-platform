# Runtime Health

- Phase: 3.5.3
- Contract: `1.0.0` (frozen)
- Local exit gate: complete
- Engineering acceptance: accepted on 2026-07-26
- Phase 3.5.4: accepted on 2026-07-27
- Network endpoints: unauthorized

This accepted Phase 3.5.3 package implements callable liveness and readiness
without creating an HTTP endpoint, listener, logger, metric exporter, database
pool, or SQL dependency.

Health consumes one opaque `DatabaseChecker`. A readiness call applies the
smaller configured timeout and re-proves PostgreSQL connectivity, schema
compatibility, and capability privileges. Three consecutive failures remove
readiness by default; one success restores it. A stale success also removes
readiness. Database failure never makes post-start liveness unhealthy.

Lifecycle owns state transitions. Health returns detached immutable snapshots
with bounded statuses and reason codes only.
