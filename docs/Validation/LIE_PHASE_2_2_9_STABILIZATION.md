# LIE Phase 2.2.9 Stabilization Report

## Status

- Date: 2026-07-23
- Phase 2.2.8 acceptance commit: `d64e9e7`
- Frozen prerequisite: `GoPackageIdentityInventory 1.0.0`
- Frozen artifact: `GoSemanticInventory 1.0.0`
- Stabilization implementation: accepted on 2026-07-23
- Release decision: approved
- Version promotion and tags: complete

## Scope

This milestone adds no semantic capability. It audits and hardens memory,
performance, public contracts, artifact immutability, dependency direction,
configuration, wire formats, documentation, and release evidence.

## Memory Stabilization

Targeted allocation profiling showed that one rebuild repeatedly cloned large
syntax/package-proof inventories and retained per-file candidates after they
had been consolidated. Artifact construction also copied private result slices
that no external caller could reference.

The candidate now:

- snapshots each immutable prerequisite collection once per rebuild;
- uses those detached snapshots throughout validation and resolution;
- releases per-file declaration/reference/type/diagnostic buffers after
  consolidation;
- releases verified source bytes after bounded interface analysis;
- transfers freshly created private result slices into the immutable artifact
  without another construction copy;
- continues to deep-copy every public accessor and detached JSON view.

### Real-Repository Comparison

| Repository | Accepted peak | Stabilized peak | Artifact equivalence |
|---|---:|---:|---:|
| OpenTelemetry Go | 1,384.6 MiB | 1,059.1 MiB | identical hash |
| Kubernetes | 5,080.3–5,250.7 MiB | 3,884.9–3,918.0 MiB | identical hash |

Kubernetes produced the same
`0ee39ff75da62a68e0e674e8cd758b8d26b59f69fa690cd65af4f6cba50f6ce0`
hash, 7,061,102 omitted relationships, 1,000 diagnostics, and semantic counts
in both stabilization passes. OpenTelemetry likewise preserved its accepted
artifact hash.

A full Kubernetes memory-profile attempt was rejected as measurement evidence:
profiling overhead raised process commitment above 11 GiB on a 7.7-GiB
workstation and forced paging. The exact isolated test process was stopped
safely. Targeted profiles and unprofiled real-run peak sampling produced stable
actionable measurements instead.

## Performance Gate

The proposed release gate is documented in
[BENCHMARK_SUMMARY.md](../Releases/GoSemanticInventory-1.0.0/BENCHMARK_SUMMARY.md).
Every proposed target passes:

- package identity 10,000-proof benchmark: 55.0–58.3 ms;
- full semantic candidate, 1,000 files: 51–248 ms;
- OpenTelemetry semantic rebuild: 14.14 s, 1,059.1 MiB peak;
- Kubernetes semantic rebuilds: 116.9 s and 283.0 s, 3.88–3.92 GiB peak;
- pinned OpenTelemetry cancellation: observed in 471.28 ms after a 10 ms
  request delay.

The release proposal gates semantic engine time, not uncontrolled Windows
filesystem cache time. Filesystem-inclusive pipeline observations remain in the
benchmark record.

## Package Identity Contract Review

- Proposed version: `1.0.0`.
- Stable proof-ID scheme: `go-package-proof-id/v1`; golden vector unchanged.
- Proof precedence, manifest evidence, path boundaries, and ordering reviewed.
- Config zero/default behavior and eight-worker cap reviewed.
- Added explicit JSON field names to configuration.
- Added detached `GoPackageIdentityInventoryView` and direct artifact JSON.
- Added strict marshal/unmarshal for every public enum; invalid/unknown wire
  values fail.
- Deep-copy accessors and the new JSON view pass mutation tests.

## Semantic Contract Review

- Proposed version: `1.0.0`.
- Stable ID scheme: `go-semantic-id/v1`; accepted IDs unchanged.
- Exported artifact models, enum strings, JSON keys, ordering, locations,
  diagnostic behavior, limit aggregation, and error boundaries reviewed.
- `Config` zero values remain documented defaults; workers remain capped at
  eight.
- Typed accessors and `GoSemanticInventoryView` remain deeply immutable.
- Artifact-source references will promote package identity and semantic
  versions together only after acceptance.

The proposed contracts are recorded in
[GO_PACKAGE_IDENTITY_PUBLIC_API.md](../API/GO_PACKAGE_IDENTITY_PUBLIC_API.md)
and [GO_SEMANTIC_PUBLIC_API.md](../API/GO_SEMANTIC_PUBLIC_API.md).

## Dependency and Security Audit

- `go list` resolves the full dependency graph without a cycle.
- Semantic dependency direction remains `rie` -> Go syntax -> package identity
  -> semantic; no prerequisite imports the semantic package.
- Runtime imports contain no `go/packages`, `os/exec`, `net/http`, or module
  cache integration.
- Real validation confirms no repository mutation, boundary escape, network
  access, dependency download, or repository command execution.
- Source, AST, token, and `go/types` state are absent from both artifacts.

## Quality Gates

| Check | Result |
|---|---:|
| Package-identity tests | PASS |
| Semantic tests | PASS |
| Ten shuffled runs | PASS |
| Full backend regression | PASS |
| `go vet ./...` | PASS |
| Package-identity statement coverage | 86.9% |
| Semantic statement coverage | 86.0% |
| Targeted race tests | PASS |
| Full backend race tests | PASS |
| Data races | 0 |
| `gofmt` / `git diff --check` | PASS |

## Release Package

Prepared release-candidate packages contain changelogs, release notes, known
limitations, supported feature matrices, benchmark evidence, architecture
references, public API references, and validation links.

## Remaining Documented Limitations

- no compiler-equivalent build context;
- no external dependency or standard-library importer;
- full rebuild only;
- default relationship cap can exhaust later relationship categories;
- Kubernetes-scale execution remains memory intensive despite the reduction;
- cooperative cancellation is bounded by synchronous parser/type-check units;
- no reproducible local Windows cold-cache gate.

None is classified as a correctness, security, crash, race, mutation, or
determinism defect. Each produces an explicit bounded state or is assigned to a
future approved milestone.

## Exit-Gate Assessment

Engineering accepted the Phase 2.2.9 stabilization implementation and release
evidence on 2026-07-23. The following release-management actions are authorized:

1. changing `GoPackageIdentityInventory` and its engine to `1.0.0`;
2. changing the semantic prerequisite reference and `GoSemanticInventory` to
   `1.0.0`;
3. committing the freeze changes;
4. creating and pushing the annotated namespaced release tag.

Package Identity was promoted and tagged first. Semantic Inventory was then
promoted, passed the final versioned gates, and received its annotated tag.
