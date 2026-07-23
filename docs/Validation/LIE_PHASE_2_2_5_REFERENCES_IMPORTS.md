# LIE Phase 2.2.5 Validation — References and Imports

## Status

- Date: 2026-07-23
- Milestone: Phase 2.2.5
- Implementation: complete
- Local exit gate: passed
- Governance: accepted by engineering on 2026-07-23
- Next milestone: Phase 2.2.6 authorized; Phase 2.2.7 and later remain unauthorized

## Scope Validated

Phase 2.2.5 emits deterministic identifier, selector, type, and instantiation
references; resolves lexical, package-local, same-package cross-file, and
proof-backed local import targets; and models default, named, dot, and blank
imports. It preserves external, unresolved, ambiguous, and partial states
without executing tools, reading the module cache, using the network, or
guessing package identity from path suffixes.

The implementation re-hashes every repository manifest named by a consumed
`PackageIdentityProof`. Stale evidence prevents a resolved import and emits one
bounded `semantic_package_proof_stale` diagnostic. Multiple applicable proof
contexts resolve a repository import only when every fresh usable context
agrees on one target.

## Behavioral Evidence

Tests cover:

- lexical shadowing over package declarations;
- local values shadowing imported package aliases;
- same-package cross-file references;
- local default imports with exact target package names;
- named external imports and external selector identities;
- dot imports with exported-name enforcement;
- blank imports that retain proof without creating selector scope;
- unexported imported names remaining unresolved;
- unresolved imports and ambiguous declaration candidates;
- unanimous and conflicting package-proof contexts;
- stale package-manifest evidence;
- deterministic output across one and eight workers;
- deterministic global relationship limits that preserve imports before
  receivers, type relations, and references;
- immutable reference candidate collections and statistics maps through the
  existing artifact immutability suite.

Phase 2.2.6 interface satisfaction is not implemented. Selectors on local
runtime values are emitted but remain unresolved because field/method type
checking is outside this milestone. A default external import keeps an empty
local name when no exact package-name proof exists; the engine does not use the
last import-path segment as a substitute.

## Validation Results

| Check | Result |
|---|---:|
| `go test -count=1 -timeout=3m ./lie/golang/semantic` | PASS |
| `go test -shuffle=on -count=10 -timeout=5m ./lie/golang/semantic` | PASS |
| `go test -count=1 -timeout=5m ./...` | PASS |
| `go vet ./...` | PASS |
| Semantic statement coverage | 85.6% |
| Targeted `go test -race` | PASS |
| Full backend `go test -race ./...` | PASS |
| Data races | 0 |
| Forbidden `os/exec`, `net/http`, `go/importer`, `go/packages` dependencies | 0 |

Race validation used Go 1.26.2 on Windows/amd64 with MSYS2 UCRT64 GCC 16.1.0
and `CGO_ENABLED=1`.

## Repeatable Benchmark

Reference CI baseline machine:

- OS: Windows
- Architecture: amd64
- CPU: 12th Gen Intel Core i5-12450H
- Go scheduler benchmark suffix: `-12`

The fixture has one local shared package and one importing package containing
100 or 1,000 files. Every importing file has one default local import and one
resolved selector reference. Prerequisite construction occurs outside the
timed region.

| Fixture | Time | Bytes/op | Allocations/op |
|---|---:|---:|---:|
| 101 files / 100 imports | 6.51–7.53 ms | 2.40–2.43 MB | 17,346–17,387 |
| 1,001 files / 1,000 imports | 29.90–37.01 ms | 23.16–23.31 MB | 166,268–166,357 |

Commands:

```text
go test -run ^$ -bench ^BenchmarkReferencesAndImports100Files$ -benchtime=3x -count=1 ./lie/golang/semantic
go test -run ^$ -bench ^BenchmarkReferencesAndImports1000Files$ -benchtime=1x -count=1 ./lie/golang/semantic
```

Each command was run three times. The results measure a complete authorized
full rebuild through Phase 2.2.5 and are not the final
`GoSemanticInventory 1.0.0` release gate.

## Exit-Gate Decision

The Phase 2.2.5 implementation satisfies its documented local exit gate:
reference kinds and scope resolution, import alias modes, explicit status
handling, package-proof freshness, context conflict handling, deterministic
relationship limits, coverage above 80%, full regression, vet, shuffled
execution, repeatable benchmarks, and race detection all pass.

Engineering accepted this evidence on 2026-07-23 and authorized Phase 2.2.6
only. Later milestones remain gated by their own acceptance criteria.
