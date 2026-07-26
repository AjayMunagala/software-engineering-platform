# Application Runtime Lifecycle

- Phase: 3.5.3
- Contract: `1.0.0` (frozen)
- Local exit gate: complete
- Engineering acceptance: accepted on 2026-07-26
- Phase 3.5.4: accepted on 2026-07-27

Phase 3.5.3 owns non-networked startup, narrow persistence-capability routing,
work admission, callable health, drain, cancellation, graceful shutdown, and
resource closure.

It treats the PostgreSQL runtime as an opaque dependency. It cannot access a
pool, SQL, credentials, migration history, or driver statistics.

The runtime starts ready only after configuration and PostgreSQL construction
succeed. Draining removes readiness and rejects new work immediately. Admitted
work may complete within the configured drain timeout; remaining contexts are
canceled, resources close in their owning PostgreSQL runtime, and repeated
shutdown callers observe one stable result.

HTTP endpoints, signal registration, logging/metrics exporters, APIs, UI,
authentication, business logic, AI orchestration, and engines remain outside
this package.
