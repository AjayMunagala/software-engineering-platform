# Health and Observability Specification

## Status

- Phase: 3.5 design
- Status: Accepted on 2026-07-26
- Network endpoints: out of scope
- Default logging implementation: Go structured logging (`log/slog`)

## Principles

- Health answers whether the process should run or receive work; it is not an
  authorization or product API.
- Logs explain events, metrics measure behavior, and health reports current
  state. The three signals do not duplicate full payloads.
- All names and labels are bounded and low-cardinality.
- Secrets, SQL text, source content, artifact bytes, authenticated URLs, and
  private repository details never enter telemetry.
- Telemetry failure must not corrupt persistence behavior. Required startup
  logging failures may block startup; optional metric-export failures may not.

## Health Model

```text
HealthSnapshot
  ObservedAt
  RuntimeState
  Liveness
    Status
    ReasonCode
  Readiness
    Status
    ReasonCode
    LastSuccessfulDatabaseCheck
    LastSuccessfulCompatibilityCheck
```

Status is one of `healthy`, `unhealthy`, or `unknown`. Reason codes are stable
machine values. Human messages are safe and bounded.

## Liveness

Liveness is healthy when:

- the process lifecycle has not entered `failed` or `stopped`;
- the lifecycle coordinator can make progress;
- no internal invariant or unrecoverable resource-owner failure is present.

Database reachability, pool saturation, schema readiness, and downstream
service failure do not make liveness unhealthy after startup. This avoids a
database outage causing process restart loops.

Liveness becomes unhealthy for an internal deadlock watchdog, impossible state
transition, corrupted immutable runtime state, or failed mandatory lifecycle
worker.

## Readiness

Readiness is healthy only when:

- lifecycle state is `ready`;
- configuration and secret/TLS construction succeeded;
- an acquire/ping succeeds inside `Health.CheckTimeout`;
- schema compatibility proof matches Adapter `1.0.0`;
- the required runtime role privileges remain available;
- every required capability pool is open and not draining;
- the most recent successful checks are fresher than the configured interval
  plus one tolerance interval.

Pool saturation alone does not immediately fail readiness. An actual bounded
acquire/ping failure does. Default policy uses three consecutive failures to
remove post-start readiness and one success to restore it. Startup requires one
success and has no failure grace period.

## Health Execution

Phase 3.5 exposes callable interfaces only:

```text
Liveness(ctx) -> HealthSnapshot
Readiness(ctx) -> HealthSnapshot
```

The caller supplies a context; the checker applies the smaller configured
timeout. Calls are safe concurrently, do not mutate product data, and do not
run migrations. Results are detached immutable values.

A future HTTP/Kubernetes/CLI projection must consume this interface rather than
reimplementing health rules. No listener or endpoint is authorized here.

## Structured Logging

The runtime uses `log/slog` semantics behind a small runtime-owned logger
capability. Application packages receive only the capability they need, not a
global logger.

Required common fields:

```text
timestamp
level
service
component
event
outcome
duration_ms (when applicable)
correlation_id (when supplied)
error_kind (safe category only)
runtime_state (lifecycle events)
```

Allowed optional fields include bounded counts, byte counts, artifact contract
name/version, schema contract version, adapter version, and deployment profile.

Forbidden fields include:

- passwords, tokens, private-key material, secret references, or hashes of
  secrets;
- connection URLs, DSNs, or combined host/user/database strings;
- SQL statements, parameters, SQLSTATE details, or constraint names;
- source code, artifact payloads, projection JSON, repository paths, file
  paths from analyzed projects, or commit messages;
- certificate contents;
- unbounded error text or stack traces from drivers.

Stack traces, when enabled for internal defects, remain process-local and must
pass the same redaction boundary before export.

## Log Levels

| Level | Use |
|---|---|
| debug | bounded lifecycle details; local opt-in only |
| info | startup milestones, ready/draining/stopped, compatibility success |
| warn | transient readiness loss, retry, pool reset, approaching saturation |
| error | startup failure, persistent readiness failure, forced shutdown |

Successful per-query logs are disabled by default. They create volume and can
leak operation timing patterns. Aggregated metrics own routine performance.

## Correlation IDs

Correlation IDs are validated machine identifiers supplied through context by
application orchestration. Infrastructure may generate one for startup and
shutdown sequences. They are not repository, scan, artifact, user, credential,
or database identifiers. Missing correlation is allowed for periodic metrics
and health work.

## Metrics

Metric names below are the proposed stable semantic names. Exporter-specific
formatting is deferred.

| Metric | Type | Labels |
|---|---|---|
| `aegis_runtime_health` | gauge 0/1 | `kind=liveness|readiness` |
| `aegis_runtime_state` | one-hot gauge | bounded `state` |
| `aegis_db_pool_connections` | gauge | `pool=ingest|read|retention`, `state=acquired|idle|constructing|total|max` |
| `aegis_db_pool_acquire_seconds` | histogram | bounded `pool`, `outcome=success|timeout|canceled|error` |
| `aegis_db_pool_acquire_total` | counter | bounded `pool`, `outcome` |
| `aegis_db_pool_empty_wait_seconds_total` | counter | bounded `pool` |
| `aegis_db_pool_connections_created_total` | counter | bounded `pool` |
| `aegis_db_pool_connections_destroyed_total` | counter | bounded `pool`, `reason=idle|lifetime|reset` |
| `aegis_persistence_operation_seconds` | histogram | bounded `operation`, `outcome` |
| `aegis_persistence_errors_total` | counter | `operation`, neutral `error_kind` |
| `aegis_persistence_payload_bytes_total` | counter | `direction=stage|export` |
| `aegis_persistence_publication_seconds` | histogram | bounded `outcome` |
| `aegis_schema_compatibility` | gauge 0/1 | none |
| `aegis_health_check_seconds` | histogram | `kind`, bounded `outcome` |

Stage throughput is derived from payload bytes and stage duration rather than
published as a noisy instantaneous gauge.

Repository IDs, scan IDs, artifact IDs/digests, scope IDs, database host, user,
query text, error messages, and correlation IDs are forbidden metric labels.

## Pool Metrics Source

The runtime takes snapshots from each pool's `pgxpool.Stat`, which exposes acquired, idle,
constructing, total/max connections and cumulative acquire/wait/create/destroy
counters. Delta counters are calculated under synchronization and reset safely
when the underlying pool is replaced or reset.

Metric collection defaults to every 15 seconds, is cancellable, and stops
before pool closure. One collection may not overlap the next. A slow exporter
drops/coalesces snapshots rather than blocking persistence operations.

## Error and Diagnostic Stability

Runtime reason codes are separate from Persistence Port `ErrorKind` but may
carry a neutral error kind as evidence. Proposed runtime categories:

```text
config_invalid
secret_unavailable
tls_invalid
database_unavailable
database_timeout
schema_missing
schema_incompatible
privilege_incompatible
runtime_draining
runtime_internal
```

Ordering is deterministic. Repeated identical health failures are rate-limited
in logs while counters continue to increment. Recovery emits one info event.

## Environment Defaults

| Profile | Log format/level | Metrics | Health cadence |
|---|---|---|---|
| local | text/info; debug opt-in | enabled in-process | 15 s |
| ci | JSON/info | enabled and captured | 5 s |
| staging | JSON/info | required sink | 15 s |
| production | JSON/info or stricter | required sink | 15 s |

Production cannot enable unredacted debug logging. Failure to configure the
required production log sink blocks startup; temporary metric-export failure
removes observability health but not persistence readiness unless resource use
becomes unsafe.

## Shutdown Observability

Shutdown records:

- transition to draining;
- admitted/in-flight counts;
- drain duration and timeout outcome;
- canceled remaining operation count;
- pool close completion;
- logger/metric flush outcome;
- final exit category.

It does not record operation payloads or caller identities. Observability sinks
flush before their own closure and after work/pool shutdown events are recorded.
