# Go Package Identity Proof

## Status

- Phase: 2.2 architecture prerequisite
- Status: Stable; `1.0.0` frozen on 2026-07-23
- Artifact: `go-package-identity-inventory` `1.0.0`
- ID scheme: `go-package-proof-id/v1`

## Purpose

Provide deterministic, location-aware proof that a Go import path maps to one repository package without executing Go tools, accessing the network, reading the module cache, or guessing from directory names.

The Go Semantic Resolution Engine may mark a cross-package import `resolved` only when it consumes a valid `PackageIdentityProof`. Same-package lexical and cross-file resolution does not require an import proof.

## Artifact Boundary

Package/module/workspace identity is a separate prerequisite concern, not hidden logic inside semantic resolution.

```text
RepositorySnapshot 1.0.0 ─┐
                          ├─> GoPackageIdentityInventory 1.0.0
GoLanguageInventory 1.0.0 ┘

RepositorySnapshot 1.0.0 ───────┐
GoLanguageInventory 1.0.0 ──────┼─> GoSemanticInventory 0.1.0
GoPackageIdentityInventory 1.0.0┘
```

The identity engine reads only manifests present in `RepositorySnapshot`, records the exact digest of every manifest used, and maps only packages already present in `GoLanguageInventory`.

## Candidate Model

```go
type ProofKind uint8

const (
    ProofSameModule ProofKind = iota + 1
    ProofWorkspaceModule
    ProofLocalReplace
    ProofVendor
    ProofStandardLibrary
)

type ProofStatus uint8

const (
    ProofResolved ProofStatus = iota + 1
    ProofUnresolved
    ProofAmbiguous
    ProofExternal
    ProofStale
)

type ResolutionContextKind uint8

const (
    ContextSingleModule ResolutionContextKind = iota + 1
    ContextWorkspace
    ContextModuleVendor
    ContextWorkspaceVendor
    ContextUnmanaged
)

type ResolutionContext struct {
    ID             string                `json:"id"`
    Kind           ResolutionContextKind `json:"kind"`
    Root           string                `json:"root"`
    ManifestFiles  []string              `json:"manifest_files"`
    MainModuleIDs  []string              `json:"main_module_ids"`
    Evidence       []PackageIdentityEvidence `json:"evidence"`
}

type ModuleIdentity struct {
    ID             string                    `json:"id"`
    ModulePath     string                    `json:"module_path"`
    Root           string                    `json:"root"`
    GoVersion      string                    `json:"go_version,omitempty"`
    Evidence       []PackageIdentityEvidence `json:"evidence"`
}

type PackageIdentityEvidence struct {
    File          string           `json:"file,omitempty"`
    ContentDigest string           `json:"content_digest"`
    Rule          string           `json:"rule"`
    Value         string           `json:"value"`
    Location      *lie.SourceRange `json:"location,omitempty"`
}

type PackageIdentityProof struct {
    ID                 string                    `json:"id"`
    ResolutionContextID string                   `json:"resolution_context_id"`
    ImportingPackageID string                    `json:"importing_package_id"`
    ImportPath         string                    `json:"import_path"`
    TargetPackageID    string                    `json:"target_package_id,omitempty"`
    TargetDirectory    string                    `json:"target_directory,omitempty"`
    Kinds              []ProofKind               `json:"kinds"`
    Status             ProofStatus               `json:"status"`
    Evidence           []PackageIdentityEvidence `json:"evidence"`
    CandidatePackageIDs []string                 `json:"candidate_package_ids,omitempty"`
}

type PackageIdentityStatistics struct {
    ManifestsInspected int            `json:"manifests_inspected"`
    Contexts           int            `json:"contexts"`
    Modules            int            `json:"modules"`
    ProofsByStatus     map[string]int `json:"proofs_by_status"`
    Diagnostics        int            `json:"diagnostics"`
    OmittedDiagnostics int            `json:"omitted_diagnostics"`
}

type GoPackageIdentityInventory struct {
    metadata                    Metadata
    sources                     []rie.ArtifactReference
    contexts                    []ResolutionContext
    modules                     []ModuleIdentity
    proofs                      []PackageIdentityProof
    diagnostics                 []lie.Diagnostic
    statistics                  PackageIdentityStatistics
}
```

`GoPackageIdentityInventory` contains metadata, source-artifact references, resolution contexts, module identities, proofs, diagnostics, and statistics. Managed contexts contain the evidence that selected them. Manifest evidence requires `File`, digest, and exact `Location`. `Kinds` preserves every rule category in a multi-step proof and is sorted/deduplicated. The inventory uses private construction, stable sorting, and deep-copy accessors. Exact standard-library indexing is deferred and no index-version field is published until that feature is approved.

## What Counts as Authoritative Proof

A resolved repository-package proof requires an unbroken evidence chain:

1. The importing file maps to exactly one `GoPackage` in the frozen syntax inventory.
2. The importing package belongs to exactly one effective module boundary.
3. An explicit deterministic resolution context is recorded.
4. An exact manifest rule maps the import path to one module root within that context.
5. The import suffix maps to one normalized directory beneath that root.
6. That directory is inside `RepositorySnapshot`, does not cross an unselected nested-module boundary, and maps to exactly one `GoPackage.ID` with verified Go source.
7. Every manifest used by the chain has a recorded SHA-256 digest.

Directory-name similarity, path suffixes without a module rule, common conventions, framework knowledge, and statistical confidence are never proof.

## Resolution Context

The ambient `GOWORK`, `GOFLAGS`, working directory, module cache, and `GOROOT` are not hidden inputs. The identity engine derives explicit candidate contexts from repository artifacts:

- a single-module context rooted at the importing package's nearest ancestor `go.mod`;
- one context for each repository `go.work` that verifiably includes that module through `use`;
- the default vendor context only when the applicable Go version and consistent vendor manifest meet the official automatic-vendoring rule.
- an unmanaged context for import-bearing packages outside any verified module; it produces unresolved proofs without fabricated evidence.

Each proof records its context. If all applicable contexts resolve to the same target package, the combined import binding may be `resolved`. If contexts select different targets and no explicit approved build-context artifact chooses one, the result is `ambiguous`. The engine does not inspect process environment to break the tie.

## Deterministic Rule Precedence Within a Context

1. Determine the importing package's nearest ancestor `go.mod`; nested modules are hard boundaries.
2. If an approved exact-version standard-library index contains the import, classify it as standard-library external identity; repository module rules cannot shadow it.
3. In a vendor context, map main/workspace module packages to their own roots, but map dependency packages only through the applicable consistent `vendor/modules.txt`. Replacement directives validate vendor consistency; they do not bypass the selected vendor copy.
4. In a non-vendor workspace context, map every `go.work use` module as a main module and apply `go.work replace` before module-level replacements.
5. In a non-vendor single-module context, map the importing main module's own `module` path to its root.
6. For any remaining dependency path, require a matching selected/declared module identity and then apply the applicable exact or wildcard local `replace` rule. A `go.mod replace` alone does not add a dependency to the module graph.

Within import-prefix matching, use the longest valid module-path prefix on a path-segment boundary. If two rules at the same applicable precedence map to different valid targets, the result is `ambiguous`. Lower-precedence rules do not break the tie. If no authoritative rule completes the chain, the result is `unresolved` or `external`.

## Evidence Sources

### `go.mod`

- `module` provides the declared module-path prefix.
- The nearest ancestor manifest owns a package.
- The target directory must not cross into another nested module unless another explicit proof step selects that module.
- `require` alone proves a dependency declaration, not a local target package.
- Phase 2.2 does not perform full minimal-version selection. A dependency without an exact workspace, local-replace, or vendor mapping remains external even when it is required.
- Manifest bytes and the exact directive range are recorded.

### `replace`

- A filesystem replacement is local only when its normalized target remains inside `RepositorySnapshot` and its target module/package identity is independently verified.
- Relative paths are resolved from the declaring `go.mod` directory.
- An absolute path, root escape, missing target, or target outside the snapshot is not locally resolved.
- A replacement to another module path/version without local source remains external.
- Version-qualified replacements apply only when their declared version condition is exactly known; otherwise the proof is unresolved.
- A module-level `replace` requires the old module to participate through a matching `require`/selected module fact; `replace` alone is not proof that the dependency is active.

### `go.work`

- Each `use` entry must resolve inside the snapshot to a directory with a verified `go.mod`.
- The used module's own `module` directive supplies its import prefix.
- A workspace context is applicable only when the importing module is a verified `use` member of that workspace.
- Multiple applicable workspace files or duplicate module prefixes with different targets produce `ambiguous`; filesystem proximity is not a tie-breaker.
- `replace` directives in the applicable workspace override module-level replacements and require the same local-target validation.

### Nested Modules

- The nearest ancestor `go.mod` defines the package's module.
- Parent-module prefix mapping stops at a nested-module root.
- Nested modules are independent unless an applicable workspace or local replace rule connects them.

### `vendor/`

- Vendored resolution requires a verified and manifest-consistent `vendor/modules.txt` entry, the corresponding repository-relative vendor directory, and a matching syntax-inventory package.
- A directory under `vendor/` without `vendor/modules.txt` evidence is not authoritative.
- Vendoring is scoped to its owning module; an unrelated parent/sibling vendor directory is ignored.
- The engine models documented default vendoring only: module vendoring when the applicable `go` directive enables it, and workspace vendoring only under the supported workspace-vendor rules. It does not infer ambient `-mod` flags.

### Standard Library

- Import-path shape is not sufficient proof.
- Standard-library identity requires an immutable, engine-versioned standard-library index for an exact supported Go release.
- An exact release may come only from an approved explicit target-toolchain input. The `go` directive is a minimum requirement and the `toolchain` directive is a suggestion; neither alone proves the toolchain actually selected for a build.
- If the exact Go release cannot be proven or the import is absent from that release's index, it remains external/unresolved.
- The engine does not execute `go list std` or inspect ambient `GOROOT` as an undocumented dependency.

### Module Cache and Downloaded Dependencies

- The ambient Go module cache is outside the repository snapshot and is never read in Phase 2.2.
- Network/module downloads are forbidden.
- A dependency available only in the module cache or network is `external`; it cannot target a repository `GoPackage.ID`.

## Conflict and Staleness Policy

- Duplicate module paths with different local roots are `ambiguous` unless an exact applicable replace/workspace rule selects one.
- Conflicting manifest facts never use discovery order as a tie-breaker.
- Before the semantic engine uses a proof, it re-hashes every manifest evidence file.
- Any digest mismatch causes the semantic consumer to treat that immutable proof as `stale`; dependent imports and semantic relationships become partial/unresolved. The consumer never mutates the identity inventory.
- A caller must rebuild the package-identity artifact after repository changes.

## Stable Identity

Proof IDs include their scheme version:

```text
go:package-proof:v1:<context-id-byte-length>:<resolution-context-id>#<package-id-byte-length>:<importing-package-id>#<import-path-byte-length>:<import-path>#<proof-kinds>
```

Lengths are UTF-8 byte lengths. Proof kinds are sorted and deduplicated. The Phase 2.2.1 production golden vector is:

```text
go:package-proof:v1:38:go:package-context:v1:single-module:0:#17:go:package:.#main#19:example.com/app/lib#same-module
```

IDs use normalized repository-relative paths and semantic values, never absolute paths, timestamps, map order, or worker order.

Candidate metadata records `go-package-proof-id/v1`. During `0.x`, re-keying increments the artifact minor and proof-ID scheme. After `1.0.0`, a re-key requires a new artifact major and scheme; the canonical migration is a full identity-artifact rebuild.

## Empty and Partial Repositories

- A repository without `go.mod`, `go.work`, or `vendor/modules.txt` produces a valid empty/partial identity inventory without warnings.
- Same-package semantic resolution remains available.
- Cross-package imports are external/unresolved unless another allowed proof source applies.
- Malformed manifests produce bounded diagnostics; valid independent manifests continue.

## Side Effects and Security

The identity engine and its consumers do not execute commands, access the network, read outside the snapshot, inspect the ambient module cache, modify files, or persist manifest contents. Only normalized facts, exact ranges, digests, and evidence are stored.

## Package Structure

If approved, the supporting implementation follows the package standard:

```text
backend/lie/golang/packageidentity/
    interface.go
    implementation.go
    config.go
    model.go
    errors.go
    README.md
    implementation_test.go
    implementation_benchmark_test.go
```

## Phase 2.2.1 Exit Gate

Phase 2.2.1 validates rule precedence against fixtures containing:

- a single module;
- multiple independent modules;
- nested modules;
- `go.work` with multiple `use` directives;
- local and external `replace` directives;
- conflicting module paths;
- vendored packages;
- absent/malformed manifests;
- stale manifest digests;
- ambiguous workspace membership.

Engineering accepted ADR 0008 and authorized this package on 2026-07-22. Phase 2.2.9 accepted the validated public contract and authorized the `1.0.0` freeze on 2026-07-23.

## Specification Sources

Proof behavior is checked against the official [Go Modules Reference](https://go.dev/ref/mod), [go.mod file reference](https://go.dev/doc/modules/gomod-ref), and [Go command workspace documentation](https://pkg.go.dev/cmd/go). Phase 2.2 records the supported Go versions because workspace and vendor semantics can evolve between toolchain releases.
