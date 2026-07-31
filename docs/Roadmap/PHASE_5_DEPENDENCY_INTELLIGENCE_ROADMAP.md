# Phase 5 - Dependency Intelligence Engine Roadmap

## Status

- Repository Service `1.0.0`: released and frozen
- Phase 5.0 design: complete review candidate
- ADR 0020: Proposed
- Phase 5.0.1 and production implementation: not authorized
- Date: 2026-07-31

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

## Phase 5.0.2 - Neutral graph artifact and core

Implement only:

- immutable `DependencyInventory` candidate;
- node, containment, edge, evidence, diagnostic, and statistics models;
- stable constructors and errors;
- core normalization/aggregation;
- reusable conformance harness;
- unit, property, fuzz, race, and benchmark tests.

No Go adapter, persistence, runtime, service, transport, or UI integration.

## Phase 5.0.3 - Go dependency adapter

Implement:

- exact frozen artifact input validation;
- Go module graph;
- Go package import graph;
- proof-backed file graph;
- standard-library/external/unresolved/ambiguous/stale boundaries;
- deterministic evidence translation.

The adapter does not parse files, manifests, or ASTs and does not run Go tools.

## Phase 5.0.4 - SCC, cycles, and impact

Implement:

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
validation, engineering acceptance, then commit/release. Phase 5.0 design does
not authorize implementation.
