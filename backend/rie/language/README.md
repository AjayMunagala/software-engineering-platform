# RIE v0.3 — Language Engine

Language Engine consumes entries retained by Ignore Engine and detects programming languages from lowercase file extensions only. It does not inspect file contents, parse syntax, invoke Tree-sitter, or use AI.

## Supported languages

- Go: `.go`
- TypeScript: `.ts`, `.tsx`
- JavaScript: `.js`, `.jsx`
- Python: `.py`
- Java: `.java`
- C#: `.cs`
- SQL: `.sql`

## Output

The versioned report records detected files, unknown files, and each language's file count and percentage of recognized language files. Results are sorted by descending count and then by name.

Future engines consume the immutable `LanguageInventory` artifact, not the JSON-oriented report summary. The artifact contract is version `1.0.0`; its item slice is private and returned only through defensive-copy accessors.

## Unknown-file policy

- Unknown extensions and files without extensions are counted.
- Unknown files are not treated as errors.
- Unknown files do not generate diagnostics.
- Future engines may classify them later.

## Package standard

- `interface.go` — public engine contract
- `implementation.go` — extension detector
- `config.go` — extension mappings
- `model.go` — result aliases
- `errors.go` — stable sentinel errors
- `implementation_test.go` — behavior tests
- `implementation_benchmark_test.go` — performance benchmark
