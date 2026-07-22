# ADR 0009: Use `golang.org/x/mod` for Go Module Manifests

- Status: Accepted
- Date: 2026-07-22
- Scope: Phase 2.2.1 Go Package Identity Engine

## Context

Package identity requires correct parsing of `go.mod` and `go.work`, including grouped directives, quoted values, comments, versions, replacements, workspace uses, and source positions. A handwritten subset parser would duplicate Go grammar and could silently produce incorrect identity proofs.

## Decision

Use the Go-maintained `golang.org/x/mod/modfile` package, pinned in `backend/go.mod`, to parse module and workspace manifests.

The dependency is an in-process parser. The engine does not invoke the Go command, access the network, download modules, or inspect the ambient module cache at runtime. Manifest bytes come only from paths authorized by `RepositorySnapshot`.

`vendor/modules.txt` remains a small deterministic package-owned parser because `x/mod/modfile` does not expose the vendor manifest as the same public syntax model.

## Alternatives

### Handwritten `go.mod`/`go.work` parser

Rejected because it would recreate evolving Go grammar, weaken source-range accuracy, and increase false proof risk.

### Execute `go env`, `go list`, or `go work edit -json`

Rejected because execution and environment-dependent module loading violate the engine boundary.

### Use `go/packages`

Rejected because it is a package loader rather than a manifest parser and may invoke the Go toolchain.

## Consequences

- Go manifest syntax follows the official maintained parser.
- The dependency version is explicit and reviewable.
- Dependency updates require tests against all package-identity fixtures.
- Runtime behavior remains local, read-only, command-free, and network-free.
