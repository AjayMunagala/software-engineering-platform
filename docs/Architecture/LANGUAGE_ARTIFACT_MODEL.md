# LIE Artifact Model

## Status

Candidate contract for Phase 2.1. Field names illustrate the approved concepts; implementation review may refine Go spelling without changing their meaning.

## Design Principles

1. Artifacts contain facts, not presentation summaries.
2. Every fact identifies its exact source location.
3. Parsed bytes are attributable through a content digest.
4. IDs are deterministic and opaque to consumers.
5. Collections are immutable after publication.
6. Partial file failures are explicit and never converted into guessed facts.
7. Language-specific facts stay in language-specific artifacts.

## Shared LIE Contracts

```go
type Position struct {
    Offset int // zero-based byte offset
    Line   int // one-based
    Column int // one-based byte column
}

type SourceRange struct {
    File  string // normalized repository-relative path
    Start Position
    End   Position
}

type Diagnostic struct {
    Engine   string
    Severity Severity
    Code     string
    Message  string
    Location *SourceRange
}

type LanguageArtifact interface {
    rie.Artifact
    Language() string
}
```

Ranges use an inclusive start and exclusive end. Columns are byte columns because `go/token` reports byte positions. Consumers must not reinterpret them as Unicode code-point or display columns.

## GoLanguageInventory

```go
type GoLanguageInventory struct {
    metadata    Metadata
    sources     []rie.ArtifactReference
    files       []GoFile
    packages    []GoPackage
    symbols     []GoSymbol
    diagnostics []lie.Diagnostic
    statistics  Statistics
}
```

Candidate artifact identity:

```text
Name:    go-language-inventory
Version: 0.1.0 during implementation
Target:  1.0.0 after Phase 2.1 stabilization
```

Source references must include:

- `repository-snapshot 1.0.0`
- `language-inventory 1.0.0`

## GoFile

```go
type GoFile struct {
    ID            string
    Path          string
    PackageID     string
    PackageName   string
    Status        FileStatus
    IsTest        bool
    SizeBytes     int64
    ContentDigest string
    Imports       []GoImport
}
```

`ContentDigest` uses `sha256:<lowercase hex>`. Failed or skipped files have no digest unless bytes were successfully read and hashed before the later failure.

## GoPackage

```go
type GoPackage struct {
    ID        string
    Name      string
    Directory string
    FileIDs   []string
}
```

Package identity includes directory and declared package name. The artifact does not claim a module import path; that belongs to later dependency intelligence.

## GoImport

```go
type GoImport struct {
    Path      string
    Alias     string
    AliasKind ImportAliasKind
    Location  lie.SourceRange
}
```

Imports are nested under the owning file to avoid duplicating file and package ownership. A future dependency artifact may create resolved edges from these declarations.

## GoSymbol

```go
type GoSymbol struct {
    ID               string
    Kind             SymbolKind
    Name             string
    PackageID        string
    FileID           string
    Exported         bool
    ReceiverBase     string
    PointerReceiver  bool
    GenericReceiver  bool
    Location         lie.SourceRange
}
```

Initial symbol kinds:

```text
struct
interface
function
method
constant
variable
```

Methods use receiver fields; all other symbol kinds leave them empty or false.

## Statistics

```go
type Statistics struct {
    CandidateFiles  int
    ParsedFiles     int
    FailedFiles     int
    SkippedFiles    int
    ParsedBytes     int64
    Packages        int
    Imports         int
    SymbolsByKind   map[SymbolKind]int
    Diagnostics     int
    OmittedDiagnostics int
}
```

Map accessors return copies. A zero-source run returns zero values and non-nil empty collections in presentation output.

## Deterministic IDs

IDs are stable for unchanged facts in the same repository layout:

```text
File:    go:file:<normalized-path>
Package: go:package:<directory>#<package-name>
Symbol:  go:symbol:<normalized-path>#<start-offset>:<kind>:<name>
```

IDs are opaque. Consumers must not parse them to recover fields. Moving a file or declaration intentionally changes its ID.

## Immutability

- Struct fields containing slices or maps remain private.
- Accessors return defensive copies.
- Visitor APIs may expose value copies for allocation-sensitive traversal.
- Nested imports and package file IDs are copied recursively.
- `ArtifactStore.Put` remains single-assignment by artifact name.
- No method mutates an artifact after construction.

## Evolution Rules

- Additive optional facts may increment the artifact minor version.
- Changed meanings, ID rules, ordering guarantees, or source-position semantics require a major version.
- Presentation schemas may evolve separately and are never engine inputs.
- Type checking, resolved dependencies, call graphs, and architecture facts require separate artifacts rather than expanding `GoLanguageInventory` into a God artifact.
