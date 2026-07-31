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
- Phase 4.0.1: bounded design spike accepted on 2026-07-27.
- Phase 4.0.2: neutral service contract and conformance harness accepted on
  2026-07-27.
- Phase 4.0.3: repository lifecycle accepted on 2026-07-27.
- Phase 4.0.4: scan execution core accepted on 2026-07-28.
- Phase 4.0.5: intelligence and materialization adapters accepted on
  2026-07-28.
- Phase 4.0.6: persistence and runtime integration accepted and frozen on
  2026-07-29 after its design, golden-vector, implementation, and validation
  gates passed; implementation commit `abdb395` is on `main`.
- Phase 4.0.7: real-repository validation design and ADR 0018 accepted with
  recommendations on 2026-07-29; accepted on 2026-07-30 with completion of the
  Kubernetes one-worker Windows and Ubuntu matrix retained as an open release
  qualification. Phase 4.0.8 design is accepted; stabilization evidence is
  accepted. Repository Service `1.0.0` is frozen under the annotated
  `repository-service/v1.0.0` tag with the larger-host qualification retained.
- Phase 4.1: REST/gRPC query APIs remain gated.

PostgreSQL remains downstream from immutable artifacts and is not an engine
dependency.

## Phase 4 — Repository Services and Query Access

### Phase 4.0 — Repository Service Layer (Repository Service 1.0.0 released)

- Define storage-neutral repository service interfaces.
- Coordinate repository and scan lifecycle through application services.
- Wire immutable intelligence artifacts, runtime capabilities, and persistence
  through dependency injection.
- Validate service behavior, failure isolation, idempotency, and concurrency.
- Do not introduce REST/gRPC endpoints, UI, authentication, AI orchestration,
  IDE integration, or new intelligence-engine behavior.
- Architecture, candidate API, ADR 0016, staged implementation roadmap, and
  validation plan were accepted together on 2026-07-27.
- The bounded Phase 4.0.1 design spike and Phase 4.0.2 neutral contract are
  accepted. Phase 4.0.3 Repository Lifecycle and Phase 4.0.4 Scan Execution
  Core and Phase 4.0.5 Intelligence & Materialization Adapters are accepted.
  Phase 4.0.6 is accepted and frozen after its production implementation and
  validation passed under the frozen golden-vector contracts. Implementation
  commit `abdb395` is pushed to `main`. Phase 4.0.7 design is accepted with
  recommendations and accepted on 2026-07-30 with a larger-host Kubernetes
  release qualification. Phase 4.0.8 design and ADR 0019 are accepted.
  Stabilization validation, the release package, and the larger-host
  qualification were accepted on 2026-07-30. Repository Service is frozen at
  `1.0.0` under `repository-service/v1.0.0`. Later milestones remain separately
  gated.

Detailed milestones are defined in
`docs/Roadmap/PHASE_4_REPOSITORY_SERVICE_ROADMAP.md`.

### Phase 4.1 — Query APIs (Design eligible; not started)

- Design REST/gRPC only after the service-layer contract is accepted.

### Later Phase 4 Work

- React UI only after API stabilization.
- Add separately approved Git, logs, database-schema, configuration,
  dependency, architecture, documentation, and testing intelligence.

## Phase 5 - Dependency Intelligence Engine (Design review candidate)

- Phase 5.0 is design-only.
- Define a language-neutral immutable dependency artifact and Go-backed first
  adapter over released Repository/Go Language artifacts.
- Model module, package, and file dependency graphs, containment, explicit
  resolution states, SCCs, cycles, and bounded impact traversal.
- Preserve exact provenance and cross-platform deterministic bytes.
- Do not reread source, execute repositories, use the network, infer
  architecture policy, introduce AI, or modify released 1.0 contracts.
- ADR 0020 remains Proposed; the design package does not authorize the design
  spike or production implementation.

Detailed milestones are defined in
`docs/Roadmap/PHASE_5_DEPENDENCY_INTELLIGENCE_ROADMAP.md`.

## Phase 6 - Architecture Intelligence Engine

- Infer separately approved layers, boundaries, modules, services, and
  architecture smells from released evidence.
- Keep observed dependency truth separate from architecture policy.

## Phase 7 - Evidence-Based Assistance

- Add planning, root-cause reasoning, minimal patch proposals, and validation workflows.

## Phase 8 - Durable Knowledge and Governed Automation

- Add memory, multi-repository analysis, and carefully governed automation.
