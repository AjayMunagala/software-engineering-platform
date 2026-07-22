# Go Package Identity Engine

## Purpose

Deterministically prove how Go import paths map to repository packages using
only snapshot-authorized local manifests and the frozen Go syntax inventory.

## Inputs

- `RepositorySnapshot 1.0.0`
- `GoLanguageInventory 1.0.0`

## Output

- `GoPackageIdentityInventory 0.1.0`
- ID scheme `go-package-proof-id/v1`

## Boundaries

The engine is local and read-only. It uses `golang.org/x/mod/modfile` as an
in-process parser for `go.mod` and `go.work`. It does not execute Go commands,
load packages, inspect the ambient module cache, access the network, or modify
repository files.

## Candidate Behavior

- Preserves single-module, workspace, vendor, and unmanaged contexts.
- Treats nested modules as hard ownership boundaries.
- Applies workspace replacement precedence before module replacements.
- Requires selected/required module facts before a replacement is active.
- Produces explicit resolved, external, unresolved, ambiguous, and stale states.
- Records exact manifest digests and source ranges for every proof step.
- Records immutable evidence on managed resolution contexts so their selection is explainable.

Standard-library proof requires a future explicit exact-version index. The
candidate never guesses standard-library identity from import-path shape.

## Package Standard

This package contains the mandatory interface, implementation, configuration,
models, errors, README, tests, and benchmarks.

## Validation

Targeted tests, ten shuffled runs, full backend regression, vet, and 87.8%
statement coverage pass on Go 1.26.2/Windows amd64. A 10,000-proof rebuild
measures 15.56–19.24 ms in the local benchmark fixture. Targeted package and
full-backend race tests pass with MSYS2 UCRT64 GCC 16.1.0.

Passing local validation completes the implementation evidence for Phase
2.2.1 only. Phase 2.2.2 is not authorized by this package.
