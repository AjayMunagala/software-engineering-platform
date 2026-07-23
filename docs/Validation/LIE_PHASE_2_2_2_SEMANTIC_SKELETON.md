# LIE Phase 2.2.2 Validation — Semantic Artifact Skeleton and Source Verification

## Decision State

- Implementation: complete
- Local validation: passed
- Candidate artifact: `GoSemanticInventory 0.1.0`
- Stable-ID scheme: `go-semantic-id/v1`
- Governance: accepted by engineering on 2026-07-23
- Next milestone: Phase 2.2.3 authorized; Phase 2.2.4 and later remain unauthorized

## Scope Proven

Phase 2.2.2 validates the three typed prerequisites, authorizes source only from `RepositorySnapshot`, enforces repository containment and file limits, re-hashes Go source, and emits deterministic immutable file outcomes, diagnostics, provenance, and statistics.

A verified file is intentionally reported as `partial`. This milestone proves source identity and safe access only. It does not parse ASTs, reconcile declarations, resolve imports or references, bind receivers or types, evaluate interfaces, or emit semantic relationships.

## Functional Results

| Check | Result |
| --- | --- |
| Empty repository | PASS — valid empty inventory without diagnostics |
| Digest match | PASS — verified digest retained; file is `partial` |
| Digest mismatch | PASS — file is `stale`; no analyzed digest retained |
| Missing source | PASS — explicit failed outcome and diagnostic |
| Unreadable/non-regular source | PASS — explicit failed outcome and diagnostic |
| Oversized source | PASS — explicit skipped outcome and diagnostic |
| Symlink/root escape | PASS — rejected with an error diagnostic |
| Phase 2.1 failed/skipped files | PASS — state remains explicit |
| Prerequisite versions/provenance | PASS — invalid inputs fail the run |
| Input immutability | PASS |
| Artifact deep immutability | PASS |
| Worker-count determinism | PASS |
| Diagnostic ordering/deduplication/limits | PASS |
| Enum JSON contracts | PASS; invalid values are rejected |
| Artifact-store publication/retrieval | PASS |

The validation review found and fixed one candidate-contract defect: `DeclarationKind(0)` had serialized as an empty string. Invalid declaration kinds now consistently fail JSON marshaling, matching every other semantic enum.

## Commands and Results

Environment:

- Go `1.26.2`, `windows/amd64`
- Windows 11 Home Single Language
- Intel Core i5-12450H
- Race toolchain: MSYS2 UCRT64 GCC 16.1.0

```text
go test -cover -count=1 ./lie/golang/semantic
PASS — 88.9% statement coverage

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
go test -run=^$ -bench=SourceVerification -benchtime=1x -count=3 -benchmem ./lie/golang/semantic
```

Results:

| Fixture | Time | Memory | Allocations |
| --- | ---: | ---: | ---: |
| 100 Go files | 4.08–4.51 ms | 265,664–268,096 B | 2,437–2,449 |
| 1,000 Go files | 14.51–20.13 ms | 2,674,016–2,742,280 B | 22,395–22,399 |

Fixture construction and prerequisite generation occur outside the timed region. These figures measure one full source-verification run and are a development baseline, not the future complete semantic-resolution performance gate.

## Boundaries and Known Limitations

- Phase 2.2.2 validates the package-identity artifact structurally but does not consume individual proofs.
- Proof manifest digests must be revalidated when import resolution is implemented in the separately authorized Phase 2.2.5 milestone.
- Package and relationship configuration limits are validated now but become active only when those bounded operations exist.
- No AST, token set, source text, `go/types` state, cache, or previous semantic inventory is persisted.
- No repository command, network access, download, module-cache read, or repository write occurs.

## Exit Assessment

The Phase 2.2.2 implementation satisfies its local exit gate: deterministic source verification, immutable candidate artifact construction, security-boundary behavior, statement coverage above 80%, full regression, vet, repeatable benchmarks, and race detection all pass.

Engineering accepted this evidence on 2026-07-23 and authorized Phase 2.2.3 only. Later milestones remain gated by their own acceptance criteria.
