# ADR 0007: Go Standard Library Parser for Go Language Intelligence

## Status

Proposed for Phase 2.1 approval.

## Context

LIE needs exact Go packages, imports, declarations, and source positions without executing a toolchain command, resolving modules, or adding an unnecessary runtime dependency. ADR 0003 proposes Tree-sitter as a common multi-language syntax layer and explicitly allows language-native parsers where deeper or more accurate language support is appropriate.

## Decision

Use the Go standard library packages `go/parser`, `go/ast`, and `go/token` for the Go Language Engine.

Parse each authorized `.go` file independently with `parser.SkipObjectResolution`. Extract only the syntax facts approved for Phase 2.1. Do not use `go/packages`, `go/types`, `go list`, compiler execution, or network-backed module resolution.

## Rationale

- It is the language-native parser maintained with Go.
- It provides exact Go syntax nodes and token positions.
- It adds no third-party dependency or native runtime.
- It parses source without executing repository code.
- It supports deterministic, file-local operation.
- It keeps syntax inventory separate from future type and dependency analysis.

## Relationship to ADR 0003

This decision refines rather than rejects ADR 0003. Tree-sitter remains a candidate shared strategy for languages without a suitable native parser and for future cross-language tooling. Go uses its native parser because it provides the smallest, most accurate dependency for the approved scope.

## Consequences

- Go syntax support follows the Go version used to build Aegis CodeMind.
- Newer unsupported syntax produces an explicit parse diagnostic.
- Build selection, type information, and resolved imports are unavailable.
- Other language engines may use different approved parser adapters while producing artifacts that follow shared LIE evidence and immutability rules.
- Parser output remains internal; consumers depend on `GoLanguageInventory`, not `go/ast` nodes.

## Alternatives Considered

### Tree-sitter Go

Rejected for Phase 2.1 because it adds a grammar/runtime dependency without improving the required Go syntax facts. It remains viable for future cross-language features.

### `go/packages`

Rejected because it can load packages, inspect build configuration, and invoke Go tooling. Those behaviors exceed the read-only syntax milestone.

### `go/types`

Deferred to a separate semantic intelligence stage. Type checking requires import resolution and substantially changes the artifact contract and failure model.

### Text or regular-expression parsing

Rejected because it cannot reliably represent Go syntax and violates the evidence-first correctness standard.
