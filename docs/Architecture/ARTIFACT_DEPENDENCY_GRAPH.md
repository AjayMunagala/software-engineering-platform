# Platform Artifact Dependency Graph

This document is the source of truth for typed artifact dependencies. RIE provides the frozen foundation shown first; later platform engines must depend on the lowest-level artifact containing the facts they need.

```text
Discovery Engine
    └── DiscoveryInventory 1.0.0

Ignore Engine
    └── RepositorySnapshot 1.0.0
            ├── Language Engine
            │       └── LanguageInventory 1.0.0
            │               └── Framework Engine
            │                       └── FrameworkInventory 1.0.0
            │
            └── Build & Package Intelligence Engine
                    └── BuildInventory 1.0.0

Repository Metadata Engine consumes:
    ├── DiscoveryInventory 1.0.0
    ├── RepositorySnapshot 1.0.0
    ├── LanguageInventory 1.0.0
    ├── FrameworkInventory 1.0.0
    └── BuildInventory 1.0.0
            └── RepositoryMetadata 1.0.0

Repository Intelligence Summary consumes:
    └── RepositoryMetadata 1.0.0
            └── RepositoryIntelligenceSummary 1.0.0
```

## Rules

1. Artifacts are immutable after publication.
2. Engines consume typed artifacts through `ArtifactStore`.
3. Presentation report fields are never engine inputs.
4. A later engine must not re-detect facts already owned by an earlier artifact.
5. An engine must not depend on an unrelated higher-level artifact merely to enforce execution order.
6. Artifact version mismatches fail explicitly; engines never guess a compatible shape.
7. Additive artifact evolution uses a minor version. Semantic contract changes use a major version.

## Current compatibility bridge

Discovery and Ignore still populate `RunContext.Entries` for backward compatibility with the original v0.1–v0.2 orchestration API. Frozen repository-level consumers use `RepositorySnapshot`; new engines must not use mutable entry fields. Removing the compatibility fields is a future pipeline-major-version change.

## Planned extensions

A shared `ManifestInventory` may later become a child of `RepositorySnapshot` and a common input to Framework, Build, and Metadata engines. It is intentionally deferred until repeated bounded manifest parsing creates measurable cost or coupling.

## Phase 2.0 LIE candidate extension

LIE is a consumer layer, not an additional RIE engine. It uses the frozen RIE artifacts through `ArtifactStore` and does not consume `RunContext` or presentation JSON.

```text
RepositorySnapshot 1.0.0 ─┐
                          ├── Go Language Engine
LanguageInventory 1.0.0 ──┘       └── GoLanguageInventory 1.0.0 frozen
```

Future language engines are siblings of Go and publish separate language-specific artifacts. Dependency Intelligence may consume these artifacts only after their 1.0.0 contracts are frozen.

## Phase 2.2 package-identity candidate

Package identity depends on the lowest-level artifacts that contain its required repository and Go syntax facts. It is a prerequisite sibling artifact, not hidden mutable state in the future semantic engine.

```text
RepositorySnapshot 1.0.0 ──────┐
                               ├── Go Package Identity Engine
GoLanguageInventory 1.0.0 ─────┘       └── GoPackageIdentityInventory 0.1.0

RepositorySnapshot 1.0.0 ───────────────┐
GoLanguageInventory 1.0.0 ───────────────┼── Go Semantic Resolution Engine (through Phase 2.2.3 candidate)
GoPackageIdentityInventory 0.1.0 ────────┘       └── GoSemanticInventory 0.1.0 candidate
```

`GoPackageIdentityInventory 0.1.0` is a candidate contract and does not change the frozen RIE or Phase 2.1 artifacts. The semantic candidate now verifies source and publishes reconciled declarations and stable ownership without mutating any prerequisite or persisting AST/scope state. Receiver/type binding and all later relationships remain gated behind Phase 2.2.4 and later approvals.
