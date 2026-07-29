# Persistence & Runtime Integration Validation Report

## Status

- Phase: 4.0.6 - Persistence & Runtime Integration
- Repository Service contract: `0.1.0` candidate
- Integration contract: `0.1.0` candidate
- Base commit: `24fc66bc186f9e89474d4ed52a11e8e869a5c044`
- Implementation review commit: `abdb395`
- Local implementation: complete
- Local validation: pass
- Engineering acceptance: accepted on 2026-07-29
- Commit and push: complete on `main`
- Phase 4.0.7: design authorized; execution unauthorized pending design review
- Date: 2026-07-29

## Scope delivered

Phase 4.0.6 connects the accepted Repository Service lifecycle and scan
coordinators to Runtime Infrastructure 1.0.0, Persistence Port 1.0.0, and the
PostgreSQL Adapter 1.0.0 through one internal translation boundary.

Implemented:

- canonical lowercase UUID validation for scope, repository, and scan IDs;
- frozen `repository-service-storage-artifact-id/v1` physical UUID mapping;
- frozen `repository-service-manifest/v1` canonical manifest construction;
- lifecycle mapping for register, get, list, and archive;
- runtime admission and work-item ownership;
- source-proof consistency before a durable scan begins;
- sequential exact-byte staging with SHA-256 and byte-count verification;
- atomic publication with ordered artifacts and dependency edges;
- publication ambiguity reconciliation;
- exact artifact get, list, and streaming export;
- persistence-authoritative repository, scan, and artifact timestamps;
- stable storage-neutral error translation;
- a strict neutral in-memory integration fixture;
- a disposable PostgreSQL 18 end-to-end runtime fixture.

Explicitly excluded:

- REST, gRPC, HTTP listeners, UI, authentication, authorization, and AI;
- distributed workers, queues, leases, remote repository fetch, or cloning;
- runtime pool creation, TLS ownership, migration execution, or shutdown by the
  integration package;
- changes to released RIE, Go LIE, Persistence Port, PostgreSQL Adapter, or
  Runtime Infrastructure contracts;
- Phase 4.0.7 real-repository validation.

## Frozen pre-implementation contracts

The normative values remain in
`docs/API/REPOSITORY_SERVICE_INTEGRATION_GOLDEN_VECTORS.md`.

| Contract | Frozen result |
|---|---|
| Scope UUID | `00000000-0000-4000-8000-000000000001` |
| Repository UUID | `11111111-1111-4111-8111-111111111111` |
| Scan UUID | `22222222-2222-4222-8222-222222222222` |
| Discovery public artifact ID | `rsaid1_19546f40503fdddd85481edb5cf47f7189874a252c63bddbb6b39e8c9b032886` |
| Discovery physical UUID | `628b097f-84cb-87cf-a81c-5627222e948c` |
| Snapshot public artifact ID | `rsaid1_26ddb640b08b957c57512820bc99a18b2bd3b7f168edf5e6b95a77e12c2573b9` |
| Snapshot physical UUID | `30594582-1bb4-8bef-bb5b-2aa43b338a9e` |
| Canonical manifest size | 818 bytes |
| Canonical manifest SHA-256 | `888b4f65c1a34881d2b762a474a65ada819d6064fd99d30f1958f6872c133598` |

Windows and Ubuntu execute the same golden tests. Physical UUIDs have RFC 9562
version 8 and RFC variant bits. Reordered artifacts, invalid dependency
ordinals, missing dependencies, self-dependencies, and mismatched stored UUIDs
fail closed.

## Architecture and ownership

The production package is:

```text
backend/internal/service/repository/integration
```

It follows the project eight-file package standard. The package imports the
neutral service, persistence, runtime, and Phase 4.0.5 adapter boundaries. It
contains no SQL, pgx, pool construction, TLS loading, migration logic, HTTP,
network listener, command execution, or engine-specific analysis logic.

The integration bundle borrows only Runtime `Admit`, `Ingest`, and `Read`
capabilities. Runtime remains the owner of pool creation, readiness, draining,
and shutdown. The integration package never closes Runtime or a pool.

## Disposable PostgreSQL proof

The accepted runtime harness creates a temporary PostgreSQL cluster, applies
the checksum-verified migrations, creates disposable least-privilege roles,
and destroys the cluster after the run. No local personal database, persistent
credential, shared database, or production resource is used.

The final run proved:

- PostgreSQL neutral persistence conformance: PASS in 2.647 seconds;
- PostgreSQL Runtime package: PASS, 88.1% coverage;
- Application Runtime package: PASS, 93.0% coverage;
- Repository Service integration: PASS, 85.2% coverage;
- schema compatibility version: `1.0.0`;
- migration revision: `202607260008`;
- 100 start-ready-register-scan-publish-export-drain-stop cycles: PASS;
- every scan produced and durably published ten released Go-profile artifacts;
- exact export digest and byte count matched published metadata;
- all 100 independent runtime instances shut down cleanly.

The disposable server used PostgreSQL 18.4 on loopback, `fsync=off` for the
ephemeral validation cluster, accepted migrations only, a combined CI
capability login for the service cycles, and the existing verify-full TLS
runtime proof in the same harness.

## Performance results

### Pure integration translation

Windows/amd64, five runs:

| Benchmark | Time range | Allocated bytes | Allocations |
|---|---:|---:|---:|
| Physical artifact UUID | 458.5-473.8 ns/op | 448 B/op | 11 |
| Ten-artifact canonical manifest | 5.769-6.055 us/op | 10,560 B/op | 39 |
| Ten-artifact ID + manifest translation | 12.058-14.581 us/op | 15,049-15,050 B/op | 149 |

Ubuntu/amd64, three runs:

| Benchmark | Time range | Allocated bytes | Allocations |
|---|---:|---:|---:|
| Physical artifact UUID | 564.4-633.5 ns/op | 448 B/op | 11 |
| Ten-artifact canonical manifest | 5.779-6.874 us/op | 10,560 B/op | 39 |
| Ten-artifact ID + manifest translation | 11.136-12.579 us/op | 15,050 B/op | 149 |

The ten-artifact manifest and combined translation remain far below their 5 ms
and 25 ms candidate gates.

### Complete disposable PostgreSQL cycles

The measured scan interval includes fresh RIE/Go LIE analysis, serialization,
ten exact payload stages, atomic publication, and the post-publication read.
It is intentionally broader than pure service overhead.

| Metric | p50 | p95 | Maximum |
|---|---:|---:|---:|
| Complete scan | 152.456 ms | 310.291 ms | 387.482 ms |
| Start-ready-register-scan-export-stop cycle | 182.438 ms | 352.169 ms | 439.373 ms |

All full cycles remained bounded and the previously accepted PostgreSQL atomic
publication target remains below 500 ms p95.

## Validation environments

### Windows

- Windows 11 Home Single Language `10.0.26200`, amd64;
- 12th Gen Intel Core i5-12450H, 8 cores / 12 logical processors;
- 8,284,266,496 bytes physical RAM;
- 512 GB SSSTC CL4-8D512 NVMe SSD;
- Go `1.26.2 windows/amd64`;
- MSYS2 UCRT64 GCC `16.1.0` for race builds.

### Ubuntu and PostgreSQL

- Ubuntu 24.04 under WSL2, kernel `6.18.33.2-microsoft-standard-WSL2`;
- checksum-verified disposable official Go `1.26.2 linux/amd64`;
- GCC `13.3.0`;
- PostgreSQL `18.4 (Ubuntu 18.4-1.pgdg24.04+1)`;
- Atlas Community CLI `1.2.3`;
- pgx `v5.10.0` through the released runtime/adapter contracts.

## Test and quality results

| Gate | Result |
|---|---:|
| Frozen UUID and manifest vectors | PASS on Windows and Ubuntu |
| Repository Service conformance | PASS |
| PostgreSQL neutral conformance | PASS |
| Real RIE + Go LIE integration | PASS |
| Exact stage/export round trip | PASS |
| 100 PostgreSQL/runtime service cycles | PASS |
| Five shuffled target repetitions | PASS |
| Full backend regression | PASS on Windows and Ubuntu |
| `go vet ./...` | PASS on Windows and Ubuntu |
| Windows targeted/full race | PASS, zero races |
| Ubuntu targeted/full race | PASS, zero races |
| Integration coverage | 85.2% |
| Service contract coverage | 94.8% |
| Service conformance coverage | 85.0% |
| Lifecycle coverage | 85.4% |
| Scan coverage | 88.6% |
| Intelligence adapter coverage | 86.4% |
| Integration fuzz executions | 748,486, PASS |
| SQL/pgx/listener/network/command audit | PASS |
| Secret and durable-path audit | PASS |
| `git diff --check` | PASS |

The fuzz total comprises 140,442 physical-ID, 367,439 manifest, and 240,605
record/error-translation executions. Seed regression tests also cover hostile
cursor and service-constructor inputs in the existing contract packages.

## Defects found and corrected

### Durable begin timestamps were not adopted by the coordinator

The coordinator originally continued with its pre-persistence running value
after `Begin` succeeded. The in-memory test clock masked the difference, while
PostgreSQL correctly returned database-owned requested/started timestamps. The
post-publication comparison therefore rejected a valid durable result.

The `BeginStarted` branch now replaces the candidate with the authoritative
running record before analysis and publication. Unit, race, Ubuntu, and 100
disposable PostgreSQL lifecycle cycles verify the correction.

### Multi-cycle fixture reused one durable source proof

The first 100-cycle fixture assigned one source fingerprint to distinct
repository IDs. Persistence correctly rejected the second registration as an
identity conflict. Each independent fixture repository now receives a unique
path-free proof. This was a validation-fixture defect, not a production defect.

### Fuzz leakage assertion produced a false positive

The first error-leak assertion searched for an arbitrary one-character fuzz
input in a fixed safe error string. A corpus value such as `a` could naturally
occur in the word `persistence`. The assertion now derives a unique SHA-256
marker, injects that marker only into the raw cause, and verifies that the
marker is absent. The replay and 240,605 further executions pass.

### Persisted record mapping was hardened

Repository, scan, and artifact translation now converts invalid persisted
records, scope mismatches, repository/scan mismatches, unknown profiles, and
physical artifact mismatches into stable `integrity_failure` errors. Raw
constructor or storage details never cross the service boundary.

## Reproducibility

From `backend/` on Windows:

```text
go test ./...
go vet ./...
go test -shuffle=on -count=5 ./service/repository/... ./internal/service/repository/adapters ./internal/service/repository/integration
$env:CGO_ENABLED='1'
$env:PATH='C:\msys64\ucrt64\bin;'+$env:PATH
go test -race ./...
go test ./internal/service/repository/integration -run '^$' -bench . -benchmem -count=5
```

Fuzz each integration target for ten seconds:

```text
go test ./internal/service/repository/integration -run '^$' -fuzz '^FuzzPhysicalArtifactIDNeverPanics$' -fuzztime=10s
go test ./internal/service/repository/integration -run '^$' -fuzz '^FuzzCanonicalManifestNeverPanics$' -fuzztime=10s
go test ./internal/service/repository/integration -run '^$' -fuzz '^FuzzRecordAndErrorTranslationNeverPanics$' -fuzztime=10s
```

Ubuntu Go/race/benchmark validation:

```text
bash backend/service/repository/tests/bootstrap_linux.sh
```

Disposable PostgreSQL conformance and 100-cycle service validation:

```text
wsl -d Ubuntu-24.04 -u postgres -- bash -lc \
  'cd /mnt/d/Project_Ai/backend && bash internal/runtime/postgres/tests/validate.sh'
```

## Known limitations and deferred work

- The service is synchronous and single-process; distributed scheduling is
  intentionally deferred.
- Source resolution supports only deployment-authorized local sources; clone
  and remote fetch remain out of scope.
- PostgreSQL-backed validation uses the released ten-artifact Go profile. The
  accepted Phase 4.0.5 suite retains the seven-artifact non-Go exact-byte gate.
- Real large-repository validation belongs to Phase 4.0.7 and has not started.
- APIs, authentication, UI, AI orchestration, and higher intelligence engines
  remain outside this milestone.

## Exit-gate assessment

Phase 4.0.6 has reached its documented local implementation and validation
gate. All mandatory local, Windows, Ubuntu, PostgreSQL, deterministic,
integrity, race, fuzz, coverage, audit, and performance gates pass.

Review candidate `abdb395` is committed and pushed to `main`. Engineering
accepted and froze Phase 4.0.6 on 2026-07-29 based on the reviewed architecture,
API, ADR, validation plan, and the recorded implementation evidence. The review
did not claim a line-by-line audit of every changed source file. Phase 4.0.7 is
authorized for design only; validation execution requires separate design
acceptance.
