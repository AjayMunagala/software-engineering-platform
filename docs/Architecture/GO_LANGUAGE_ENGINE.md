# Go Language Engine

## Status

Released as Go Language Engine 1.0.0. The Phase 2.1 public artifact contract is frozen.

## Exact Responsibility

The Go Language Engine deterministically parses authorized `.go` files and inventories packages, imports, structs, interfaces, functions, methods, constants, and variables with exact source evidence.

## In Scope

- Case-insensitive `.go` source selection from `RepositorySnapshot`.
- Go package clause discovery.
- Regular and external-test package grouping.
- Import path and explicit alias extraction.
- Named struct and interface declarations.
- Package functions and receiver methods.
- Package-level constants and variables.
- Test-file identification through the `_test.go` suffix.
- File byte count and SHA-256 digest.
- Exact file, byte-offset, line, and byte-column ranges.
- Structured parse and file diagnostics.

Custom RIE mappings that label non-`.go` extensions as Go are not accepted by Phase 2.1. A candidate-count mismatch with `LanguageInventory` is a fatal consistency error rather than permission to guess which extra files are Go source.

## Out of Scope

- Local variables inside functions.
- Struct fields and embedded fields.
- Interface method sets and type terms.
- Function parameters, results, bodies, complexity, or documentation.
- Named scalar types, aliases, and generic constraint semantics.
- Identifier resolution, type checking, method sets, or package loading.
- Import graph, call graph, control flow, or data flow.
- Build-tag, GOOS, GOARCH, cgo, or vendor selection semantics.
- Generated-code identification.
- Running `go list`, `go env`, `go test`, `go vet`, or any external tool.

## Parser Strategy

Use the Go standard library:

```text
go/parser  -> syntax tree
go/ast     -> deterministic declaration traversal
go/token   -> byte offsets, lines, and columns
```

Use `parser.SkipObjectResolution`. Phase 2.1 does not need deprecated AST object resolution. Do not use `go/packages` or `go/types` because they introduce module loading and semantic responsibilities outside this milestone.

The engine parses every selected file independently. Build constraints are recorded only as source text in future versions; they are not evaluated in Phase 2.1.

## Package Discovery

A package is identified by normalized directory plus parsed package name:

```text
go:package:<directory>#<package-name>
```

Files named `*_test.go` are marked as tests. A package named `<name>_test` remains an independent external-test package in the same directory. Invalid mixed package declarations are represented as separate factual groups; syntax failures produce diagnostics.

## Import Extraction

For each valid import spec, record:

- Unquoted import path.
- Explicit alias when present.
- Alias kind: default, named, blank (`_`), or dot (`.`).
- Exact source range of the import spec.
- Owning file and package IDs.

Imports are declarations only. The engine does not decide whether an import is standard-library, external, available, used, or resolvable.

## Declaration Extraction

Each declared name becomes one symbol:

| Syntax | Symbol kind | Notes |
|---|---|---|
| `type T struct {}` | `struct` | Named structs only |
| `type T interface {}` | `interface` | Named interfaces only |
| `func F()` | `function` | No receiver |
| `func (t T) M()` | `method` | Base receiver name and pointer flag recorded |
| `const A, B = ...` | `constant` | One symbol per declared name |
| `var A, B = ...` | `variable` | Package scope only; one symbol per name |

`Exported` is computed with `ast.IsExported`. Declaration bodies and literal values are not stored.

## Receiver Model

Methods record:

- Base receiver type name when it can be extracted structurally.
- Whether the receiver is a pointer.
- Whether the receiver expression uses generic indexing.

If a syntactically valid receiver shape cannot be reduced safely, the symbol remains a method with an empty receiver base and a diagnostic. The engine never invents a receiver name.

## File Outcomes

Every selected candidate produces one `GoFile` record:

- `parsed`: syntax accepted and facts emitted.
- `failed`: read, containment, digest, or parse failure; no facts emitted.
- `skipped`: configured size limit exceeded; no facts emitted.

An empty repository or repository without Go produces an empty `GoLanguageInventory` without warnings or errors.

## Source Consistency

`RepositorySnapshot` authorizes paths but does not freeze file bytes. The Go engine therefore computes `sha256:<hex>` over the exact bytes passed to `go/parser` and stores the digest and byte count in `GoFile`. Downstream consumers can verify that later filesystem content still matches the parsed artifact.

The runner should execute promptly after RIE. Concurrent repository mutation cannot be prevented and must not be silently treated as a stable content snapshot.

## Error and Diagnostic Codes

| Code | Severity | Behavior |
|---|---|---|
| `go_source_missing` | warning | File record failed; continue |
| `go_source_unreadable` | warning | File record failed; continue |
| `go_source_oversized` | warning | File record skipped; continue |
| `go_source_outside_root` | error | File record failed; continue and audit |
| `go_parse_error` | warning | File record failed; no partial facts |
| `go_receiver_unsupported` | warning | Method retained without guessed receiver base |
| `go_diagnostic_limit` | warning | Further local diagnostics counted but omitted |

Missing prerequisites, invalid configuration, cancellation, and publication conflicts remain fatal runner errors.

## Configuration

```text
MaxWorkers          default min(logical CPUs, 8)
MaxSourceFileSize   default 10 MiB
MaxDiagnostics      default 1,000
IncludeTests        default true
```

`IncludeTests=false` may exclude `_test.go` candidates. It does not run Go build selection.

## Package Structure

Implementation must use:

```text
backend/lie/
    interface.go
    implementation.go
    config.go
    model.go
    errors.go
    README.md
    implementation_test.go
    implementation_benchmark_test.go

backend/lie/golang/
    interface.go
    implementation.go
    config.go
    model.go
    errors.go
    README.md
    implementation_test.go
    implementation_benchmark_test.go
```

Additional files require architecture review; package responsibilities must not be hidden in a single large implementation.

## Acceptance Tests

- Empty repository and repository with no Go.
- One package and multiple packages.
- Regular and external-test packages in one directory.
- Single, grouped, aliased, blank, and dot imports.
- Structs and interfaces.
- Functions and value, pointer, and generic receiver methods.
- Grouped and individual constants and variables.
- Exported and unexported names.
- Uppercase `.GO` selection consistent with RIE.
- Malformed, missing, unreadable, oversized, and escaping source files.
- Deterministic results with one worker and multiple workers.
- Artifact immutability and content-digest verification.
- Cancellation and diagnostic-limit behavior.

## Freeze Gate

Passed on 2026-07-20 after unit tests, benchmarks, real-repository validation,
public API review, and an immutability audit. `GoLanguageInventory 1.0.0` is
frozen; compatible `1.0.x` changes are limited to defect fixes.
