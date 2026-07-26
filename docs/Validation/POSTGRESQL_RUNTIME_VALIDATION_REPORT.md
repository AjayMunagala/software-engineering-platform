# Phase 3.5.2 PostgreSQL Runtime Validation Report

- Date: 2026-07-26
- Milestone: Phase 3.5.2 — PostgreSQL Runtime
- Candidate contract: `0.1.0`
- Local exit gate: PASS
- Engineering acceptance: accepted on 2026-07-26
- Phase 3.5.3 authorization: granted by subsequent engineering decision

## Scope

Phase 3.5.2 implements only PostgreSQL runtime resource ownership downstream
from accepted runtime configuration and upstream from PostgreSQL Adapter
`1.0.0`.

Implemented:

- bounded TLS material loading and certificate validation;
- verify-full TLS transport enforcement;
- one combined pool for local/CI;
- independent ingest/read/retention pools for staging/production;
- `MinConns = 0` and exact approved pool configuration mapping;
- immediate pool ping;
- PostgreSQL 18 and session-setting verification;
- migration-maintained compatibility proof;
- positive and negative capability privilege proof;
- narrow adapter routing;
- reverse-order cleanup and idempotent close;
- disposable PostgreSQL 18 integration with disabled and verify-full TLS.

Excluded:

- health and readiness state machines;
- observability/logging/metrics implementation;
- application startup coordinator, admission, and drain;
- APIs, listeners, UI, authentication, and business logic;
- runtime migrations or Atlas-history access;
- persistence or intelligence-engine contract changes.

## Migration Evidence

Migration `202607260008_create_runtime_compatibility.sql` creates a singleton,
owner-maintained compatibility record. Runtime roles receive only `SELECT`.
They cannot insert, update, delete, perform DDL, or read Atlas history.

| Metric | Result |
|---|---:|
| Atlas | 1.2.3 |
| PostgreSQL | 18.4 |
| Migration files | 8 |
| Schema tables | 12 |
| Constraints | 198 |
| Indexes | 35 |
| Capability roles | 9 |
| Empty install | 1,311 ms |
| No-op apply | 404 ms |

All checksum, tamper, empty-install, partial-upgrade, repeat-apply,
newer-database, rollback, advisory-lock, schema, ownership, and privilege gates
passed.

## Functional Validation

| Gate | Result |
|---|---|
| Combined local/CI pool | PASS |
| Separate production capability pools | PASS |
| Exact configuration mapping | PASS |
| Verify-full custom CA connection | PASS |
| TLS transport mismatch rejection | PASS |
| CA and client-certificate validity checks | PASS |
| Encrypted client-key rejection | PASS |
| Immediate ping | PASS |
| PostgreSQL major/session proof | PASS |
| Compatibility proof | PASS |
| Same proof across all pools | PASS |
| Capability privilege proof | PASS |
| Cancellation/timeout classification | PASS |
| Stable redacted errors | PASS |
| Idempotent close | PASS |
| 1,000 open/close cycles | PASS; 1,000 created, 1,000 closed |

The specifically requested partial-startup failure passed:

```text
create ingest: PASS
create read: PASS
create retention: FAIL
cleanup: close read, then close ingest
returned runtime: nil
driver/secret detail in error: none
```

## Disposable PostgreSQL Integration

The Ubuntu 24.04 WSL harness creates PostgreSQL 18.4 under `/tmp`, applies the
accepted migrations, creates disposable login roles, generates a one-use CA
and server certificate, and invokes the Windows Go 1.26.2 tests. Cleanup
removes the cluster, database, roles, certificate keys, certificates, and data.

| Integration case | Result | Observed test duration |
|---|---|---:|
| CI combined pool, TLS disabled, loopback | PASS | 0.03 s |
| Production separate pools, verify-full TLS | PASS | 0.30 s |

No personal PostgreSQL database, persistent local tables, user credentials, or
committed secrets were used.

## Regression, Race, and Coverage

| Gate | Result |
|---|---|
| Target package tests | PASS |
| Full backend regression | PASS |
| `go vet ./...` | PASS |
| Three shuffled full-backend repetitions | PASS |
| Targeted Windows race | PASS |
| Full Windows backend race | PASS |
| Data races | 0 |
| Runtime package statement coverage | 85.9% |

Windows race builds used Go 1.26.2 with MSYS2 UCRT64 GCC 16.1.0 and
`CGO_ENABLED=1`. Ubuntu supplied the disposable PostgreSQL 18.4 server and
Atlas 1.2.3 migration environment.

## Benchmark

Reference client environment:

- OS: Windows 11 10.0.26200.8894;
- CPU: 12th Gen Intel Core i5-12450H;
- Go: 1.26.2 windows/amd64;
- benchmark: complete combined-runtime construction and proof using a
  deterministic in-memory database capability; configuration loading is
  outside the timed loop.

Five repetitions:

| Range | Bytes/op | Allocations/op |
|---:|---:|---:|
| 42.042–70.410 µs/op | 14,098–14,105 | 130 |

Network and TLS handshake latency are reported separately by the disposable
integration cases because they are environment-dependent.

## Defects Found During Validation

### Least-privilege TLS proof

Initial validation attempted to read the current connection from
`pg_stat_ssl` after `SET ROLE`. PostgreSQL correctly hides statistics rows from
the reduced capability role, producing a false negative.

Resolution: TLS is now proved in pgx's post-negotiation transport callback by
requiring a `*tls.Conn` for verify-full profiles and a plain connection for
disabled-TLS profiles. SQL no longer depends on statistics-view access. The
disposable TLS probe and all runtime integration tests pass after this change.

### Cross-environment CA permissions

The first disposable CA was placed below a PostgreSQL-owned `0700` directory,
so the Windows client could not traverse the WSL path. Only the public CA path
was made traversable/readable; server and CA private keys remain restricted
and are deleted at exit.

## Security Review

- no authenticated URLs or credentials are committed;
- password bytes are cleared immediately after pgx configuration construction;
- public errors contain stable schema-owned codes only;
- SQL, driver messages, paths, addresses, users, payloads, and certificate
  contents are not formatted into errors;
- migration and runtime responsibilities remain separate;
- no engine imports runtime or persistence;
- no runtime code receives migrator/owner rights.

## Governance Decision Requested

Engineering accepted the Phase 3.5.2 implementation and validation evidence on
2026-07-26. Commit and push are approved. Phase 3.5.3 Runtime Lifecycle &
Health is authorized; later milestones remain separately gated.
