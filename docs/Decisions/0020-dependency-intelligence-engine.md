# ADR 0020 - Dependency Intelligence Engine

## Status

Accepted

## Date

2026-07-31

Phase 5.0 design was accepted on 2026-09-07. The Phase 5.0.1 design-spike
evidence was accepted on 2026-09-08. This acceptance authorizes only Phase
5.0.2; Phase 5.0.3 and production release remain separately gated.

## Context

Repository Intelligence 1.0.0 identifies repository facts. Go Language
Intelligence 1.0.0 publishes syntax, package identity, and semantic facts.
Repository Service 1.0.0 coordinates their production and storage. The
platform does not yet publish a stable structural answer to "what depends on
what?"

Placing dependency inference inside RIE would make repository scanning depend
on higher semantic artifacts. Placing it inside Go LIE would freeze graph and
impact policy into a language-specific contract. Re-parsing source or invoking
build tools would duplicate accepted ownership and weaken determinism.

## Decision

1. Create Dependency Intelligence as a separate downstream engine family.
2. Make Phase 5.0 design-only and keep production implementation gated.
3. Publish one language-neutral immutable `DependencyInventory`, initially
   populated by a Go adapter consuming released artifacts.
4. Consume only the lowest frozen artifacts containing required facts;
   never reread source, manifests, mutable run context, or presentation data.
5. Model module, package, and file graphs separately and model containment
   separately from dependency.
6. Preserve local, standard-library, external, unresolved, ambiguous, and
   stale states; never guess targets.
7. Aggregate direct edges and compute SCCs in `O(V + E)` graph work.
8. Do not serialize all-pairs transitive closure or enumerate every simple
   cycle. Provide bounded deterministic reachability/impact queries instead.
9. Treat graph direction as evidence, not architecture policy. Defer layering,
   smells, risk scores, and policy violations to Architecture Intelligence.
10. Freeze stable-ID canonical bytes through design-spike golden vectors before
    production implementation.
11. Keep RIE, Go LIE, Persistence, Runtime, and Repository Service 1.0
    contracts unchanged.
12. Treat `MaxNodes` as a mandatory production safety invariant; production
    normalization must never inherit the spike's unbounded-node behavior.
13. Use a two-GiB peak-live-Go-heap candidate gate for the exact 100,000-node,
    1,000,000-edge synthetic fixture and continue reporting total allocations.
14. Do not publish containment records until `dependency-containment-id/v1`
    canonical fields and cross-platform golden vectors are frozen.

## Rationale

A distinct engine preserves ownership boundaries and allows future language
adapters to normalize facts into a shared graph artifact. Direct adjacency and
SCCs provide reusable structural truth without the unbounded cost and storage
of all-pairs closure. Explicit unresolved states retain honesty when released
artifacts cannot prove a relationship.

## Alternatives

### Add dependency fields to GoSemanticInventory 1.0.0

Rejected. The artifact is released and frozen, and dependency graphs are a
separate cross-language concern.

### Compute dependencies inside Repository Service

Rejected. Repository Service coordinates released artifacts; it must not own
new intelligence semantics.

### Parse repositories again

Rejected. It duplicates LIE work, creates conflicting source authority, and
weakens exact artifact provenance.

### Store only a relational graph in PostgreSQL

Rejected. Intelligence artifacts must remain reproducible without a database.
Persistence is downstream and stores the exact immutable artifact.

### Precompute transitive closure

Rejected for v1. It can require quadratic storage and makes configured limits
harder to explain. Bounded traversal over direct adjacency is sufficient.

## Consequences

- The first adapter supports Go, while the artifact remains technology-neutral.
- Other languages require released identity/semantic facts and a separate
  adapter milestone.
- Architecture Intelligence can consume dependency evidence without forcing
  dependency construction to understand architecture policy.
- Repository Service integration, if later required, must use a new compatible
  optional capability/profile rather than modifying its frozen 1.0 interfaces.
- Candidate API and ID schemes remain `0.1.0` until validation and release
  stabilization.

## Acceptance gate

The Phase 5.0 design package and Phase 5.0.1 evidence have been reviewed and
accepted. The validated node, edge, SCC, and cycle vectors are frozen. Phase
5.0.2 is authorized subject to the node-limit and containment-vector rules
above. Phase 5.0.3 and later milestones require separate evidence review and
explicit authorization.
