# RIE v0.2 — Ignore Engine

Ignore Engine consumes normalized entries from Discovery Engine, loads ordered `.gitignore` files and shared configuration patterns, applies last-match-wins rules including negation, and recomputes repository statistics.

## Inputs

- `RunContext.Entries` from Discovery Engine
- Repository root
- `.gitignore` files at the root or in nested directories
- Shared `IgnorePatterns` and `ScanHidden` configuration

## Outputs

- Filtered entries for later engines
- Final file and folder counts
- Rule/source/ignored-entry summary
- Standardized warnings for unreadable files or unsupported patterns

## Supported patterns

Blank lines, comments, negation (`!`), root anchoring (`/`), directory patterns, `*`, `**`, and `?`. Character classes are reported as warnings and skipped in v0.2.

## Package standard

- `interface.go` — public engine contract
- `implementation.go` — ordered ignore-rule implementation
- `config.go` — engine-owned configuration
- `model.go` — rule and summary models
- `errors.go` — stable sentinel errors
- `implementation_test.go` — behavior tests
- `implementation_benchmark_test.go` — performance benchmark
