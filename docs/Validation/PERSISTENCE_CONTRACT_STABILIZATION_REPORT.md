# Persistence Contract 1.0 Stabilization Report

## Status

- Phase: 3.4.4 — Adapter Validation and Freeze
- Evidence date: 2026-07-26
- Persistence Port: `1.0.0`
- PostgreSQL Adapter: `1.0.0`
- Result: accepted
- Governance: engineering acceptance recorded on 2026-07-26
- Phase 3.5: authorized

## Scope

This phase freezes behavior rather than adding persistence capabilities. It
reviews the storage-neutral Go API, PostgreSQL adapter conformance, immutable
models, safe errors, operational limits, dependency direction, security,
documentation, performance, memory, regression, and race evidence.

It does not add runtime connection configuration, credentials, connection-pool
policy, application orchestration, APIs, UI, authentication, or engine logic.

## Release-Candidate Refinements

Final review made four pre-1.0 corrections:

1. `MaxPayloadBytes` can no longer be configured above the accepted 4 GiB
   operational boundary. The 8 GiB database ceiling is physical capacity, not
   an approved runtime limit.
2. The unused public `ErrContextRequired` sentinel was removed. Go callers must
   follow the standard non-nil `context.Context` convention; adapters preserve
   cancellation and deadline errors.
3. The unnecessary exported `EqualJSON` helper was removed. Projection
   integrity remains exact-byte SHA-256 validation; persistence values are not
   a JSON wire API.
4. `ContractVersion`, `AdapterVersion`, and a compile-time
   `persistence.Port` implementation assertion were added.

The projection benchmark fixture was also corrected from a JSON array to the
JSON object required by the accepted projection contract. Production behavior
did not change.

## Public API Review

| Area | Decision |
|---|---|
| Capability interfaces | Freeze proposed: seven narrow interfaces plus composed `Port` |
| Generic transaction API | Excluded |
| Engine/artifact types | Excluded; orchestration supplies detached neutral values |
| Requests and records | Immutable through private fields and defensive copies |
| Payload I/O | Caller-owned streaming `io.Reader`/`io.Writer` |
| Scope | Required on all 18 public operations |
| Errors | Stable neutral kinds; SQLSTATE and driver text remain private |
| Operational payload maximum | 4 GiB |
| Physical schema ceiling | 8 GiB; not caller-configurable through v1 |
| Existing interface extension | Breaking; optional capabilities require new interfaces |
| JSON contract | None for neutral Go values |

The proposed `1.x` compatibility policy is recorded in
`docs/API/PERSISTENCE_PORT_V1.md`. Existing method sets, signatures,
error meanings, validation behavior, and zero-value defaults remain compatible
through `1.x`. Adding a method to an existing interface is explicitly treated
as breaking for third-party adapters.

## Conformance and Correctness

| Gate | Result |
|---|---|
| Neutral conformance before adapter-specific tests | PASS |
| Scope isolation across 18/18 operations | PASS |
| Exact payload round trip and SHA-256 recheck | PASS |
| Fixed ordered 4 MiB chunks | PASS |
| Short-stream rollback and successful retry | PASS |
| Idempotency and conflict semantics | PASS |
| Atomic publication and current-scan update | PASS |
| Failed publication invisibility | PASS |
| Stored corruption detection | PASS |
| Concurrent same-digest staging | PASS |
| Dependency/projection/diagnostic/statistic persistence | PASS |
| Pagination and lifecycle operations | PASS |
| Retention and delayed garbage collection | PASS |
| Safe SQLSTATE translation | PASS |
| Adapter compile-time conformance assertion | PASS |

## Regression and Race Evidence

| Gate | Result |
|---|---|
| Full backend tests | PASS |
| Full backend shuffled tests, three repetitions | PASS |
| Full backend `go vet` | PASS |
| Full Windows race test with MSYS2 UCRT64 GCC | PASS — zero races |
| Linux adapter race test | PASS — zero races |
| Neutral persistence coverage | 86.0% |
| Conformance coverage | 88.1% |
| PostgreSQL integration coverage | 85.1% |

## Performance Comparison

The Phase 3.2 physical-schema spike and Phase 3.4.4 adapter test use different
client implementations and therefore are compared against gates, not treated
as statistically interchangeable samples.

| Kubernetes-scale metric | Phase 3.2 accepted baseline | Phase 3.4.4 adapter |
|---|---:|---:|
| Exact bytes | 1,556,379,091 | 1,556,379,091 |
| Ordered 4 MiB chunks | 372 | 372 |
| Stage throughput | minimum 52.42 MiB/s | 215.32 MiB/s |
| Verified-read throughput | minimum 499.64 MiB/s | 931.75 MiB/s |
| End-to-end adapter test | not comparable | 9.33 s |
| Timed process maximum RSS | client delta 46.91 MiB | 176,760 KiB total RSS |

The adapter exceeds the accepted warm stage/read throughput gates. The RSS
figures use different baselines, so no percentage improvement is claimed.

Windows helper benchmarks, five repetitions:

| Benchmark | Result |
|---|---:|
| 100-artifact publication construction | 14.92–15.69 µs/op, 44,647–44,648 B/op, 107 allocs/op |
| 1 MiB projection validation and defensive copy | 5.46–5.95 ms/op, 176.21–192.07 MiB/s, about 1.05 MiB/op |
| 18-operation conformance catalogue | 70.97–79.51 ns/op, 288 B/op, 1 alloc/op |
| 256-entry manifest digest | 6.552–6.846 µs/op, 32 B/op, 1 alloc/op |
| 4 GiB chunk-count calculation | 0.134–0.192 ns/op, zero allocations |

Publication construction improved from the Phase 3.4.2 recorded range of
24.64–50.56 µs/op. Projection validation remains bounded and includes full
JSON-object validation, digest verification, and defensive copying.

## Dependency and Security Audit

- pgx, `database/sql`, and SQL statements remain confined to
  `backend/internal/storage/postgres`.
- `backend/persistence`, RIE, and LIE contain no PostgreSQL dependency.
- Engines do not import persistence.
- The adapter does not create pools, read environment variables, run
  migrations, or load credentials.
- Parameterized SQL is used for values; raw SQLSTATE, constraint names, and
  driver errors are not exposed through neutral errors.
- Cross-scope target operations return `not_found`; scoped lists omit foreign
  records.
- Repository secret scan passed. No password, authenticated URL, `.env`, or
  private key is committed.

## Documentation Audit

Current architecture, API, package READMEs, roadmap, ADR 0014, adapter
validation, dependency graph, project README, and metrics register reflect the
accepted Phase 3.4.3 state and the Phase 3.4.4 release candidate. Historical
milestone reports retain their original `0.1.0` observations intentionally.

The adapter implementation remains internally replaceable. Splitting its
current implementation file into lifecycle-focused files is a non-blocking
maintainability improvement and does not change the proposed v1 API.

## Known Boundaries

- The reference adapter requires UUID-form physical identifiers while the
  neutral contract intentionally represents identifiers as strings.
- Runtime pool sizing, TLS, health policy, timeouts, and environment loading
  remain Phase 3.5 work.
- The 4 GiB maximum is a rejection boundary. The released-scale adapter gate
  proves approximately 1.56 GiB, matching the largest accepted fixture.
- Exact output already written to an `io.Writer` is untrusted until
  `ExportPayload` returns success.

## Conclusion

Phase 3.4.4 satisfies its documented technical gate. Engineering accepted the
evidence on 2026-07-26 and promoted the Persistence Port and PostgreSQL Adapter
to `1.0.0`. Phase 3.5 runtime infrastructure is authorized. APIs, UI, AI
orchestration, new language engines, and repository mutation remain outside
this acceptance.

## Final 1.0.0 Promotion Validation

After version promotion, the exact release candidate passed:

- full backend regression;
- three shuffled full-backend repetitions;
- full backend `go vet`;
- full Windows race testing with zero races;
- conformance-first disposable PostgreSQL 18 integration;
- Linux adapter race testing with zero races;
- 85.1% PostgreSQL integration coverage;
- exact 1,556,379,091-byte staging, publication, export, and SHA-256
  verification.

The final large run used 372 ordered chunks, staged at 216.41 MiB/s, read and
reverified at 943.80 MiB/s, completed the adapter test in 9.29 seconds, and
recorded 176,264 KiB maximum process RSS.
