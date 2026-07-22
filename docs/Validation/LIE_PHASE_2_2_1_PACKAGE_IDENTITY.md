# LIE Phase 2.2.1 Package Identity Validation

## Status

- Date: 2026-07-22
- Milestone: Phase 2.2.1 — Go Package Identity Engine
- Artifact: `GoPackageIdentityInventory 0.1.0`
- ID scheme: `go-package-proof-id/v1`
- Implementation: complete
- Exit gate: passed locally, including targeted and full-backend race validation
- Engineering acceptance: accepted on 2026-07-22
- Authorization impact: Phase 2.2.2 is authorized; Phase 2.2.3 and later remain unauthorized

## Responsibility Validated

The engine deterministically maps Go import paths to repository packages using only immutable `RepositorySnapshot 1.0.0`, immutable `GoLanguageInventory 1.0.0`, and snapshot-authorized local manifest evidence.

It performs a full identity rebuild on every call. It does not execute Go commands, load external packages, inspect the ambient module cache, access the network, download dependencies, modify repository files, or infer identities from directory suffixes.

## Implemented Contract

- Exact eight-file package standard under `backend/lie/golang/packageidentity`.
- Candidate artifact with private construction and deep-copy accessors.
- Separate module identities, resolution contexts, package proofs, diagnostics, and statistics.
- Context kinds: single module, workspace, module vendor, workspace vendor, and unmanaged.
- Proof statuses: resolved, unresolved, ambiguous, external, and stale-reserved-for-consumers.
- Location-aware SHA-256 evidence for manifest facts and managed context selection.
- Official in-process `go.mod` and `go.work` parsing through `golang.org/x/mod/modfile`.
- Deterministic worker-bounded manifest parsing with a maximum of eight workers.
- Stable UTF-8 byte-length-prefixed proof IDs.
- Bounded global and per-file diagnostics.
- Lexical and symlink repository-boundary checks.

## Fixture Coverage

The automated suite validates:

- valid empty and non-Go repositories;
- unmanaged Go packages;
- one module with local and external imports;
- multiple independent modules;
- `go.work` with multiple `use` directives;
- workspace and single-module resolution as independent contexts;
- workspace replacement precedence over module replacement;
- local replacement to another verified repository module;
- nested-module ownership boundaries;
- automatic module-vendor context and `vendor/modules.txt` evidence;
- duplicate workspace module paths producing ambiguity;
- malformed `go.mod`;
- workspace paths escaping the repository;
- manifest deletion after snapshot creation;
- manifest symlink escape where supported by the operating system;
- invalid configuration and worker caps;
- cancellation before analysis;
- prerequisite provenance mismatch;
- deterministic diagnostic limiting;
- deep artifact immutability;
- deterministic output across one and eight workers;
- exact proof-ID golden vector;
- artifact-store publication and retrieval;
- stable JSON enum rendering.

## Commands and Results

Run from `backend/`:

```text
go test -count=1 ./lie/golang/packageidentity
PASS

go test -shuffle=on -count=10 ./lie/golang/packageidentity
PASS

go test -count=1 ./...
PASS

go vet ./...
PASS

go test -cover -count=1 ./lie/golang/packageidentity
87.8% statement coverage
```

Production dependency inspection found none of the forbidden dependencies `os/exec`, `net/http`, `go/importer`, or `golang.org/x/tools/go/packages`.

## Benchmark Summary

Reference environment:

- OS/architecture: Windows/amd64
- CPU: 12th Gen Intel Core i5-12450H
- Go: 1.26.2
- Engine workers: default, capped at eight
- Benchmark mode: one complete identity rebuild per iteration
- Fixture: one module and one parsed Go package containing 100, 1,000, or 10,000 deterministic external imports
- Setup for RIE and Go syntax artifacts is outside the timed section

Three one-iteration samples:

| Imports/proofs | Time | Bytes | Allocations |
|---:|---:|---:|---:|
| 100 | 1.85–3.95 ms | ~178 KB | ~1,211 |
| 1,000 | 4.03–4.42 ms | ~1.73 MB | ~9,326 |
| 10,000 | 15.56–19.24 ms | ~20.0 MB | ~90,423 |

The measured workload remains linear in the number of imports/proofs. These are local development measurements, not a frozen cross-platform performance SLA.

## Race-Test Validation

Race validation completed on `2026-07-22T23:01:30+05:30` using Go `1.26.2` on Windows/amd64 and MSYS2 UCRT64 GCC `16.1.0`.

Compiler compatibility was verified before testing:

```text
C:\msys64\ucrt64\bin\gcc.exe --print-file-name libsynchronization.a
C:/msys64/ucrt64/bin/../lib/gcc/x86_64-w64-mingw32/16.1.0/../../../../lib/libsynchronization.a
```

Both race-instrumented commands passed:

```text
CGO_ENABLED=1 go test -race -count=1 -timeout=120s ./lie/golang/packageidentity
PASS

CGO_ENABLED=1 go test -race -count=1 -timeout=180s ./...
PASS
```

| Check | Result |
|---|---|
| `go test -count=1 ./...` | PASS |
| `go test -shuffle=on -count=10 ./lie/golang/packageidentity` | PASS |
| `go vet ./...` | PASS |
| `go test -cover -count=1 ./lie/golang/packageidentity` | PASS, 87.8% statement coverage |
| Targeted package race test | PASS, no race detected |
| Full-backend race test | PASS, no race detected |

The previously unavailable race gate is now satisfied. No waiver is required.

## Known Limitations

- Exact standard-library identity is deferred; standard-library-looking imports are not guessed and remain external.
- External module versions are not selected or downloaded.
- The ambient module cache and `GOROOT` are not read.
- Vendor consistency is proof-local: the package line must belong to a module header whose path/version exactly matches an importing-module `require`, and the vendor directory must map to one syntax-inventory package. Whole-manifest completeness and compiler-equivalent replacement consistency are deferred.
- Proof staleness is represented for future consumers; Phase 2.2.1 produces fresh proofs and does not consume a prior proof artifact.
- Artifact instance provenance is currently limited by the platform-wide `ArtifactReference` name/version contract; structural snapshot membership is additionally verified.

## Decision

The Phase 2.2.1 implementation and validation evidence are accepted. Functional, security, determinism, immutability, coverage, benchmark, regression, vet, targeted race, and full-backend race checks pass. Phase 2.2.2 — Semantic Artifact Skeleton and Source Verification is authorized to begin. This authorization does not extend to Phase 2.2.3 or later milestones.
