# RIE v0.5 — Build & Package Intelligence Engine Design

**Status:** Approved — implementation authorized.

## Purpose

The Build & Package Intelligence Engine deterministically discovers build systems, package managers, workspaces, lock files, and declared toolchain requirements without executing external tools.

## In Scope

- Detect supported package managers and build systems from local repository evidence.
- Detect workspace and multi-module declarations and preserve their repository-relative locations.
- Inventory lock files separately from package managers and build systems.
- Extract explicitly declared toolchain or runtime version constraints, including Go directives and `package.json` `engines` values.
- Parse only bounded, supported local manifests using deterministic rules.
- Attach shared `rie.Evidence` to every detected item.
- Report ambiguous or contradictory evidence without choosing a winner.

## Out of Scope

- Execute Maven, Gradle, Go, Cargo, npm, pnpm, Yarn, Poetry, Composer, or any other tool.
- Build, test, install, validate, or modify repository code.
- Read or resolve dependency trees.
- Resolve versions online, contact registries, or download anything.
- Infer the installed tool version from the developer machine.
- Claim a Node package manager from `package.json` alone when no manager-specific evidence exists.
- Determine whether a build succeeds.
- Extract project ownership, business metadata, or architecture.
- Detect frameworks; Framework Engine owns that responsibility.

## Input Artifact

The engine consumes an immutable `RepositorySnapshot` containing the authorized repository root and normalized entries retained by Ignore Engine.

```go
type RepositorySnapshot struct {
    metadata Metadata
    rootPath string
    entries  []RepositoryEntry
    statistics Statistics
    diagnostics []Diagnostic
}
```

Accessors return defensive copies. Build intelligence does not consume `FrameworkInventory`: requiring it would create a false dependency and would still omit build files that Framework Engine does not inspect.

### Prerequisite decision

The current ignore-filtered `RunContext.Entries` is mutable pipeline state rather than a frozen artifact. Before v0.5 implementation, Ignore Engine should publish `RepositorySnapshot` version `1.0.0` as an additive output. This does not change or remove an existing Ignore Engine API.

## Output Artifact

The engine produces an immutable, versioned `BuildInventory` rather than a presentation-oriented summary.

```go
type BuildInventory struct {
    metadata              Metadata
    packageManagers       []PackageManager
    buildSystems          []BuildSystem
    workspaces            []Workspace
    lockFiles             []LockFile
    toolchainRequirements []ToolchainRequirement
}

type PackageManager struct {
    ID       string
    Name     string
    Location string
    Evidence []rie.Evidence
}

type BuildSystem struct {
    ID       string
    Name     string
    Location string
    Evidence []rie.Evidence
}

type Workspace struct {
    ID       string
    Kind     string
    Location string
    Members  []string
    Evidence []rie.Evidence
}

type LockFile struct {
    PackageManagerID string
    Path             string
    Location         string
    Evidence         []rie.Evidence
}

type ToolchainRequirement struct {
    Tool       string
    Constraint string
    Location   string
    Evidence   []rie.Evidence
}
```

The concrete Go implementation will keep slice state private and expose defensive-copy accessors. Stable IDs are machine-facing; names are presentation-facing. Repository root is represented by `.` and all other locations use normalized slash-separated relative paths.

A repository with no supported build system is valid. The engine always publishes an empty `BuildInventory` with initialized empty collections and produces no warning or error merely because nothing was detected.

Module and project names are intentionally deferred to RIE v0.6 Repository Metadata unless a name is required to identify a workspace member.

## Definitions

| Concept | Definition | Examples |
|---|---|---|
| Package manager | Manages declared packages, dependency resolution, installation, or reproducibility metadata. | npm, pnpm, Yarn, Go Modules, Cargo, Poetry, Composer |
| Build system | Defines or orchestrates compilation, transformation, packaging, tests, or build lifecycle tasks. | Go toolchain, Maven, Gradle, Cargo |
| Workspace | Groups multiple projects or modules under one coordinated declaration. | `go.work`, pnpm workspace, Cargo workspace, Gradle multi-project, Maven multi-module |
| Lock file | Records a reproducible resolved dependency state and is inventoried independently. | `package-lock.json`, `pnpm-lock.yaml`, `Cargo.lock`, `poetry.lock` |
| Toolchain requirement | A version constraint explicitly declared by repository configuration. | Go `1.24`, Node `>=20`, Python `>=3.12` |

`go.mod` is a Go module manifest. It is evidence for the Go Modules package-management system and the conventional Go toolchain, but it is not itself called a package manager or build system. `go.work` is a workspace declaration.

A technology may have more than one role. Cargo is represented once as a package manager and once as a build system, using the same stable ID and location-aware evidence. The model does not force a tool into one category.

## Detection Rules

Initial rules are deterministic and local:

| Ecosystem | Evidence | Result |
|---|---|---|
| Go | `go.mod` | Go Modules package manager; Go toolchain build system; `go` and `toolchain` requirements when declared |
| Go | `go.work` | Go workspace and declared members |
| Node | `packageManager` field | Explicit npm, pnpm, Yarn, or Bun manager and declared constraint |
| Node | manager-specific lock file | Matching package manager and separate lock-file item |
| Node | `package.json` `workspaces` | Node workspace with declared member patterns; no package-manager guess |
| Node | `pnpm-workspace.yaml` | pnpm workspace and package manager |
| Java | `pom.xml` | Maven build system/package manager; multi-module workspace when modules exist |
| Java | Gradle build/settings files | Gradle build system; multi-project workspace from settings declarations |
| Python | `requirements.txt` | pip-style package management evidence |
| Python | `pyproject.toml` | Declared build backend, package manager, workspace, and Python requirement only when explicit |
| Python | manager-specific lock file | Matching package manager and separate lock-file item |
| Rust | `Cargo.toml` | Cargo package manager/build system; workspace and Rust requirement when declared |
| Rust | `Cargo.lock` | Cargo lock-file item |
| PHP | `composer.json` | Composer package manager and explicit PHP requirement |
| PHP | `composer.lock` | Composer lock-file item |

Known filenames use their ecosystem-defined canonical casing. The engine does not reinterpret arbitrary differently-cased files on case-sensitive platforms.

Version constraints are preserved as declared. The engine does not resolve, compare, normalize semantically, or claim that the installed environment satisfies them.

When conflicting manifests or lock files exist, every evidence-backed item is retained. For example, both `package-lock.json` and `yarn.lock` produce npm and Yarn detections plus a `multiple_package_managers` warning; the engine never guesses which one is authoritative.

## Evidence

Every item must answer why it exists through at least one shared evidence record:

```go
rie.Evidence{
    File:  "frontend/pnpm-lock.yaml",
    Rule:  "lockfile.presence",
    Value: "pnpm-lock.yaml",
}
```

`File` preserves manifest and project location. `Rule` identifies the stable deterministic detector. `Value` contains only the matched filename, field, tool name, or declared constraint—not arbitrary manifest contents or secrets. Exact duplicate evidence is removed; distinct evidence is retained.

## Multi-Project Repositories

Items are keyed by stable ID and location, not only by display name. A repository containing `frontend/package.json`, `backend/go.mod`, and `cli/Cargo.toml` therefore produces independent location-aware items. Identical tools in two project directories remain distinguishable.

Workspace roots and members are normalized repository-relative paths. Member patterns remain declared patterns when deterministic expansion would require package-manager behavior.

## Extensibility

The orchestration loop operates on a simple in-code Go registry of detector definitions:

```go
type DetectorDefinition struct {
    ID             string
    DisplayName    string
    Categories     []Category
    FileMatchers   []FileMatcher
    Extractor      ExtractorKind
    EvidenceRule   string
}
```

The initial registry is a Go slice or map, not JSON configuration. Adding Buck, Bazel, Meson, Nix, or Pants through supported filename, JSON-field, TOML-field, YAML-field, XML-field, or line-directive matchers requires one registry entry and tests; engine orchestration does not change. A genuinely new file grammar may require one reusable extractor adapter plus its registry entry, but never a rewrite of the engine.

Registry-driven detection is a v0.5 requirement because this engine's purpose spans many build ecosystems. Framework Engine's separate registry refactor remains planned technical debt.

## Error Handling

- Missing or wrong-version `RepositorySnapshot`: return a stable prerequisite error and publish no partial inventory.
- Unsupported files: ignore silently; absence of a supported detector is not an error.
- Unreadable, oversized, or malformed supported files: add a standardized warning with file evidence and continue scanning other candidates.
- Contradictory valid evidence: retain all detections and add a warning; never choose without evidence.
- Context cancellation: stop promptly and return the context error.
- Artifact publication failure: return the stable artifact-store error.

No diagnostic contains full manifest contents or secret values.

## Tests

Required coverage before API freeze:

- Empty repository and repositories with no supported build evidence.
- Every supported package manager, build system, workspace, lock file, and version field.
- `go.mod` concept separation and Cargo's dual role.
- `package.json` without manager evidence does not guess npm.
- Multiple independent projects preserve locations.
- Multiple managers and contradictory evidence are retained with warnings.
- Duplicate evidence is removed.
- Ignored files never participate.
- Malformed, unreadable, and oversized manifests produce warnings without fabricated detections.
- Workspace members and declared patterns are deterministic.
- Inventory and nested slices are immutable to consumers.
- Registry validation rejects duplicate IDs and invalid rules.
- Engine removal does not break pipeline orchestration.
- JSON presentation output is derived from, but not used as, the engine artifact.

## Benchmarks and Performance

- File candidate discovery remains `O(n)` over snapshot entries.
- Detection memory is `O(c + d)`, where `c` is supported candidate files and `d` is retained detections; unrelated filenames are not duplicated.
- Only supported candidate files are opened, and every read has a configured byte limit.
- A repeatable benchmark uses 100,000 normalized entries with realistic nested manifests and lock files.
- A separate optional stress benchmark uses 1,000,000 entries and is not part of normal unit-test execution.
- The benchmark reports time, bytes, and allocations and measures Build Engine execution without timing prerequisite-engine setup.

Performance is measured and recorded before freezing; no unverified latency guarantee is part of the contract.

## Security and Audit

The engine is local and read-only. It does not execute repository code, spawn processes, access the network, follow unauthorized paths, or modify files. Reads remain inside the authorized snapshot root and respect Ignore Engine results and configured size limits.

The scan report records engine version, artifact version, warnings, and evidence. It never logs entire manifests.

## API Stability and Approval Gate

The proposed concepts are intended to freeze as `BuildInventory` version `1.0.0`. New tools add items and registry definitions without changing existing fields. New optional artifact sections require an additive minor version; semantic changes require a new major version.

The API is frozen only after:

1. `RepositorySnapshot` input is approved and versioned.
2. All data-model and policy questions in this document are approved.
3. Implementation tests, immutability tests, `go vet`, and benchmarks pass.
4. Example inventories for Go, Node, Java, Python, Rust, PHP, and a mixed monorepo are reviewed.

## Future Extension Points

- Additional registry definitions and reusable manifest extractors.
- Additional toolchain requirement sources.
- Build target and task discovery in a later artifact version.
- Dependency graph construction by Dependency Intelligence Engine.
- Optional manifest-content cache shared through a separate versioned artifact.

External execution, online resolution, dependency-tree analysis, and build validation remain separate future engines and are not extensions of v0.5.
