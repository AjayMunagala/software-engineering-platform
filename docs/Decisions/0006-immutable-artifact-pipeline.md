# ADR 0006: Immutable Artifact Pipeline

## Status

Accepted for RIE 1.0.0.

## Context

Early RIE stages exchanged repository data through mutable `RunContext` fields. That approach risked accidental coupling, hidden dependencies, and a growing God Object as additional engines were added.

## Decision

Every stable engine output is an immutable, versioned artifact published once through `ArtifactStore`. Engines consume the lowest-level typed artifact containing the facts they require. Presentation report fields are outputs only and are never engine inputs.

The RIE 1.0.0 artifact chain is:

- `DiscoveryInventory 1.0.0`
- `RepositorySnapshot 1.0.0`
- `LanguageInventory 1.0.0`
- `FrameworkInventory 1.0.0`
- `BuildInventory 1.0.0`
- `RepositoryMetadata 1.0.0`
- `RepositoryIntelligenceSummary 1.0.0`

Artifact slices remain private and accessors return defensive copies or immutable values. Artifact version mismatches fail explicitly.

## Consequences

- Engine dependencies are visible and testable.
- Later engines cannot silently mutate earlier facts.
- Summary and metadata layers synthesize compact views without copying complete inventories.
- Artifact evolution must follow semantic versioning.
- Discovery-to-Ignore entry transfer remains a documented internal compatibility bridge until a future pipeline-major-version change.

## Alternatives rejected

- A single mutable pipeline context: creates hidden coupling and weak ownership.
- One giant repository artifact: forces unrelated consumers to depend on unstable data.
- Presentation JSON as an engine contract: couples computation to serialization.
- LLM-based inference between stages: violates deterministic evidence requirements.
