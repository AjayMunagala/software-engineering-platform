# Language Intelligence Engine Implementation Roadmap

## Phase 2.0 — Architecture and Design

Status: completed and approved.

Deliverables:

- LIE architecture and boundaries.
- Go Language Engine specification.
- Candidate immutable artifact model.
- Candidate public API.
- Go parser ADR.
- Updated artifact dependency graph.
- Explicit implementation and freeze gates.

Approval gate:

- Inputs are limited to `RepositorySnapshot` and `LanguageInventory`.
- No `RunContext` or presentation-report dependency.
- Artifact facts, IDs, locations, hashing, diagnostics, and versioning are accepted.
- Security, performance, and out-of-scope decisions are accepted.
- ADR 0007 is accepted.

## Phase 2.1a — LIE Core and Go Artifact Skeleton

- Create `backend/lie` and `backend/lie/golang` using the mandated package structure.
- Implement runner registry and prerequisite resolution.
- Implement shared source ranges and diagnostics.
- Implement immutable `GoLanguageInventory` candidate models and accessors.
- Add contract, immutability, ordering, and empty-inventory tests.
- No parsing beyond the minimum needed to compile test fixtures.

Gate: public surface matches the approved candidate API and all artifact mutation tests pass.

## Phase 2.1b — Go Syntax Extraction

Implement in this order:

1. Source selection, containment, bounded read, SHA-256 digest.
2. File parsing and package grouping.
3. Import extraction.
4. Struct and interface discovery.
5. Function and method discovery.
6. Package constant and variable discovery.
7. Diagnostics, cancellation, and worker concurrency.
8. Deterministic inventory construction.

Each step requires focused unit tests before the next begins.

## Phase 2.1c — Go Engine Stabilization

- Full regression and static analysis.
- CPU, memory, and allocation profiles.
- Worker-count determinism comparison.
- 1,000-file and 10,000-file benchmarks.
- Real-world validation on pinned Go repositories.
- Artifact and public API review.
- Documentation and ADR review.
- Technical debt review.

Gate: freeze `GoLanguageInventory 1.0.0` only if correctness, performance, immutability, evidence, and API checks pass.

Status: completed. All gates passed and the artifact contract was frozen on
2026-07-20.

## Phase 2.2 — TypeScript Language Engine

Begin only after Go artifact 1.0.0 is frozen. Design parser strategy and TypeScript-specific artifact details separately. TypeScript, TSX, JavaScript, and JSX syntax may share an engine, while React and Node remain framework/runtime classifications rather than languages.

## Phase 2.3 — SQL Language Engine

Design dialect-aware extraction for tables, views, functions, procedures, and triggers. Do not pretend SQL dialects share identical grammar or semantics.

## Phase 2.4+ — Python, Java, and Other Languages

Each language requires its own parser ADR, artifact extension review, fixtures, benchmarks, and stabilization gate. Do not build multiple language engines in parallel before the shared LIE contract is proven by Go.

## Prohibited Scope During Phase 2

- Call graphs and resolved dependency graphs.
- Architecture inference.
- Bug or smell detection.
- LLM integration.
- Patch generation or code editing.
- Build or test execution.

Those capabilities belong to later approved platform layers.
