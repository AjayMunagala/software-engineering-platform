# ADR 0008: Digest-Verified Go Semantic Resolution

- Status: Accepted on 2026-07-22
- Date: 2026-07-20
- Decision owners: Phase 2.2 architecture review
- Supersedes: No prior ADR

## Context

`GoLanguageInventory 1.0.0` is frozen and intentionally stores repository files, packages, imports, and top-level declarations. It does not store source text, ASTs, function bodies, parameters, results, fields, interface methods, local declarations, type expressions, or identifier uses.

Phase 2.2 needs some of those facts to resolve lexical/package references, receiver types, generic scopes, and interface satisfaction. They cannot be derived reliably from the syntax inventory alone. Expanding the frozen artifact would break its responsibility and API contract.

The semantic engine must also follow project rules:

- never guess;
- use the lowest-level artifact containing required facts;
- keep artifacts immutable and versioned;
- stay deterministic and explainable;
- avoid external execution, downloads, and hidden dependencies;
- preserve repository boundaries.

## Decision

The Go Semantic Resolution Engine will use digest-verified ephemeral source re-parsing plus bounded Go standard-library type analysis.

Specifically:

1. It consumes `RepositorySnapshot 1.0.0`, `GoLanguageInventory 1.0.0`, and stable `GoPackageIdentityInventory 1.0.0`.
2. It reads only Go paths authorized by both artifacts.
3. It recalculates SHA-256 and analyzes a file only when the digest matches `GoFile.ContentDigest`.
4. It rebuilds AST/token/type state in memory with `go/parser`, `go/ast`, `go/token`, and bounded `go/types` use.
5. It discards source bytes, ASTs, token sets, and `go/types` objects after the run.
6. It uses a controlled importer that never runs repository commands and never downloads modules.
7. It resolves same-package and cross-file facts directly from verified local source.
8. It resolves cross-package repository imports only through a non-stale `PackageIdentityProof` from the approved immutable identity artifact. It never guesses from directory suffixes.
9. Package identity uses exact, digest-backed `go.mod`, `go.work`, local `replace`, verified vendor, nested-module, or versioned standard-library evidence as defined in [GO_PACKAGE_IDENTITY_PROOF.md](../Architecture/GO_PACKAGE_IDENTITY_PROOF.md).
10. It emits a separate immutable `GoSemanticInventory 0.1.0` candidate and leaves every prerequisite artifact unchanged.
11. Every Phase 2.2 run is a full rebuild; incremental invalidation and persistent caches are deferred to a separate ADR.
12. Stable IDs contain `go-semantic-id/v1`; re-keying after `1.0.0` requires a new artifact major version and full rebuild migration.

## Important Clarification About `go/types`

`go/types` is an in-process Go standard-library type checker. It does not inherently require cgo, network access, or external command execution.

Uncontrolled package loading/importing is the actual boundary risk: default loaders or future integrations can depend on installed export data, module state, toolchain behavior, or external commands. Therefore the engine owns a deterministic importer policy and does not use `go/packages`, `go list`, module download, or repository tool execution in Phase 2.2.

## Initial Resolution Boundary

The `0.1.0` candidate is allowed to prove:

- declaration reconciliation against Phase 2.1 symbol IDs;
- file, lexical, and same-package scopes;
- same-package cross-file references;
- receiver-to-local-type binding;
- local type relations, embeddings, and generic scopes;
- interface satisfaction only when all required types and method sets are complete;
- import aliases and cross-package targets supported by a valid package-identity proof;
- explicit unresolved/external/ambiguous/stale states when proof is incomplete.

The candidate must not claim complete repository-wide import resolution without the authoritative package-identity input. Module-cache and online dependencies remain external.

## Package Identity Decision

`GoPackageIdentityInventory` is a separate prerequisite because parsing module/workspace/vendor manifests is package identity discovery, not semantic relationship synthesis. It consumes the snapshot and Go syntax inventory, records exact manifest digests/ranges, and produces `PackageIdentityProof` values. The semantic engine revalidates proof-source digests before use.

Authoritative proof and precedence are frozen separately from semantic reference rules. Directory suffixes, common module conventions, ambient `GOROOT`, and the module cache are not accepted evidence.

## Rebuild and ID Decision

- Every `Resolve` call revalidates and reparses all eligible files and rebuilds all package/type state.
- No previous semantic artifact, AST, type state, or dependency cache participates in the candidate run.
- A future incremental path must prove equivalence to a full rebuild and define transitive invalidation in a new ADR.
- Candidate IDs begin `go:semantic:v1:` and metadata records `go-semantic-id/v1`.
- `0.x` may re-key only with an artifact minor and ID-scheme bump.
- After `1.0.0`, a re-key requires an artifact major and scheme bump. The canonical migration is full recomputation because semantic inventories are derived artifacts.

## Rejected Alternatives

### Resolve From `GoLanguageInventory` Alone

Rejected because the frozen artifact does not contain identifier occurrences, bodies, signatures, fields, interface members, or complete type expressions. Producing those relationships would require guessing.

### Expand `GoLanguageInventory 1.0.0`

Rejected because it would violate the frozen API, mix syntax and semantics, greatly increase memory, and create a God artifact.

### Persist ASTs or `go/types` Objects

Rejected because these are implementation structures, not stable portable contracts. They are memory-heavy, tied to toolchain internals, and unsuitable for deterministic JSON artifacts.

### Use `go/packages` or Execute `go list`

Rejected for Phase 2.2 because loading may execute the Go toolchain, inspect environment-dependent module state, or download dependencies. Those behaviors violate the read-only deterministic boundary.

### Build a Text/Regex Semantic Resolver

Rejected because it would duplicate the Go specification poorly and fail on scopes, aliases, embedding, generics, and method sets.

### Guess Local Packages From Import-Path Suffixes

Rejected because identical directory suffixes, replace directives, workspaces, nested modules, and vendoring make the result ambiguous. Unknown is preferable to a false edge.

### Hide Module Parsing Inside the Semantic Resolver

Rejected because it combines identity discovery with semantic synthesis, obscures provenance, and prevents other future engines from reusing the same exact package mapping.

### Add Incremental Caching to the Initial Candidate

Rejected because invalidation across imports, embedded types, method sets, and generics requires a separate measured design. Full rebuild is the deterministic reference implementation.

## Consequences

### Positive

- The frozen Phase 2.1 contract remains stable.
- Semantic analysis receives the syntax facts it actually requires.
- Source changes between phases are detected explicitly.
- Go language semantics come from standard-library parsing/type machinery rather than custom approximations.
- External dependencies and incomplete packages have honest, queryable states.
- The output is deterministic, explainable, and independently versioned.

### Costs and Limitations

- Go files are parsed again, increasing CPU and filesystem work.
- Type checking costs more memory than declaration inventory.
- Results can be partial when external packages or authoritative module identities are unavailable.
- Full rebuild repeats work when only a small number of files changed.
- Build tags, GOOS/GOARCH, cgo, generated sources, and compiler flags can change real build semantics and are not reproduced in the initial candidate.
- The final performance gate must be measured rather than copied from Phase 2.1.

## Guardrails

- No source is analyzed after digest mismatch.
- No engine result mutates prerequisite artifacts.
- No external process or network access is permitted.
- No per-reference warning flood; unresolved states are represented and aggregated.
- No interface-satisfaction claim is emitted as proven/disproven without complete comparable method sets.
- No absolute path or traversal order is used in stable IDs.
- No interface Cartesian product; candidates come only from verified assertions, assignments/conversions/calls/returns, embeddings, receiver sites, or existing local type relations.
- Cooperative cancellation checks occur at the documented file/package/node/relationship boundaries; synchronous parser/type-check calls are bounded by configuration.
- Diagnostics use deterministic ordering, exact duplicate suppression, per-file/global limits, and one aggregate omission record.
- No `1.0.0` semantic API freeze occurs before real-repository validation.

## Validation Required Before Approval

A design spike must prove and document:

- partial `go/types` behavior when imports are unavailable;
- receiver/method-set behavior for values and pointers;
- embedded interfaces and generic constraints;
- deterministic source positions and IDs;
- cancellation and memory bounds;
- digest mismatch and repository-boundary enforcement;
- no external command or network activity.
- authoritative module/workspace/replace/vendor/stdlib proof precedence;
- full-rebuild determinism and future incremental equivalence requirements;
- ID-scheme evolution and full-rebuild migration;
- cancellation latency and interface-candidate bounds;
- diagnostic ordering, suppression, and aggregation.

## Approval Record

## Acceptance Record

Engineering accepted the Phase 2.2.0 findings on 2026-07-22. The spike validated the package-identity proof model, controlled importer behavior, full-rebuild determinism, ID encoding, bounded interface candidates, cancellation checkpoints, diagnostic stability, and security boundaries within the documented limitations.

This acceptance authorizes only Phase 2.2.1 — Go Package Identity Engine. Phase 2.2.2 and later remain gated by their roadmap exit criteria and separate authorization. Neither candidate public API is frozen by this decision.
