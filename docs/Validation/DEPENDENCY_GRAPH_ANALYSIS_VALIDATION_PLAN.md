# Phase 5.0.4 — Validation Architecture and Plan

Design approved at commit `a2680da36cde620cf436b6ba1473b23f7c18e607`.
No tests or algorithms are implemented/executed by this package. Execution still
requires explicit implementation authorization; design approval does not grant it.

## Layered evidence

First run accepted core conformance and frozen ID vectors; then candidate model/
limit tests, independent graph oracles, deterministic query/cursor tests, and
adapter-produced fixtures. Full regression, cross-platform race/fuzz, and scale
measurements follow correctness. Fixture engine execution is outside timed analysis.

Keep independent reference algorithms in tests only: small-graph transitive
reachability equivalence for SCC membership (not another copy of production
Tarjan), simple bounded BFS for reachability, hand-authored direct pages. Generated
graphs have fixed seeds and explicit limits. Never use same-implementation repeat
equality as the sole correctness oracle.

## Mandatory assertions

| Area | Required evidence |
|---|---|
| Inputs | Zero metadata, unknown schemes/enums, duplicate/conflicting IDs, dangling endpoints fail with zero output |
| Graph projection | Module/package/file separated; containment and file-to-package edges never form local SCC paths |
| SCC | Empty, isolated, DAG, self-loop, two-node cycle, disconnected cycles, deep chain, large SCC, parallel edge kinds |
| Partition | Every matching local vertex occurs exactly once; condensation is acyclic; independent oracle agrees |
| Cycle summaries | Exactly one per cyclic SCC; frozen IDs; structural rule only; no language-invalid guess |
| Partial inputs | Omitted topology and unresolved boundaries marked; no repository acyclic/full-impact assertion |
| Explanations | Evidence/containment loss distinguished from adjacency loss; unknown diagnostics conservatively partial |
| Direct queries | Incoming/outgoing, deduped neighbors, exact edge IDs, isolated roots, invalid roots, page boundaries |
| Cursors | Independent vectors; wrong digest/root/graph/direction/page size; malformed/noncanonical/oversized input; changed evidence |
| Impact | Multiple seeds, duplicate seeds, reverse/forward, shortest depth, cycles, terminal boundaries, no seed re-addition |
| Limits | Depth/node/edge exact boundary and one-over; stop-before-exceed; no invented remaining counts; deterministic prefix |
| Hard caps | Input evidence/aux/graph and SCC/cycle limits return no inventory; integer overflow tests |
| Cancellation | Nil, canceled, deadline, each batch, before return; no partial result on cancellation |
| Immutability | Prerequisites unchanged; mutating detached metadata/pages/slices cannot change results |
| Compatibility | Existing Normalize/Go adapter golden bytes unchanged; envelope absent for core-only output |
| Scope | No Go-engine invocation/source/network/process/database/runtime/service imports in new neutral behavior |

Freeze new fingerprint and cursor expected bytes using an independent calculation
committed BEFORE production encoding. Include empty arrays, Unicode byte lengths,
JSON escaping, changed source evidence, and cursor query separation. Reuse existing
SCC/cycle vectors without regeneration. Any conflict requires a reviewed scheme
change, not replacement of expected values to match code.

## Determinism and quality gates

Compare full output JSON, derived metadata, counts/reasons, and cursor bytes under
shuffled inputs, random map insertion, repeat processes, Windows and Ubuntu.
Algorithms are serial; compare supplied one/eight-worker adapter outputs as inputs
without claiming algorithm parallelism. Run full backend tests/vet/shuffle/race on
both systems and targeted fuzz campaigns for graphs, budgets, and cursors. Candidate
new-code coverage target >=85%; record exact commands/counts and excluded tests.

## Performance and memory

Use 100/1,000/10,000 vertex fixtures, duplicate-heavy edges, a deep chain, and the
accepted exact 100,000-node/1,000,000-edge scale fixture. Record corpus seed, graph
counts, input fingerprint, Go/Git/OS/CPU/RAM, GOMAXPROCS, GC policy, and sampled heap.
Separate input clones/digest, index construction/sort, SCC work, publication, direct
query rebuild/page, and BFS. Keep allocations, sampled peak live heap, and RSS
separate. Peak scope must say whether retained input is included. No inflated
claim that count caps alone guarantee two GiB.

The existing 30-second/two-GiB core scale gate remains unchanged for its original
fixture/workload. Report new analysis overhead separately; do not silently apply
or revise that gate to a different end-to-end workload. Propose any new threshold
from measurements for engineering approval. Report failures as correctness,
timeout, memory ceiling, cancellation, or environment—not generic 'resource error'.

Time cancellation with controlled mid-work triggers plus checkpoint tests. Publish
maximum observed latency, workload, and uninterruptible clone/sort/encoder units;
observations do not establish an unconditional upper bound.

## Exit and reporting

Implementation (when authorized) supplies a report with commands, machine-readable
counts/digests, benchmark methodology, limitations, defects and traceable fixes,
and commit URLs. No fake data or presumed pass results. Preserve both open direct-
core hardening obligations. Engineering acceptance is required to close 5.0.4;
integration and release remain separately gated. This design commit only checks
documentation consistency/links/scope and claims no new execution metrics.
