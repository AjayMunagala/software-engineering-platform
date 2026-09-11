# Phase 5.0.4 — SCC, Cycle, and Impact Design

## Status

Design approved at `a2680da36cde620cf436b6ba1473b23f7c18e607` and recorded at
`140457d`. Engineering subsequently authorized Phase 5.0.4 implementation only.
Phase 5.0.3 is accepted at governance commit `7a04db8`; candidate remains 0.1.0.
Engineering accepted implementation and evidence at `2390347` on 2026-09-11.
ADR 0022 is Accepted. Phase 5.0.5, integration, and release remain unauthorized.
DIE-HARDEN-001 and DIE-HARDEN-002 remain open.

Review together with `DEPENDENCY_GRAPH_ANALYSIS_ARTIFACTS.md`,
`docs/API/DEPENDENCY_GRAPH_ANALYSIS_CANDIDATE_API.md`, ADR 0022, and
`docs/Validation/DEPENDENCY_GRAPH_ANALYSIS_VALIDATION_PLAN.md`.

## Ownership and compatibility

Propose a separate opt-in analysis capability in the existing neutral `backend/die`
package. It consumes only an immutable `die.DependencyInventory` produced by the
accepted core/adapter. It does not consume Go syntax, source, manifests, runtime,
persistence, Repository Service, or any mutable run context.

`Core.Normalize` and the Go adapter retain their behavior: neither automatically
runs algorithms. A new analyzer returns a new inventory with derived SCC/cycle
collections and explicit analysis metadata. The input is never mutated.
Private construction remains in `die`; no public mutable artifact setter or
cross-package constructor backdoor is introduced. Narrow query capabilities
consume the inventory independently; they do not require SCC computation first.

The sole proposed model refinement is an optional analysis envelope in the
candidate inventory view, defined in the artifact supplement. Original normalized
inventories omit it, preserving existing JSON bytes and golden vectors. No
released 1.0.0 contract changes. New encodings require independent golden vectors
before implementation publishes them. Candidate refinements require design approval.

## Graph interpretation

For each graph, select local nodes whose kind matches it: module/module,
package/package, file/file. Include isolated matching nodes. Dependency direction
is always dependent -> dependency. Containment never becomes an adjacency edge.

SCC and transitive local adjacency use only edges with `resolved_local` state and
two matching local endpoints in the selected graph. File-to-package import edges
remain visible as boundary edges to direct queries; they are not file-to-file
edges. Nonlocal, stale, ambiguous, or mismatched-kind endpoints never join SCCs.
Do not bridge graphs via containment or silently select an uncertain candidate.

Validate artifact metadata/ID schemes, uniqueness, endpoint existence, graph and
resolution enums, positive occurrence counts, and nonnegative bounded counters
before indexing. Unknown versions, inconsistent IDs, or unexplained dangling edges
fail closed. Metadata fields that are unsigned cannot represent negative counts.
Input component/cycle data is ignored and recomputed for analysis, never trusted
as an optimization. Queries operate on base graph records, not derived metadata.

## SCC and cycle processing

Use an iterative Tarjan-style traversal: explicit frames avoid recursion-stack
exhaustion on deep chains. Visit root IDs and adjacency in ordinal UTF-8 byte order.
Deduplicate adjacency endpoint pairs for SCC work; retain original edge IDs for
queries/provenance. Distinct edge kinds must not duplicate component membership.

Graph work is O(V+E); canonical sorting adds O(V log V + E log E) worst-case work.
Do not claim the complete pipeline is linear. Auxiliary adjacency/state is O(V+E)
with checked input caps, not an unconditional byte-memory guarantee.

Every local node belongs to one component per applicable graph. A component is
cyclic when it has more than one member or a singleton self-edge. Emit one cycle
summary per cyclic component, not every simple cycle and not a witness path.
Reuse frozen SCC and cycle ID fields/order. Sort members bytewise; publish SCCs
and cycles using the already documented canonical orders.

All Phase 5.0.4 cycles use classification `structural` and rule
`dependency-structural-scc/v1`. Do NOT label a Go cycle `language_invalid` just
because nodes say Go: no selected build context or dedicated language-rule proof
exists in the supplied neutral contract. Richer language classification and witness
paths require a separately approved proof contract. This conservatively specializes
the earlier illustrative classification API without weakening ADR 0020.

## Partial knowledge

SCCs are exact for the published eligible subgraph only. Missing topology may merge
components; negative cycle findings cannot prove repository acyclicity. Existing
cycles are still witnessed by present edges. Keep computed components even when
upstream knowledge is partial, accompanied by the mandatory analysis envelope.

Topology omissions, uncertain boundary edges, upstream partial diagnostics, and
unknown/unrecognized diagnostics must produce sorted reason codes. Omitted evidence
alone does not change adjacency, but sets an explanation-limited flag. Omitted
diagnostics prevent claiming full knowledge. No assertion of complete repository
coverage is available from these artifacts, even if all omission counters are zero.

## Direct queries and impact

Direct queries return unique adjacent nodes plus their sorted connecting edge IDs;
occurrences are not duplicate neighbors. Dependencies follow outgoing edges,
dependents follow incoming edges, in the selected graph. Roots may be matching
local vertices or a boundary node actually incident in that graph. Isolated local
roots return an empty page. Unknown roots or graph-incompatible roots fail input
validation. Pages use bytewise node-ID order, not map order.

Impact is canonical multi-source breadth-first traversal of matching local vertices.
Seeds must be matching local nodes, are deduplicated/sorted, and have depth zero.
Report seeds separately from reached nodes; cycles never re-add a seed. Dependents
means potential reverse structural impact, NOT proven runtime/build breakage.
Dependencies means forward structural reachability, NOT change impact on callers.
Nonlocal/mismatched endpoints encountered are terminal boundaries, never expanded.

At each BFS level, process current node IDs in bytewise order and edges ordered by
neighbor ID then edge ID. A unique reached node keeps shortest depth. Returned
edge IDs include examined edges whose endpoints are represented in the result,
not just an arbitrary spanning tree.
Record direction, graph, seeds, visited count, scanned edge count, and reasons.

Depth/node/edge query limits return a deterministic partial result, not a complete
claim. MaxDepth D includes nodes at depth D; if a nonempty frontier at D is not
expanded, report `depth_limit` conservatively without scanning past the budget.
Node capacity includes seeds, reached nodes, and recorded terminal boundary nodes.
Edge budget counts examined edge records, including revisits; retained IDs dedupe.
Stop before exceeding either budget. No exact unvisited count is invented.
Checking an edge consumes one scan unit before examining its target. If that
target would exceed node capacity, stop without adding it or that edge ID; the
scan still counts. Already represented endpoints can retain connecting edges
without consuming another node unit. Boundary edges count even when not expanded.
Impact has no continuation cursor; callers may retry with approved higher limits.

## Lifecycle, limits, cancellation, security

Every call rebuilds indexes; there is no global cache, worker pool, or background
job. Configuration is immutable, calls independent, indexes private. Limit breach
during input validation/SCC output returns an error and no artifact, never a
prefix of components presented as an SCC partition. Query traversal caps are
the distinct partial-result policy above.

Check nil context before use, cancellation before/after each released accessor,
at most every 1,024 indexed/scanned records, each SCC frame batch and BFS batch,
around sorting/digesting, and before return. A clone or standard sort can remain
an uninterruptible work unit; no new wall-clock guarantee is claimed. Cancellation
returns an error and no artifact/page/result, even if some work finished.

Implementation note: because analysis is owned by the same neutral `die` package,
it reads the private immutable backing view without invoking cloning accessors.
This permits count validation before index allocation and avoids an unnecessary
full input clone. It never mutates that view. Returned inventory publication still
defensively clones base data; standard sorts and individual JSON records remain
uninterruptible units. SCC adjacency endpoint deduplication occurs while scanning
the sorted edge records; original edge IDs remain available to queries.

All error text is fixed and redacted using existing neutral error kinds. No paths,
payload text, database handles, auth policy, network, tool execution, AI, codegen,
refactoring, dynamic dependencies, or build execution. Cursor integrity is query
consistency, not authentication or authorization.

## Implementation roadmap after approval only

1. Independently freeze analysis-input digest and cursor vectors; review candidate
   additive model/API changes before production publication.
2. Add validated config/query models and bounded index extraction, with conformance.
3. Implement iterative SCCs and structural cycle summaries plus metadata.
4. Implement direct pages, then bounded forward/reverse BFS.
5. Validate independent oracles, adversarial bounds, immutability, determinism,
   cancellation, Windows/Ubuntu regression/race/fuzz, performance, and documentation.
6. Commit evidence and request Phase 5.0.4 acceptance. No automatic integration,
   release, or direct-core hardening closure.
