# Phase 3.5.1 Runtime Configuration Validation Report

## Status

- Milestone: Phase 3.5.1 — Runtime Configuration and Secret Boundaries
- Evidence date: 2026-07-26
- Design baseline: `d1f65b8` (`Accept runtime infrastructure design`)
- Local implementation gate: reached
- Engineering acceptance: accepted on 2026-07-26
- Phase 3.5.2: authorized by subsequent engineering decision

## Scope

Validated:

- deterministic defaults/file/environment/command-line precedence;
- strict UTF-8 JSON with duplicate and unknown-field rejection;
- unknown and duplicate `AEGIS_` environment rejection;
- allowlisted, non-secret command-line overrides;
- immutable secret-free runtime configuration and detached getters;
- profile-specific local, CI, staging, and production validation;
- combined local/CI and distinct staging/production database identities;
- explicit duration, connection-budget, pool, host, and TLS policy validation;
- one secret-provider boundary, required-secret proof, and safe redaction;
- stable safe error codes and field names;
- cancellation, fuzz, benchmark, regression, vet, and race behavior.

Not implemented or validated in this milestone:

- PostgreSQL pool or TLS object construction;
- certificate-chain or hostname verification against a live server;
- mounted-file secret provider permission enforcement;
- schema compatibility checks or migrations;
- runtime startup/shutdown, health, logging, or metrics;
- listeners, APIs, UI, authentication, business logic, or engines.

## Implementation Contract

Package:

```text
backend/internal/runtime/config
```

Candidate contract version: `0.1.0`.

The loader receives environment and command-line values explicitly. It reads
only an explicitly selected absolute JSON configuration file, never process
global environment or arguments. It creates no connection, goroutine, logger,
listener, pool, adapter, or engine.

Implementation review identified and corrected one design-model gap before the
candidate API was frozen: staging and production now require distinct ingest,
read, and retention usernames as well as distinct secret references. The
single database user remains restricted to the disposable local/CI combined
pool. Persistence Port and PostgreSQL Adapter `1.0.0` are unchanged.

## Reference Environments

### Windows

- OS: Microsoft Windows 11 Home Single Language `10.0.26200`, 64-bit;
- CPU: 12th Gen Intel Core i5-12450H, 12 logical processors;
- memory: 7.7 GiB visible;
- storage: SSSTC CL4-8D512 SSD;
- Go: `go1.26.2 windows/amd64`;
- race compiler: MSYS2 UCRT64 GCC 16.1.0.

The Windows race command requires both `CGO_ENABLED=1` and the MSYS2 UCRT64
`bin` directory on `PATH`. With that documented environment, targeted and full
race suites pass.

### Ubuntu WSL

- distribution: Ubuntu 24.04 on WSL2;
- kernel: `6.18.33.2-microsoft-standard-WSL2`;
- architecture: `linux/amd64`;
- Go: `go1.26.2 linux/amd64`;
- logical processors: 12;
- WSL memory: 3.7 GiB.

No PostgreSQL instance or credential was required or used.

## Functional and Security Results

| Gate | Result |
|---|---|
| Local defaults and immutable request/config views | PASS |
| File → environment → CLI precedence | PASS |
| Unknown JSON/environment/argument rejection | PASS |
| Same-source duplicate rejection | PASS |
| Malformed, wrong-type, trailing, and invalid UTF-8 JSON | PASS |
| Absolute regular configuration-file boundary | PASS |
| Explicit duration and numeric bounds | PASS |
| Combined local/CI pool consistency | PASS |
| Separate production users, pools, and budget | PASS |
| Production TLS downgrade rejection | PASS |
| Disabled TLS restricted to local/CI loopback/socket | PASS |
| TLS path and client certificate/key pairing | PASS |
| Missing and ambiguous secret providers | PASS |
| Required-secret proof and cancellation | PASS |
| Secret-free errors, `String`, and `SafeView` | PASS |
| Persistence/engine/SQL/pgx dependency exclusion | PASS |

All secret values used by tests are disposable placeholders. No personal or
database credential is present in the implementation or report.

## Automated Validation

| Command | Result |
|---|---|
| `go test ./internal/runtime/config` | PASS |
| `go test -shuffle=on -count=3 ./internal/runtime/config` | PASS |
| `go test ./...` | PASS |
| `go test -shuffle=on -count=3 ./...` | PASS |
| `go vet ./...` | PASS |
| Windows targeted `go test -race` | PASS, zero races |
| Windows full `go test -race ./...` | PASS, zero races |
| Ubuntu WSL targeted `go test -race` | PASS, zero races |
| Ubuntu WSL full `go test -race ./...` | PASS, zero races |
| `go test -fuzz=FuzzStrictJSONNeverPanics -fuzztime=5s` | PASS, ~181,570 executions |
| `go test -fuzz=FuzzEnvironmentAndValidationNeverPanics -fuzztime=5s` | PASS, ~577,724 executions |
| Statement coverage | PASS, 86.6% |

The two fuzz campaigns executed approximately 759,294 inputs and found no panic
or failing corpus entry.

## Benchmarks

Reference: Windows environment above, five repetitions, warm operating-system
cache, `go test -run '^$' -bench ... -benchmem -count=5`.

| Benchmark | Observed range | Memory | Allocations |
|---|---:|---:|---:|
| Local environment load | 6.107–14.973 µs/op | 8,088 B/op | 34 allocs/op |
| Production environment load | 17.598–21.887 µs/op | 9,064 B/op | 46 allocs/op |
| Strict representative JSON decode | 29.295–37.703 µs/op | 8,888 B/op | 315 allocs/op |
| 64-KiB file compose/validate | 493.861–837.859 µs/op | 408,639–408,665 B/op | 121 allocs/op |
| Safe redacted view | 4.339–5.736 µs/op | 2,776 B/op | 18 allocs/op |

The approved gates are met:

- 64-KiB composition and validation remains below 10 ms;
- redacted-view construction remains below 1 ms.

## Determinism and Immutability

- caller environment and argument slices are copied by the request constructor;
- returned source maps and secret-reference slices are detached;
- the frozen configuration contains no mutable byte slices or secret values;
- safe JSON maps are encoded deterministically by key;
- environment/file mutation after load cannot alter the active configuration;
- provider results are copied, the provider-owned copy is cleared, and callers
  are required to clear their short-lived returned copy.

## Known Limitations

1. The environment secret provider and the provider interface are implemented;
   mounted-file permissions and secret-manager adapters remain future runtime
   work.
2. TLS material paths and profile rules are validated, but TLS material loading
   and cryptographic validation belong to Phase 3.5.2.
3. The configuration contract remains `0.1.0` until implementation evidence is
   accepted and later runtime consumers validate its usability.
4. No hot reload is supported; controlled restart remains the accepted model.
5. The requested disposable-PostgreSQL runtime integration suite is retained in
   the accepted full-runtime validation plan. Startup, readiness, shutdown, and
   pool-ownership cases begin only when Phases 3.5.2 and 3.5.3 authorize those
   capabilities.

## Exit Assessment

The Phase 3.5.1 local implementation gate is reached. Functional, security,
coverage, fuzz, performance, regression, vet, and Windows/Linux race evidence
passes within the authorized scope.

Engineering accepted this evidence on 2026-07-26. Phase 3.5.1 may be committed
and pushed, and Phase 3.5.2 PostgreSQL runtime work is authorized. Phase 3.5.3
and later remain separately gated.
