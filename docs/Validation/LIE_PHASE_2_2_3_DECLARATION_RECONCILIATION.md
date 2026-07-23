# LIE Phase 2.2.3 Validation — Declaration Reconciliation and Scopes

## Decision State

- Implementation: complete
- Local validation: passed
- Candidate artifact: `GoSemanticInventory 0.1.0`
- Stable-ID scheme: `go-semantic-id/v1`
- Governance: accepted by engineering on 2026-07-23
- Next milestone: Phase 2.2.4 authorized; Phase 2.2.5 and later remain unauthorized

## Scope Proven

Phase 2.2.3 re-parses only source that passed Phase 2.2.2 authorization and digest verification. AST, token, and scope state remain ephemeral. The persisted artifact contains stable semantic declarations, exact links to Phase 2.1 syntax symbols where available, normalized type displays, declaration ownership, explicit resolution states, diagnostics, and derived statistics.

Files remain `partial` because receiver binding, references, imports, type relations, and interface satisfaction are later milestones. Phase 2.2.3 emits none of those relationships.

## Functional Results

| Check | Result |
| --- | --- |
| Struct/interface/function/method reconciliation | PASS — exact Phase 2.1 symbol IDs retained |
| Grouped constants and variables | PASS |
| Defined scalar types and aliases | PASS — stable semantic IDs without invented syntax IDs |
| Parameters and named results | PASS |
| Function/type parameters and generics | PASS |
| Struct and embedded fields | PASS |
| Interface methods and their parameters/results | PASS |
| Local var/const/type declarations | PASS |
| Short declarations and same-scope redeclaration | PASS |
| Nested block and control scopes | PASS |
| Range variables and labels | PASS |
| Declaration ownership | PASS |
| Cross-file package scope | PASS |
| Package-name conflicts | PASS — declarations become `ambiguous` with a diagnostic |
| Multiple `init` declarations | PASS — not treated as a package-name conflict |
| Unicode paths and identifiers | PASS — byte lengths and offsets remain stable |
| Comments and grouped declarations | PASS — exact source ranges retained |
| Worker-count determinism | PASS |
| Prerequisite and artifact immutability | PASS |
| Later semantic collections | PASS — remain empty |

Exact reconciliation requires matching syntax kind, name, file ID, package ID, start byte, and end byte. The engine never selects a declaration based only on a matching name.

## Commands and Results

Environment:

- Go `1.26.2`, `windows/amd64`
- Windows 11 Home Single Language
- Intel Core i5-12450H
- Race toolchain: MSYS2 UCRT64 GCC 16.1.0

```text
go test -cover -count=1 ./lie/golang/semantic
PASS — 87.4% statement coverage

go test -shuffle=on -count=10 ./lie/golang/semantic
PASS

go vet ./lie/golang/semantic
PASS

go test -count=1 ./...
PASS

go vet ./...
PASS

go test -race -count=1 ./lie/golang/semantic
PASS — no data races detected

go test -race -count=1 ./...
PASS — no data races detected
```

Dependency inspection found no semantic-package dependency on `os/exec`, `net/http`, `go/importer`, or `golang.org/x/tools/go/packages`.

## Repeatable Benchmark

Command:

```text
go test -run=^$ -bench=DeclarationReconciliation -benchtime=1x -count=3 -benchmem ./lie/golang/semantic
```

Results:

| Fixture | Time | Memory | Allocations |
| --- | ---: | ---: | ---: |
| 100 verified Go files / 100 declarations | 5.25–8.17 ms | 902,464–922,016 B | 7,992–7,999 |
| 1,000 verified Go files / 1,000 declarations | 256.08–462.45 ms | 8,946,632–9,006,960 B | 77,549–77,582 |

Fixture construction and prerequisite generation occur outside the timed region. Each timed operation performs the full authorized source-verification and declaration-reconciliation rebuild. These numbers are a development baseline, not the completed semantic engine's release gate.

## Boundaries and Known Limitations

- Scope trees are rebuilt and discarded; only stable declaration ownership is published.
- Package scope conflict detection is syntactic and deterministic. Compiler-equivalent type correctness remains outside this milestone.
- Function-literal declarations retain the nearest named declaration as their stable owner because anonymous functions are not yet a public declaration kind.
- Type-switch variable refinement is deferred to later type analysis; the source declaration remains explicit without a guessed concrete type.
- Receiver targets, type relationships, references, imports, and interface satisfaction are intentionally absent.
- No source, AST, token set, lexical scope, `go/types` state, command output, or cache is persisted.

## Exit Assessment

The Phase 2.2.3 implementation satisfies its documented local exit gate: exact reconciliation, stable Unicode-aware byte positions and IDs, declaration ownership, package and lexical scope construction, deterministic ordering, immutable artifacts, coverage above 80%, full regression, vet, repeatable benchmarks, and race detection all pass.

Engineering accepted this evidence on 2026-07-23 and authorized Phase 2.2.4 only. Later milestones remain gated by their own acceptance criteria.
