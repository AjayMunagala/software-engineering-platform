# LIE Phase 2.2.6 Validation — Conditional Interface Satisfaction

## Status

- Date: 2026-07-23
- Milestone: Phase 2.2.6
- Implementation: complete
- Local exit gate: passed
- Governance: accepted by engineering on 2026-07-23
- Next milestone: Phase 2.2.7 authorized; Phase 2.2.8 and later remain unauthorized

## Scope Validated

Phase 2.2.6 rebuilds ephemeral package type state with the Go standard
library's `go/types` package and no importer. It evaluates only evidence-derived
local candidate pairs from:

- compile-time interface assertions;
- typed package variables;
- assignment statements;
- interface conversions;
- function and method arguments;
- return statements; and
- exact resolved local embedded-interface relations.

The engine never compares every concrete type with every interface. Candidate
identity is `(concrete declaration ID, interface declaration ID, pointer mode)`;
duplicates merge deterministically. Results are `proven`, `disproven`, or
`unknown`. Missing or mismatched method names are sorted and published only for
reproducible disproven results.

## Behavioral Evidence

Tests cover:

- implicit value method-set satisfaction;
- pointer-only satisfaction and `PointerRequired`;
- embedded interfaces and embedded-interface candidate derivation;
- missing methods;
- wrong method signatures;
- instantiated local generic interfaces and concrete types;
- compile-time assertion evidence;
- ordinary typed assignments without assertion evidence;
- assignment, conversion, argument, and return candidate sites;
- unavailable external imports producing `unknown`;
- unrelated package type errors preventing semantic claims;
- packages exceeding configured file limits;
- deterministic candidate and assertion aggregation across worker counts;
- global relationship limiting with explicit omission reporting; and
- repositories with unrelated types/interfaces producing zero checks.

External package importing, compiler build-context selection, call graphs,
and candidate integration are not implemented. A package with missing imports,
incomplete local type state, ambiguous declarations, or unrelated type errors
produces `unknown`, never a guessed interface result.

## Validation Results

| Check | Result |
|---|---:|
| `go test -count=1 -timeout=3m ./lie/golang/semantic` | PASS |
| `go test -shuffle=on -count=10 -timeout=5m ./lie/golang/semantic` | PASS |
| `go test -count=1 -timeout=6m ./...` | PASS |
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

The fixture contains one package with 100 or 1,000 files. Each file declares a
unique interface, concrete type, matching value-receiver method, and compile-
time assertion. Prerequisite construction occurs outside the timed region.

| Fixture | Time | Bytes/op | Allocations/op |
|---|---:|---:|---:|
| 100 interface checks | 8.91–12.36 ms | 5.48–5.49 MB | 46,483–46,488 |
| 1,000 interface checks | 54.81–59.96 ms | 55.74–55.81 MB | 455,837–455,923 |

Commands:

```text
go test -run ^$ -bench ^BenchmarkInterfaceSatisfaction100Files$ -benchtime=3x -count=1 ./lie/golang/semantic
go test -run ^$ -bench ^BenchmarkInterfaceSatisfaction1000Files$ -benchtime=1x -count=1 ./lie/golang/semantic
```

Each command was run three times. The results measure a complete authorized
full rebuild through Phase 2.2.6 and are not the final
`GoSemanticInventory 1.0.0` release gate.

## Exit-Gate Decision

The Phase 2.2.6 implementation satisfies its documented local exit gate:
bounded evidence-derived candidates, value/pointer method sets, embedded
interfaces, generics, missing and mismatched methods, incomplete-evidence
handling, deterministic output, explicit limits, coverage above 80%, full
regression, vet, shuffled execution, repeatable benchmarks, and race detection
all pass.

Engineering accepted this evidence on 2026-07-23 and authorized Phase 2.2.7
only. Later milestones remain gated by their own acceptance criteria.
