# RIE v0.7 — Repository Intelligence Summary

## Purpose

Repository Intelligence Summary provides one immutable entry point to the important known and unavailable intelligence about a repository without rediscovering or duplicating source artifact data.

## Input

`RepositoryMetadata 1.0.0` is the only required input. It already contains compact synthesized views and versioned references to Discovery, Snapshot, Language, Framework, and Build artifacts.

## Output

`RepositoryIntelligenceSummary 1.0.0` composes the immutable `RepositoryMetadata` value and adds section/capability availability. It does not copy language, framework, build, layout, manifest, file-list, or evidence collections.

```go
type RepositoryIntelligenceSummary struct {
    metadata           ArtifactMetadata
    repositoryMetadata RepositoryMetadata
    sections           []SectionStatus
    capabilities       []CapabilityStatus
}
```

Consumers retrieve the composed metadata through an accessor. The artifact remains immutable and all returned slices are defensively copied.

## Known Sections

- Repository identity and local Git identity
- Filtered repository statistics and top-level layout
- Monorepo, workspace, and declared-module summary
- Languages
- Frameworks and their project locations
- Package managers, build systems, toolchains, and lock-file count
- Exact source-artifact names and versions

## Explicitly Unavailable in v0.7

- Controllers
- Services
- Tests
- Coverage
- Complete cross-engine diagnostic count

These values are marked `unavailable`, not zero. RIE does not guess facts that require code classification, test intelligence, coverage tooling, or a future diagnostic artifact.

## Non-Responsibilities

- Scan files or read manifests.
- Detect or classify technologies.
- Count code structures.
- Execute tests or coverage tools.
- Copy full source inventories into another God Artifact.
- Render PDF, HTML, or another document format.
- Call an LLM or prepare prompts.

## Presentation

The JSON report includes a compact `summary` index containing artifact references and availability states. The existing `metadata` section remains the cover-page data. This avoids repeating the same repository facts twice in one report.

## Performance

Summary construction is `O(s + c)`, where `s` is the fixed number of known sections and `c` is the fixed number of capability statuses. It does not scale with repository file count.

## Freeze Gate

The `1.0.0` artifact freezes only after immutability, missing-input, empty-repository, non-duplication, availability, report, full-suite, vet, and benchmark checks pass.
