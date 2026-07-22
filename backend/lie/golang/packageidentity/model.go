package packageidentity

import (
	"encoding/json"
	"fmt"

	"github.com/AjayMunagala/software-engineering-platform/backend/lie"
	"github.com/AjayMunagala/software-engineering-platform/backend/lie/golang"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
)

const (
	ArtifactName         = "go-package-identity-inventory"
	ArtifactVersion      = "0.1.0"
	ProofIDSchemeVersion = "go-package-proof-id/v1"
	engineVersion        = "0.1.0"
)

type ProofKind uint8

const (
	ProofSameModule ProofKind = iota + 1
	ProofWorkspaceModule
	ProofLocalReplace
	ProofVendor
	ProofStandardLibrary
)

func (kind ProofKind) String() string {
	switch kind {
	case ProofSameModule:
		return "same-module"
	case ProofWorkspaceModule:
		return "workspace-module"
	case ProofLocalReplace:
		return "local-replace"
	case ProofVendor:
		return "vendor"
	case ProofStandardLibrary:
		return "standard-library"
	default:
		return "unknown"
	}
}

func (kind ProofKind) MarshalJSON() ([]byte, error) { return json.Marshal(kind.String()) }

type ProofStatus uint8

const (
	ProofResolved ProofStatus = iota + 1
	ProofUnresolved
	ProofAmbiguous
	ProofExternal
	ProofStale
)

func (status ProofStatus) String() string {
	switch status {
	case ProofResolved:
		return "resolved"
	case ProofUnresolved:
		return "unresolved"
	case ProofAmbiguous:
		return "ambiguous"
	case ProofExternal:
		return "external"
	case ProofStale:
		return "stale"
	default:
		return "unknown"
	}
}

func (status ProofStatus) MarshalJSON() ([]byte, error) { return json.Marshal(status.String()) }

type ResolutionContextKind uint8

const (
	ContextSingleModule ResolutionContextKind = iota + 1
	ContextWorkspace
	ContextModuleVendor
	ContextWorkspaceVendor
	ContextUnmanaged
)

func (kind ResolutionContextKind) String() string {
	switch kind {
	case ContextSingleModule:
		return "single-module"
	case ContextWorkspace:
		return "workspace"
	case ContextModuleVendor:
		return "module-vendor"
	case ContextWorkspaceVendor:
		return "workspace-vendor"
	case ContextUnmanaged:
		return "unmanaged"
	default:
		return "unknown"
	}
}

func (kind ResolutionContextKind) MarshalJSON() ([]byte, error) { return json.Marshal(kind.String()) }

type Metadata struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	IDSchemeVersion string `json:"id_scheme_version"`
	EngineName      string `json:"engine_name"`
	EngineVersion   string `json:"engine_version"`
}

type PackageIdentityEvidence struct {
	File          string           `json:"file,omitempty"`
	ContentDigest string           `json:"content_digest"`
	Rule          string           `json:"rule"`
	Value         string           `json:"value"`
	Location      *lie.SourceRange `json:"location,omitempty"`
}

type ResolutionContext struct {
	ID            string                    `json:"id"`
	Kind          ResolutionContextKind     `json:"kind"`
	Root          string                    `json:"root"`
	ManifestFiles []string                  `json:"manifest_files"`
	MainModuleIDs []string                  `json:"main_module_ids"`
	Evidence      []PackageIdentityEvidence `json:"evidence"`
}

type ModuleIdentity struct {
	ID         string                    `json:"id"`
	ModulePath string                    `json:"module_path"`
	Root       string                    `json:"root"`
	GoVersion  string                    `json:"go_version,omitempty"`
	Evidence   []PackageIdentityEvidence `json:"evidence"`
}

type PackageIdentityProof struct {
	ID                  string                    `json:"id"`
	ResolutionContextID string                    `json:"resolution_context_id"`
	ImportingPackageID  string                    `json:"importing_package_id"`
	ImportPath          string                    `json:"import_path"`
	TargetPackageID     string                    `json:"target_package_id,omitempty"`
	TargetDirectory     string                    `json:"target_directory,omitempty"`
	Kinds               []ProofKind               `json:"kinds"`
	Status              ProofStatus               `json:"status"`
	Evidence            []PackageIdentityEvidence `json:"evidence"`
	CandidatePackageIDs []string                  `json:"candidate_package_ids,omitempty"`
}

type PackageIdentityStatistics struct {
	ManifestsInspected int            `json:"manifests_inspected"`
	Contexts           int            `json:"contexts"`
	Modules            int            `json:"modules"`
	ProofsByStatus     map[string]int `json:"proofs_by_status"`
	Diagnostics        int            `json:"diagnostics"`
	OmittedDiagnostics int            `json:"omitted_diagnostics"`
}

// GoPackageIdentityInventory is the immutable Phase 2.2.1 candidate artifact.
type GoPackageIdentityInventory struct {
	metadata    Metadata
	sources     []rie.ArtifactReference
	contexts    []ResolutionContext
	modules     []ModuleIdentity
	proofs      []PackageIdentityProof
	diagnostics []lie.Diagnostic
	statistics  PackageIdentityStatistics
}

func newInventory(contexts []ResolutionContext, modules []ModuleIdentity, proofs []PackageIdentityProof, diagnostics []lie.Diagnostic, statistics PackageIdentityStatistics) GoPackageIdentityInventory {
	return GoPackageIdentityInventory{
		metadata: Metadata{Name: ArtifactName, Version: ArtifactVersion, IDSchemeVersion: ProofIDSchemeVersion, EngineName: "go-package-identity", EngineVersion: engineVersion},
		sources: []rie.ArtifactReference{
			{Name: rie.RepositorySnapshotArtifactName, Version: rie.RepositorySnapshotArtifactVersion},
			{Name: golang.ArtifactName, Version: golang.ArtifactVersion},
		},
		contexts: cloneContexts(contexts), modules: cloneModules(modules), proofs: cloneProofs(proofs), diagnostics: cloneDiagnostics(diagnostics), statistics: cloneStatistics(statistics),
	}
}

func (GoPackageIdentityInventory) ArtifactName() string         { return ArtifactName }
func (GoPackageIdentityInventory) ArtifactVersion() string      { return ArtifactVersion }
func (inventory GoPackageIdentityInventory) Metadata() Metadata { return inventory.metadata }
func (inventory GoPackageIdentityInventory) SourceArtifacts() []rie.ArtifactReference {
	return append([]rie.ArtifactReference(nil), inventory.sources...)
}
func (inventory GoPackageIdentityInventory) Contexts() []ResolutionContext {
	return cloneContexts(inventory.contexts)
}
func (inventory GoPackageIdentityInventory) Modules() []ModuleIdentity {
	return cloneModules(inventory.modules)
}
func (inventory GoPackageIdentityInventory) Proofs() []PackageIdentityProof {
	return cloneProofs(inventory.proofs)
}
func (inventory GoPackageIdentityInventory) Diagnostics() []lie.Diagnostic {
	return cloneDiagnostics(inventory.diagnostics)
}
func (inventory GoPackageIdentityInventory) Statistics() PackageIdentityStatistics {
	return cloneStatistics(inventory.statistics)
}

func cloneContexts(source []ResolutionContext) []ResolutionContext {
	result := make([]ResolutionContext, len(source))
	for index, value := range source {
		result[index] = value
		result[index].ManifestFiles = append([]string(nil), value.ManifestFiles...)
		result[index].MainModuleIDs = append([]string(nil), value.MainModuleIDs...)
		result[index].Evidence = cloneEvidence(value.Evidence)
	}
	return result
}

func cloneModules(source []ModuleIdentity) []ModuleIdentity {
	result := make([]ModuleIdentity, len(source))
	for index, value := range source {
		result[index] = value
		result[index].Evidence = cloneEvidence(value.Evidence)
	}
	return result
}

func cloneProofs(source []PackageIdentityProof) []PackageIdentityProof {
	result := make([]PackageIdentityProof, len(source))
	for index, value := range source {
		result[index] = value
		result[index].Kinds = append([]ProofKind(nil), value.Kinds...)
		result[index].Evidence = cloneEvidence(value.Evidence)
		result[index].CandidatePackageIDs = append([]string(nil), value.CandidatePackageIDs...)
	}
	return result
}

func cloneEvidence(source []PackageIdentityEvidence) []PackageIdentityEvidence {
	result := make([]PackageIdentityEvidence, len(source))
	for index, value := range source {
		result[index] = value
		if value.Location != nil {
			location := *value.Location
			result[index].Location = &location
		}
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

func cloneStatistics(source PackageIdentityStatistics) PackageIdentityStatistics {
	result := source
	result.ProofsByStatus = make(map[string]int, len(source.ProofsByStatus))
	for status, count := range source.ProofsByStatus {
		result.ProofsByStatus[status] = count
	}
	return result
}

func (metadata Metadata) String() string {
	return fmt.Sprintf("%s@%s (%s)", metadata.Name, metadata.Version, metadata.IDSchemeVersion)
}
