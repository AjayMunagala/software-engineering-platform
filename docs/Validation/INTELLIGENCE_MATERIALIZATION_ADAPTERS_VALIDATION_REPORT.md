# Intelligence & Materialization Adapters Validation Report

## Status

- Phase: 4.0.5 — Intelligence & Materialization Adapters
- Contract version: `0.1.0`
- Analysis profile: `repository-go/v1`
- Codec contract: `canonical-json/1.0.0`
- Local implementation: complete
- Local validation: pass
- Engineering acceptance: pending
- Phase 4.0.6 Persistence & Runtime Integration: unauthorized
- Date: 2026-07-28

## Scope

Phase 4.0.5 connects the neutral scan coordinator to released intelligence
engines and deterministic artifact materialization without introducing a
persistence or runtime dependency.

Implemented:

- fresh RIE execution through the released seven-stage pipeline;
- conditional Go syntax, package-identity, and semantic execution;
- frozen `repository-go/v1` artifact selection;
- versioned `canonical-json/1.0.0` codecs;
- deterministic, validated dependency ordinals and DAG construction;
- permission-restricted, reopenable, sealed file-backed payloads;
- SHA-256 and exact byte-count calculation during the only serialization;
- four-GiB payload enforcement;
- deployment-root and source-path redaction verification;
- bounded, idempotent spool and source cleanup;
- Windows and Ubuntu deterministic-byte verification.

Explicitly excluded:

- Persistence Port and PostgreSQL integration;
- Runtime Infrastructure integration;
- SQL, pgx, connection pools, migrations, or credentials;
- REST, gRPC, HTTP endpoints, authentication, UI, or AI;
- repository clone, fetch, mutation, command execution, or network access.

## Architecture

The production package is:

```text
backend/internal/service/repository/adapters
```

It implements the existing `scan.AnalysisPreparer` boundary. Its only runtime
dependencies are a narrow deployment-owned `SourceResolver` and one
`AuthorizedSource`. The local root is visible only inside the internal engine
adapter. It is never returned in a candidate, service model, error, or durable
payload.

Each `Prepare` call creates a single-use session. Each `Analyze` call creates a
fresh RIE artifact store and fresh Go LIE execution. Engine state and artifacts
are never reused across scans.

## Frozen artifact set

Every repository produces these seven RIE artifacts:

1. `discovery-inventory 1.0.0`
2. `repository-snapshot 1.0.0`
3. `language-inventory 1.0.0`
4. `framework-inventory 1.0.0`
5. `build-inventory 1.0.0`
6. `repository-metadata 1.0.0`
7. `repository-intelligence-summary 1.0.0`

Repositories containing recognized Go files additionally require:

8. `go-language-inventory 1.0.0`
9. `go-package-identity-inventory 1.0.0`
10. `go-semantic-inventory 1.0.0`

A missing artifact, version mismatch, producer mismatch, codec mismatch,
missing dependency, self-dependency, or dependency cycle fails closed.

## Exact-byte materialization

The codec writes fixed-order JSON fields and streams top-level collections one
item at a time. The materializer writes once through a cancellation-aware,
size-bounded SHA-256 writer into a mode-`0600` temporary file outside the
repository. It flushes, syncs, closes, verifies the physical size, scans for
all native, slash-normalized, and JSON-escaped root variants, and only then
publishes a reopenable payload source.

The materializer rejects a spool directory located inside the repository,
including a symlink-resolved location. Session cleanup removes every spool in
reverse order and closes the authorized source exactly once.

## Durable source privacy

RIE root paths are omitted from durable views. The path-derived RIE repository
name is replaced by the stable service Repository ID. Diagnostics are redacted
and absolute diagnostic paths are converted to safe repository-relative paths
or removed.

No production adapter code calls `SourceHandle.Reveal`. Handle interpretation
remains the deployment-owned resolver's responsibility.

## Deterministic cross-platform fixture

The same Go repository content was analyzed under different Windows-style and
Linux-style mount directory names. Every exact payload was identical on
Windows and Ubuntu:

| Artifact | Bytes | SHA-256 |
|---|---:|---|
| build-inventory | 570 | `7ff3bfd4f740286e5f0b33a610534834d4a04b0c70d9a34b7c937fb101e6de9c` |
| discovery-inventory | 196 | `c277ae0b1cb87f54c16cdb9d8f479c93c1b4f099fc2e4c509728a1d32c8bd523` |
| framework-inventory | 156 | `bcf694d6add1db2e134e4f5e7f3b68ef9d223b5beb1cbf7dae5f40d08ae1e5ff` |
| go-language-inventory | 2,187 | `6bc8d9084160b3d03c2625404310ee14ea9f22048bd288d884af84fcfa5e3ed0` |
| go-package-identity-inventory | 1,842 | `961c3dfeb043a222894c8ab9c477a943b14bd17fdb643813ca77cf62547d3112` |
| go-semantic-inventory | 9,251 | `a317b1ff588be275ee2f297eeb19549b48017f3e0f47a6f03c8a5f41a97e4abf` |
| language-inventory | 206 | `e69ff9187e14697d7de23c3426ec65e429e988d9f4810e6482f7369eee1c48ce` |
| repository-intelligence-summary | 1,721 | `865995a6c9ba739288a404b928a47cac8b58b5ab8f28ec9a9bf87c9871a124cd` |
| repository-metadata | 1,023 | `d69df2286951d114d871fd830e253e6256305a7ba35da915f64b4697c1121d8a` |
| repository-snapshot | 275 | `22df9807cbc44f05afd9ff5a91cc5f8c89eb0b9520294196901b903d2ed0fa0e` |

## Defects found and corrected

### Deployment-local repository name affected durable bytes

The first codec candidate preserved the RIE repository name, which is derived
from the local root-directory basename. Identical repositories mounted under
different directory names could therefore produce different payloads. Durable
discovery and metadata views now use the stable service Repository ID. The
Windows and Ubuntu hashes above prove the correction.

### Dependency candidates did not carry a durable graph

The Phase 4.0.4 fake candidates contained payload metadata only. Phase 4.0.5
adds immutable ordered dependencies, validates missing edges, duplicate
ordinals, self-edges, and cycles, and exposes the graph to the later Phase
4.0.6 persistence adapter without making the scan coordinator understand
engine-specific behavior.

### Spool placement needed an explicit repository boundary

A caller-supplied spool directory could initially point inside the analyzed
repository. Preparation now resolves the root and spool locations and rejects
that configuration before engine execution or file creation.

## Validation environments

### Windows

- Windows 11 `10.0.26200`;
- 12th Gen Intel Core i5-12450H, 12 logical processors;
- Go `1.26.2 windows/amd64`;
- MSYS2 UCRT64 GCC `16.1.0` for race builds.

### Ubuntu

- Ubuntu 24.04 under WSL2;
- Go `1.26.2 linux/amd64`;
- CGO-enabled targeted and full-backend race validation.

## Validation results

| Gate | Result |
|---|---:|
| Real released RIE + Go LIE profile | PASS |
| Conditional non-Go seven-artifact profile | PASS |
| Exact bytes, size, digest, and valid JSON | PASS |
| Different-root and cross-platform hashes | PASS, all ten identical |
| Dependency graph validation | PASS |
| Source/root redaction and spool boundary | PASS |
| Five shuffled targeted repetitions | PASS |
| Full backend regression and vet | PASS |
| Windows targeted race | PASS, zero races |
| Windows full-backend race | PASS, zero races |
| Ubuntu targeted race | PASS, zero races |
| Ubuntu full-backend race | PASS, zero races |
| Adapter statement coverage | 86.4% |
| Scan statement coverage | 89.4% |
| Forbidden-root fuzz campaign | 87,896 executions, PASS |
| Forbidden dependency and source-handle audit | PASS |

## Benchmark summary

The benchmark materializes a deterministic 10,000-entry RepositorySnapshot,
including exact JSON streaming, hashing, sync, size verification, redaction
scan, sealing, reopening capability, and cleanup.

| Platform | Time range | Allocated bytes | Allocations |
|---|---:|---:|---:|
| Windows, five runs | 62.03–90.20 ms/op | 2,621,119–2,622,031 B/op | 20,091–20,094 |
| Ubuntu, three runs | 9.65–17.11 ms/op | 1,561,661–1,561,715 B/op | 20,051 |

Filesystem sync and filter-driver behavior account for much of the Windows
variance. No persistence staging, database, network, runtime, or transport work
is included.

## Reproducibility

From `backend/` on Windows:

```text
go test -cover ./internal/service/repository/adapters ./service/repository/scan
go test -shuffle=on -count=5 ./internal/service/repository/adapters ./service/repository/scan
go test ./...
go vet ./...
$env:PATH='C:\msys64\ucrt64\bin;'+$env:PATH
$env:CGO_ENABLED='1'
$env:CC='gcc'
go test -race ./...
go test -run '^$' -fuzz '^FuzzForbiddenVariantsNeverPanic$' -fuzztime=5s ./internal/service/repository/adapters
go test -run '^$' -bench . -benchmem -count=5 ./internal/service/repository/adapters
```

Ubuntu validation uses `backend/service/repository/tests/validate_linux.sh`
with Go 1.26.2.

## Exit-gate assessment

Phase 4.0.5 reached its documented implementation and validation gate and was
accepted by engineering on 2026-07-28. Commit and push are authorized. Phase
4.0.6 design is authorized; its production implementation and every later
milestone remain unauthorized pending their respective reviews.
