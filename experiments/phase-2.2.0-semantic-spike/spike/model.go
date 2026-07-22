package spike

// SourceFile is one in-memory, repository-relative Go source file.
type SourceFile struct {
	Path    string
	Content string
}

// Input is the complete source set for one full rebuild.
type Input struct {
	Files []SourceFile
}

// Result contains only deterministic evidence needed by the spike.
type Result struct {
	ParseCount       int
	PackageChecks    int
	DeclarationIDs   []string
	Definitions      int
	Uses             int
	GenericInstances int
	TypeErrors       []string
}

type proofKind string

const (
	proofSameModule      proofKind = "same-module"
	proofWorkspaceModule proofKind = "workspace-module"
	proofLocalReplace    proofKind = "local-replace"
	proofVendor          proofKind = "vendor"
	proofStandardLibrary proofKind = "standard-library"
)

type proofStatus string

const (
	proofResolved   proofStatus = "resolved"
	proofUnresolved proofStatus = "unresolved"
	proofAmbiguous  proofStatus = "ambiguous"
	proofExternal   proofStatus = "external"
)

type contextKind string

const (
	contextSingleModule    contextKind = "single-module"
	contextWorkspace       contextKind = "workspace"
	contextModuleVendor    contextKind = "module-vendor"
	contextWorkspaceVendor contextKind = "workspace-vendor"
)

type moduleFact struct {
	ID                 string
	Path               string
	PackagesByRelative map[string]string
	Requires           map[string]string
	Replaces           []replaceFact
}

type replaceFact struct {
	OldPath        string
	OldVersion     string
	TargetModuleID string
}

type resolutionContext struct {
	ID                   string
	Kind                 contextKind
	ImportingModuleID    string
	MainModuleIDs        []string
	Modules              map[string]moduleFact
	WorkspaceReplaces    []replaceFact
	VendorPackages       map[string]string
	ExactStandardLibrary map[string]struct{}
}

type identityDecision struct {
	Status              proofStatus
	TargetPackageID     string
	Kinds               []proofKind
	CandidatePackageIDs []string
}

type diagnostic struct {
	File     string
	Start    int
	End      int
	Severity string
	Code     string
	Message  string
}

type diagnosticResult struct {
	Items   []diagnostic
	Omitted int
}

type candidateEvent struct {
	Kind                 string
	ConcreteDeclaration  string
	InterfaceDeclaration string
	Pointer              bool
}

type interfaceCandidate struct {
	ConcreteDeclaration  string
	InterfaceDeclaration string
	Pointer              bool
}
