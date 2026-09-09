# Phase 5 - Dependency Intelligence Engine Roadmap

## Status

- Repository Service `1.0.0`: released and frozen
- Phase 5.0 design: accepted on 2026-09-07
- ADR 0020: Accepted on 2026-09-08
- Phase 5.0.1 design spike: engineering accepted on 2026-09-08
- Phase 5.0.2 neutral graph artifact and core: engineering accepted on 2026-09-08
- Phase 5.0.3 design: approved on 2026-09-08
- Phase 5.0.3 implementation: engineering accepted at `a0a1a779734b23a0f6c2a8f711130063822a9761`
- ADR 0021: Accepted; candidate version remains 0.1.0
- Phase 5.0.4 design preparation: authorized 2026-09-09; review pending
- Phase 5.0.4 implementation and later milestones: not authorized
- Production release: not authorized
- Date: 2026-09-08

## Goal

Publish deterministic, immutable, evidence-backed module, package, and file
dependency intelligence from released artifacts, then provide bounded impact
queries without executing or modifying repositories.

## Frozen prerequisites

- Repository Intelligence Engine `1.0.0`
- Go Language Inventory `1.0.0`
- Go Package Identity Inventory `1.0.0`
- Go Semantic Inventory `1.0.0`
- Persistence Port `1.0.0`
- PostgreSQL Adapter `1.0.0`
- Runtime Infrastructure `1.0.0`
- Repository Service `1.0.0`

Phase 5 may consume released artifact contracts but may not change them.

## Phase 5.0 - Architecture and candidate contract (review candidate)

Deliverables:

- Dependency Intelligence architecture;
- immutable artifact specification;
- candidate Go API `0.1.0`;
- ADR 0020;
- validation architecture;
- validation plan;
- this staged roadmap;
- artifact dependency graph and platform roadmap updates.

Exit gate:

- all design documents reviewed together;
- graph semantics, evidence, unknown-state, identity, SCC, cycle, impact,
  complexity, cancellation, and security policies approved;
- authorization granted only for Phase 5.0.1.

## Phase 5.0.1 - Design spike

Validate risky assumptions with disposable spike code only:

- canonical node/edge/SCC ID byte representations and golden vectors;
- normalization from released Go artifacts;
- duplicate edge/evidence aggregation;
- deterministic SCC algorithm and cycle classification;
- bounded forward/reverse impact traversal;
- one/eight-worker and Windows/Ubuntu determinism;
- memory behavior at 100,000 nodes and 1,000,000 edges;
- cancellation latency and omission policy.

Exit gate: spike report accepted, ADR 0020 accepted, golden vectors frozen, and
Phase 5.0.2 explicitly authorized. Spike code must not become a production
dependency accidentally.

Local evidence is recorded in
`docs/Validation/DEPENDENCY_INTELLIGENCE_DESIGN_SPIKE_REPORT.md`. The isolated
harness and evidence were engineering accepted on 2026-09-08. The accepted
node, edge, SCC, and cycle vectors are frozen, and the measured two-GiB
peak-live-heap gate applies to the exact synthetic scale fixture.

## Phase 5.0.2 - Neutral graph artifact and core

Implement only:

- immutable `DependencyInventory` candidate;
- node, containment, edge, evidence, diagnostic, and statistics models;
- stable constructors and errors;
- core normalization/aggregation;
- reusable conformance harness;
- unit, property, fuzz, race, and benchmark tests.

`MaxNodes` is a mandatory production invariant and must be enforced and tested.
Containment may not be published until the canonical
`dependency-containment-id/v1` identity and golden vectors are frozen during
this milestone. The 100,000-node/1,000,000-edge fixture has a candidate
two-GiB peak-live-Go-heap gate; total allocations must also be reported.

No Go adapter, persistence, runtime, service, transport, or UI integration.

Implementation commit `e8c78a2b762131bd679c007ea2d26b73c67bd475` and its
validation evidence were engineering accepted on 2026-09-08. This acceptance
does not authorize Phase 5.0.3.

## Phase 5.0.3 - Go dependency adapter

Approved design package:

- `docs/Architecture/GO_DEPENDENCY_ADAPTER.md`;
- `docs/API/GO_DEPENDENCY_ADAPTER_CANDIDATE_API.md`;
- `docs/Decisions/0021-go-dependency-adapter.md` (Accepted);
- `docs/Validation/GO_DEPENDENCY_ADAPTER_VALIDATION_PLAN.md`.

Design commit `4b77de2034af7619704d8df5f5f65ef31b5dd552` was approved on
2026-09-08. Only the implementation scope below is authorized. Boundary-name
vectors are frozen in `docs/API/GO_DEPENDENCY_BOUNDARY_GOLDEN_VECTORS.md`
before production encoding in `7eb2a98`. Engineering has now accepted the
implementation evidence and explicitly authorized ADR 0021 promotion.

Implement:

- exact frozen artifact input validation;
- Go module graph;
- Go package import graph;
- proof-backed file graph;
- standard-library/external/unresolved/ambiguous/stale boundaries;
- deterministic evidence translation.

The adapter does not parse files, manifests, or ASTs and does not run Go tools.

The accepted implementation and measured evidence are recorded in
`docs/Validation/GO_DEPENDENCY_ADAPTER_VALIDATION_REPORT.md`. Candidate version
remains 0.1.0; ADR 0021 is Accepted. The reviewed implementation commit is
`a0a1a779734b23a0f6c2a8f711130063822a9761`. Both direct-core hardening items
and all documented limitations remain open/recorded. No later milestone or
release is authorized by this acceptance.

## Phase 5.0.4 - SCC, cycles, and impact

Design preparation only is authorized. Review package:

- `docs/Architecture/DEPENDENCY_GRAPH_ANALYSIS.md`;
- `docs/Architecture/DEPENDENCY_GRAPH_ANALYSIS_ARTIFACTS.md`;
- `docs/API/DEPENDENCY_GRAPH_ANALYSIS_CANDIDATE_API.md`;
- `docs/Decisions/0022-dependency-graph-analysis.md` (Proposed);
- `docs/Validation/DEPENDENCY_GRAPH_ANALYSIS_VALIDATION_PLAN.md`.

Proposed implementation scope, only after design review and explicit authorization:

- deterministic SCC analysis;
- bounded cycle classification;
- direct dependency/dependent queries;
- bounded forward/reverse reachability and impact results;
- deterministic pagination/truncation;
- cancellation and safety limits.

Architecture policy, smells, scores, and AI reasoning remain excluded.

## Phase 5.0.5 - Platform integration

Design and implement integration only after a separate accepted design:

- versioned canonical artifact codec;
- persistence and runtime wiring through frozen ports;
- optional Repository Service capability/profile extension without modifying
  frozen 1.0 interfaces;
- exact-byte stage, publication, export, and recovery validation.

If integration would require a breaking Repository Service change, defer it to
a separately governed major version rather than changing 1.0.x.

## Phase 5.0.6 - Real-repository validation

Run the accepted pinned corpus and synthetic fixtures across Windows, Ubuntu,
one/eight workers, repeats, race-capable environments, and configured safety
limits. Fix only compatible defects and record known limitations.

## Phase 5.0.7 - Stabilization and 1.0.0 freeze

- bound intermediate evidence accumulation so pathological adapters cannot
  retain an effectively unlimited pre-cap collection;
- reject a nil analysis context through the stable error model instead of
  allowing a panic;
- public API and immutable artifact review;
- identity/canonical JSON freeze;
- correctness, dependency, security, memory, and performance audit;
- final regression, vet, shuffle, race, fuzz, and coverage evidence;
- changelog, release notes, feature matrix, benchmark summary, known
  limitations, and validation report;
- explicit engineering acceptance;
- version promotion and annotated namespaced tags.

## Deferred work

- additional language adapters;
- call, control-flow, and data-flow graphs;
- Dependency Intelligence persistence projections;
- Architecture Intelligence and policy analysis;
- vulnerability/external ecosystem intelligence;
- AI reasoning, patches, validation execution, APIs, and UI.

## Governance

Every milestone is separately gated: design, review, implementation,
validation, engineering acceptance, then commit/release. Phase 5.0.2 is
closed and accepted. Phase 5.0.3 is also closed and engineering accepted. Only
Phase 5.0.4 design preparation is authorized; its design is not yet approved.
Implementation, later milestones, and production release remain unauthorized.
