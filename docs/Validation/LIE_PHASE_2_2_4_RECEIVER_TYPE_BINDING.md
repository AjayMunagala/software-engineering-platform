# LIE Phase 2.2.4 Validation — Receiver and Local Type Binding

## Decision State

- Implementation: complete
- Local validation: passed
- Candidate artifact: `GoSemanticInventory 0.1.0`
- Stable-ID scheme: `go-semantic-id/v1`
- Governance: accepted by engineering on 2026-07-23
- Next milestone: Phase 2.2.5 authorized; Phase 2.2.6 and later remain unauthorized

## Scope Proven

Phase 2.2.4 binds method receivers to exact local declared types and emits bounded type relationships from declared type contexts. It preserves value/pointer/generic syntax and explicit resolved, unresolved, ambiguous, external, and structural states.

Receiver lookup is package-local and type-restricted. A matching text name is insufficient: aliases, interfaces, missing types, invalid shapes, and conflicting declarations are never selected as receiver targets.

General identifier references, selector references, import bindings, package-identity proof consumption, and interface satisfaction remain outside this milestone.

## Functional Results

| Check | Result |
| --- | --- |
| Cross-file value receiver | PASS |
| Pointer receiver | PASS |
| Generic receiver and receiver type parameters | PASS |
| Missing receiver type | PASS — unresolved |
| Alias receiver | PASS — unresolved, never guessed |
| Interface receiver | PASS — unresolved, never guessed |
| Duplicate receiver types | PASS — ambiguous |
| Same-package enforcement | PASS |
| `uses` relations | PASS |
| Struct/interface embedding | PASS |
| `alias-of` relations | PASS |
| Generic instantiation and type arguments | PASS |
| Type-parameter constraints | PASS |
| Local type/type-parameter targets | PASS |
| Predeclared types | PASS — explicit `builtin:` external identity |
| Qualified types before import resolution | PASS — unresolved |
| Relationship ordering and IDs | PASS — deterministic across worker counts |
| Relationship budget | PASS — deterministic truncation, omission count, diagnostic |
| Later relationship collections | PASS — references/imports/interface satisfaction remain empty |
| Artifact and prerequisite immutability | PASS |

## Commands and Results

Environment:

- Go `1.26.2`, `windows/amd64`
- Windows 11 Home Single Language
- Intel Core i5-12450H
- Race toolchain: MSYS2 UCRT64 GCC 16.1.0

```text
go test -cover -count=1 ./lie/golang/semantic
PASS — 85.8% statement coverage

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
go test -run=^$ -bench=ReceiverAndTypeBinding -benchtime=1x -count=3 -benchmem ./lie/golang/semantic
```

Results:

| Fixture | Time | Memory | Allocations |
| --- | ---: | ---: | ---: |
| 100 files / 100 receiver bindings | 6.94–8.37 ms | 3,383,680–3,541,144 B | 24,095–24,191 |
| 1,000 files / 1,000 receiver bindings | 37.96–42.94 ms | 34,095,920–34,287,080 B | 236,947–237,047 |

Each file declares a unique local struct, embedded self-use, pointer receiver, and typed parameter. Fixture construction and prerequisite generation occur outside the timed region. These figures measure the complete authorized rebuild through Phase 2.2.4 and are not the final semantic-engine release gate.

## Boundaries and Known Limitations

- Binding is deterministic syntax/type-namespace analysis, not compiler-equivalent `go/types` validation.
- Qualified type targets remain unresolved until Phase 2.2.5 validates and consumes package-identity proofs.
- Structural type identities and type-argument text are deterministic presentation facts, not substitutes for a future canonical compiler type graph.
- Receiver aliases and interface types are intentionally not accepted as valid receiver base types.
- Type-switch refinement, reference resolution, import scopes, method-set analysis, and interface satisfaction remain deferred.
- Relationship budgeting retains receiver bindings first, then sorted type relations; `OmittedRelationships` and `semantic_relationship_limit` make omission explicit.

## Exit Assessment

The Phase 2.2.4 implementation satisfies its documented local exit gate: exact local receiver proof, pointer/generic preservation, explicit ambiguity and unknown states, bounded deterministic type relations, relationship limits, coverage above 80%, full regression, vet, repeatable benchmarks, and race detection all pass.

Engineering accepted this evidence on 2026-07-23 and authorized Phase 2.2.5 only. Later milestones remain gated by their own acceptance criteria.
