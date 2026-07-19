# RIE v0.6 — Repository Metadata Engine

Repository Metadata Engine produces the repository's executive cover page by synthesizing frozen artifacts. It does not rescan manifests, redetect technologies, or execute external tools.

## Required artifacts

- `DiscoveryInventory 1.0.0` — repository and local Git identity
- `RepositorySnapshot 1.0.0` — filtered layout and statistics
- `LanguageInventory 1.0.0` — language summary
- `FrameworkInventory 1.0.0` — framework summary and locations
- `BuildInventory 1.0.0` — build, package, workspace, lock-file, and toolchain facts

## Output

`RepositoryMetadata 1.0.0` includes identity, Git branches when locally available, layout, monorepo classification, workspace and declared-module counts, languages, frameworks, build summary, and exact source-artifact versions.

A repository with no detected languages, frameworks, or build systems produces a valid empty summary without warnings.

## Package standard

- `interface.go` — engine and artifact retrieval contracts
- `implementation.go` — artifact-only synthesis
- `config.go` — monorepo classification thresholds
- `model.go` — immutable executive metadata artifact
- `errors.go` — stable prerequisite and configuration errors
- `implementation_test.go` — synthesis, prerequisite, and immutability tests
- `implementation_benchmark_test.go` — isolated 100,000-entry benchmark
