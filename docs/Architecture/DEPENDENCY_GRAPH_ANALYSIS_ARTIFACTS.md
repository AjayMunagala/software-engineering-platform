# Phase 5.0.4 — Candidate Artifact and Query Supplement

Design only, 2026-09-09. Not implemented or approved. This supplement specializes
the Phase 5.0 illustrative models against the actual accepted `die` structs.

## Input and output

Input is `DependencyInventory 0.1.0`, with existing node/edge/containment v1 schemes.
No raw GraphInput or language inventory is accepted by algorithms. Existing base
nodes, dependencies, containment, source references, diagnostics, statistics, and
artifact metadata are preserved byte-for-byte as values in the returned inventory.
Only StrongComponents, Cycles, and a new optional `analysis` envelope differ.

Proposed `DependencyInventoryView.Analysis *AnalysisMetadata` uses
`json:"analysis,omitempty"`. Private immutable storage/accessors defensively copy
all nested slices. Existing core/adapter output keeps nil and thus unchanged bytes.
No algorithm counters are inserted into existing base statistics. No synthetic
self-referencing source-artifact edge is added.

AnalysisMetadata fields:

| Field | Meaning |
|---|---|
| EngineName / EngineVersion | `dependency-graph-analysis` / `0.1.0` |
| InputDigest / DigestScheme | Exact base-input fingerprint / `dependency-analysis-input/v1` |
| SCCIDScheme / CycleIDScheme | Existing frozen v1 schemes |
| ProjectionPolicy | `dependency-local-projection/v1` |
| Graphs | Three ordered per-graph GraphAnalysis records (file, module, package) |

GraphAnalysis contains Graph, EligibleNodes, EligibleEdges (original eligible edge
records), Components, CyclicComponents, BoundaryEdges, TopologyLimited,
ExplanationLimited, and sorted unique ReasonCodes. Counts describe supplied data,
not omitted upstream totals. All three graph records exist even for empty input.
Envelope presence distinguishes 'computed empty' from 'not analyzed'.

Base TopologyLimited reasons: `input_edges_omitted`, `input_nodes_omitted`,
`input_diagnostics_omitted`, `input_partial_diagnostics`, `boundary_edges`.
If upstream omission counters cannot be allocated by graph, mark all graphs.
Any nonempty base diagnostics conservatively mark partial diagnostics; do not
parse free-form messages. Containment omissions and evidence omissions mark
ExplanationLimited; containment is not adjacency. Unknown diagnostics are never
treated as proof of completeness. Even TopologyLimited=false means only that
the supplied graph reports no such loss, not complete repository knowledge.

## Identity and canonicalization

Reuse `DEPENDENCY_INTELLIGENCE_GOLDEN_VECTORS.md` unchanged. SCC identity is graph
plus sorted member IDs; cycle identity is graph plus SCC ID, not classification.
Canonical cycle member order is the SCC member order. No all-path enumeration.

The new input fingerprint is SHA-256 over: uint64 big-endian length plus UTF-8
domain `dependency-analysis-input/v1`, then base inventory JSON bytes through EOF.
Base JSON means the accepted inventory View field order, canonical existing
collections/maps, StrongComponents=[] and Cycles=[], and Analysis omitted. Encode
using the accepted Go JSON escaping rules, no indentation, with exactly one final
LF (streaming encoder contract). Include all existing base evidence and statistics;
therefore explanation changes also invalidate cursors. Do not retain an
artifact-sized serialized byte buffer; stream encoding into the hash sink.
Encoder traversal itself must be measured as a potentially uninterruptible unit.

This is a proposed consistency fingerprint, not a claim of source freshness,
with textual form `dependency-analysis-input/v1:sha256:<64 lowercase hex>`.
It is not proof of
common-scan authority, a storage codec, or an authenticated identity. Independently
calculated empty/Unicode/escaping/omission vectors must be committed before the
production encoder. Different canonical rules require a new scheme and review.

## Direct NodePage

Immutable page: InputDigest, Graph, Direction, RootNodeID, Neighbors, HasMore,
NextCursor, TopologyLimited, ExplanationLimited, ReasonCodes. Each neighbor has
NodeID, Resolution, sorted EdgeIDs. No mutable node map or copied inventory.
Total cardinality is known from the fully bounded index but need not be published.
Page limits do not mean unknown topology: HasMore and topology flags are distinct.

Proposed cursor: base64url without padding of canonical compact JSON array
`["dependency-node-page/v1", inputDigest, graph, direction, rootID, pageSize, lastID]`.
Strings use the same JSON escaping rules, no trailing newline; pageSize is an
unsigned decimal integer. Decode at most 2,048 bytes of encoded text, reject
unknown fields/versions, noncanonical encoding, mismatched request/digest, and
lastID not in the canonical adjacency. No offset, timestamp, process state, or
raw path. A cursor is forgeable and confers no authority; it is not a security token.
Freeze independently computed vectors before implementation emits cursors.

## ImpactResult

Immutable result: InputDigest, Graph, Direction, Seeds, Reached (NodeID, Depth),
BoundaryNodeIDs, EdgeIDs, VisitedNodes, ScannedEdges, MaxDepthReached, Truncated,
TopologyLimited, ExplanationLimited, ReasonCodes. Reached ordered by depth then
bytewise ID; other ID sets bytewise sorted. Return no duplicate seed/reached nodes.
Reasons extend base flags with `depth_limit`, `node_limit`, `edge_limit`.
Unknown remaining counts are omitted, never reported as zero. No cursor, risk
score, probability, or promise of exhaustive repository impact.

## Compatibility boundary

All names/fields above are candidate proposals for review, not existing public
methods. `Core` and Go adapter interfaces remain unchanged. This supplement
governs Phase 5.0.4 where the older illustrative subsystem API differs. Acceptance
does not freeze 1.0.0, add persistence serialization, or close core hardening work.
