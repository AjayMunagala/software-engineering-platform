# Development Roadmap

## Phase 0 — Foundation ✅

- Establish project structure and documentation.
- Confirm working name and Version 1 scope.
- Record architectural and technology decisions.

## Phase 1 — Repository Intelligence Engine 1.0.0 ✅

- Immutable artifact pipeline from Discovery through Repository Intelligence Summary.
- Versioned JSON schema and artifact contracts.
- Deterministic repository, language, framework, build, workspace, metadata, and summary intelligence.
- Regression, coverage, performance, memory, API, dependency, and documentation audits.

## Phase 1.1 — Real-world validation and hardening ✅

- Validated the frozen release on 15 pinned public repositories.
- Verified file totals, supported-language counts, Git identity, detection evidence, determinism, and performance.
- Recorded non-blocking coverage gaps without expanding the RIE 1.0 contract.
- Confirmed no `v1.0.1` bug-fix release was required.

## Phase 2.0 — Language Intelligence architecture ✅

- Define LIE boundaries, immutable artifacts, public API, diagnostics, security limits, and performance targets.
- Use the Go standard library parser for the first language engine.
- Keep dependency relationships outside LIE.

## Phase 2.1 — Go Language Engine 1.0.0 ✅

- Discover Go packages, imports, structs, interfaces, functions, methods, constants, and package variables.
- Publish an evidence-backed `GoLanguageInventory`.
- Do not implement type checking, call graphs, AI, or code modification.

## Phase 2.2 — Go Package Identity and Semantic Inventory 1.0.0 ✅

- Frozen package identity and semantic contracts.
- Deterministic declarations, scopes, references, imports, receiver/type
  binding, and bounded conditional interface satisfaction.
- Real-repository validation, stabilization, regression, coverage, benchmarks,
  race testing, release documentation, and annotated tags.

## Additional language engines — Deferred

- Add TypeScript, SQL, Python, Java, and other languages one at a time after Go stabilizes.

## Phase 3 — Durable Artifact Persistence and Runtime ✅

- Phase 3.1: persistence architecture and ADR accepted.
- Phase 3.2: accepted and frozen with four-MiB chunks and a four-GiB
  operational artifact limit.
- Phase 3.3: migration framework — accepted and frozen.
- Phase 3.4.1: storage-neutral persistence port architecture and candidate API
  — accepted.
- Phase 3.4.2: neutral Go port and reusable conformance harness — accepted.
- Phase 3.4.3: PostgreSQL adapter — accepted on 2026-07-26.
- Phase 3.4.4: Persistence Port and PostgreSQL Adapter `1.0.0` — accepted and
  frozen on 2026-07-26.
- Phase 3.5.0: runtime infrastructure design and ADR 0015 accepted on
  2026-07-26.
- Phase 3.5.1: runtime configuration accepted on 2026-07-26.
- Phase 3.5.2: PostgreSQL runtime accepted on 2026-07-26.
- Phase 3.5.3: Runtime Lifecycle & Health accepted on 2026-07-26.
- Phase 3.5.4: Runtime Integration & Release Freeze accepted and frozen at
  `1.0.0` on 2026-07-27.
- Phase 4.0.0: Repository Service Layer design package and ADR 0016 accepted on
  2026-07-27.
- Phase 4.0.1: bounded design spike authorized; production implementation
  remains gated.
- Phase 4.1: REST/gRPC query APIs remain gated.

PostgreSQL remains downstream from immutable artifacts and is not an engine
dependency.

## Phase 4 — Repository Services and Query Access

### Phase 4.0 — Repository Service Layer (Design accepted; spike authorized)

- Define storage-neutral repository service interfaces.
- Coordinate repository and scan lifecycle through application services.
- Wire immutable intelligence artifacts, runtime capabilities, and persistence
  through dependency injection.
- Validate service behavior, failure isolation, idempotency, and concurrency.
- Do not introduce REST/gRPC endpoints, UI, authentication, AI orchestration,
  IDE integration, or new intelligence-engine behavior.
- Architecture, candidate API, ADR 0016, staged implementation roadmap, and
  validation plan were accepted together on 2026-07-27.
- Only the bounded Phase 4.0.1 design spike is authorized; production
  implementation remains separately gated.

Detailed milestones are defined in
`docs/Roadmap/PHASE_4_REPOSITORY_SERVICE_ROADMAP.md`.

### Phase 4.1 — Query APIs (Gated)

- Design REST/gRPC only after the service-layer contract is accepted.

### Later Phase 4 Work

- React UI only after API stabilization.
- Add separately approved Git, logs, database-schema, configuration,
  dependency, architecture, documentation, and testing intelligence.

## Phase 5 — Evidence-Based Assistance

- Add planning, root-cause reasoning, minimal patch proposals, and validation workflows.

## Phase 6 — Durable Knowledge and Governed Automation

- Add memory, multi-repository analysis, and carefully governed automation.
