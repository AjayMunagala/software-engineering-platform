# Go Semantic Resolution Engine

## Document Status

- Phase: 2.2.7 candidate integration
- Status: Phase 2.2.8 accepted; Phase 2.2.9 stabilization authorized
- Candidate engine version: `0.1.0`
- Candidate artifact: `go-semantic-inventory` `0.1.0`
- Stable artifact target: `1.0.0`, only after implementation, stabilization, and real-repository validation

## Responsibility

The Go Semantic Resolution Engine deterministically re-parses digest-verified Go source authorized by `RepositorySnapshot` and produces bounded semantic relationships without executing repository tools, downloading dependencies, modifying source files, or mutating prerequisite artifacts.

## Why Source Re-parsing Is Required

`GoLanguageInventory 1.0.0` is intentionally a syntax inventory. It contains files, packages, imports, and top-level declarations, but it does not contain function bodies, parameter and result types, fields, interface methods, identifier occurrences, or persistent ASTs.

Those omitted facts are required for semantic resolution. Therefore, Phase 2.2 must not pretend that the frozen inventory alone is sufficient and must not add semantic data to that frozen contract. The engine may read source only through the controlled process defined below.

## In Scope

- Verify required artifact names and versions.
- Consume authoritative cross-package mappings only from `GoPackageIdentityInventory`.
- Read only Go files present in both `RepositorySnapshot 1.0.0` and `GoLanguageInventory 1.0.0`.
- Recalculate SHA-256 before analysis and require it to match `GoFile.ContentDigest`.
- Re-parse verified source into ephemeral Go ASTs.
- Reconcile parsed declarations with frozen syntax symbols.
- Build file and package lexical scopes.
- Resolve package-local and cross-file identifier references when evidence is complete.
- Bind methods to locally declared receiver types.
- Classify import bindings as local, external, unresolved, or ambiguous.
- Resolve type-parameter scopes and bounded generic instantiations.
- Evaluate local interface satisfaction only when complete method-set and type information is available.
- Preserve unresolved, ambiguous, partial, stale, and failed states explicitly.
- Produce a deterministic, immutable, versioned `GoSemanticInventory`.

## Out of Scope

- Call graphs, control-flow graphs, and data-flow analysis.
- Runtime dispatch, reflection behavior, generated-at-runtime code, or behavioral inference.
- Architecture, dependency-risk, bug, security, or code-quality judgments.
- Cross-language symbol resolution.
- Executing `go`, `go list`, compilers, tests, generators, package managers, or repository scripts.
- Network access, module download, proxy lookup, or online version resolution.
- Compiler-equivalent build-tag, GOOS, GOARCH, cgo, and generated-file selection.
- Editing files, generating patches, AI/LLM calls, or autonomous actions.
- Claiming complete resolution when external packages or required source facts are unavailable.

## Inputs

The engine consumes exactly three immutable artifacts:

1. `RepositorySnapshot 1.0.0` — authorizes the repository root and ignore-filtered paths.
2. `GoLanguageInventory 1.0.0` — supplies candidate Go files, content digests, packages, imports, and stable declaration IDs.
3. `GoPackageIdentityInventory 0.1.0` — supplies exact, digest-backed module/workspace/replace/vendor package mappings.

It does not consume mutable `RunContext` fields, framework/build/metadata/summary artifacts, or presentation reports.

The package-identity proof contract is defined in [GO_PACKAGE_IDENTITY_PROOF.md](GO_PACKAGE_IDENTITY_PROOF.md). Without a valid proof, a cross-package import cannot be marked `resolved`.

## Controlled Source Access

For every candidate file, the engine must:

1. Normalize the inventory path using the same repository-relative convention as Phase 2.1.
2. Confirm that the path exists as a regular file in `RepositorySnapshot`.
3. Join it beneath the canonical repository root and reject root escape.
4. Reject symbolic-link or special-file boundary violations according to the repository snapshot policy.
5. Enforce `MaxSourceFileSize` before reading.
6. Read without writing or locking the repository file.
7. Calculate SHA-256 and compare it with `GoFile.ContentDigest`.
8. Parse only when the digest matches.

A missing, changed, unreadable, oversized, or out-of-root file produces a file outcome and bounded diagnostic. The engine must never use stale syntax facts to create guessed semantic edges.

Source bytes, token sets, ASTs, and `go/types` state are ephemeral run data. They are never stored in `GoSemanticInventory` or the shared artifact store.

Phase 2.2.5 re-hashes every repository manifest listed by a proof immediately before using that proof. A changed manifest makes the consumer treat the proof as stale and prevents dependent resolved edges; the source identity artifact is never mutated. Applicable contexts are combined without environment inference: every usable context must resolve to the same repository package before the import is `resolved`; conflicting targets are `ambiguous`, and stale or incomplete context results remain unresolved.

## Rebuild and Invalidation Policy

Phase 2.2 performs a full semantic rebuild on every `Resolve` call. In Phase 2.2.2 this means every eligible file is re-authorized and re-hashed. As later milestones are authorized, it additionally means:

- every eligible Go file is re-authorized, re-hashed, and re-parsed;
- every eligible package scope/type state is rebuilt;
- every emitted relationship is recomputed;
- no AST, type state, dependency cache, or prior semantic artifact is reused;
- one changed source/manifest requires the caller to regenerate prerequisite artifacts before rerunning semantics.

Incremental package invalidation, transitive cache reuse, and persistent semantic indexes are deferred. They require a separate ADR and additive cache/provenance contract. A future incremental implementation must produce byte-for-byte equivalent semantic facts to a full rebuild before it can replace this policy.

## Resolution Levels

Semantic results use explicit confidence-by-state, not numerical confidence:

- `resolved` — exactly one target is proven from available evidence.
- `unresolved` — no target can be proven.
- `ambiguous` — more than one valid target remains.
- `external` — the reference belongs to a package whose source/type information is unavailable locally.
- `partial` — analysis completed but prerequisites prevented a complete result.
- `stale` — the source digest no longer matches the frozen syntax artifact.
- `failed` — the file/package could not be analyzed safely.

Unknown is a valid result. Evidence is stronger than inference.

## Processing Pipeline

```text
RepositorySnapshot 1.0.0 ───────┐
GoLanguageInventory 1.0.0 ──────┼─> validate inputs
GoPackageIdentityInventory 0.1.0┘
                                  ↓
                         authorize and verify files
                                  ↓
                         ephemeral Go re-parse
                                  ↓
                       declaration reconciliation
                                  ↓
                      package and lexical scopes
                                  ↓
                references / receivers / imports / types
                                  ↓
                      conditional interface checks
                                  ↓
                    immutable GoSemanticInventory
```

The stages are deterministic and bounded. Later stages consume only verified results from earlier stages.

## Type-Resolution Strategy

- Use Go standard-library `go/parser`, `go/ast`, `go/token`, and `go/types` in process.
- Use a controlled importer that never invokes repository commands or downloads modules.
- Resolve repository-local packages only when their import identity is proven by `PackageIdentityProof`.
- Treat unavailable external packages as external/unresolved rather than synthesizing types.
- Emit interface-satisfaction results only when both method sets are complete. Otherwise emit `unknown`/partial state or no assertion.

`go/types` itself does not require cgo or external command execution. The risk lies in unrestricted import loading, which this design forbids.

## Interface Candidate Bounds

The engine never evaluates every concrete type against every interface. Candidate pairs are created only from verified syntax/type relationships:

1. compile-time interface assertions;
2. assignments, conversions, arguments, and returns where both static interface and concrete types are resolved;
3. embedded interface/type relationships;
4. receiver/selector sites that require a method-set comparison;
5. explicit local type relations already emitted by the semantic pass.

Pairs are deduplicated by `(concrete declaration ID, interface declaration ID, pointer mode)`, sorted by that key, and evaluated only while the global `MaxRelationships` budget remains. Exhaustion creates partial outcomes and one bounded aggregate diagnostic; it never falls back to a Cartesian scan.

## Output

The output is `GoSemanticInventory`, defined in [GO_SEMANTIC_MODEL.md](GO_SEMANTIC_MODEL.md). It contains:

- artifact metadata and source-artifact references;
- semantic file outcomes;
- declaration bindings;
- identifier references and resolution states;
- receiver bindings;
- import bindings;
- type relations and generic facts;
- conditional interface-satisfaction results;
- diagnostics and statistics.

It contains references to Phase 2.1 symbol/file/package IDs rather than copied `GoSymbol`, `GoFile`, or `GoPackage` inventories.

## Determinism and Immutability

- IDs contain an explicit scheme version and are derived from normalized source identity and exact source ranges, never traversal order.
- Collections are sorted by stable keys before artifact construction.
- Duplicate semantic facts are removed by stable identity.
- The public constructor remains private to the package.
- Every collection accessor returns a deep defensive copy.
- Prerequisite artifacts are read-only and never modified.

### Stable ID Evolution

- Candidate scheme: `go-semantic-id/v1`; IDs begin with `go:semantic:v1:`.
- Variable text and nested IDs use UTF-8 byte-length-prefixed segments so delimiters cannot create ambiguous encodings.
- During `0.x`, an ID re-key increments the artifact minor version and ID-scheme version and is documented as a candidate breaking change.
- After artifact `1.0.0`, existing IDs and their meaning are immutable throughout the major version.
- An algorithm change that re-keys existing facts requires a new artifact major version and new scheme prefix, such as `go-semantic-id/v2`.
- Semantic artifacts are derived data, so the canonical migration is a full rebuild from frozen prerequisites and verified source. IDs from different schemes are never silently compared or mixed.
- If persisted external consumers require old-to-new linkage, the major-version release must include a migration note and may include a deterministic mapping only where the relationship is provably one-to-one.

## Error Handling

Fatal errors stop the run and produce no artifact:

- missing or incompatible prerequisite artifact;
- invalid configuration;
- invalid repository root or artifact identity conflict;
- canceled context;
- internal invariant failure that makes the artifact unreliable.

File/package problems produce bounded diagnostics and explicit outcomes where safe. Planned diagnostic codes include:

- `semantic_source_missing`
- `semantic_source_unreadable`
- `semantic_source_oversized`
- `semantic_source_outside_root`
- `semantic_digest_mismatch`
- `semantic_parse_error`
- `semantic_declaration_unmatched`
- `semantic_declaration_ambiguous`
- `semantic_syntax_symbol_unmatched`
- `semantic_package_scope_conflict`
- `semantic_receiver_unresolved`
- `semantic_receiver_ambiguous`
- `semantic_package_proof_stale`
- `semantic_package_limit`
- `semantic_interface_parse_error`
- `semantic_relationship_limit`
- `semantic_type_error`
- `semantic_import_unresolved`
- `semantic_reference_ambiguous`
- `semantic_diagnostic_limit`

Ordinary unresolved external references are counted and represented in the artifact; they do not each create noisy warnings.

### Diagnostic Stability

- Normalize repository paths and strip absolute-machine paths from messages.
- Suppress exact duplicates by `(code, file, start offset, end offset, normalized message)` before applying limits.
- Sort by `(file, start offset, end offset, severity, code, normalized message)`.
- Apply `MaxDiagnosticsPerFile` first and `MaxDiagnostics` globally second.
- `MaxDiagnostics` includes the aggregate limit diagnostic. When any item is omitted, reserve the final available slot for exactly one deterministic run-level `semantic_diagnostic_limit`; if the limit is one, that aggregate is the only emitted diagnostic.
- Count every omitted candidate, including an otherwise retained item displaced by the reserved aggregate slot.
- Stable-sort retained ordinary diagnostics first; the aggregate limit diagnostic is the documented final-item exception to the ordinary location sort.
- External/unresolved references use relationship states and aggregate statistics rather than one diagnostic per occurrence.
- Diagnostic code meaning, ordering, suppression, and aggregation must be frozen with artifact `1.0.0`.

## Cancellation Contract

The engine checks `context.Context`:

- before and after each file read and hash;
- before and after each file parse;
- at least every 1,024 AST nodes during custom traversal;
- before and after each package type-check;
- at least every 256 emitted references or interface candidates;
- before artifact construction and publication.

Go parser and `go/types` calls are synchronous and cannot be interrupted mid-call. Therefore candidate `0.1.0` defines maximum cooperative cancellation latency as the slowest of one size-bounded file parse, one size-bounded package type-check, a 1,024-node traversal batch, or a 256-relationship batch, plus worker scheduling overhead. Phase 2.2.0 must measure those units and establish an approved wall-clock target before `1.0.0`.

## Security and Side Effects

The engine is read-only. It must not:

- execute any external process;
- access the network;
- follow unapproved paths;
- read files omitted by the snapshot;
- write caches into the repository;
- persist source text or ASTs;
- log source contents, secrets, or full environment variables.

## Package Structure

The Go-specific implementation uses the project package standard:

```text
backend/lie/golang/semantic/
    interface.go
    implementation.go
    config.go
    model.go
    errors.go
    README.md
    implementation_test.go
    implementation_benchmark_test.go
```

No language-neutral semantic abstraction is added to `backend/lie` until a second semantic language engine proves that abstraction necessary.

## Testing Requirements

- Empty repository and empty Go inventory.
- Missing or wrong artifact version.
- Digest match and digest mismatch.
- Root escape, symlink/special-file, oversized, unreadable, and missing source.
- Syntax failure and declaration reconciliation failure.
- Package-local, cross-file, unresolved, ambiguous, dot-import, blank-import, and external references.
- Value and pointer receiver bindings.
- Embedded types and interface method sets.
- Generic declarations, constraints, and instantiations.
- Partial packages and unavailable external imports.
- Deterministic output across worker counts and shuffled input ordering.
- Deep immutability of all accessors.
- Cancellation, diagnostic limits, race tests, and repeatable benchmarks.
- Full-rebuild equivalence and proof-manifest staleness.
- Interface candidate derivation without Cartesian evaluation.
- Diagnostic ordering, duplicate suppression, per-file/global limits, and aggregation.

Coverage is not accepted as a substitute for behavior tests. Security-boundary, digest-verification, deterministic-ID, and immutability helpers require complete branch coverage; the package target is at least 80% statement coverage.

## Performance Policy

Complexity should remain approximately O(files + syntax nodes + emitted relationships). Work is bounded by configuration and uses no more than eight workers.

The final release gate will be established from a checked-in repeatable benchmark after the Phase 2.2 design spike. Phase 2.1's 30-second/10,000-file target is not copied blindly because semantic type checking has a different cost profile. Warm-cache engine time and cold-cache filesystem time must be recorded separately.

## Dependency Rule

```text
RepositorySnapshot 1.0.0 ───────┐
GoLanguageInventory 1.0.0 ──────┼─> GoSemanticInventory 0.1.0 candidate
GoPackageIdentityInventory 0.1.0┘
```

The semantic artifact is additive. It does not change or replace either frozen input.

## Approval Gate

Phase 2.2.0 through Phase 2.2.8 are accepted. Phase 2.2.9 stabilization and the `1.0.0` freeze are authorized. The approved architecture covers:

- controlled source re-parsing and digest verification;
- the authoritative `PackageIdentityProof` contract and supporting artifact;
- full-rebuild/invalidation policy;
- semantic ID evolution and migration policy;
- bounded local-only import/type strategy;
- `GoSemanticInventory 0.1.0` as a candidate rather than a prematurely frozen `1.0.0`;
- explicit unresolved/partial/stale behavior;
- package/API model;
- staged roadmap and validation gate.

## Phase 2.2.7 Integration Contract

The candidate integrator reads `RepositorySnapshot 1.0.0`,
`GoLanguageInventory 1.0.0`, and `GoPackageIdentityInventory 0.1.0` by exact
type from one per-run `rie.ArtifactStore`. It resolves from those facts only,
then publishes `GoSemanticInventory 0.1.0` through the store's existing
single-assignment contract.

The integrator holds only immutable configuration and an engine reference. It
does not read an earlier semantic artifact, cache AST/type state, mutate any
prerequisite, or add fields to `RunContext`. Reusing an integrator with a new
store performs a full rebuild. Running twice against the same store fails
before resolution because semantic publication is already complete.

JSON and reporting use a detached `GoSemanticInventoryView`; presentation data
is not an artifact and cannot become a semantic input.
