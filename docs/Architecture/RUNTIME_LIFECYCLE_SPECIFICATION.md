# Runtime Lifecycle Specification

## Status

- Phase: 3.5 design
- Status: Accepted on 2026-07-26
- Lifecycle owner: application runtime bootstrap
- Automatic migrations: prohibited

## Lifecycle Invariants

1. Construction has one owner and one reverse-order cleanup path.
2. Readiness is never published before a real database connection and schema
   compatibility proof succeed.
3. A startup failure cleans up every resource already created.
4. Shutdown stops new work before waiting for in-flight work.
5. The adapter never closes or resets the pool supplied to it.
6. The application never applies, repairs, or rolls back a migration.
7. Repeated shutdown requests are idempotent; a second operating-system signal
   may request forced termination.

## Startup Sequence

```text
1. Establish startup deadline and signal handling
2. Load ordinary configuration sources
3. Validate and freeze ordinary configuration
4. Resolve secrets into private startup input
5. Construct structured logger and metric sink
6. Load/validate TLS material
7. Build per-capability pgxpool configs from typed fields
8. Construct ingest, read, and retention pools in deterministic order
9. Acquire/Ping every required pool under connection timeout
10. Verify PostgreSQL major and session invariants
11. Verify migration-maintained schema compatibility record
12. Verify required runtime privileges without mutation
13. Construct PostgreSQL Adapter 1.0.0 instances with pool capabilities
14. Route only narrow neutral capabilities to application services
15. Publish ready state
```

pgxpool construction does not prove a connection was established. The explicit
acquire/ping gate is mandatory. Startup uses one bounded parent deadline and
smaller per-step deadlines; a step may not extend the parent.

## Startup Session Invariants

Every new connection must use:

- UTC time zone;
- approved `application_name`;
- UTF-8 client encoding;
- an approved least-privilege runtime role;
- TLS state matching the selected profile;
- no elevated role, object ownership, or migration capability.

The pool `AfterConnect` hook may establish deterministic safe session settings.
It may not run DDL, migrations, artifact queries, or application callbacks.

## Schema Compatibility

Compatibility has two independent gates.

### Deployment preflight

The deployment pipeline runs the accepted Atlas checksum and compatibility
scripts using a dedicated migrator principal. It proves:

- committed migration bytes match `atlas.sum`;
- the database has no unknown or incomplete migration revision;
- the database is not newer than the deployment migration set;
- pending migrations are explicitly visible to the operator.

Production runtime startup never substitutes for this gate and never possesses
the migrator credential.

### Runtime proof

Using the runtime role, startup reads one migration-maintained compatibility
record and verifies:

- contract key is exactly the expected PostgreSQL persistence contract;
- schema contract version is compatible with Persistence Port `1.0.0`;
- PostgreSQL Adapter major `1` is inside the declared compatible range;
- the record is complete and singular;
- required platform relations are visible;
- required role privileges support the configured runtime capability set.

Missing, old, new, malformed, or inaccessible proof is a readiness-blocking
startup failure. Diagnostics report safe expected/actual version identifiers,
not connection data or SQL.

## Compatibility Record Governance

The proposed operational table is not product data and is not accessible
through the Persistence Port. Only a migration may create or update it. A
future schema migration that changes adapter-visible behavior must update the
record in the same migration transaction.

Conceptual fields:

```text
contract_key
schema_contract_version
minimum_adapter_major
maximum_adapter_major
migration_revision
published_at
```

The implementation design and additive migration require separate approval.

## Runtime Handle

Successful startup returns one owning runtime handle containing only private
resource references and narrow getters:

- frozen redacted configuration view;
- persistence capability provider;
- health evaluator;
- observability snapshot provider;
- `Drain` and `Close` lifecycle operations.

The handle does not expose `*pgxpool.Pool` to engines or ordinary business
components. Infrastructure tests may use an internal pool inspection
capability.

## Work Admission and Draining

The runtime owns admission gates and in-flight counters around application
operations. Draining proceeds as follows:

1. atomically transition `ready -> draining`;
2. reject new admitted work with a safe unavailable/draining error;
3. make readiness false immediately;
4. wait for tracked in-flight work up to `DrainTimeout` (default 30 seconds);
5. cancel remaining operation contexts when the timeout expires;
6. wait up to `ForcedShutdownTimeout` (default 5 seconds) for cancellation;
7. flush owned observability sinks;
8. close retention, read, then ingest pools in reverse construction order;
9. transition to `stopped`.

The order matters because pgxpool `Close` blocks until acquired connections are
returned. Work tracking prevents an unbounded normal shutdown. If forced
shutdown expires, the process exits non-zero after recording only safe counts
and durations.

## Signals

- First termination signal begins graceful drain.
- Repeated first-class shutdown calls return the same completion result.
- Second termination signal cancels the drain and enters forced shutdown.
- Fatal startup errors never publish readiness.
- Recoverable database health failures after startup remove readiness and allow
  recovery; they do not automatically terminate the process.

Signal names remain operating-system-specific; the lifecycle model is not.

## Failure Matrix

| Failure | State | Cleanup | Diagnostic |
|---|---|---|---|
| invalid config | failed | none required | safe config code + field |
| secret unavailable | failed | erase private buffers | provider/category only |
| TLS material invalid | failed | erase private buffers | safe certificate role/path category |
| pool construction error | failed | close all partial pools | database unavailable category |
| initial ping timeout | failed | close all pools | timeout/unavailable |
| unsupported PostgreSQL major | failed | close all pools | expected/actual major |
| schema incompatibility | failed | close all pools | safe schema versions |
| adapter construction error | failed | close all pools | adapter/config category |
| post-ready database outage | not ready | retain/reset pool by policy | health reason code |
| drain timeout | stopping | cancel work, close after return | counts/duration only |

## Pool Reset Policy

The runtime may reset one affected pool only after a classified server-state or
network event that can invalidate all of its connections. Reset is rate-limited
and observable.
Individual query or integrity errors do not reset the pool. The adapter cannot
request reset through the Persistence Port.

## Startup and Shutdown Targets

On the reference local runner:

- configuration load/validation p95 below 10 ms for a 64 KiB file;
- runtime construction with reachable local PostgreSQL p95 below 2 seconds;
- failed local connection bounded by the configured 10-second connection
  timeout;
- readiness evaluation p95 below 100 ms on loopback;
- normal zero-work shutdown below 1 second;
- memory retained after failed startup returns to within 5 MiB of baseline
  after garbage collection in the validation harness.

Targets are acceptance gates only after the implementation environment and
sample methodology are recorded.

## No Hidden Startup Side Effects

Startup may read configuration, secret/certificate files, DNS, the database
health/compatibility facts, and operating-system signals. It may create network
connections and structured operational telemetry.

It may not scan repositories, execute repository code, run migrations, create
product records, publish artifacts, start an API listener, contact an AI model,
or mutate source code.
