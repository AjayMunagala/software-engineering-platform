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

## Package standard

- `interface.go` — public engine contract
- `implementation.go` — extension detector
- `config.go` — extension mappings
- `model.go` — result aliases
- `errors.go` — stable sentinel errors
- `implementation_test.go` — behavior tests
- `implementation_benchmark_test.go` — performance benchmark
