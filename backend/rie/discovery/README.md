# RIE v0.1 — Discovery Engine

This package provides the first capability of the Repository Intelligence Engine: a read-only local repository inventory.

## Contract

Input: a readable directory path.

Output: immutable `DiscoveryInventory 1.0.0` plus a schema-versioned report containing scan identity/timing, repository name, cleaned root path, file count, folder count, Git presence, metrics, warnings, and errors. The `.git` directory is detected but excluded from file and folder counts.

`DiscoveryInventory` records repository identity, Git presence, and symbolic current/default branches when available from bounded local Git metadata reads. It never executes Git and never infers a default branch when `origin/HEAD` is unavailable.

The engine exposes `Name`, `Version`, `Description`, and `Execute`, allowing it to run in an ordered data-driven RIE pipeline.

## Package standard

- `interface.go` — public engine contract
- `implementation.go` — filesystem implementation
- `config.go` — engine-owned configuration
- `model.go` — public and shared model aliases
- `errors.go` — stable sentinel errors
- `implementation_test.go` — behavior tests
- `implementation_benchmark_test.go` — performance benchmark

## Constraints

The package never changes files, executes repository code, or accesses the network. Later RIE releases add language, framework, build, and metadata detection as separate packages.
