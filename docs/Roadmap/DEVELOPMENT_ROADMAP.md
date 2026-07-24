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

## Phase 3 — Durable Artifact Persistence (current implementation phase)

- Phase 3.1: persistence architecture and ADR accepted.
- Phase 3.2: accepted and frozen with four-MiB chunks and a four-GiB
  operational artifact limit.
- Phase 3.3: migration framework — authorized and current.
- Phase 3.4: storage-neutral Go port and PostgreSQL adapter.
- Phase 3.5: disposable development environment and secret handling.
- Phase 3.6: REST/gRPC query APIs.

PostgreSQL remains downstream from immutable artifacts and is not an engine
dependency.

## Phase 4 — Query UI and Engineering Context

- React UI only after API stabilization.
- Add separately approved Git, logs, database-schema, configuration,
  dependency, architecture, documentation, and testing intelligence.

## Phase 5 — Evidence-Based Assistance

- Add planning, root-cause reasoning, minimal patch proposals, and validation workflows.

## Phase 6 — Durable Knowledge and Governed Automation

- Add memory, multi-repository analysis, and carefully governed automation.
