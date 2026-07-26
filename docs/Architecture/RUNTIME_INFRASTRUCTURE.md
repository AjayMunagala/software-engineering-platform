# Runtime Infrastructure Architecture

## Status

- Phase: 3.5 — Runtime Infrastructure
- Status: Accepted on 2026-07-26
- Design date: 2026-07-26
- Implementation: Phase 3.5.1 only authorized after design acceptance
- Frozen dependencies: Persistence Port `1.0.0`, PostgreSQL Adapter `1.0.0`

## Purpose

The runtime infrastructure deterministically loads and validates operational
configuration, constructs and owns PostgreSQL connectivity, proves schema
compatibility, exposes internal health and observability signals, wires the
frozen adapter, and shuts down safely without changing an intelligence engine
or persistence contract.

## Architectural Boundary

```text
Command / process host
        |
        v
Application bootstrap and lifecycle owner
        |
        +--> Runtime configuration (immutable after startup)
        +--> Secret provider (external values, redacted)
        +--> Structured logger and metric sink
        +--> TLS material loader
        +--> pgxpool.Pool owner
        +--> Schema compatibility verifier
        +--> Health evaluator
        |
        v
PostgreSQL Adapter 1.0.0
        |
        v
Persistence Port 1.0.0
```

Dependency direction is one-way. The adapter continues to accept the minimal
database capability defined in `backend/internal/storage/postgres`; it does not
construct, configure, monitor, or close a pool.

## Responsibilities

Phase 3.5 owns:

- configuration source composition, precedence, validation, and redaction;
- external secret resolution at process startup;
- PostgreSQL connection and TLS configuration;
- pool construction, warm-up, metrics, reset policy, and closure;
- application startup, readiness, draining, and shutdown state;
- runtime schema and adapter compatibility verification;
- structured operational logging and bounded metrics;
- callable liveness and readiness checks;
- local, CI, staging, and production profile rules;
- deployment preflight and safe failure diagnostics.

## Explicitly Out of Scope

- modifying Persistence Port or PostgreSQL Adapter `1.0.0`;
- adding persistence operations, tables for product facts, or artifact fields;
- running migrations automatically at application startup;
- REST, gRPC, GraphQL, or health HTTP endpoints;
- React or any user interface;
- authentication or authorization-policy design;
- AI orchestration, prompts, agents, or models;
- business workflows or repository scheduling;
- intelligence-engine changes;
- source-code or artifact mutation;
- committed credentials or production topology.

Health is a callable runtime capability in this phase. A future API or platform
integration may expose it over a network only after that transport is approved.

## Ownership

| Concern | Owner | Forbidden dependency |
|---|---|---|
| Configuration schema and merge | runtime configuration package | persistence and engines |
| Secret resolution | application bootstrap boundary | persistence and adapter |
| TLS construction | runtime PostgreSQL package | persistence |
| Pool lifecycle | application runtime | adapter and engines |
| SQL persistence behavior | PostgreSQL adapter `1.0.0` | runtime policy |
| Schema evolution | Atlas migration framework | runtime application |
| Compatibility proof | migration-maintained record + runtime verifier | engine artifacts |
| Health state | runtime health package | network transport |
| Logging and metrics contracts | runtime observability package | product APIs |

The application bootstrap is the sole owner of created resources. Ownership is
represented by a runtime handle that closes resources in reverse construction
order. No package-level global pool, logger, configuration, or mutable health
state is permitted.

## Proposed Package Direction

```text
backend/internal/runtime/config/          loading, merge, validation, redaction
backend/internal/runtime/postgres/        TLS, pool factory, compatibility proof
backend/internal/runtime/health/          liveness/readiness state and checks
backend/internal/runtime/observability/   logger/metric-neutral contracts
backend/internal/runtime/app/             startup, ownership, drain, shutdown
```

Each package follows the project package standard: `interface.go`,
`implementation.go`, `config.go`, `model.go`, `errors.go`, `README.md`, unit
tests, and benchmarks. Integration fixtures live in an explicit `tests/`
directory and use only disposable databases and certificates.

No new command is frozen by this design. Phase 3.5 implementation may propose a
non-networked operational command for configuration/health verification, but
its spelling requires implementation review.

## Runtime State Machine

```text
new
  -> loading
  -> validating
  -> connecting
  -> compatibility-checking
  -> ready
  -> draining
  -> stopped

Any pre-ready failure -> failed
Any forced shutdown  -> stopping -> stopped
```

Transitions are monotonic. `ready` is published only after a real pool
connection and compatibility proof succeed. A database outage after readiness
removes readiness but does not make process liveness false.

## Pool Ownership Decision

Application runtime constructs a capability pool set and passes each pool to a
separate adapter instance through the existing `Database` capability. The
runtime owns `Ping`, statistics, reset, and `Close` for every pool.

Staging and production preserve the accepted PostgreSQL capability roles:

| Pool | Database capability role | Exposed neutral capabilities |
|---|---|---|
| ingest | `platform_ingestor` | repository/scan lifecycle, staging, publication |
| read | `platform_artifact_reader` | artifact metadata/exact reads and verification |
| retention | `platform_retention_worker` | purge and garbage collection |

Although each internal adapter type implements the complete `Port`, runtime
wiring exposes only the narrow interfaces authorized for that pool. Ordinary
consumers do not receive a combined `Port`. Local/CI may collapse pools through
one disposable login granted the required roles; staging/production may not.

The design uses `MinIdleConns`, not `MinConns`, for warm capacity. pgx documents
`MinIdleConns` as the better tail-latency control. `MinConns` remains zero in
v1 to avoid two competing minimum policies.

Frozen initial policies:

| Setting | Local/CI | Staging/Production |
|---|---:|---:|
| total `MaxConns` | 4 | required explicit value, 3–64 across enabled pools |
| `MinConns` | 0 | 0 |
| `MinIdleConns` | 0 | required explicit value, 0–`MaxConns` |
| max lifetime | 1 hour | 1 hour unless approved override |
| lifetime jitter | 5 minutes | 5 minutes |
| max idle time | 30 minutes | 30 minutes |
| health-check period | 1 minute | 1 minute |
| ping timeout | 5 seconds | 5 seconds |
| startup connection timeout | 10 seconds | 10 seconds |

Each enabled production pool has at least one connection and its own explicit
maximum/minimum-idle budget; their maximum sum cannot exceed the configured
database budget. Values are immutable after startup. Pool resizing and hot
configuration reload are deferred until measured operational requirements
exist.

## Schema Compatibility Boundary

Runtime roles remain denied access to `atlas_schema_revisions`. Deployment
preflight continues to run the accepted Atlas checksum and compatibility
checks using the migrator principal outside the application process.

Runtime startup additionally needs a least-privilege proof. Phase 3.5
implementation therefore proposes one additive operational migration creating a
singleton compatibility record under `platform`, owned and updated only by
`platform_owner`/`platform_migrator`, with `SELECT` granted to approved runtime
roles. It contains no credentials or product data and does not change the
Persistence Port.

The record identifies:

- PostgreSQL schema contract name and version;
- minimum and maximum compatible adapter major;
- migration revision that published the record;
- immutable update timestamp.

Startup fails closed if the record is absent, duplicated, incomplete, newer
than supported, older than required, or incompatible with Adapter `1.0.0`.
The application never inserts or updates this record and never applies a
migration.

## Security Invariants

- TLS is disabled only for `local` or `ci` with loopback/Unix-socket targets.
- Staging and production require certificate and hostname verification.
- Secret values are resolved after ordinary configuration validation and are
  never copied into public configuration views.
- No URL containing a password is logged, returned, or persisted.
- SQL text, payload bytes, digests, repository paths, and credentials are not
  operational log fields.
- Runtime database principals receive only approved capability roles and no
  object ownership, DDL, role creation, or migration authority.
- Unknown configuration keys fail startup.

## Design Deliverables

- `RUNTIME_CONFIGURATION_SPECIFICATION.md`;
- `RUNTIME_LIFECYCLE_SPECIFICATION.md`;
- `HEALTH_OBSERVABILITY_SPECIFICATION.md`;
- ADR 0015;
- `RUNTIME_INFRASTRUCTURE_VALIDATION_PLAN.md`.

## Implementation Gates

The design is accepted only as a set. Acceptance authorizes a separately gated
Phase 3.5 implementation plan, not all runtime code at once. At minimum,
implementation should remain staged as configuration, PostgreSQL runtime,
lifecycle/health/observability, then integrated validation.

Phase 3.6 APIs remain unauthorized until Phase 3.5 implementation, security,
failure recovery, race, and deployment evidence are accepted.
