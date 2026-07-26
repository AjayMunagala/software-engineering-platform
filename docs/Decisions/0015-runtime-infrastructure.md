# ADR 0015 — Runtime Infrastructure Ownership and Safety

- Status: Accepted on 2026-07-26
- Date: 2026-07-26
- Scope: runtime configuration, PostgreSQL pool/TLS, lifecycle, health, and observability
- Frozen dependencies: Persistence Port `1.0.0`, PostgreSQL Adapter `1.0.0`

## Context

The platform can deterministically produce and persist immutable artifacts, but
no application runtime yet owns configuration, secrets, PostgreSQL connection
pools, schema readiness, health, observability, or graceful shutdown.

Adding these concerns inside the adapter would break its accepted responsibility
and make Persistence Port consumers depend on one deployment model. Granting
the application migrator authority would also violate the migration and
least-privilege decisions.

## Decision

1. Add an application-owned runtime layer downstream from command/process hosts
   and upstream from the PostgreSQL Adapter.
2. Keep Persistence Port and PostgreSQL Adapter `1.0.0` unchanged.
3. Compose non-secret configuration from defaults, strict JSON file,
   environment, and allowlisted safe CLI overrides in that precedence order.
4. Resolve secrets through exactly one runtime-only `SecretProvider`; never
   pass them through the persistence package or ordinary config views.
5. Construct capability-specific `pgxpool.Pool` instances in the runtime. Pass
   each to a separate adapter through the existing minimal `Database`
   capability. The runtime alone closes/resets them. Staging/production keep
   ingest, artifact-read, and retention principals separate; wiring exposes
   only the corresponding narrow neutral interfaces.
6. Use `MinIdleConns` for optional warm capacity and keep `MinConns` zero.
   Require explicit bounded pool sizes in staging and production.
7. Permit TLS-disabled connections only for local/CI loopback or Unix sockets.
   Require certificate-chain and hostname verification in staging/production.
8. Never run migrations at application startup. Deployment preflight uses the
   existing migrator workflow outside the process.
9. Add, during an approved implementation subphase, a migration-maintained,
   read-only runtime compatibility record. Runtime roles can read it but cannot
   read or modify Atlas migration history.
10. Publish readiness only after a real connection, compatibility proof, and
    privilege verification succeed.
11. Keep database availability out of post-start liveness. Database failure
    removes readiness and permits recovery without restart loops.
12. Expose health as an internal callable capability; no HTTP/gRPC listener is
    introduced in Phase 3.5.
13. Use structured safe logs and bounded low-cardinality metrics. Never emit
    secrets, SQL, payloads, authenticated URLs, or repository-private facts.
14. Stop new work, drain tracked work, cancel on deadline, flush telemetry, and
    close capability pools in reverse construction order.
15. Freeze active configuration after startup. Hot reload and pool resizing are
    future decisions.

## Rationale

The runtime is the lowest layer that knows deployment profile and the highest
layer that may own concrete resources. It can therefore configure pgx/TLS and
operational policy without contaminating the adapter or engines.

pgxpool officially provides explicit maximum/minimum idle/lifetime/health
settings, pool statistics, `Ping`, `Reset`, and blocking `Close`. The design
uses those capabilities behind runtime ownership. Because pool construction
returns before a connection is proven, startup explicitly acquires/pings every
required capability pool.

PostgreSQL recommends `verify-full` in security-sensitive environments because
it verifies both trust chain and hostname. The runtime rejects weaker
production modes rather than relying on the insecure compatibility default.

## Alternatives

### Construct the pool inside the PostgreSQL adapter

Rejected. It introduces environment, secret, TLS, deployment, and lifecycle
policy into an adapter whose frozen responsibility is SQL mapping and
transactions.

### Pass one authenticated database URL through every layer

Rejected. It mixes secrets with ordinary settings, is easily logged, prevents
field-level validation/redaction, and couples the runtime to driver syntax.

### Run migrations automatically during startup

Rejected. It gives runtime credentials DDL/migration authority, couples
availability to schema mutation, and contradicts accepted migration governance.

### Let the runtime read Atlas migration history

Rejected. Runtime roles are intentionally excluded. A narrow
migration-maintained compatibility record reveals only the proof the adapter
needs.

### Treat database reachability as liveness

Rejected. A shared database outage would restart healthy processes repeatedly.
Database availability is readiness.

### Use libpq/pgx `sslmode=require` in production

Rejected. It encrypts without guaranteeing server identity. Production uses
certificate and hostname verification.

### Expose an HTTP health endpoint now

Rejected. Health semantics can be implemented and validated without authorizing
a transport/API surface.

### Hot-reload configuration and certificates

Rejected for v1. It adds concurrency and partial-reconfiguration states before
an operational need is measured. Controlled restart is deterministic.

## Consequences

- Runtime packages gain pgxpool and TLS dependencies; persistence and engines
  remain unchanged.
- Production deployment must provide explicit per-capability pool bounds, TLS
  trust, and one secret provider that resolves separate role credentials.
- One additive operational migration will be required before runtime schema
  verification can ship.
- Deployment pipelines retain a separate migration compatibility preflight.
- Database outages make instances not ready but still live.
- Configuration/certificate changes require restart in the first release.
- Observability exporters can be added later without changing health semantics.

## Security Consequences

- No application process receives migrator credentials.
- No runtime role receives object ownership or DDL rights.
- Secret ambiguity and unknown configuration fail closed.
- Production cannot fall back to disabled or non-verifying TLS.
- Metric labels and logs have explicit cardinality/redaction rules.

## Validation Gate

ADR acceptance requires the architecture, configuration, lifecycle,
health/observability, and validation-plan documents to be accepted together.
Acceptance authorizes only the first staged Phase 3.5 implementation milestone.
Phase 3.6 APIs remain unauthorized.

## Authoritative References

- [pgxpool v5.10.0 documentation](https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool)
- [PostgreSQL 18 SSL support](https://www.postgresql.org/docs/18/libpq-ssl.html)
- `docs/API/PERSISTENCE_PORT_V1.md`
- `docs/Decisions/0012-atlas-migration-framework.md`
- `docs/Decisions/0014-pgx-postgresql-adapter.md`
