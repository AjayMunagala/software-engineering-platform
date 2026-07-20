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

## Phase 2.0 — Language Intelligence architecture (design review pending)

- Define LIE boundaries, immutable artifacts, public API, diagnostics, security limits, and performance targets.
- Use the Go standard library parser for the first language engine.
- Keep dependency relationships outside LIE.

## Phase 2.1 — Go Language Engine (not started)

- Discover Go packages, imports, structs, interfaces, functions, methods, constants, and package variables.
- Publish an evidence-backed `GoLanguageInventory`.
- Do not implement type checking, call graphs, AI, or code modification.

## Phase 2.2+ — Additional language engines (not started)

- Add TypeScript, SQL, Python, Java, and other languages one at a time after Go stabilizes.

## Phase 3 — Engineering Context

- Add Git, logs, database schema, and configuration intelligence.

## Phase 4 — Evidence-Based Assistance

- Add planning, root-cause reasoning, minimal patch proposals, and validation workflows.

## Phase 5 — Durable Knowledge

- Add memory, multi-repository analysis, and carefully governed automation.
