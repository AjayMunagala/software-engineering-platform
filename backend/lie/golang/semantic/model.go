package semantic

import (
	"encoding/json"
	"fmt"

	"github.com/AjayMunagala/software-engineering-platform/backend/lie"
	"github.com/AjayMunagala/software-engineering-platform/backend/lie/golang"
	"github.com/AjayMunagala/software-engineering-platform/backend/lie/golang/packageidentity"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
)

const (
	// ArtifactName is the candidate semantic artifact identity.
	ArtifactName = "go-semantic-inventory"
	// ArtifactVersion is the current candidate contract version.
	ArtifactVersion = "0.1.0"
	// IDSchemeVersion identifies the candidate stable-ID algorithm.
	IDSchemeVersion = "go-semantic-id/v1"
	engineVersion   = "0.1.0"
)

// ResolutionStatus describes whether one semantic target is proven.
type ResolutionStatus uint8

const (
	ResolutionResolved ResolutionStatus = iota + 1
	ResolutionUnresolved
	ResolutionAmbiguous
	ResolutionExternal
	ResolutionPartial
)

func (status ResolutionStatus) String() string {
	switch status {
	case ResolutionResolved:
		return "resolved"
	case ResolutionUnresolved:
		return "unresolved"
	case ResolutionAmbiguous:
		return "ambiguous"
	case ResolutionExternal:
		return "external"
	case ResolutionPartial:
		return "partial"
	default:
		return "unknown"
	}
}

func (status ResolutionStatus) MarshalJSON() ([]byte, error) {
	return marshalKnownEnum("resolution status", status.String())
}

func (status *ResolutionStatus) UnmarshalJSON(data []byte) error {
	value, err := unmarshalEnum(data)
	if err != nil {
		return err
	}
	switch value {
	case "resolved":
		*status = ResolutionResolved
	case "unresolved":
		*status = ResolutionUnresolved
	case "ambiguous":
		*status = ResolutionAmbiguous
	case "external":
		*status = ResolutionExternal
	case "partial":
		*status = ResolutionPartial
	default:
		return fmt.Errorf("unknown resolution status: %s", value)
	}
	return nil
}

// SemanticFileStatus describes the safe outcome for one syntax-inventory file.
type SemanticFileStatus uint8

const (
	SemanticFileResolved SemanticFileStatus = iota + 1
	SemanticFilePartial
	SemanticFileFailed
	SemanticFileStale
	SemanticFileSkipped
)

func (status SemanticFileStatus) String() string {
	switch status {
	case SemanticFileResolved:
		return "resolved"
	case SemanticFilePartial:
		return "partial"
	case SemanticFileFailed:
		return "failed"
	case SemanticFileStale:
		return "stale"
	case SemanticFileSkipped:
		return "skipped"
	default:
		return "unknown"
	}
}

func (status SemanticFileStatus) MarshalJSON() ([]byte, error) {
	return marshalKnownEnum("semantic file status", status.String())
}

func (status *SemanticFileStatus) UnmarshalJSON(data []byte) error {
	value, err := unmarshalEnum(data)
	if err != nil {
		return err
	}
	switch value {
	case "resolved":
		*status = SemanticFileResolved
	case "partial":
		*status = SemanticFilePartial
	case "failed":
		*status = SemanticFileFailed
	case "stale":
		*status = SemanticFileStale
	case "skipped":
		*status = SemanticFileSkipped
	default:
		return fmt.Errorf("unknown semantic file status: %s", value)
	}
	return nil
}

// DeclarationKind classifies a semantic declaration.
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

func (kind DeclarationKind) String() string {
	values := [...]string{"", "struct", "interface", "defined-type", "type-alias", "function", "method", "field", "parameter", "result", "variable", "constant", "label", "type-parameter"}
	if kind >= DeclarationStruct && int(kind) < len(values) {
		return values[kind]
	}
	return "unknown"
}

func (kind DeclarationKind) MarshalJSON() ([]byte, error) {
	return marshalKnownEnum("declaration kind", kind.String())
}

func (kind *DeclarationKind) UnmarshalJSON(data []byte) error {
	value, err := unmarshalEnum(data)
	if err != nil {
		return err
	}
	for candidate := DeclarationStruct; candidate <= DeclarationTypeParameter; candidate++ {
		if candidate.String() == value {
			*kind = candidate
			return nil
		}
	}
	return fmt.Errorf("unknown declaration kind: %s", value)
}

// ReferenceKind classifies one source reference.
type ReferenceKind uint8

const (
	ReferenceIdentifier ReferenceKind = iota + 1
	ReferenceSelector
	ReferenceType
	ReferenceInstantiation
)

func (kind ReferenceKind) String() string {
	switch kind {
	case ReferenceIdentifier:
		return "identifier"
	case ReferenceSelector:
		return "selector"
	case ReferenceType:
		return "type"
	case ReferenceInstantiation:
		return "instantiation"
	default:
		return "unknown"
	}
}

func (kind ReferenceKind) MarshalJSON() ([]byte, error) {
	return marshalKnownEnum("reference kind", kind.String())
}

func (kind *ReferenceKind) UnmarshalJSON(data []byte) error {
	value, err := unmarshalEnum(data)
	if err != nil {
		return err
	}
	switch value {
	case "identifier":
		*kind = ReferenceIdentifier
	case "selector":
		*kind = ReferenceSelector
	case "type":
		*kind = ReferenceType
	case "instantiation":
		*kind = ReferenceInstantiation
	default:
		return fmt.Errorf("unknown reference kind: %s", value)
	}
	return nil
}

// TypeRelationKind classifies a semantic type relationship.
type TypeRelationKind uint8

const (
	TypeRelationUses TypeRelationKind = iota + 1
	TypeRelationEmbeds
	TypeRelationAliasOf
	TypeRelationInstantiates
	TypeRelationConstrains
)

func (kind TypeRelationKind) String() string {
	switch kind {
	case TypeRelationUses:
		return "uses"
	case TypeRelationEmbeds:
		return "embeds"
	case TypeRelationAliasOf:
		return "alias-of"
	case TypeRelationInstantiates:
		return "instantiates"
	case TypeRelationConstrains:
		return "constrains"
	default:
		return "unknown"
	}
}

func (kind TypeRelationKind) MarshalJSON() ([]byte, error) {
	return marshalKnownEnum("type relation kind", kind.String())
}

func (kind *TypeRelationKind) UnmarshalJSON(data []byte) error {
	value, err := unmarshalEnum(data)
	if err != nil {
		return err
	}
	switch value {
	case "uses":
		*kind = TypeRelationUses
	case "embeds":
		*kind = TypeRelationEmbeds
	case "alias-of":
		*kind = TypeRelationAliasOf
	case "instantiates":
		*kind = TypeRelationInstantiates
	case "constrains":
		*kind = TypeRelationConstrains
	default:
		return fmt.Errorf("unknown type relation kind: %s", value)
	}
	return nil
}

// SatisfactionStatus describes a bounded interface-satisfaction result.
type SatisfactionStatus uint8

const (
	SatisfactionProven SatisfactionStatus = iota + 1
	SatisfactionDisproven
	SatisfactionUnknown
)

func (status SatisfactionStatus) String() string {
	switch status {
	case SatisfactionProven:
		return "proven"
	case SatisfactionDisproven:
		return "disproven"
	case SatisfactionUnknown:
		return "unknown"
	default:
		return "invalid"
	}
}

func (status SatisfactionStatus) MarshalJSON() ([]byte, error) {
	if status.String() == "invalid" {
		return nil, fmt.Errorf("invalid satisfaction status")
	}
	return json.Marshal(status.String())
}

func (status *SatisfactionStatus) UnmarshalJSON(data []byte) error {
	value, err := unmarshalEnum(data)
	if err != nil {
		return err
	}
	switch value {
	case "proven":
		*status = SatisfactionProven
	case "disproven":
		*status = SatisfactionDisproven
	case "unknown":
		*status = SatisfactionUnknown
	default:
		return fmt.Errorf("unknown satisfaction status: %s", value)
	}
	return nil
}

// Metadata identifies the semantic artifact and its stable-ID scheme.
type Metadata struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	IDSchemeVersion string `json:"id_scheme_version"`
	EngineName      string `json:"engine_name"`
	EngineVersion   string `json:"engine_version"`
}

// SemanticFile records one source-verification and semantic-analysis outcome.
type SemanticFile struct {
	FileID          string             `json:"file_id"`
	PackageID       string             `json:"package_id,omitempty"`
	Status          SemanticFileStatus `json:"status"`
	ContentDigest   string             `json:"content_digest"`
	ReferenceCount  int                `json:"reference_count"`
	UnresolvedCount int                `json:"unresolved_count"`
}

// SemanticDeclaration records a semantic declaration without copying syntax models.
type SemanticDeclaration struct {
	ID                 string           `json:"id"`
	SyntaxSymbolID     string           `json:"syntax_symbol_id,omitempty"`
	Name               string           `json:"name"`
	FileID             string           `json:"file_id"`
	PackageID          string           `json:"package_id"`
	OwnerDeclarationID string           `json:"owner_declaration_id,omitempty"`
	Kind               DeclarationKind  `json:"kind"`
	TypeDisplay        string           `json:"type_display,omitempty"`
	Location           lie.SourceRange  `json:"location"`
	Status             ResolutionStatus `json:"status"`
}

// SemanticReference records one identifier-like reference and its resolution state.
type SemanticReference struct {
	ID                      string           `json:"id"`
	Name                    string           `json:"name"`
	Kind                    ReferenceKind    `json:"kind"`
	FileID                  string           `json:"file_id"`
	PackageID               string           `json:"package_id"`
	OwnerDeclarationID      string           `json:"owner_declaration_id,omitempty"`
	Location                lie.SourceRange  `json:"location"`
	Status                  ResolutionStatus `json:"status"`
	TargetDeclarationID     string           `json:"target_declaration_id,omitempty"`
	CandidateDeclarationIDs []string         `json:"candidate_declaration_ids,omitempty"`
	ExternalIdentity        string           `json:"external_identity,omitempty"`
}

// ReceiverBinding links a method declaration to a proven local receiver type.
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

// ImportBinding links one source import to package-identity evidence.
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

// TypeRelation records one bounded semantic type relationship.
type TypeRelation struct {
	ID                  string           `json:"id"`
	Kind                TypeRelationKind `json:"kind"`
	FileID              string           `json:"file_id"`
	PackageID           string           `json:"package_id"`
	OwnerDeclarationID  string           `json:"owner_declaration_id,omitempty"`
	Location            lie.SourceRange  `json:"location"`
	Status              ResolutionStatus `json:"status"`
	TargetDeclarationID string           `json:"target_declaration_id,omitempty"`
	TargetIdentity      string           `json:"target_identity,omitempty"`
	TypeArgumentText    []string         `json:"type_arguments,omitempty"`
}

// InterfaceSatisfaction records one bounded concrete/interface candidate result.
type InterfaceSatisfaction struct {
	ID                        string             `json:"id"`
	ConcreteTypeDeclarationID string             `json:"concrete_type_declaration_id"`
	InterfaceDeclarationID    string             `json:"interface_declaration_id"`
	Status                    SatisfactionStatus `json:"status"`
	PointerRequired           bool               `json:"pointer_required"`
	MissingMethodNames        []string           `json:"missing_method_names,omitempty"`
	CompileTimeAssertions     []rie.Evidence     `json:"compile_time_assertions,omitempty"`
}

// SemanticStatistics contains compact derived counts for one full rebuild.
type SemanticStatistics struct {
	CandidateFiles          int            `json:"candidate_files"`
	ResolvedFiles           int            `json:"resolved_files"`
	PartialFiles            int            `json:"partial_files"`
	FailedFiles             int            `json:"failed_files"`
	StaleFiles              int            `json:"stale_files"`
	SkippedFiles            int            `json:"skipped_files"`
	ResolvedDeclarations    int            `json:"resolved_declarations"`
	UnresolvedDeclarations  int            `json:"unresolved_declarations"`
	PartialDeclarations     int            `json:"partial_declarations"`
	ExternalDeclarations    int            `json:"external_declarations"`
	AmbiguousDeclarations   int            `json:"ambiguous_declarations"`
	ReferencesByStatus      map[string]int `json:"references_by_status"`
	ReceiverBindings        int            `json:"receiver_bindings"`
	ImportBindingsByStatus  map[string]int `json:"import_bindings_by_status"`
	TypeRelations           int            `json:"type_relations"`
	OmittedRelationships    int            `json:"omitted_relationships"`
	InterfaceChecksByStatus map[string]int `json:"interface_checks_by_status"`
	Diagnostics             int            `json:"diagnostics"`
	OmittedDiagnostics      int            `json:"omitted_diagnostics"`
}

// GoSemanticInventory is the immutable candidate semantic artifact.
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

func newInventory(files []SemanticFile, declarations []SemanticDeclaration, references []SemanticReference, receivers []ReceiverBinding, imports []ImportBinding, typeRelations []TypeRelation, satisfaction []InterfaceSatisfaction, diagnostics []lie.Diagnostic, statistics SemanticStatistics) GoSemanticInventory {
	return GoSemanticInventory{
		metadata: Metadata{Name: ArtifactName, Version: ArtifactVersion, IDSchemeVersion: IDSchemeVersion, EngineName: "go-semantic", EngineVersion: engineVersion},
		sources: []rie.ArtifactReference{
			{Name: rie.RepositorySnapshotArtifactName, Version: rie.RepositorySnapshotArtifactVersion},
			{Name: golang.ArtifactName, Version: golang.ArtifactVersion},
			{Name: packageidentity.ArtifactName, Version: packageidentity.ArtifactVersion},
		},
		files: append([]SemanticFile(nil), files...), declarations: append([]SemanticDeclaration(nil), declarations...), references: cloneReferences(references),
		receivers: append([]ReceiverBinding(nil), receivers...), imports: append([]ImportBinding(nil), imports...), typeRelations: cloneTypeRelations(typeRelations),
		interfaceSatisfaction: cloneInterfaceSatisfaction(satisfaction), diagnostics: cloneDiagnostics(diagnostics), statistics: cloneStatistics(statistics),
	}
}

func (GoSemanticInventory) ArtifactName() string    { return ArtifactName }
func (GoSemanticInventory) ArtifactVersion() string { return ArtifactVersion }
func (GoSemanticInventory) Language() string        { return "Go" }
func (inventory GoSemanticInventory) Metadata() Metadata {
	return inventory.metadata
}
func (inventory GoSemanticInventory) SourceArtifacts() []rie.ArtifactReference {
	return append([]rie.ArtifactReference(nil), inventory.sources...)
}
func (inventory GoSemanticInventory) Files() []SemanticFile {
	return append([]SemanticFile(nil), inventory.files...)
}
func (inventory GoSemanticInventory) Declarations() []SemanticDeclaration {
	return append([]SemanticDeclaration(nil), inventory.declarations...)
}
func (inventory GoSemanticInventory) References() []SemanticReference {
	return cloneReferences(inventory.references)
}
func (inventory GoSemanticInventory) ReceiverBindings() []ReceiverBinding {
	return append([]ReceiverBinding(nil), inventory.receivers...)
}
func (inventory GoSemanticInventory) ImportBindings() []ImportBinding {
	return append([]ImportBinding(nil), inventory.imports...)
}
func (inventory GoSemanticInventory) TypeRelations() []TypeRelation {
	return cloneTypeRelations(inventory.typeRelations)
}
func (inventory GoSemanticInventory) InterfaceSatisfaction() []InterfaceSatisfaction {
	return cloneInterfaceSatisfaction(inventory.interfaceSatisfaction)
}
func (inventory GoSemanticInventory) Diagnostics() []lie.Diagnostic {
	return cloneDiagnostics(inventory.diagnostics)
}
func (inventory GoSemanticInventory) Statistics() SemanticStatistics {
	return cloneStatistics(inventory.statistics)
}

func cloneReferences(source []SemanticReference) []SemanticReference {
	result := make([]SemanticReference, len(source))
	for index, value := range source {
		result[index] = value
		result[index].CandidateDeclarationIDs = append([]string(nil), value.CandidateDeclarationIDs...)
	}
	return result
}

func cloneTypeRelations(source []TypeRelation) []TypeRelation {
	result := make([]TypeRelation, len(source))
	for index, value := range source {
		result[index] = value
		result[index].TypeArgumentText = append([]string(nil), value.TypeArgumentText...)
	}
	return result
}

func cloneInterfaceSatisfaction(source []InterfaceSatisfaction) []InterfaceSatisfaction {
	result := make([]InterfaceSatisfaction, len(source))
	for index, value := range source {
		result[index] = value
		result[index].MissingMethodNames = append([]string(nil), value.MissingMethodNames...)
		result[index].CompileTimeAssertions = append([]rie.Evidence(nil), value.CompileTimeAssertions...)
	}
	return result
}

func cloneDiagnostics(source []lie.Diagnostic) []lie.Diagnostic {
	result := make([]lie.Diagnostic, len(source))
	for index, value := range source {
		result[index] = value
		if value.Location != nil {
			location := *value.Location
			result[index].Location = &location
		}
	}
	return result
}

func cloneStatistics(source SemanticStatistics) SemanticStatistics {
	result := source
	result.ReferencesByStatus = cloneStringCountMap(source.ReferencesByStatus)
	result.ImportBindingsByStatus = cloneStringCountMap(source.ImportBindingsByStatus)
	result.InterfaceChecksByStatus = cloneStringCountMap(source.InterfaceChecksByStatus)
	return result
}

func cloneStringCountMap(source map[string]int) map[string]int {
	result := make(map[string]int, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func emptyStatistics() SemanticStatistics {
	return SemanticStatistics{
		ReferencesByStatus: map[string]int{}, ImportBindingsByStatus: map[string]int{}, InterfaceChecksByStatus: map[string]int{},
	}
}

func marshalKnownEnum(name, value string) ([]byte, error) {
	if value == "unknown" || value == "invalid" {
		return nil, fmt.Errorf("invalid %s", name)
	}
	return json.Marshal(value)
}

func unmarshalEnum(data []byte) (string, error) {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return "", err
	}
	return value, nil
}
