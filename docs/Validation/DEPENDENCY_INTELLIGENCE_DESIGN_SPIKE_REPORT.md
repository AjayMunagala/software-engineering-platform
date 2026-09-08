# Phase 5.0.1 Dependency Intelligence Design Spike Report

## Status

- Date: 2026-09-08
- Spike implementation: complete
- Review status: engineering accepted
- ADR 0020: `Accepted`
- Phase 5.0.2 neutral graph artifact and core: authorized
- Phase 5.0.3 and later: not authorized
- Production release: not authorized
- Evidence commit: `289f51d881da29ad8b5f128574b4d01106a99272`

## Purpose

Validate the risky assumptions in the accepted Phase 5.0 design before
creating production Dependency Intelligence packages.

The reproducible spike is isolated at
`experiments/phase-5.0.1-dependency-spike`. `backend/` does not import it. The
graph core accepts typed in-memory facts; a spike-only adapter proves that the
released RIE and Go LIE artifact types can be consumed without changing or
mutating their `1.0.0` contracts.

## Environment

| Item | Windows | Ubuntu/WSL |
|---|---|---|
| OS | Windows 11 Home Single Language `10.0.26200` | Ubuntu `24.04.4 LTS` |
| Architecture | amd64 | amd64 |
| CPU | 12th Gen Intel Core i5-12450H, 12 logical processors | same host |
| Visible memory | 8,090,104 KiB | 3,861,132 KiB |
| Go module toolchain | `go1.26.2` | `go1.26.2` selected from local Go `1.22.2` |
| Git | `2.54.0.windows.1` | `2.43.0` |
| External services | none | none |

## Validation matrix

| Assumption | Evidence | Result |
|---|---|---|
| Released artifacts are sufficient | Disposable two-module Go workspace produced real `RepositorySnapshot`, `GoLanguageInventory`, `GoPackageIdentityInventory`, and `GoSemanticInventory`; the adapter emitted package and module edges | Pass |
| Prerequisites remain immutable | Exact JSON bytes of all three Go artifacts were unchanged after normalization | Pass |
| IDs are unambiguous and portable | Domain-separated unsigned 64-bit big-endian UTF-8 length-prefixed segments produced fixed node, edge, SCC, and cycle vectors on Windows and Ubuntu | Pass |
| Full result bytes are deterministic | Three-graph fixture canonical JSON SHA-256 was identical on both operating systems | Pass |
| Input order and workers do not affect output | Shuffled nodes/edges and one/eight-worker runs were deeply equal | Pass |
| Duplicate aggregation is deterministic | Occurrences were exact; unique evidence was sorted before its cap; omissions were counted | Pass |
| Edge limits are deterministic | The canonical edge order was capped identically after shuffled input under one/eight workers | Pass |
| SCC work is bounded | Deterministic Tarjan traversal partitioned every eligible node once without enumerating simple cycles | Pass |
| Unknown is not guessed | Unresolved and ambiguous edges did not participate in local SCCs | Pass |
| Cycle semantics need provenance | File cycles are informational; Go package cycles use explicit rule `go-import-cycle`; neutral structural cycles remain structural | Pass with design refinement |
| Impact is bounded | Canonical breadth-first forward/reverse traversal enforced depth/node caps and explicit truncation | Pass |
| Cancellation is bounded | Work canceled at the 1,024-edge checkpoint; ten-run benchmark averaged 45.43 ms including node normalization | Pass |
| Scale target is viable | 100,000 nodes and 1,000,000 unique edges completed build plus SCC below 30 seconds on both hosts | Pass |

## Released-artifact normalization finding

The spike constructed a disposable Go workspace with two local modules and an
explicit local replacement. It ran the released RIE discovery/ignore/language
pipeline, Go Language Engine, Go Package Identity Engine, and Go Semantic
Engine to create real frozen artifacts.

The adapter consumed only public immutable accessors and produced:

- local module nodes;
- local package and file nodes;
- a proof-backed package import edge;
- a cross-module edge aggregated from that package relationship;
- semantic file relationships only where the released semantic artifact
  provided an exact target.

No existing contract required modification. The spike confirms that the
artifact boundary is sufficient. It also confirms an intentional limitation:
Dependency Intelligence cannot invent cross-package file targets where
`GoSemanticInventory 1.0.0` does not prove an exact declaration target.

## Stable identity finding

Delimiter-only concatenation is not accepted. The candidate encoding is:

1. scheme domain as the first segment;
2. ordered logical identity segments;
3. unsigned 64-bit big-endian byte length;
4. exact UTF-8 bytes;
5. lowercase SHA-256 textual result.

Committed candidate vectors are documented in
`docs/API/DEPENDENCY_INTELLIGENCE_GOLDEN_VECTORS.md`.

Representative node vector:

```text
dependency-node-id/v1:sha256:16c0f7392130f9bad33f66087e9c0b46c1f2823ba8cf7ea491187b35494c8a64
```

Canonical cyclic-fixture JSON SHA-256:

```text
5ee754fe619d0920f2473e06a04f6bd1372b768c565c82c65281c7b2e922b6dc
```

Decision: these are accepted as Phase 5 production golden-vector conditions.
Containment ID vectors must be added with the Phase 5.0.2 model before any
containment artifact is published.

## Graph and cycle findings

- Direct dependency adjacency is sufficient for deterministic SCC and impact
  operations.
- Persisting all-pairs reachability remains unjustified.
- SCC membership is a bounded representation of cycles; enumerating all simple
  cycles remains prohibited.
- Containment remains distinct from dependency.
- Language validity is not a property of graph shape alone. The design now
  requires an explicit adapter rule for `language_invalid` classifications.
- SCC construction ignores non-local resolution states, so unresolved,
  ambiguous, external, standard-library, and stale boundaries cannot create a
  false local cycle.

## Performance and memory evidence

Command:

```text
go test -run "^$" -bench "Benchmark(BuildAndSCC100KNodes1MEdges|Impact100KNodes1MEdges)$" -benchtime=1x -benchmem ./...
```

| Operation | Windows | Ubuntu/WSL |
|---|---:|---:|
| Build + SCC, 100k nodes / 1m unique edges | 5.373 s | 8.429 s |
| Total allocated bytes/op | 2,777,534,432 | 2,776,551,104 |
| Allocations/op | 34,718,227 | 34,718,207 |
| Impact, 100k reachable nodes / 999,945 traversable edges | 201.67 ms | 281.83 ms |
| Impact allocated bytes/op | 87,121,336 | 87,121,336 |
| Impact allocations/op | 512,181 | 512,181 |

The explicit scale gate independently sampled live heap:

| Host | Scale duration | Peak live Go heap |
|---|---:|---:|
| Windows | 6.954 s | 1,572,774,464 bytes (about 1,500 MiB) |
| Ubuntu/WSL | 7.181 s | 1,516,811,456 bytes (about 1,447 MiB) |

Both runs stayed below the spike's four-GiB host-protection ceiling and the
proposed 30-second time gate. The original "1.5 times normalized input plus
output" memory ratio cannot be measured reproducibly from Go object graphs and
is therefore not suitable as written. The recommendation is to replace it with
an initial two-GiB peak-live-heap gate for this exact synthetic fixture while
continuing to report total allocations. Production Phase 5.0.2 should reduce
allocation count through compact internal keys before release stabilization.

## Cancellation evidence

The experimental worker checks context at most every 1,024 raw edges. The
cancellation benchmark includes normalization of 10,000 nodes plus processing
through the first cancellation checkpoint:

| Operation | Time/op | Bytes/op | Allocations/op |
|---|---:|---:|---:|
| Cancel at 1,024 edges (Windows) | 2.17 ms | 4,791,988 | 35,310 |
| Cancel at 1,024 edges (Ubuntu/WSL) | 3.07 ms | 4,791,987 | 35,310 |

This passes the proposed 250 ms reference-host target. Production code must
retain bounded units for normalization, SCC roots, and traversal as specified
by the architecture.

## Verification results

| Gate | Windows | Ubuntu/WSL |
|---|---|---|
| Functional tests | Pass | Pass |
| Ten shuffled runs | Pass | Pass |
| `go vet` | Pass | Pass |
| Race test | Pass, zero races | Pass, zero races |
| Statement coverage | 85.4% | 85.4% |
| Golden ID/result vectors | Pass | Pass |
| 100k/1m scale gate | Pass | Pass |

Full backend regression and `go vet ./...` pass on Windows and Ubuntu. The
experiment's `go.mod` uses a local replacement for the released backend module;
there is no remote dependency on unpublished code.

## Security and dependency audit

- `backend/` does not import the experiment module.
- The graph core imports no filesystem, command, network, database, transport,
  persistence, runtime, or model/LLM package.
- Filesystem writes occur only in test setup to produce released artifacts in
  a temporary directory.
- No credentials, source handles, absolute host paths, raw source, or database
  identities are emitted by graph results.
- The adapter consumes released public artifact accessors and does not mutate
  prerequisites.

## Candidate API finding

The proposed workflows are sufficient: full deterministic rebuild plus pure
bounded queries. To remain consistent with all released artifact packages, the
production `DependencyInventory` should be a concrete immutable value with
private fields and defensive accessors rather than a public artifact interface.
Consumers should depend on the narrow engine/query interfaces.

No public database, runtime, Repository Service, filesystem, parser, or
transport type is needed.

## Design changes resulting from the spike

1. Specified exact stable-ID canonical bytes and committed cross-platform
   vectors.
2. Added explicit cycle-classification rule provenance; neutral graph code must
   not hardcode language validity.
3. Recommended a concrete immutable inventory value for consistency with
   released artifacts.
4. Replaced the ambiguous memory-ratio proposal with a measurable candidate
   two-GiB peak-live-heap gate for the exact 100k/1m fixture.
5. Retained direct adjacency, SCCs, bounded BFS impact, and the full-rebuild
   reference model without architectural redesign.

## Known limitations

- The code is deliberately disposable and not optimized for production
  allocation counts.
- The first real adapter evidence is Go-only.
- No exact cross-package file edge is emitted without a released semantic
  declaration target.
- Containment normalization and its golden IDs remain Phase 5.0.2 work.
- No real-repository corpus is executed; that remains Phase 5.0.6.
- No persistence, Repository Service profile, runtime, API, or UI integration
  is implemented.
- No call graph, control flow, data flow, architecture policy, AI, or patching
  capability is present.

## Exit-gate assessment

The spike provides positive evidence for the proposed architecture, artifact
boundaries, deterministic IDs, normalization, aggregation, SCCs, cycle policy,
bounded impact, cancellation, and scale targets. No finding requires changing
the subsystem boundary.

## Recommendation

The engineering review accepted this report, the golden vectors, design
refinements, and isolated harness on 2026-09-08. The resulting state is:

1. ADR 0020 is `Accepted`;
2. the node, edge, SCC, and cycle stable-ID encoding/vectors are frozen;
3. the measurable two-GiB synthetic peak-heap gate is adopted;
4. production `MaxNodes` enforcement is mandatory;
5. containment publication requires a frozen Phase 5.0.2 golden vector;
6. only Phase 5.0.2 - Neutral Graph Artifact and Core is authorized;
7. Phase 5.0.3 and later milestones remain unauthorized.
