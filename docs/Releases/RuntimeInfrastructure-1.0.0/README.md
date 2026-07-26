# Runtime Infrastructure 1.0.0

## Release

- Version: `1.0.0`
- Release date: 2026-07-27
- Status: accepted and frozen

## Included contracts

- Runtime Configuration `1.0.0`
- PostgreSQL Runtime `1.0.0`
- Runtime Health `1.0.0`
- Application Runtime Lifecycle `1.0.0`
- Runtime Observability `1.0.0`

## Included capabilities

- deterministic configuration precedence and strict validation;
- external secret-provider boundary and redacted views;
- verify-full TLS policy for staging and production;
- capability-specific pgx pools and schema/privilege proof;
- monotonic lifecycle, admission, drain, forced cancellation, and cleanup;
- callable liveness and readiness without a listener;
- typed structured events with no arbitrary fields or raw errors;
- bounded, low-cardinality metric snapshots and non-overlapping collection;
- native Windows and Linux race validation;
- disposable PostgreSQL 18.4 and TLS integration validation.

## Compatibility

The runtime consumes Persistence Port `1.0.0`, PostgreSQL Adapter `1.0.0`, and
the migration-maintained persistence schema contract `1.0.0`. It does not
change those frozen contracts.

The `1.x` line preserves existing public contract behavior.
New optional capabilities require additive interfaces. Breaking semantic or
signature changes require a new major version.

## Explicit exclusions

- HTTP, REST, gRPC, GraphQL, and Kubernetes probe endpoints
- authentication and authorization
- API listeners and UI
- business logic and AI orchestration
- intelligence-engine changes
- runtime migrations
- exporter-specific protocols
- committed credentials or production configuration

## Evidence

- [Phase 3.5.4 validation report](../../Validation/RUNTIME_INTEGRATION_FREEZE_VALIDATION_REPORT.md)
- [Deployment runbook](../../Operations/RUNTIME_DEPLOYMENT_RUNBOOK.md)
- [Operator release checklist](../../Operations/RUNTIME_RELEASE_CHECKLIST.md)
- [Accepted runtime architecture](../../Architecture/RUNTIME_INFRASTRUCTURE.md)
- [Accepted health and observability specification](../../Architecture/HEALTH_OBSERVABILITY_SPECIFICATION.md)

## Release tags

- `runtime-infrastructure/v1.0.0`
- `runtime-configuration/v1.0.0`
- `postgresql-runtime/v1.0.0`
- `runtime-health/v1.0.0`
- `application-runtime/v1.0.0`
- `runtime-observability/v1.0.0`

All tags identify the same accepted release commit.
