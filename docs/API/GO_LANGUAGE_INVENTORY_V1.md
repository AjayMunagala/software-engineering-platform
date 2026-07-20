# GoLanguageInventory Public API 1.0

## Status

Frozen on 2026-07-20. Artifact identity:

```text
name:    go-language-inventory
version: 1.0.0
language: Go
```

The Git release tag is `go-language-inventory-v1.0.0`. The namespaced tag is
required because the repository-level `v1.0.0` tag already identifies RIE.

## Construction and Retrieval

```go
func golang.DefaultConfig() golang.Config
func golang.New(configs ...golang.Config) (lie.Engine, error)
func golang.InventoryFrom(*rie.ArtifactStore) (golang.GoLanguageInventory, bool)
```

`GoLanguageInventory` has no public constructor. Only the Go engine creates a
valid artifact, and `ArtifactStore` publishes it exactly once by artifact name.

## Artifact Accessors

```go
func (GoLanguageInventory) ArtifactName() string
func (GoLanguageInventory) ArtifactVersion() string
func (GoLanguageInventory) Language() string
func (GoLanguageInventory) Metadata() Metadata
func (GoLanguageInventory) SourceArtifacts() []rie.ArtifactReference
func (GoLanguageInventory) Files() []GoFile
func (GoLanguageInventory) Packages() []GoPackage
func (GoLanguageInventory) Symbols() []GoSymbol
func (GoLanguageInventory) Diagnostics() []lie.Diagnostic
func (GoLanguageInventory) Statistics() ParseStatistics
```

All collection accessors return defensive copies, including nested imports,
package file IDs, diagnostic locations, and statistics maps.

## Frozen Models

- `Config`: `MaxWorkers`, `MaxSourceFileSize`, `MaxDiagnostics`, `IncludeTests`.
- `GoFile`: identity, path, package, status, test flag, byte size, SHA-256
  content digest, and imports.
- `GoPackage`: deterministic identity, name, directory, and file IDs.
- `GoImport`: path, alias, alias kind, and exact location.
- `GoSymbol`: deterministic identity, kind, name, ownership, export status,
  receiver facts, and exact location.
- `ParseStatistics`: candidate/parsed/failed/skipped files, parsed bytes,
  package/import counts, symbols by kind, and diagnostic counts.
- `Metadata`: artifact and producing-engine identity.

Frozen enums are `FileStatus`, `ImportAliasKind`, and `SymbolKind`. Exact Go
field spelling is defined by the source at the release tag.

## Compatibility Policy

- Patch releases (`1.0.x`) contain compatible defect fixes only.
- Additive optional facts require a minor release.
- Changed ID rules, ordering, position semantics, field meanings, or removals
  require a major release.
- Type resolution, dependency graphs, and semantic relationships belong to
  separate artifacts and will not expand this artifact into a God object.
