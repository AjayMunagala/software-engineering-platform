# ADR 0022 — Bounded Neutral SCC and Impact Analysis

## Status

Proposed, 2026-09-09. Only Phase 5.0.4 design preparation is authorized.
Design approval and implementation authorization are separate gates. Phase 5.0.3
and ADR 0021 remain accepted at governance commit `7a04db8`. Version remains
candidate 0.1.0. No experiment, production implementation, or release authorized.

## Context

The accepted core normalizes immutable graph facts and the accepted Go adapter
produces structural dependencies. Their output may be capped or uncertain and
includes file-to-package import boundaries. Treating every edge as local adjacency
or claiming full repository acyclicity would misrepresent those facts.

## Proposed decision

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

Review architecture, API, artifact supplement, and validation plan together.
Only explicit approval can make implementation eligible for separate authorization.
Keep this ADR Proposed during design review. Later implementation evidence needs
engineering acceptance before promotion. No automatic 5.0.5/integration/release.
