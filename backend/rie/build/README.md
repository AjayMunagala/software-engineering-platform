# RIE v0.5 — Build & Package Intelligence Engine

The engine consumes immutable `RepositorySnapshot 1.0.0` and deterministically inventories package managers, build systems, workspaces, lock files, and explicitly declared toolchain constraints. It is local, read-only, and never executes external tools.

## Contract

- `BuildInventory 1.0.0` is technology-neutral and immutable to consumers.
- Every item has normalized project location and structured `rie.Evidence`.
- An unsupported or documentation-only repository produces a valid empty inventory without diagnostics.
- Multiple valid tools are retained. Conflicting Node package-manager evidence produces a warning and no guessed winner.
- `go.mod` evidences Go Modules and the Go toolchain; Cargo is independently represented in package-manager and build-system roles.

## Registry

Detection uses the in-code `[]Detector` registry. Common tool support is added with a detector entry and tests. There is no JSON registry or remote configuration.

## Package standard

- `interface.go` — engine, detector, and artifact retrieval contracts
- `implementation.go` — orchestration, bounded readers, and deterministic detectors
- `config.go` — size limit and detector registry
- `model.go` — immutable inventory and technology-neutral models
- `errors.go` — stable sentinel errors
- `implementation_test.go` — behavior and contract tests
- `implementation_benchmark_test.go` — isolated 100,000-entry benchmark
