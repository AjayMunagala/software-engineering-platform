# Runtime Infrastructure Validation Plan

## Status

- Phase: 3.5 design
- Status: Accepted design validation plan on 2026-07-26
- Execution: not authorized by this document
- Credentials: disposable test secrets only; never committed

## Goal

Prove that runtime configuration, secret boundaries, TLS, pool ownership,
schema compatibility, health, observability, startup, and shutdown behave
deterministically and safely without modifying Persistence Port/PostgreSQL
Adapter `1.0.0` or starting product APIs.

## Test Environments

Record for every validation run:

- commit and Go version;
- OS/kernel and architecture;
- CPU, RAM, storage type, and WSL/container limits;
- PostgreSQL exact version and configuration differences;
- pgx version;
- profile and safe non-secret configuration;
- TLS mode and certificate fixture identity (never private key bytes);
- worker/pool limits;
- cold/warm state and sample count.

Required environments:

1. Windows unit/race suite with MSYS2 race support.
2. Ubuntu WSL disposable PostgreSQL 18 integration suite.
3. Linux CI race-capable runner before release.
4. Disposable TLS-enabled PostgreSQL fixture for certificate matrix tests.

No personal, shared, staging, or production database is used for acceptance.

## Configuration Tests

Required table-driven cases:

- defaults only for local and CI;
- every precedence boundary: default/file/environment/CLI;
- same-precedence duplicate rejection;
- unknown JSON field rejection;
- unknown `AEGIS_` environment key rejection;
- malformed JSON and invalid UTF-8;
- explicit duration units and overflow rejection;
- all numeric boundaries and cross-field pool constraints;
- staging/production missing explicit max/min idle;
- production TLS downgrade rejection;
- disabled TLS rejection for non-loopback targets;
- authenticated URL and secret CLI rejection;
- client certificate/key pair validation;
- immutable getters and defensive copies;
- deterministic redacted config view;
- no environment/file mutation after configuration freeze.

Run fuzz tests against strict JSON decoding, durations, host/path validation,
environment name mapping, and redaction. A fuzz failure must never print the
input if classified sensitive.

## Secret Tests

- exactly one provider succeeds;
- missing provider/value fails safely;
- two providers for one secret fail as ambiguous;
- provider cancellation/deadline propagation;
- mounted-file permission enforcement on Unix;
- no secret in errors, logs, metrics, snapshots, panic output, or test reports;
- temporary private buffers become unreachable after capability-pool construction;
- secret rotation has no effect until controlled restart.

Use generated disposable strings and certificates. Secret scanning covers the
working tree, staged diff, test output, and generated validation report.

## TLS Matrix

| Case | Local/CI | Staging/Production |
|---|---|---|
| disabled + loopback/socket | PASS | FAIL |
| disabled + remote host | FAIL | FAIL |
| valid CA + matching hostname | PASS | PASS |
| untrusted CA | FAIL | FAIL |
| expired/not-yet-valid server certificate | FAIL | FAIL |
| hostname mismatch | FAIL | FAIL |
| custom CA | PASS | PASS |
| valid client cert/key when server requires mTLS | PASS | PASS |
| client cert without key | FAIL | FAIL |
| insecure Unix key permissions | FAIL on Unix | FAIL |

Certificates are generated inside the disposable harness. The repository may
commit only generation scripts and public non-sensitive fixtures when needed;
private keys remain temporary and are deleted on exit.

## Pool Ownership Tests

- validated config maps exactly to pgxpool config;
- `MinConns` remains zero;
- capability pool boundaries and role grants match ingest/read/retention;
- max/min idle/lifetime/jitter/idle/health/ping values match configuration;
- total maxima remain inside the global connection budget;
- constructor does not claim readiness without acquire/ping;
- failed startup closes a created pool;
- runtime handle is sole close/reset owner for every pool;
- separate adapter instances receive the existing minimal database capability;
- staging/production never expose a combined over-privileged pool or `Port`;
- repeated close is safe and deterministic;
- pool reset is rate-limited and used only for classified global failures;
- no resource/goroutine leak after 1,000 start/fail/close cycles with a stub;
- concurrent health, metrics, persistence, and shutdown pass race tests.

## Schema Compatibility Tests

Disposable migration states:

- empty database;
- one migration behind compatibility-record creation;
- exact supported schema;
- compatible additive migration;
- incompatible older contract;
- incompatible newer adapter major requirement;
- unknown newer Atlas migration;
- incomplete/failed migration;
- missing, duplicate, or malformed compatibility record;
- runtime role denied the compatibility record;
- runtime role unexpectedly has DDL or Atlas-history access.

Deployment preflight must reject unknown/incomplete revisions. Runtime startup
must reject absent/incompatible proof and must never try to repair it.

Migration validation must prove the compatibility record can be updated only by
the owner/migrator role and selected by the intended runtime roles.

## Lifecycle and Failure Injection

Inject failure after every startup step and verify reverse-order cleanup.
Required cases:

- signal during config load, secret resolution, TLS load, connect, ping,
  compatibility check, and service construction;
- first signal begins draining and removes readiness;
- new work is rejected while existing work finishes;
- zero-work normal shutdown;
- in-flight work completes before deadline;
- drain timeout cancels remaining operations;
- second signal forces shutdown;
- pool close waits for acquired resources but remains bounded by tracked work;
- logger/metric flush success, timeout, and failure;
- repeated concurrent shutdown calls return one stable result;
- post-ready database outage removes/restores readiness without changing
  liveness or restarting the process.

## Health Tests

- liveness independent from database after startup;
- startup readiness requires immediate success;
- three consecutive post-start failures remove readiness;
- one success restores readiness;
- stale successful checks do not remain ready;
- check context timeout and cancellation;
- health checks perform no product writes or migrations;
- concurrent snapshots are immutable and deterministic;
- no endpoint/listener is created.

## Logging and Metric Tests

Capture every startup, failure, health, persistence instrumentation, and
shutdown event, then scan for:

- supplied secrets and substrings;
- connection URLs/DSNs;
- SQL verbs/text and parameters;
- payload bytes/digests;
- repository paths and IDs;
- unbounded driver messages.

Verify required structured fields, stable reason codes, deterministic event
ordering where ordered, rate limiting, and one recovery event.

Metric tests verify:

- exact names/types/units;
- bounded label sets;
- absence of forbidden high-cardinality labels;
- pgxpool counter-delta/reset behavior;
- no overlapping collectors;
- slow/failing sink does not block persistence;
- bytes and duration permit derived throughput;
- no negative counters after pool reset.

## Regression and Dependency Gates

- `go test ./...`;
- `go test -shuffle=on -count=3 ./...`;
- `go vet ./...`;
- targeted and full `go test -race ./...` on capable environments;
- minimum 85% statement coverage per new runtime package;
- repeatable unit and integration benchmarks;
- `git diff --check` and secret scan;
- no runtime import from RIE/LIE engines;
- no engine import of runtime/persistence;
- no change to Persistence Port/PostgreSQL Adapter exported API or version;
- migration checksum validation and disposable install/upgrade tests.

## Performance and Resource Gates

On the recorded reference runner:

| Gate | Target |
|---|---:|
| 64 KiB config compose/validate p95 | < 10 ms |
| redacted view construction p95 | < 1 ms |
| reachable local PostgreSQL startup p95 | < 2 s |
| loopback readiness p95 | < 100 ms |
| zero-work shutdown p95 | < 1 s |
| post-failure retained memory | within 5 MiB of baseline after GC |
| health/metric goroutine growth over 10,000 checks | zero unbounded growth |

Database outage timeout is the configured bound, not a latency regression.
Measure p50, p95, max, standard deviation, allocations, RSS/heap, goroutines,
and connection counts where applicable.

## Documentation and Operations Gates

- package READMEs and configuration reference;
- `.env.example` contains names/placeholders only and passes secret scan;
- local/CI disposable startup and cleanup instructions;
- TLS/certificate provisioning and rotation-by-restart instructions;
- migration preflight, schema incompatibility, and safe recovery runbook;
- readiness/liveness semantics and operator response;
- graceful/forced shutdown runbook;
- profile comparison and production checklist;
- known limitations and deferred features.

## Exit Criteria

Phase 3.5 implementation can be accepted only when:

- all mandatory functional, security, TLS, compatibility, lifecycle, health,
  observability, regression, race, and resource gates pass;
- no credential appears in Git or evidence;
- startup never runs migrations or publishes readiness prematurely;
- Persistence Port/PostgreSQL Adapter `1.0.0` remain unchanged;
- only documented known limitations remain;
- engineering explicitly accepts the evidence.

Only that acceptance may authorize Phase 3.6 API design.
