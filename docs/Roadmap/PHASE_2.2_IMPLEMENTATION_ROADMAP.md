# Phase 2.2 Implementation Roadmap — Go Semantic Resolution

## Status

- Current state: Phase 2.2.8 accepted; Phase 2.2.9 stabilization authorized
- Implementation authorization: Phase 2.2.9 stabilization and `1.0.0` freeze only
- Stable prerequisite: `GoLanguageInventory 1.0.0`
- Candidate output: `GoSemanticInventory 0.1.0`
- Stable target: `GoSemanticInventory 1.0.0`

Engineering accepted the Phase 2.2.0 evidence and ADR 0008 on 2026-07-22. Phase 2.2.1 was accepted on 2026-07-22. Phase 2.2.2 through Phase 2.2.8 were accepted on 2026-07-23. Phase 2.2.9 stabilization and the `1.0.0` freeze are authorized; no later milestone is authorized.

Phase 2.2.1 implementation and local validation are complete and accepted. Targeted package and full-backend race tests pass with MSYS2 UCRT64 GCC 16.1.0. See [LIE_PHASE_2_2_1_PACKAGE_IDENTITY.md](../Validation/LIE_PHASE_2_2_1_PACKAGE_IDENTITY.md). Authorization advances only to Phase 2.2.2.

## Goal

Produce deterministic, explainable semantic relationships from digest-verified Go source while preserving the frozen Phase 2.1 artifact and performing no repository command execution, downloads, network access, source modification, call-graph analysis, or AI inference.

## Delivery Rule

Each milestone must be correct, testable, benchmarked where applicable, documented, and runnable before the next milestone begins. No public `1.0.0` freeze occurs during feature construction.

## Phase 2.2.0 — Design Spike and Approval

### Work

- Prototype `go/types` with missing external imports, partial packages, generics, embedded interfaces, and pointer/value receivers.
- Prove that the importer performs no command execution or network access.
- Validate source-range and stable-ID rules against Phase 2.1 byte offsets.
- Measure an initial warm-cache CPU/memory baseline on repeatable fixtures.
- Confirm bounded cancellation and relationship limits.
- Validate authoritative proof precedence for single/nested/workspace/replaced/vendored modules and exact standard-library indexing.
- Measure cancellation after file, package, 1,024-node, and 256-relationship checkpoints.
- Prove that a full rebuild is deterministic and define the `go-semantic-id/v1` golden vectors.
- Record findings in the architecture document or a dedicated spike report.

### Exit Gate

- Package identity proof rules and conflict behavior are approved.
- Full-rebuild/invalidation and semantic-ID evolution policies are approved.
- Interface candidate bounds, cancellation checkpoints, and diagnostic stability are approved.
- ADR 0008 changes from `Proposed` to `Accepted`.
- Any model/API changes discovered by the spike are reviewed.
- The engineering manager explicitly authorizes implementation.

No production implementation starts before this gate.

Execution evidence: [LIE_PHASE_2_2_0_DESIGN_SPIKE.md](../Validation/LIE_PHASE_2_2_0_DESIGN_SPIKE.md). The report and ADR 0008 are accepted; authorization advances only to Phase 2.2.1.

## Phase 2.2.1 — Go Package Identity Prerequisite

### Work

- Create `backend/lie/golang/packageidentity` using the eight-file package standard.
- Produce immutable `GoPackageIdentityInventory 0.1.0` from the snapshot and syntax inventory.
- Parse and digest approved `go.mod`, `go.work`, local `replace`, nested-module, and `vendor/modules.txt` evidence.
- Implement exact proof precedence, ambiguity, staleness, and empty-repository behavior.
- Exclude the ambient module cache, network, external tools, and directory-suffix guesses.

### Exit Gate

- All proof fixtures listed in `GO_PACKAGE_IDENTITY_PROOF.md` pass.
- Proof and evidence IDs are deterministic across worker counts and shuffled inputs.
- Manifest paths remain inside the snapshot; all proof evidence has exact digest and range.
- No command, network, module-cache, or repository write occurs.
- Artifact accessors are deeply immutable and race tests pass.

Local evidence: package coverage is 87.8%, full regression and vet pass, the 10,000-proof benchmark completes in 15.56–19.24 ms, and both targeted and full-backend race tests pass. The Phase 2.2.1 exit gate is accepted.

## Phase 2.2.2 — Semantic Artifact Skeleton and Source Verification

### Work

- Create `backend/lie/golang/semantic` using the eight-file package standard.
- Implement configuration validation and engine metadata.
- Validate artifact names, versions, and provenance.
- Require and validate the package-identity prerequisite.
- Enforce root containment, snapshot authorization, file type, size, and SHA-256 checks.
- Implement immutable candidate artifact construction and deep-copy accessors.
- Emit semantic file outcomes for empty, verified, stale, missing, unreadable, oversized, and failed files.

### Exit Gate

- Security-boundary and digest tests cover every branch.
- Empty repositories produce a valid empty inventory without warnings/errors.
- Input artifacts remain byte-for-byte/logically unchanged.
- `go test -race` passes.

Accepted evidence: all behavior tests pass with 88.9% package statement coverage; shuffled tests, package and full-backend regression, vet, targeted race, and full-backend race pass. Repeatable source-verification benchmarks cover 100- and 1,000-file fixtures. See [LIE_PHASE_2_2_2_SEMANTIC_SKELETON.md](../Validation/LIE_PHASE_2_2_2_SEMANTIC_SKELETON.md). Engineering accepted the exit gate on 2026-07-23 and authorized Phase 2.2.3 only.

## Phase 2.2.3 — Declaration Reconciliation and Scopes

### Work

- Re-parse verified source into ephemeral AST/token state.
- Reconcile top-level declarations with Phase 2.1 symbol IDs.
- Build file, lexical, and package scopes.
- Represent local variables, parameters, results, labels, fields, and type parameters with semantic declaration IDs.
- Mark declaration mismatches as stale/partial instead of inventing mappings.

### Exit Gate

- Exact location and ID tests cover Unicode, comments, grouped declarations, and generics.
- Output is identical across worker counts and shuffled input ordering.
- No AST, token set, or source text is stored in the artifact.

Accepted evidence: exact reconciliation and stable-ID tests cover comments, grouped declarations, Unicode paths/identifiers, generics, structs, interfaces, functions, methods, constants, variables, aliases, defined types, parameters, results, fields, locals, labels, type parameters, nested scopes, and package-scope conflicts. Package coverage is 87.4%; shuffled tests, package/full regression, vet, targeted/full race tests, and repeatable benchmarks pass. See [LIE_PHASE_2_2_3_DECLARATION_RECONCILIATION.md](../Validation/LIE_PHASE_2_2_3_DECLARATION_RECONCILIATION.md). Engineering accepted the exit gate on 2026-07-23 and authorized Phase 2.2.4 only.

## Phase 2.2.4 — Receiver and Local Type Binding

### Work

- Bind methods to local declared receiver types.
- Distinguish pointer, value, and generic receivers.
- Model embedded, alias, use, constraint, and instantiation type relations.
- Preserve unresolved/ambiguous states where evidence is incomplete.

### Exit Gate

- Tests cover value/pointer receivers, aliases, embedding, generic receivers, duplicate names, and malformed declarations.
- No receiver target is selected only by an unqualified name outside its proven package scope.

Accepted evidence: tests cover cross-file value/pointer/generic receivers, receiver type parameters, missing/alias/interface/duplicate receiver targets, local and predeclared types, unresolved qualified types, embedding, aliases, uses, constraints, instantiations, deterministic ordering, and explicit relationship limits. Package coverage is 85.8%; shuffled tests, package/full regression, vet, targeted/full race tests, and repeatable benchmarks pass. See [LIE_PHASE_2_2_4_RECEIVER_TYPE_BINDING.md](../Validation/LIE_PHASE_2_2_4_RECEIVER_TYPE_BINDING.md). Engineering accepted the exit gate on 2026-07-23 and authorized Phase 2.2.5 only.

## Phase 2.2.5 — References and Imports

### Work

- Emit identifier, selector, type, and instantiation references.
- Resolve lexical, package-local, and same-package cross-file targets.
- Model default, named, dot, and blank imports.
- Classify import targets as resolved, external, unresolved, or ambiguous.
- Do not infer repository package identity from path suffixes.
- Link every resolved cross-package import to a non-stale `PackageIdentityProof`.

### Exit Gate

- Tests cover shadowing, dot imports, blank imports, package aliases, unresolved/external packages, ambiguous candidates, and cross-file references.
- Relationship bounds create explicit partial outcomes rather than silent truncation.
- No external tool or network activity is observed.

Accepted evidence: lexical and import-alias shadowing, same-package cross-file references, default/named/dot/blank imports, local/external/unresolved/ambiguous states, stale manifest proof rejection, unanimous/conflicting context handling, deterministic worker output, and explicit relationship limiting pass. Package coverage is 85.6%; shuffled tests, package/full regression, vet, targeted/full race tests, and repeatable 100/1,000-import benchmarks pass. See [LIE_PHASE_2_2_5_REFERENCES_IMPORTS.md](../Validation/LIE_PHASE_2_2_5_REFERENCES_IMPORTS.md). Engineering accepted the exit gate on 2026-07-23 and authorized Phase 2.2.6 only.

## Phase 2.2.6 — Conditional Interface Satisfaction

### Work

- Calculate value and pointer method sets with bounded `go/types` analysis.
- Evaluate only bounded candidate type/interface pairs.
- Derive candidates only from assertions, resolved assignments/conversions/arguments/returns, embeddings, receiver/selector sites, and existing local type relations.
- Emit `proven`, `disproven`, or `unknown`.
- Preserve compile-time assertions as evidence, not as a separate implementation category.
- Skip claims when embedded/external types or signatures are incomplete.

### Exit Gate

- Tests cover implicit satisfaction, pointer-only satisfaction, embedded interfaces, missing methods, signature mismatch, generics, incomplete imports, and compile-time assertions.
- Every `proven`/`disproven` result is reproducible from complete evidence.
- No all-types × all-interfaces scan appears in code or benchmarks.

Accepted evidence: value and pointer method sets, implicit assignments, compile-time assertions, embedded interfaces, missing methods, signature mismatches, local generics, incomplete imports, unrelated type errors, bounded candidate derivation from assignments/conversions/arguments/returns/embeddings, deterministic aggregation, package limits, and relationship limits pass. Package coverage is 85.6%; shuffled tests, package/full regression, vet, targeted/full race tests, and repeatable 100/1,000-check benchmarks pass. See [LIE_PHASE_2_2_6_INTERFACE_SATISFACTION.md](../Validation/LIE_PHASE_2_2_6_INTERFACE_SATISFACTION.md). Engineering accepted the exit gate on 2026-07-23 and authorized Phase 2.2.7 only.

## Phase 2.2.7 — Candidate Integration (`0.1.0`)

### Work

- Integrate the semantic engine additively after Phase 2.1.
- Publish through `rie.ArtifactStore` without mutable context fields.
- Add JSON/reporting support only as a view over the immutable artifact.
- Document diagnostics, limitations, configuration, examples, and artifact dependencies.
- Run tests, race tests, vet, formatting, shuffled tests, coverage, and repeatable benchmarks.
- Prove that each run is a full rebuild and that no previous semantic state is reused.
- Validate context checkpoints and deterministic diagnostic suppression/ordering/limits.

### Quality Gate

- All behavior and security tests pass.
- No race, panic, repository escape, source mutation, command execution, or network access.
- Package statement coverage is at least 80%.
- Security-boundary, digest, ID, ordering, and immutability helpers have complete branch coverage.
- Benchmarks record time, bytes, allocations, worker count, fixture size, Go version, OS, and hardware/CI identity.

Accepted evidence: typed prerequisite retrieval, additive single-assignment
publication, immutable JSON view, full-rebuild isolation, deterministic JSON,
missing/wrong prerequisite rejection, cancellation, diagnostic limits, and
artifact coexistence pass. Package coverage is 85.5%; ten shuffled runs, full
backend regression, vet, targeted/full race tests, and a repeatable 1,000-file
integration benchmark pass. See
[LIE_PHASE_2_2_7_CANDIDATE_INTEGRATION.md](../Validation/LIE_PHASE_2_2_7_CANDIDATE_INTEGRATION.md).
Engineering accepted the exit gate on 2026-07-23 and authorized Phase 2.2.8
only. Phase 2.2.9 remains unauthorized.

## Phase 2.2.8 — Real-Repository Validation

Validate the candidate against at least:

- a small Go CLI;
- a medium Go service;
- a large Kubernetes-style repository;
- a generics-heavy repository;
- a monorepo with multiple Go modules;
- a repository with unavailable external dependencies;
- intentionally broken and stale-source fixtures.

For every repository, record:

- candidate, resolved, partial, failed, stale, and skipped files;
- declarations reconciled;
- references by status;
- import bindings by status;
- package identity proofs by kind/status, conflicts, and stale evidence;
- resolved, unresolved, external, ambiguous, and partial declaration counts;
- receiver/type/interface relationships by status;
- diagnostics and omissions;
- warm-cache engine time and cold-cache filesystem time;
- peak/allocated memory, allocations, and worker count;
- crashes, panics, boundary violations, and nondeterminism;
- cancellation latency at file, package, traversal, and relationship boundaries;
- manually verified samples and classified defects.

Defects use small follow-up commits; do not rewrite published release-candidate history.

Local evidence: five representative open-source repositories, actual
Kubernetes, one malformed fixture, and one stale-source fixture were validated
at pinned revisions. Determinism, repository boundaries, explicit failure
states, package proofs, relationship limits, diagnostics, cancellation, timing,
and memory were recorded. Validation found and fixed blank-identifier
reconciliation, struct/interface alias reconciliation, and late reference
budgeting defects in follow-up commits. Package coverage is 85.8%; shuffled
tests, full regression, vet, and targeted/full race tests pass. See
[LIE_PHASE_2_2_8_REAL_REPOSITORY_VALIDATION.md](../Validation/LIE_PHASE_2_2_8_REAL_REPOSITORY_VALIDATION.md).
Engineering accepted this evidence on 2026-07-23 and authorized Phase 2.2.9
stabilization and the `1.0.0` freeze.

## Phase 2.2.9 — Stabilization and `1.0.0` Freeze

### Review

- Performance and memory profile.
- Artifact immutability and provenance.
- Dependency direction and absence of cycles.
- Public API, enum, ID, ordering, and position semantics.
- Package proof precedence, manifest evidence, and proof IDs.
- Diagnostic code meaning, ordering, duplicate suppression, per-file/global limits, and aggregation.
- Config zero/default behavior and additive-field compatibility.
- README, architecture, API, ADR, known limitations, and examples.
- Dead code and deferred technical debt.
- Full regression and compatibility tests.

### Release Package

- `CHANGELOG.md`
- release notes
- validation report
- benchmark summary
- known limitations
- supported feature matrix
- architecture and public API references

### Freeze Criteria

- No crashes, panics, races, repository boundary violations, source mutations, external execution, or network access.
- Deterministic results across repeated runs and worker counts.
- All critical/high defects resolved.
- Performance gate is measured, documented, and approved.
- Only documented non-critical limitations remain.
- Public API is explicitly approved and changed to `1.0.0`.
- `GoPackageIdentityInventory 1.0.0` is frozen before `GoSemanticInventory 1.0.0`.
- An annotated, namespaced release tag is created and pushed.

## Deferred Work

- Incremental invalidation, persistent semantic caches, and transitive cache reuse.
- Reading the ambient module cache or importing dependency export data.
- Compiler-equivalent build contexts and generated-source selection.
- External dependency acquisition or cached export-data ingestion.
- Call graph, control flow, data flow, dependency intelligence, architecture inference, bug reasoning, and patches.
- Language-neutral semantic interfaces, which require evidence from a second language implementation.

## After Phase 2.2

Do not begin the next language or intelligence engine immediately after feature completion. Begin it only after `GoSemanticInventory 1.0.0` is tagged, release documentation is complete, and the engineering manager authorizes the next phase.
