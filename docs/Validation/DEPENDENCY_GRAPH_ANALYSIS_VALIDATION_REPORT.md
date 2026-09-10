# Phase 5.0.4 — Dependency Graph Analysis Validation Report

Date: 2026-09-10. Candidate: **0.1.0**. Implementation submitted for engineering
acceptance; this report does not accept the milestone, promote ADR 0022, authorize
5.0.5, or authorize a production release. DIE-HARDEN-001/002 remain open.

## Authorization and identity provenance

Design: `a2680da36cde620cf436b6ba1473b23f7c18e607`; design approval recorded at
`140457d4514f7056ae78a518a0ae94b248a8e379`. Engineering explicitly authorized
implementation, starting with independent vectors before production encoding.

- Independent .NET/PowerShell digest/cursor outputs were committed and pushed at
  [`ab4628c`](https://github.com/AjayMunagala/software-engineering-platform/commit/ab4628c)
  before any production encoder/cursor implementation.
- [`c0fdd00`](https://github.com/AjayMunagala/software-engineering-platform/commit/c0fdd00)
  corrects the vector document's prose from 27 bytes/`1b` to 28 bytes/`1c`.
  Independent recalculation proved the already-frozen hashes used 28 correctly.
  **No digest/cursor expected value changed.**
- Existing SCC/cycle golden vectors are reused without regeneration.

## Implemented scope

Opt-in neutral Analyzer and QueryEngine, immutable configuration/queries/results,
bounded validated input extraction, iterative Tarjan SCCs, one structural summary
per cyclic SCC, direct dependency/dependent pages, canonical fingerprint-bound
cursors, and bounded forward/reverse multi-source BFS. Input evidence, base IDs,
statistics, and diagnostics are preserved. Core.Normalize never runs algorithms.

The same-package implementation reads the private immutable backing view to check
counts before allocating indexes. It does not clone all inputs through accessors.
Publication still returns defensively copied base data. AnalysisMetadata and
GraphAnalysis are detached view records, never mutable handles into an inventory.

Excluded: changed core/Go-adapter behavior, released RIE/LIE contracts, language-
invalid cycle classification, all-simple-cycle enumeration, persistence/runtime/
Repository Service integration, network/source/process execution, AI, release,
and all downstream milestones. The Go-adapter fixture composition is test-only.

## Correctness evidence

| Area | Evidence |
|---|---|
| SCC membership | 100 fixed-seed small graphs checked against independent Floyd-Warshall reachability equivalence |
| BFS | Forward/reverse reachability and shortest depths checked by independent repeated edge relaxation |
| Deep graphs | 10,000-node chain completes with 10,000 singleton SCCs; no recursive DFS |
| Projection | Separate module/package/file cycles, parallel edge kinds, containment exclusion, cross-kind boundaries |
| Partial knowledge | Topology and explanation omissions distinguished; unknown diagnostics conservatively partial |
| Traversal | Multiple/deduplicated seeds, cycles, depth/node/edge caps, terminal boundaries, later-depth resolved promotion |
| Pages | Incoming/outgoing, deduplication, empty roots, deterministic pages, cursor replay and mismatch rejection |
| Integrity | Metadata/scheme/ID/enum/endpoint/occurrence validation; duplicate IDs and malformed inputs fail closed |
| Limits | Evidence/aux/input/SCC/cycle caps and overflow rejection; no partial artifact on hard failure |
| Cancellation | Every observable checkpoint in a representative analyzed-input fixture injected deterministically; no partial results |
| Immutability | Input bytes unchanged; detached metadata/page/impact slices mutated without changing stored results; concurrent calls |
| Compatibility | Accepted core conformance and Go adapter tests pass; core-only JSON omits analysis metadata |
| Fingerprint | Independent frozen vectors plus complete accepted JSON representation cross-check, including empty evidence |

Queries inspect the supplied graph only. TopologyLimited=false is not proof of
complete repository knowledge; a negative SCC finding never proves repository
acyclicity. Neighbor Resolution describes the node, not all incident edge states.

## Cross-platform determinism

64-node/256-edge canonical fixture, eight shuffles per execution, repeated clean
test processes on Windows and Ubuntu:

```text
input fingerprint:
dependency-analysis-input/v1:sha256:05ac645adb2d16a6f9e0f7bed74b6fcb6f6568eca02ac8fd9111f101720dde97
combined analysis/page/impact JSON SHA-256:
0d22e53485a3996b062943a135808dbd9102b26edd468956ebf88dce4e28d7ef
```

Test-only released RIE/Go LIE/Go adapter composition with upstream workers 1 and 8
produced identical analyzed JSON on both platforms:

```text
3bb8931e0c1cdf70cb56b75ee919e2cd8470c097dc01bed1226268df1717e2d6
```

The analyzer itself is serial; upstream worker comparisons do not imply parallel
SCC processing. Stable IDs, field ordering, cursors, and omission reasons matched.

## Quality checks

Executed from `backend/` on Windows and Ubuntu (Go 1.26.2):

```text
go test ./... -count=1
go vet ./...
go test ./... -count=3 -shuffle=314159
go test -race ./... -count=1
go test ./die/... -count=3 -shuffle=271828
go test ./die/... -count=1 -coverprofile=analysis_coverage.out
go test ./die -run '^$' -fuzz '^FuzzAnalysisGraph$' -fuzztime=20s -parallel=2
go test ./die -run '^$' -fuzz '^FuzzAnalysisCursor$' -fuzztime=20s -parallel=2
```

All executed regression/vet/shuffle/race commands passed. No races were reported.
Coverage was collected on Windows: die **93.9%**, conformance **86.2%**, Go adapter
**95.2%**. New analysis production files cover **528/551 statements (95.83%)**,
above the 85% candidate target. Coverage does not claim unexecuted live-database
tests: opt-in PostgreSQL/runtime integration and unrelated large-corpus tests
remain skipped by the default backend suite and were not introduced by this phase.

| Fuzz target | Windows executions | Ubuntu executions |
|---|---:|---:|
| Graph, initial fixed-budget campaign | 89,550 | 121,200 |
| Graph, final variable-budget/direction campaign | 110,590 | 69,080 |
| Cursor | 10,409 | 13,000 |
| Total | 210,549 | 203,280 |

Total **413,829** executions, all passing. Each campaign used 20 seconds and two
workers. Final graph fuzzing varies depth/node/edge budgets and both directions,
asserting that returned counts never exceed the query limits.
Fuzz throughput is not a performance
gate; cursor campaigns spent time without increasing execution counts while
processing/minimizing interesting inputs. No failure corpus was produced.

## Performance methodology

Reference host: Intel Core i5-12450H, 12 logical processors, Windows 11
10.0.26200, 8,090,104 KiB visible RAM. Ubuntu-24.04 runs under WSL2 kernel
6.18.33.2-microsoft-standard-WSL2, 3,770 MiB guest RAM and 1,024 MiB swap.
Go 1.26.2 windows/amd64 and linux/amd64. Git versions: Windows 2.54.0.windows.1,
Ubuntu 2.43.0. GOMAXPROCS=12, GOGC=100. No new external dependencies.

Large tests and benchmarks run one platform at a time with GOMEMLIMIT=1536MiB
because the host has limited available RAM. This is a **soft runtime GC target**,
not a hard process/heap ceiling. Results are characterization, not a newly approved
gate. Core's existing 30-second/2-GiB gate and test source are unchanged.

Benchmark fixtures use 100/1,000/10,000 package vertices and four edges per vertex:
from=i%n; to=(from+1+i/n)%n. Input construction/normalization occurs outside the
timed regions. Three single-iteration measurements (`-benchtime=1x -count=3`), not
p95 estimates. Stage timings are independent samples, not additive accounting:

- IndexAndDigest includes validation, indexing, adjacency sorting, and hashing.
- SCCAndPublication starts with a prepared index; includes SCC/cycles and output.
- Analyze includes all production stages.
- Direct and Impact rebuild the full index/digest for each query.
- Fingerprint measures record-streamed encoding/hash alone.
- OutputClone measures copying the base inventory, not RSS or all publication.

Detailed benchmark ranges and exact scale values are in the machine-readable
[`DEPENDENCY_GRAPH_ANALYSIS_RESULTS.json`](DEPENDENCY_GRAPH_ANALYSIS_RESULTS.json).
For 10,000 nodes/40,000 edges, final three-sample full Analyze ranges were
136.2304–140.3646 ms on Windows and 137.506335–149.346199 ms on Ubuntu.
No claim that pagination makes input work
O(page size), nor that the complete sorted pipeline is O(V+E).

## Exact 100,000-node / 1,000,000-edge fixture

Reuses the accepted core fixture's names, paths, evidence, and topology:
node i=`example.com/p%06d`, path=`p/%06d`, from=i%100000,
to=(from+1+i/100000)%100000. All edges are package imports, resolved_local.
The resulting eligible graph has one SCC and one structural cycle summary.

Input fingerprint on both platforms:

```text
dependency-analysis-input/v1:sha256:b0bc2922b9879b3c9f18494f9e975857d132a6cbe3bea4db99387ccf49985c65
```

| Measurement | Windows | Ubuntu |
|---|---:|---:|
| Analysis wall time | 6.810243 s | 4.786344373 s |
| Retained input heap before analysis | 277,161,464 B | 277,139,104 B |
| Sampled peak heap during analysis | 755,367,248 B | 711,763,352 B |
| Total allocation delta during analysis | 1,893,934,464 B | 1,893,720,296 B |
| Unchanged core gate wall time | 4.3951724 s | 4.587171417 s |
| Unchanged core gate sampled peak heap | 1,020,791,112 B | 1,024,105,960 B |

The analysis heap sampler runs every 5 ms after prerequisite normalization and
GC, includes retained immutable input and eventual output, and is joined before
reporting. TotalAlloc is a cumulative delta; it is **not** simultaneous live heap.
HeapAlloc samples can include garbage awaiting collection and can miss spikes.
These numbers must not be relabeled RSS or unconditional peak bounds. Process RSS
measurement, where recorded in results, includes preparation and runner scope and
is separate from this analysis-stage table.

A separate Ubuntu `/usr/bin/time -v` repeat reported maximum RSS **1,201,328 KiB**
for the process tree, including prerequisite preparation and the Go test runner;
zero swaps. That repeat's analysis took 4.79945743 s with 704,338,760 B sampled
heap and 1,893,942,184 B allocation delta. Windows RSS was not separately measured.

Final deterministic cancellation fixture: **207 checkpoints** across analysis,
direct queries, and impact. Maximum observed trigger-to-return latency was
458,900 ns on Windows and 9,982 ns on Ubuntu. This is a small controlled fixture;
large-record encoding, stable-ID hashing, sorting, and clone latency are not
bounded by those observations. Earlier zero-duration clock readings are not
interpreted as instantaneous cancellation.

## Findings and corrections

1. Draft BFS used one visited set for both uncertain-boundary encounters and
   proven local reachability. A later resolved path could fail to expand that
   node. Separate unique-budget/local-reachability/boundary sets now permit
   promotion without double-counting; same-level and later-depth tests cover it.
2. Streaming serialization now explicitly reproduces public-view empty evidence
   arrays without mutating backing input. Full JSON cross-check guards drift.
3. Vector prose incorrectly counted the domain bytes. Independent recalculation
   found the hashes were correct. Documentation and the test's hard-coded prefix
   were corrected; the frozen expected values were not changed.
4. Cancellation review added checkpoints to SCC pop/self-loop scans, traversal
   seeds/frontiers, and final boundary/edge collection, not merely edge DFS/BFS.

These corrections are part of the implementation submission, not unreported
changes to a released contract. See the implementation commit containing this
report and the separately traceable vector commits above.

## Limitations and acceptance boundary

- DIE-HARDEN-001 and DIE-HARDEN-002 remain open; accepted Core.Normalize is unchanged.
- Counts are not byte-memory ceilings. Arbitrarily large accepted strings and
  individual evidence records can still be expensive.
- Sorting, one JSON record, stable-ID hashing of a large member list, and final
  defensive clones remain uninterruptible units. Checkpoint latency measurements
  do not establish an unconditional wall-clock guarantee.
- Cursors are forgeable consistency tokens, never authorization credentials.
- No filesystem freshness, common-scan authority, build context, language-invalid
  classification, global acyclicity, or complete impact claim is invented.
- Single-host/WSL timings are not independent physical CI-runner qualification.
- Reanalysis must still satisfy auxiliary-input caps, including existing derived
  membership/reason records. For very large derived inventories a larger
  MaxInputAuxRecords can be necessary; derived facts are recomputed, never trusted
  or silently dropped to evade that input bound.
- Candidate API/artifact stays 0.1.0. Engineering acceptance, ADR promotion,
  integration, downstream phases, and release remain separate decisions.
