# Go Semantic Artifact Model

## Status

- Design status: Architecture and spike accepted; semantic artifact implementation remains unauthorized until Phase 2.2.2
- Candidate artifact: `go-semantic-inventory` `0.1.0`
- Stable target: `1.0.0` after validation and API freeze
- Prerequisites: `repository-snapshot` `1.0.0`, `go-language-inventory` `1.0.0`, and `go-package-identity-inventory` `0.1.0`

This document defines the conceptual public model. Exact Go spelling is frozen only when the artifact reaches `1.0.0`.

## Design Principles

1. Semantic facts are stored in a separate artifact; the frozen syntax inventory is never expanded or mutated.
2. The artifact references Phase 2.1 file, package, and symbol IDs instead of copying their models.
3. Every relationship has a stable ID, exact source location, and explicit resolution state.
4. Unresolved, ambiguous, external, partial, stale, and failed states are first-class facts.
5. Source text, ASTs, token sets, and `go/types` objects are never persisted.
6. Construction is private, ordering is deterministic, and accessors deep-copy nested collections.
7. An absent result never means the engine silently guessed that the relationship does not exist.

## Artifact Shape

```go
const (
    ArtifactName    = "go-semantic-inventory"
    ArtifactVersion = "0.1.0"
)

type GoSemanticInventory struct {
    metadata              Metadata
    sources               []rie.ArtifactReference
    files                 []SemanticFile
    declarations          []SemanticDeclaration
    references            []SemanticReference
    receivers             []ReceiverBinding
    imports               []ImportBinding
    typeRelations         []TypeRelation
    interfaceSatisfaction []InterfaceSatisfaction
    diagnostics           []lie.Diagnostic
    statistics            SemanticStatistics
}
```

The public artifact implements `rie.Artifact` and `lie.LanguageArtifact`:

```go
func (GoSemanticInventory) ArtifactName() string
func (GoSemanticInventory) ArtifactVersion() string
func (GoSemanticInventory) Language() string
```

## Metadata and Provenance

```go
type Metadata struct {
    Name            string `json:"name"`
    Version         string `json:"version"`
    IDSchemeVersion string `json:"id_scheme_version"`
    EngineName      string `json:"engine_name"`
    EngineVersion   string `json:"engine_version"`
}
```

`SourceArtifacts()` must contain exactly the snapshot, Go syntax inventory, and Go package-identity inventory versions used by the run. Individual semantic facts additionally retain source location and, where useful, shared `rie.Evidence`.

## Common States

```go
type ResolutionStatus uint8

const (
    ResolutionResolved ResolutionStatus = iota + 1
    ResolutionUnresolved
    ResolutionAmbiguous
    ResolutionExternal
    ResolutionPartial
)

type SemanticFileStatus uint8

const (
    SemanticFileResolved SemanticFileStatus = iota + 1
    SemanticFilePartial
    SemanticFileFailed
    SemanticFileStale
    SemanticFileSkipped
)
```

Enums use stable string JSON values. Unknown numeric values are rejected during unmarshalling rather than silently converted.

## Semantic File Outcome

```go
type SemanticFile struct {
    FileID          string             `json:"file_id"`
    PackageID       string             `json:"package_id,omitempty"`
    Status          SemanticFileStatus `json:"status"`
    ContentDigest   string             `json:"content_digest"`
    ReferenceCount  int                `json:"reference_count"`
    UnresolvedCount int                `json:"unresolved_count"`
}
```

`ContentDigest` is the verified digest analyzed by this engine. It must equal the Phase 2.1 digest. A stale file emits no semantic relations derived from changed source.

## Semantic Declarations and Reconciliation

```go
type DeclarationKind uint8

const (
    DeclarationStruct DeclarationKind = iota + 1
    DeclarationInterface
    DeclarationDefinedType
    DeclarationTypeAlias
    DeclarationFunction
    DeclarationMethod
    DeclarationField
    DeclarationParameter
    DeclarationResult
    DeclarationVariable
    DeclarationConstant
    DeclarationLabel
    DeclarationTypeParameter
)

type SemanticDeclaration struct {
    ID             string           `json:"id"`
    SyntaxSymbolID string           `json:"syntax_symbol_id,omitempty"`
    Name           string           `json:"name"`
    FileID         string           `json:"file_id"`
    PackageID      string           `json:"package_id"`
    OwnerDeclarationID string       `json:"owner_declaration_id,omitempty"`
    Kind           DeclarationKind  `json:"kind"`
    TypeDisplay    string           `json:"type_display,omitempty"`
    Location       lie.SourceRange  `json:"location"`
    Status         ResolutionStatus `json:"status"`
}
```

For top-level declarations known to Phase 2.1, `SyntaxSymbolID` proves the reconciliation link to the frozen symbol. Declarations intentionally absent from Phase 2.1—such as named scalar types, aliases, parameters, fields, labels, and type parameters—receive stable semantic IDs and leave `SyntaxSymbolID` empty. `OwnerDeclarationID` identifies a containing semantic declaration when applicable. `TypeDisplay` is normalized presentation text only; consumers must not parse it as a type graph.

## Identifier References

```go
type ReferenceKind uint8

const (
    ReferenceIdentifier ReferenceKind = iota + 1
    ReferenceSelector
    ReferenceType
    ReferenceInstantiation
)

type SemanticReference struct {
    ID               string           `json:"id"`
    Name             string           `json:"name"`
    Kind             ReferenceKind    `json:"kind"`
    FileID           string           `json:"file_id"`
    PackageID        string           `json:"package_id"`
    OwnerDeclarationID      string           `json:"owner_declaration_id,omitempty"`
    Location                lie.SourceRange  `json:"location"`
    Status                  ResolutionStatus `json:"status"`
    TargetDeclarationID     string           `json:"target_declaration_id,omitempty"`
    CandidateDeclarationIDs []string         `json:"candidate_declaration_ids,omitempty"`
    ExternalIdentity        string           `json:"external_identity,omitempty"`
}
```

Rules:

- `TargetDeclarationID` is populated only for exactly resolved repository declarations.
- `CandidateDeclarationIDs` is populated only for a bounded ambiguous result and is sorted.
- `ExternalIdentity` may preserve a proven package-qualified identity without pretending it is a repository symbol.
- A reference is identified by its file, exact source range, kind, and name—not by traversal order.
- Top-level semantic declarations retain their Phase 2.1 syntax-symbol link; locals, parameters, labels, fields, and type parameters exist only in this semantic artifact.

## Receiver Bindings

```go
type ReceiverBinding struct {
    ID                        string           `json:"id"`
    MethodDeclarationID       string           `json:"method_declaration_id"`
    ReceiverTypeDeclarationID string           `json:"receiver_type_declaration_id,omitempty"`
    ReceiverName              string           `json:"receiver_name"`
    Pointer                   bool             `json:"pointer"`
    Generic                   bool             `json:"generic"`
    Location                  lie.SourceRange  `json:"location"`
    Status                    ResolutionStatus `json:"status"`
}
```

Bindings point to the local declared receiver type only when proven. The engine does not synthesize a target from a matching name in another package.

## Import Bindings

```go
type ImportBinding struct {
    ID                     string           `json:"id"`
    FileID                 string           `json:"file_id"`
    ImportPath             string           `json:"import_path"`
    LocalName              string           `json:"local_name,omitempty"`
    AliasKind              string           `json:"alias_kind"`
    Location               lie.SourceRange  `json:"location"`
    Status                 ResolutionStatus `json:"status"`
    TargetPackageID        string           `json:"target_package_id,omitempty"`
    PackageIdentityProofID string           `json:"package_identity_proof_id,omitempty"`
}
```

An import becomes `resolved` only when `PackageIdentityProofID` names a non-stale resolved proof from the consumed identity inventory. A proven standard-library or dependency identity may retain an external proof ID while leaving `TargetPackageID` empty. Unavailable dependencies are `external`; malformed or unprovable local mappings are `unresolved` or `ambiguous`. Blank imports retain the proof link when available but create no selector scope binding. Proof rules are defined in [GO_PACKAGE_IDENTITY_PROOF.md](GO_PACKAGE_IDENTITY_PROOF.md).

## Type Relations and Generics

```go
type TypeRelationKind uint8

const (
    TypeRelationUses TypeRelationKind = iota + 1
    TypeRelationEmbeds
    TypeRelationAliasOf
    TypeRelationInstantiates
    TypeRelationConstrains
)

type TypeRelation struct {
    ID                   string           `json:"id"`
    Kind                 TypeRelationKind `json:"kind"`
    FileID               string           `json:"file_id"`
    PackageID            string           `json:"package_id"`
    OwnerDeclarationID   string           `json:"owner_declaration_id,omitempty"`
    Location             lie.SourceRange  `json:"location"`
    Status               ResolutionStatus `json:"status"`
    TargetDeclarationID  string           `json:"target_declaration_id,omitempty"`
    TargetIdentity       string           `json:"target_identity,omitempty"`
    TypeArgumentText     []string         `json:"type_arguments,omitempty"`
}
```

Type argument text is presentation data. Canonical type identity and source relationships are authoritative.

## Interface Satisfaction

Go interface implementation is normally implicit. A compile-time assertion is evidence, not a different implementation mechanism.

```go
type SatisfactionStatus uint8

const (
    SatisfactionProven SatisfactionStatus = iota + 1
    SatisfactionDisproven
    SatisfactionUnknown
)

type InterfaceSatisfaction struct {
    ID                        string             `json:"id"`
    ConcreteTypeDeclarationID string          `json:"concrete_type_declaration_id"`
    InterfaceDeclarationID    string          `json:"interface_declaration_id"`
    Status                    SatisfactionStatus `json:"status"`
    PointerRequired           bool               `json:"pointer_required"`
    MissingMethodNames        []string           `json:"missing_method_names,omitempty"`
    CompileTimeAssertions     []rie.Evidence     `json:"compile_time_assertions,omitempty"`
}
```

Rules:

- `proven` and `disproven` require complete method sets and comparable signatures.
- Missing imports, incomplete embedded types, or unresolved constraints produce `unknown`, never a guess.
- Missing method names are sorted and emitted only when the result is proven `disproven`.
- The engine does not generate the Cartesian product of every type and every interface. It evaluates bounded candidates derived from actual relationships and assertions.

## Diagnostics and Statistics

```go
type SemanticStatistics struct {
    CandidateFiles          int            `json:"candidate_files"`
    ResolvedFiles           int            `json:"resolved_files"`
    PartialFiles            int            `json:"partial_files"`
    FailedFiles             int            `json:"failed_files"`
    StaleFiles              int            `json:"stale_files"`
    ResolvedDeclarations    int            `json:"resolved_declarations"`
    UnresolvedDeclarations  int            `json:"unresolved_declarations"`
    PartialDeclarations     int            `json:"partial_declarations"`
    ExternalDeclarations    int            `json:"external_declarations"`
    AmbiguousDeclarations   int            `json:"ambiguous_declarations"`
    ReferencesByStatus      map[string]int `json:"references_by_status"`
    ReceiverBindings        int            `json:"receiver_bindings"`
    ImportBindingsByStatus  map[string]int `json:"import_bindings_by_status"`
    TypeRelations           int            `json:"type_relations"`
    InterfaceChecksByStatus map[string]int `json:"interface_checks_by_status"`
    Diagnostics             int            `json:"diagnostics"`
    OmittedDiagnostics      int            `json:"omitted_diagnostics"`
}
```

Statistics are derived from artifact facts and do not duplicate full inventories. `ExternalDeclarations` counts unique external declaration identities referenced by the artifact; external declarations are not copied into the local declaration collection. The other declaration counts classify local semantic declarations by resolution state. Map accessors are deep-copied.

## Stable Identity Rules

Candidate IDs use the same normalized path and byte-offset semantics as `GoLanguageInventory 1.0.0`:

```text
go:semantic:v1:file:<path-byte-length>:<normalized-path>#<start-offset>:<kind>:<name>
go:semantic:v1:relation:<source-id-byte-length>:<source-id>#<relation-kind>#<target-id-byte-length>:<target-id>
```

Lengths are UTF-8 byte lengths, matching Go's `len(string)` and Phase 2.1 byte-offset semantics. Length-prefixing prevents repository paths or nested IDs containing delimiters from making the encoding ambiguous.

Candidate golden vectors established by Phase 2.2.0 include:

```text
go:semantic:v1:file:11:pkg/main.go#11:function:Run
go:semantic:v1:relation:50:go:semantic:v1:file:11:pkg/main.go#11:function:Run#implements#52:go:semantic:v1:file:12:pkg/types.go#10:struct:Worker
```

`Metadata.IDSchemeVersion` is `go-semantic-id/v1` for this candidate. Remaining kind/status encodings must be specified and tested before `1.0.0`. IDs must not depend on worker count, map iteration, diagnostic ordering, or absolute machine paths.

During `0.x`, a re-key increments both the candidate artifact minor version and ID-scheme version. After `1.0.0`, re-keying requires a new artifact major version and scheme prefix. Because this artifact is derived, the canonical migration is a full rebuild; different schemes are never silently mixed.

## Public Accessors

```go
func (GoSemanticInventory) Metadata() Metadata
func (GoSemanticInventory) SourceArtifacts() []rie.ArtifactReference
func (GoSemanticInventory) Files() []SemanticFile
func (GoSemanticInventory) Declarations() []SemanticDeclaration
func (GoSemanticInventory) References() []SemanticReference
func (GoSemanticInventory) ReceiverBindings() []ReceiverBinding
func (GoSemanticInventory) ImportBindings() []ImportBinding
func (GoSemanticInventory) TypeRelations() []TypeRelation
func (GoSemanticInventory) InterfaceSatisfaction() []InterfaceSatisfaction
func (GoSemanticInventory) Diagnostics() []lie.Diagnostic
func (GoSemanticInventory) Statistics() SemanticStatistics
```

The artifact does not expose `LookupSymbol(id) (golang.GoSymbol, bool)`. Consumers retain and query the declared source artifact instead of embedding or copying Phase 2.1 models into the semantic artifact.

## Compatibility Policy

- `0.x` is explicitly a candidate contract and may change after measured implementation findings.
- `1.0.0` freezes field meaning, enum values, ID rules, ordering, position semantics, and immutability behavior.
- `1.0.x` permits compatible defect fixes only.
- Additive optional facts require a minor version.
- Removals or semantic changes require a major version.
