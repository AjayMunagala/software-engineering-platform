package build

import (
	"sort"

	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
)

const (
	BuildInventoryArtifactName    = "build-inventory"
	BuildInventoryArtifactVersion = "1.0.0"
)

type Metadata struct {
	Name          string
	Version       string
	EngineName    string
	EngineVersion string
}

type PackageManager struct {
	ID       string
	Name     string
	Location string
	evidence []rie.Evidence
}

func (item PackageManager) Evidence() []rie.Evidence {
	return append([]rie.Evidence(nil), item.evidence...)
}

type BuildSystem struct {
	ID       string
	Name     string
	Location string
	evidence []rie.Evidence
}

func (item BuildSystem) Evidence() []rie.Evidence {
	return append([]rie.Evidence(nil), item.evidence...)
}

type Workspace struct {
	ID       string
	Kind     string
	Location string
	members  []string
	evidence []rie.Evidence
}

func (item Workspace) Members() []string { return append([]string(nil), item.members...) }

func (item Workspace) Evidence() []rie.Evidence {
	return append([]rie.Evidence(nil), item.evidence...)
}

type LockFile struct {
	PackageManagerID string
	Path             string
	Location         string
	evidence         []rie.Evidence
}

func (item LockFile) Evidence() []rie.Evidence {
	return append([]rie.Evidence(nil), item.evidence...)
}

type Toolchain struct {
	Tool       string
	Constraint string
	Location   string
	evidence   []rie.Evidence
}

func (item Toolchain) Evidence() []rie.Evidence {
	return append([]rie.Evidence(nil), item.evidence...)
}

// BuildInventory is the immutable, technology-neutral v0.5 artifact.
type BuildInventory struct {
	metadata        Metadata
	packageManagers []PackageManager
	buildSystems    []BuildSystem
	workspaces      []Workspace
	lockFiles       []LockFile
	toolchains      []Toolchain
}

func newBuildInventory(findings []Finding) BuildInventory {
	builders := aggregate(findings)
	return BuildInventory{
		metadata: Metadata{
			Name: BuildInventoryArtifactName, Version: BuildInventoryArtifactVersion,
			EngineName: "build-package", EngineVersion: "0.5.0",
		},
		packageManagers: builders.packageManagers,
		buildSystems:    builders.buildSystems,
		workspaces:      builders.workspaces,
		lockFiles:       builders.lockFiles,
		toolchains:      builders.toolchains,
	}
}

func (BuildInventory) ArtifactName() string { return BuildInventoryArtifactName }

func (BuildInventory) ArtifactVersion() string { return BuildInventoryArtifactVersion }

func (inventory BuildInventory) Metadata() Metadata { return inventory.metadata }

func (inventory BuildInventory) PackageManagers() []PackageManager {
	return copyPackageManagers(inventory.packageManagers)
}

func (inventory BuildInventory) BuildSystems() []BuildSystem {
	return copyBuildSystems(inventory.buildSystems)
}

func (inventory BuildInventory) Workspaces() []Workspace {
	return copyWorkspaces(inventory.workspaces)
}

func (inventory BuildInventory) LockFiles() []LockFile { return copyLockFiles(inventory.lockFiles) }

func (inventory BuildInventory) Toolchains() []Toolchain { return copyToolchains(inventory.toolchains) }

type FindingKind string

const (
	PackageManagerFinding FindingKind = "package_manager"
	BuildSystemFinding    FindingKind = "build_system"
	WorkspaceFinding      FindingKind = "workspace"
	LockFileFinding       FindingKind = "lock_file"
	ToolchainFinding      FindingKind = "toolchain"
)

// Finding is the technology-neutral output of one registry detector.
type Finding struct {
	Kind             FindingKind
	ID               string
	Name             string
	Location         string
	Members          []string
	PackageManagerID string
	Path             string
	Constraint       string
	Evidence         rie.Evidence
}

type inventoryBuilder struct {
	packageManagers []PackageManager
	buildSystems    []BuildSystem
	workspaces      []Workspace
	lockFiles       []LockFile
	toolchains      []Toolchain
}

func aggregate(findings []Finding) inventoryBuilder {
	packageManagers := make(map[string]*PackageManager)
	buildSystems := make(map[string]*BuildSystem)
	workspaces := make(map[string]*Workspace)
	lockFiles := make(map[string]*LockFile)
	toolchains := make(map[string]*Toolchain)

	for _, finding := range findings {
		key := finding.ID + "\x00" + finding.Location
		switch finding.Kind {
		case PackageManagerFinding:
			item := packageManagers[key]
			if item == nil {
				item = &PackageManager{ID: finding.ID, Name: finding.Name, Location: finding.Location}
				packageManagers[key] = item
			}
			item.evidence = appendEvidence(item.evidence, finding.Evidence)
		case BuildSystemFinding:
			item := buildSystems[key]
			if item == nil {
				item = &BuildSystem{ID: finding.ID, Name: finding.Name, Location: finding.Location}
				buildSystems[key] = item
			}
			item.evidence = appendEvidence(item.evidence, finding.Evidence)
		case WorkspaceFinding:
			item := workspaces[key]
			if item == nil {
				item = &Workspace{ID: finding.ID, Kind: finding.Name, Location: finding.Location}
				workspaces[key] = item
			}
			item.members = appendUniqueStrings(item.members, finding.Members...)
			item.evidence = appendEvidence(item.evidence, finding.Evidence)
		case LockFileFinding:
			lockKey := finding.Path
			item := lockFiles[lockKey]
			if item == nil {
				item = &LockFile{PackageManagerID: finding.PackageManagerID, Path: finding.Path, Location: finding.Location}
				lockFiles[lockKey] = item
			}
			item.evidence = appendEvidence(item.evidence, finding.Evidence)
		case ToolchainFinding:
			toolKey := finding.Name + "\x00" + finding.Constraint + "\x00" + finding.Location
			item := toolchains[toolKey]
			if item == nil {
				item = &Toolchain{Tool: finding.Name, Constraint: finding.Constraint, Location: finding.Location}
				toolchains[toolKey] = item
			}
			item.evidence = appendEvidence(item.evidence, finding.Evidence)
		}
	}

	builder := inventoryBuilder{}
	for _, item := range packageManagers {
		builder.packageManagers = append(builder.packageManagers, *item)
	}
	for _, item := range buildSystems {
		builder.buildSystems = append(builder.buildSystems, *item)
	}
	for _, item := range workspaces {
		sort.Strings(item.members)
		builder.workspaces = append(builder.workspaces, *item)
	}
	for _, item := range lockFiles {
		builder.lockFiles = append(builder.lockFiles, *item)
	}
	for _, item := range toolchains {
		builder.toolchains = append(builder.toolchains, *item)
	}
	sort.Slice(builder.packageManagers, func(i, j int) bool {
		return toolLess(builder.packageManagers[i].ID, builder.packageManagers[i].Location, builder.packageManagers[j].ID, builder.packageManagers[j].Location)
	})
	sort.Slice(builder.buildSystems, func(i, j int) bool {
		return toolLess(builder.buildSystems[i].ID, builder.buildSystems[i].Location, builder.buildSystems[j].ID, builder.buildSystems[j].Location)
	})
	sort.Slice(builder.workspaces, func(i, j int) bool {
		return toolLess(builder.workspaces[i].ID, builder.workspaces[i].Location, builder.workspaces[j].ID, builder.workspaces[j].Location)
	})
	sort.Slice(builder.lockFiles, func(i, j int) bool { return builder.lockFiles[i].Path < builder.lockFiles[j].Path })
	sort.Slice(builder.toolchains, func(i, j int) bool {
		left, right := builder.toolchains[i], builder.toolchains[j]
		if left.Tool != right.Tool {
			return left.Tool < right.Tool
		}
		if left.Location != right.Location {
			return left.Location < right.Location
		}
		return left.Constraint < right.Constraint
	})
	return builder
}

func appendEvidence(items []rie.Evidence, evidence rie.Evidence) []rie.Evidence {
	for _, existing := range items {
		if existing == evidence {
			return items
		}
	}
	items = append(items, evidence)
	sort.Slice(items, func(i, j int) bool {
		if items[i].File != items[j].File {
			return items[i].File < items[j].File
		}
		if items[i].Rule != items[j].Rule {
			return items[i].Rule < items[j].Rule
		}
		return items[i].Value < items[j].Value
	})
	return items
}

func appendUniqueStrings(items []string, values ...string) []string {
	seen := make(map[string]struct{}, len(items)+len(values))
	for _, item := range items {
		seen[item] = struct{}{}
	}
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; !exists {
			seen[value] = struct{}{}
			items = append(items, value)
		}
	}
	return items
}

func toolLess(leftID, leftLocation, rightID, rightLocation string) bool {
	if leftID != rightID {
		return leftID < rightID
	}
	return leftLocation < rightLocation
}

func copyPackageManagers(items []PackageManager) []PackageManager {
	result := append([]PackageManager(nil), items...)
	for index := range result {
		result[index].evidence = append([]rie.Evidence(nil), result[index].evidence...)
	}
	return result
}

func copyBuildSystems(items []BuildSystem) []BuildSystem {
	result := append([]BuildSystem(nil), items...)
	for index := range result {
		result[index].evidence = append([]rie.Evidence(nil), result[index].evidence...)
	}
	return result
}

func copyWorkspaces(items []Workspace) []Workspace {
	result := append([]Workspace(nil), items...)
	for index := range result {
		result[index].members = append([]string(nil), result[index].members...)
		result[index].evidence = append([]rie.Evidence(nil), result[index].evidence...)
	}
	return result
}

func copyLockFiles(items []LockFile) []LockFile {
	result := append([]LockFile(nil), items...)
	for index := range result {
		result[index].evidence = append([]rie.Evidence(nil), result[index].evidence...)
	}
	return result
}

func copyToolchains(items []Toolchain) []Toolchain {
	result := append([]Toolchain(nil), items...)
	for index := range result {
		result[index].evidence = append([]rie.Evidence(nil), result[index].evidence...)
	}
	return result
}
