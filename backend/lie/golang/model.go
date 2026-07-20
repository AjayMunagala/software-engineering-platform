package golang

import (
	"encoding/json"
	"fmt"

	"github.com/AjayMunagala/software-engineering-platform/backend/lie"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
)

const (
	ArtifactName    = "go-language-inventory"
	ArtifactVersion = "0.1.0"
)

type FileStatus uint8

const (
	FileStatusParsed FileStatus = iota + 1
	FileStatusFailed
	FileStatusSkipped
)

func (status FileStatus) String() string {
	switch status {
	case FileStatusParsed:
		return "parsed"
	case FileStatusFailed:
		return "failed"
	case FileStatusSkipped:
		return "skipped"
	default:
		return "unknown"
	}
}

func (status FileStatus) MarshalJSON() ([]byte, error) { return json.Marshal(status.String()) }

func (status *FileStatus) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	switch value {
	case "parsed":
		*status = FileStatusParsed
	case "failed":
		*status = FileStatusFailed
	case "skipped":
		*status = FileStatusSkipped
	default:
		return fmt.Errorf("unknown file status: %s", value)
	}
	return nil
}

type ImportAliasKind uint8

const (
	ImportAliasDefault ImportAliasKind = iota + 1
	ImportAliasNamed
	ImportAliasBlank
	ImportAliasDot
)

func (kind ImportAliasKind) String() string {
	switch kind {
	case ImportAliasDefault:
		return "default"
	case ImportAliasNamed:
		return "named"
	case ImportAliasBlank:
		return "blank"
	case ImportAliasDot:
		return "dot"
	default:
		return "unknown"
	}
}

type SymbolKind uint8

const (
	SymbolKindStruct SymbolKind = iota + 1
	SymbolKindInterface
	SymbolKindFunction
	SymbolKindMethod
	SymbolKindConstant
	SymbolKindVariable
)

func (kind SymbolKind) String() string {
	switch kind {
	case SymbolKindStruct:
		return "struct"
	case SymbolKindInterface:
		return "interface"
	case SymbolKindFunction:
		return "function"
	case SymbolKindMethod:
		return "method"
	case SymbolKindConstant:
		return "constant"
	case SymbolKindVariable:
		return "variable"
	default:
		return "unknown"
	}
}

type GoImport struct {
	Path      string          `json:"path"`
	Alias     string          `json:"alias,omitempty"`
	AliasKind ImportAliasKind `json:"alias_kind"`
	Location  lie.SourceRange `json:"location"`
}

type GoFile struct {
	ID            string     `json:"id"`
	Path          string     `json:"path"`
	PackageID     string     `json:"package_id,omitempty"`
	PackageName   string     `json:"package_name,omitempty"`
	Status        FileStatus `json:"status"`
	IsTest        bool       `json:"is_test"`
	SizeBytes     int64      `json:"size_bytes"`
	ContentDigest string     `json:"content_digest,omitempty"`
	Imports       []GoImport `json:"imports"`
}

type GoPackage struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Directory string   `json:"directory"`
	FileIDs   []string `json:"file_ids"`
}

type GoSymbol struct {
	ID              string          `json:"id"`
	Kind            SymbolKind      `json:"kind"`
	Name            string          `json:"name"`
	PackageID       string          `json:"package_id"`
	FileID          string          `json:"file_id"`
	Exported        bool            `json:"exported"`
	ReceiverBase    string          `json:"receiver_base,omitempty"`
	PointerReceiver bool            `json:"pointer_receiver,omitempty"`
	GenericReceiver bool            `json:"generic_receiver,omitempty"`
	Location        lie.SourceRange `json:"location"`
}

type ParseStatistics struct {
	CandidateFiles     int            `json:"candidate_files"`
	ParsedFiles        int            `json:"parsed_files"`
	FailedFiles        int            `json:"failed_files"`
	SkippedFiles       int            `json:"skipped_files"`
	ParsedBytes        int64          `json:"parsed_bytes"`
	Packages           int            `json:"packages"`
	Imports            int            `json:"imports"`
	SymbolsByKind      map[string]int `json:"symbols_by_kind"`
	Diagnostics        int            `json:"diagnostics"`
	OmittedDiagnostics int            `json:"omitted_diagnostics"`
}

type Metadata struct {
	Name          string `json:"name"`
	Version       string `json:"version"`
	EngineName    string `json:"engine_name"`
	EngineVersion string `json:"engine_version"`
}

// GoLanguageInventory is immutable: every collection accessor returns a deep copy.
type GoLanguageInventory struct {
	metadata    Metadata
	sources     []rie.ArtifactReference
	files       []GoFile
	packages    []GoPackage
	symbols     []GoSymbol
	diagnostics []lie.Diagnostic
	statistics  ParseStatistics
}

func newGoLanguageInventory(sources []rie.ArtifactReference, files []GoFile, packages []GoPackage, symbols []GoSymbol, diagnostics []lie.Diagnostic, statistics ParseStatistics) GoLanguageInventory {
	return GoLanguageInventory{
		metadata:    Metadata{Name: ArtifactName, Version: ArtifactVersion, EngineName: "golang", EngineVersion: engineVersion},
		sources:     append([]rie.ArtifactReference(nil), sources...),
		files:       cloneFiles(files),
		packages:    clonePackages(packages),
		symbols:     append([]GoSymbol(nil), symbols...),
		diagnostics: cloneDiagnostics(diagnostics),
		statistics:  cloneStatistics(statistics),
	}
}

func (GoLanguageInventory) ArtifactName() string         { return ArtifactName }
func (GoLanguageInventory) ArtifactVersion() string      { return ArtifactVersion }
func (GoLanguageInventory) Language() string             { return "Go" }
func (inventory GoLanguageInventory) Metadata() Metadata { return inventory.metadata }
func (inventory GoLanguageInventory) SourceArtifacts() []rie.ArtifactReference {
	return append([]rie.ArtifactReference(nil), inventory.sources...)
}
func (inventory GoLanguageInventory) Files() []GoFile       { return cloneFiles(inventory.files) }
func (inventory GoLanguageInventory) Packages() []GoPackage { return clonePackages(inventory.packages) }
func (inventory GoLanguageInventory) Symbols() []GoSymbol {
	return append([]GoSymbol(nil), inventory.symbols...)
}
func (inventory GoLanguageInventory) Diagnostics() []lie.Diagnostic {
	return cloneDiagnostics(inventory.diagnostics)
}
func (inventory GoLanguageInventory) Statistics() ParseStatistics {
	return cloneStatistics(inventory.statistics)
}

func cloneFiles(source []GoFile) []GoFile {
	result := make([]GoFile, len(source))
	for index, file := range source {
		result[index] = file
		result[index].Imports = append([]GoImport(nil), file.Imports...)
	}
	return result
}

func clonePackages(source []GoPackage) []GoPackage {
	result := make([]GoPackage, len(source))
	for index, pkg := range source {
		result[index] = pkg
		result[index].FileIDs = append([]string(nil), pkg.FileIDs...)
	}
	return result
}

func cloneDiagnostics(source []lie.Diagnostic) []lie.Diagnostic {
	result := make([]lie.Diagnostic, len(source))
	for index, diagnostic := range source {
		result[index] = diagnostic
		if diagnostic.Location != nil {
			location := *diagnostic.Location
			result[index].Location = &location
		}
	}
	return result
}

func cloneStatistics(source ParseStatistics) ParseStatistics {
	result := source
	result.SymbolsByKind = make(map[string]int, len(source.SymbolsByKind))
	for kind, count := range source.SymbolsByKind {
		result.SymbolsByKind[kind] = count
	}
	return result
}
