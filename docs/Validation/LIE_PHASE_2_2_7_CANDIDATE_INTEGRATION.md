# LIE Phase 2.2.7 Validation — Candidate Integration

## Status

- Date: 2026-07-23
- Milestone: Phase 2.2.7
- Implementation: complete
- Local exit gate: passed
- Governance: accepted by engineering on 2026-07-23
- Next milestone: Phase 2.2.8 authorized; Phase 2.2.9 remains unauthorized

## Scope Validated

Phase 2.2.7 integrates the semantic candidate additively after the frozen Go
Phase 2.1 artifact. The integrator:

- retrieves `RepositorySnapshot 1.0.0`, `GoLanguageInventory 1.0.0`, and
  `GoPackageIdentityInventory 0.1.0` by exact type from `rie.ArtifactStore`;
- performs a complete semantic rebuild from those immutable prerequisites;
- publishes `GoSemanticInventory 0.1.0` exactly once;
- adds no mutable `RunContext` fields and does not alter prerequisite artifacts;
- rejects missing or wrongly typed artifacts, nil/canceled contexts, nil stores,
  and duplicate publication; and
- exposes reporting as a detached `GoSemanticInventoryView`, never as an engine
  input.

Phase 2.2.8 real-repository validation and Phase 2.2.9 release stabilization are
not part of this milestone.

## Behavioral Evidence

Tests prove:

- the returned inventory and the store-published inventory are identical;
- Phase 2.1 syntax and package-identity artifacts coexist after publication;
- presentation slices, nested slices, diagnostics, and statistic maps cannot
  mutate the immutable semantic artifact;
- `json.Marshal(inventory)` is the deterministic detached view;
- all reporting collections are explicit, including empty `[]` collections;
- reusing one integrator for different repositories does not leak declarations;
- identical prerequisites in separate stores produce byte-identical JSON;
- a second run against the same store is rejected rather than reusing or
  replacing prior semantic state;
- source verification, repository boundaries, relationship/diagnostic limits,
  cancellation checkpoints, deterministic ordering, and suppression behavior
  continue to pass through the complete semantic package suite.

## Validation Results

| Check | Result |
|---|---:|
| `go test ./lie/golang/semantic -count=1` | PASS |
| `go test ./lie/golang/semantic -shuffle=on -count=10` | PASS |
| `go test ./...` | PASS |
| `go vet ./...` | PASS |
| Semantic statement coverage | 85.5% |
| Targeted `go test -race ./lie/golang/semantic` | PASS |
| Full backend `go test -race ./...` | PASS |
| Data races | 0 |

Race validation used Go 1.26.2 on Windows/amd64 with MSYS2 UCRT64 GCC 16.1.0
and `CGO_ENABLED=1`.

## Repeatable Integration Benchmark

Reference local validation environment:

- identity: developer workstation (not hosted CI);
- OS: Windows 11 Home Single Language 10.0.26200, build 26200;
- architecture: amd64;
- CPU: 12th Gen Intel Core i5-12450H, 12 logical processors;
- memory: 7.7 GiB visible;
- Go: 1.26.2;
- configured semantic workers: 8.

The fixture contains 1,000 Go files in one module. Each file declares one
unique concrete type and method. Frozen prerequisite construction occurs
outside the timed region. The timed operation creates a fresh artifact store,
adds the three immutable prerequisites, performs a full semantic rebuild, and
publishes the semantic artifact.

| Fixture | Time | Bytes/op | Allocations/op |
|---|---:|---:|---:|
| 1,000-file candidate integration | 32.70–50.07 ms | 30.39–30.41 MB | 237,275–237,280 |

Command:

```text
go test ./lie/golang/semantic -run NoTests -bench BenchmarkCandidateIntegration1000Files -benchmem -count=3
```

The benchmark is a local candidate baseline, not the final
`GoSemanticInventory 1.0.0` release gate.

## Exit-Gate State

The Phase 2.2.7 implementation satisfies its documented local quality gate:
typed additive publication, immutable reporting, full-rebuild isolation,
deterministic output, explicit prerequisite failures, cancellation, coverage
above 80%, regression, vet, shuffled execution, repeatable benchmarking, and
race detection all pass.

Engineering accepted Phase 2.2.7 on 2026-07-23 and authorized Phase 2.2.8
real-repository validation only. Phase 2.2.9 remains gated by its own exit
criteria.
