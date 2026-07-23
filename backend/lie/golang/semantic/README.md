# Go Semantic Resolution Engine

## Purpose

Produce a deterministic, immutable semantic artifact from snapshot-authorized,
digest-verified Go source and the frozen Go syntax and package-identity
artifacts.

## Phase 2.2.2 Scope

This milestone implements only:

- candidate engine and artifact contracts;
- configuration and prerequisite validation;
- repository-root and snapshot-path enforcement;
- source size, regular-file, symlink, and SHA-256 verification;
- immutable semantic file outcomes, diagnostics, provenance, and statistics;
- deterministic concurrency, ordering, cancellation, and diagnostic limits.

Verified source is intentionally reported as `partial`. Phase 2.2.2 does not
claim semantic resolution before declaration reconciliation is implemented.

## Inputs

- `RepositorySnapshot 1.0.0`
- `GoLanguageInventory 1.0.0`
- `GoPackageIdentityInventory 0.1.0`

## Output

- `GoSemanticInventory 0.1.0`
- ID scheme `go-semantic-id/v1`

## Boundaries

The engine is local and read-only. It executes no commands, performs no
network access or downloads, reads no module cache, writes no repository
files, and persists no source, AST, token, or `go/types` state.

Declaration reconciliation, scopes, receiver/type binding, references,
imports, type relations, and interface satisfaction remain outside this
milestone. Phase 2.2.3 and later are not authorized by this package.

## Package Standard

This package contains the mandatory interface, implementation, configuration,
models, errors, README, tests, and benchmarks.
