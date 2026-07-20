# Known Limitations

Version 1.0.0 intentionally does not provide:

- Local-variable discovery.
- Struct fields, embedded fields, tags, or interface member details.
- Function parameters, results, bodies, complexity, or documentation.
- Named scalar types, aliases, or generic constraint semantics.
- Identifier or import resolution, type checking, or package loading.
- Method sets or interface implementation discovery.
- Import, call, control-flow, data-flow, or dependency graphs.
- Build-tag, GOOS, GOARCH, cgo, vendor, or generated-file selection semantics.
- Partial facts from malformed files.
- Go command, compiler, test, vet, network, or dependency execution.

Cold-cache performance depends heavily on the operating system, storage, and
filesystem filter drivers. It is recorded separately from the approved warm-cache
engine benchmark.

These are explicit scope boundaries, not hidden defects. Semantic capabilities
belong to Phase 2.2 and separate versioned artifacts.
