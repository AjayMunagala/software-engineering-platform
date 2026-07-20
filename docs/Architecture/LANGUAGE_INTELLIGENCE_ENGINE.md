# Language Intelligence Engine (LIE)

## Status

Phase 2.0 design candidate. Implementation requires architecture approval.

## Purpose

The Language Intelligence Engine deterministically converts authorized source files into immutable, evidence-backed language artifacts without executing repository code or using AI.

## Responsibilities

- Consume frozen repository facts rather than rediscovering repository structure.
- Select source files for registered language engines from `RepositorySnapshot`.
- Verify the corresponding language is represented by `LanguageInventory`.
- Read bounded source files inside the authorized repository root.
- Parse syntax with an approved language parser.
- Extract packages, imports, declarations, and exact source locations.
- Hash the exact bytes used to create each file result.
- Publish immutable, versioned language artifacts.
- Produce bounded, structured diagnostics for unsupported or failed files.
- Preserve deterministic ordering regardless of worker scheduling.

## Non-Responsibilities

- Type checking, name resolution, or semantic compilation.
- Call graphs, control flow, data flow, or dependency graphs.
- Framework, architecture, database, Git-history, test-quality, or coverage analysis.
- Build-tag evaluation or determining which files a real build selects.
- Executing `go`, package managers, compilers, builds, tests, or repository scripts.
- Downloading modules or resolving dependencies online.
- LLM calls, bug detection, reasoning, code generation, or editing.
- Replacing or modifying any frozen RIE artifact.

## Engine Boundaries

LIE is a new consumer layer. It does not become another RIE stage and does not extend `rie.RunContext`.

```text
RepositorySnapshot 1.0.0 ─┐
                          ├── LIE Runner
LanguageInventory 1.0.0 ──┘       │
                                  ├── Go Language Engine
                                  │       └── GoLanguageInventory
                                  ├── TypeScript Language Engine (future)
                                  ├── SQL Language Engine (future)
                                  └── Other language engines (future)
```

Each language engine is independently registered and removable. Removing one engine prevents only its artifact from being produced.

Every engine declares a unique engine name, language, and output artifact name before execution so the runner can reject registry and store conflicts before reading source files.

## Inputs

### RepositorySnapshot 1.0.0

Provides the authorized root and ignore-filtered repository-relative entries. LIE treats it as read-only. Only non-directory entries selected by a registered language engine may be read.

### LanguageInventory 1.0.0

Provides RIE's deterministic language summary. LIE uses it as a prerequisite and consistency check, not as a source-file index. The Go engine owns the narrow rule that a case-insensitive `.go` suffix selects a Go source candidate.

This limited selection rule is intentional. `LanguageInventory 1.0.0` contains aggregate counts rather than per-file mappings, and RIE must not be changed merely to pass mutable file lists to LIE.

## Outputs

Every language engine publishes one language-specific `LanguageArtifact`. Phase 2.1 produces `GoLanguageInventory`.

Output guarantees:

- Artifact identity and schema are versioned.
- Collections are private and returned through defensive-copy or visitor APIs.
- Files, packages, imports, symbols, and diagnostics have deterministic ordering.
- Every parsed file records its repository-relative path, byte size, and SHA-256 content digest.
- Every extracted fact has an exact source range.
- An empty source set produces a valid empty artifact without warnings.
- A file failure produces a diagnostic and explicit file status; facts are not fabricated.

## Dependencies

- Frozen RIE artifact contracts: authorized repository input and language prerequisite.
- Existing `rie.Artifact` and `rie.ArtifactStore`: versioned publication without a second artifact infrastructure.
- Go standard library: orchestration, bounded file I/O, hashing, sorting, and concurrency.
- Language-native parser selected by an ADR. The Go engine uses `go/parser`, `go/ast`, and `go/token`.

LIE has no network, database, LLM, Tree-sitter, compiler-command, or package-manager dependency in Phase 2.1.

## Public API

The candidate API is defined in `docs/API/LIE_PUBLIC_API.md`. Its central operation is:

```go
runner.Run(ctx, artifacts)
```

The runner retrieves the two frozen RIE inputs, invokes registered language engines, and publishes successful language artifacts into the supplied store. It never consumes RIE presentation fields.

## Internal Components

```text
LIE Runner
├── Prerequisite Resolver
├── Engine Registry
├── Source Boundary Validator
├── Diagnostic Collector
├── Deterministic Publisher
└── Language Engines
    └── Go
        ├── Source Selector
        ├── Bounded Reader and Hasher
        ├── Go Parser
        ├── Package Grouper
        ├── Import Extractor
        ├── Declaration Extractor
        └── Inventory Builder
```

The runner owns orchestration only. Syntax and extraction rules remain inside the language package.

## Error Handling

Fatal errors stop the LIE run:

- Missing or incompatible prerequisite artifacts.
- Invalid configuration.
- Duplicate engine language or artifact identity.
- Artifact publication conflict.
- Context cancellation.
- Repository root boundary validation failure affecting the entire input.

File-local conditions do not stop unrelated files:

- File disappeared after RIE.
- File is unreadable.
- File exceeds the configured size limit.
- File path resolves outside the authorized root.
- Parser rejects the file.

Each local condition produces a bounded diagnostic and a non-success file status. No declarations are emitted from a file with a parse error in Phase 2.1.

## Logging and Audit

LIE records engine metadata, artifact versions, source counts, byte counts, durations, and diagnostic codes. It must not log source contents, comments, string literals, secrets, or absolute paths in portable artifacts. Diagnostics use repository-relative paths.

## Configuration

Initial shared limits:

- `MaxWorkers`: logical CPU count capped at 8.
- `MaxSourceFileSize`: 10 MiB.
- `MaxDiagnostics`: 1,000 per language engine.

Language-specific options belong to the language package. Defaults must be deterministic and safe. Unbounded modes are not allowed.

## Determinism

Engines may parse files concurrently, but output is sorted before publication:

- Files by normalized repository-relative path.
- Packages by directory then package name.
- Imports by file, source offset, then import path.
- Symbols by file, source offset, kind, then name.
- Diagnostics by file, source offset, code, then message.

Timestamps and elapsed time may differ between runs; functional artifacts must not.

## Testing Strategy

- Unit tests for source selection, containment, IDs, ranges, sorting, limits, and diagnostics.
- Parser fixtures for every supported declaration and import form.
- Negative fixtures for malformed, unreadable, oversized, missing, and path-escape files.
- Immutability tests for every collection accessor.
- Determinism tests across worker counts and repeated runs.
- Integration tests using frozen RIE artifacts and multi-package Go repositories.
- Benchmarks for small files, large files, 1,000 files, and 10,000 files.
- Real-world validation against pinned Go repositories before artifact 1.0.0 freeze.

## Performance Targets

- Linear work in total selected source bytes plus produced syntax facts.
- Parse 10,000 Go files or 100 MiB of authorized Go source in under 30 seconds on supported developer hardware.
- Default worker count never exceeds 8.
- Peak memory target below 1 GiB for the reference workload.
- File contents are released after extraction and are never retained in the artifact.
- Cancellation is checked before each read, parse, and extraction stage.

These are acceptance targets to measure, not unverified guarantees.

## Security and Privacy

- Local and read-only.
- Treat every path and source byte as untrusted input.
- Resolve and validate every candidate beneath the authorized root before reading.
- Do not follow a source path outside the root through a symbolic link.
- Enforce file-size and diagnostic limits before allocating unbounded data.
- Do not execute build constraints, generators, directives, imports, or code.
- Do not persist raw source text in artifacts.
- Use content hashes and source locations as evidence without exposing contents.

## Future Extensions

- Additional language engines using the same runner contract.
- Optional content-addressed source cache after measured need.
- Type and semantic artifacts as separate later stages.
- Incremental parsing after a stable file-change artifact exists.
- Tree-sitter adapters for languages where native parsers are unavailable or insufficient.

None of these extensions belongs in Phase 2.1.
