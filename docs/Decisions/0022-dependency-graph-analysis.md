# ADR 0022 — Bounded Neutral SCC and Impact Analysis

## Status

Design Approved. Engineering reviewed and approved design commit
`a2680da36cde620cf436b6ba1473b23f7c18e607`. Implementation is eligible for
separate authorization. Engineering subsequently explicitly authorized Phase
5.0.4 implementation, beginning with independent digest/cursor vectors committed
before production encoding. This ADR remains Design Approved pending implementation
evidence acceptance; no later milestone or release is authorized.
Design approval and implementation authorization are separate gates. Phase 5.0.3
and ADR 0021 remain accepted at governance commit `7a04db8`. Version remains
candidate 0.1.0. Only the approved Phase 5.0.4 implementation scope is authorized.

## Context

The accepted core normalizes immutable graph facts and the accepted Go adapter
produces structural dependencies. Their output may be capped or uncertain and
includes file-to-package import boundaries. Treating every edge as local adjacency
or claiming full repository acyclicity would misrepresent those facts.

## Approved design decision

1. Add opt-in Analyzer/QueryEngine capabilities in neutral `die`, not the Go
   adapter. Keep Normalize and all released 1.0.0 contracts unchanged.
2. Use strict same-kind local projections and iterative SCC processing. Keep
   containment and cross-kind/nonlocal boundaries out of SCC adjacency.
3. Return a new inventory with existing base fields unchanged and an optional
   analysis envelope; preserve existing core/adapter JSON bytes when absent.
4. Publish one structural cycle summary per cyclic SCC. Do not enumerate simple
   cycles or infer language-invalid/build-context conclusions.
5. Offer paginated direct neighbors and bounded forward/reverse BFS, with explicit
   incomplete topology, terminal boundaries, and partial traversal reasons.
6. Bind stateless direct-page cursors to a canonical base-input fingerprint and
   request; no impact continuation or all-pairs closure. New vectors precede code.
7. Apply independent record/evidence/traversal caps and nil/cancellation guards.
   Do not claim these close direct-core DIE-HARDEN-001 or DIE-HARDEN-002.

## Alternatives and consequences

Running algorithms in Normalize would alter accepted core/adapter behavior and
costs. Returning a separate giant copied graph would duplicate the artifact model;
instead reuse reserved SCC/cycle fields with explicit computation metadata.
Recursive DFS risks stack exhaustion. All-simple-cycle enumeration and all-pairs
closure have unsuitable worst-case output sizes. Hidden persistent query caches
introduce ownership/lifetime concerns; per-call indexes are simpler but costlier.

The proposal adds candidate-only metadata, not a frozen schema. Sorting/digesting
and released clones must be measured separately from linear graph work. Partial
graphs retain useful positive evidence without making negative completeness claims.
Language-invalid classification awaits an authoritative rule/context contract.

## Acceptance gates

Design review is complete for the architecture, API, artifact supplement, and
validation plan at the commit above. Subsequent explicit implementation authorization
was granted. Independent analysis-input digest/cursor values were committed at
`ab4628c` before production encoding. Prose erratum `c0fdd00` corrects only the
written domain-byte count, with no vector changes. Existing frozen SCC/cycle
vectors are reused unchanged.
Later implementation evidence still requires engineering acceptance. This ADR's
Design Approved status is not implementation acceptance or release approval.
DIE-HARDEN-001/002 remain open. No automatic 5.0.5/integration/release.
