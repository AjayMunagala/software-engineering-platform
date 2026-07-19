package metadata

import (
	"context"
	"path"
	"sort"
	"strings"

	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
	buildengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/build"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie/discovery"
	frameworkengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/framework"
	languageengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/language"
)

// SynthesisEngine creates the repository cover page from frozen artifacts.
type SynthesisEngine struct{ config Config }

func New(configs ...Config) Engine {
	config := DefaultConfig()
	if len(configs) > 0 {
		config = configs[0]
	}
	return SynthesisEngine{config: config}
}

func (SynthesisEngine) Name() string    { return "repository-metadata" }
func (SynthesisEngine) Version() string { return "0.6.0" }
func (SynthesisEngine) Description() string {
	return "Synthesizes an executive repository summary from frozen RIE artifacts"
}

func (engine SynthesisEngine) Execute(ctx context.Context, run *rie.RunContext) error {
	if run == nil {
		return rie.ErrRunContextRequired
	}
	if engine.config.MonorepoMinimumProjects < 1 || engine.config.MonorepoMinimumMembers < 1 {
		return ErrInvalidConfig
	}
	discoveryInventory, available := discovery.InventoryFrom(run)
	if !available || discoveryInventory.ArtifactVersion() != discovery.DiscoveryInventoryArtifactVersion {
		return ErrDiscoveryRequired
	}
	snapshot, available := rie.RepositorySnapshotFrom(run)
	if !available || snapshot.ArtifactVersion() != rie.RepositorySnapshotArtifactVersion {
		return ErrSnapshotRequired
	}
	languageInventory, available := languageengine.InventoryFrom(run)
	if !available || languageInventory.ArtifactVersion() != languageengine.LanguageInventoryArtifactVersion {
		return ErrLanguageRequired
	}
	frameworkInventory, available := frameworkengine.InventoryFrom(run)
	if !available || frameworkInventory.ArtifactVersion() != frameworkengine.FrameworkInventoryArtifactVersion {
		return ErrFrameworkRequired
	}
	buildInventory, available := buildengine.InventoryFrom(run)
	if !available || buildInventory.ArtifactVersion() != buildengine.BuildInventoryArtifactVersion {
		return ErrBuildRequired
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	layout, err := synthesizeLayout(ctx, snapshot)
	if err != nil {
		return err
	}
	identity := discoveryInventory.Repository()
	languages := synthesizeLanguages(languageInventory)
	frameworks := synthesizeFrameworks(frameworkInventory)
	buildOverview, projectLocations := synthesizeBuild(buildInventory)
	workspaceCount, moduleCount, memberThresholdReached := workspaceFacts(buildInventory, engine.config.MonorepoMinimumMembers)
	monorepo := len(projectLocations) >= engine.config.MonorepoMinimumProjects || memberThresholdReached
	sources := []rie.ArtifactReference{
		{Name: discoveryInventory.ArtifactName(), Version: discoveryInventory.ArtifactVersion()},
		{Name: snapshot.ArtifactName(), Version: snapshot.ArtifactVersion()},
		{Name: languageInventory.ArtifactName(), Version: languageInventory.ArtifactVersion()},
		{Name: frameworkInventory.ArtifactName(), Version: frameworkInventory.ArtifactVersion()},
		{Name: buildInventory.ArtifactName(), Version: buildInventory.ArtifactVersion()},
	}
	inventory := RepositoryMetadata{
		metadata:   rie.ArtifactMetadata{Name: RepositoryMetadataArtifactName, Version: RepositoryMetadataArtifactVersion, EngineName: engine.Name(), EngineVersion: engine.Version()},
		repository: RepositoryIdentity{Name: identity.Name, RootPath: identity.RootPath, Git: GitInformation{Present: identity.Git, CurrentBranch: identity.CurrentBranch, DefaultBranch: identity.DefaultBranch}},
		statistics: snapshot.Statistics(), layout: layout, monorepo: monorepo,
		workspaceCount: workspaceCount, declaredModuleCount: moduleCount,
		languages: languages, frameworks: frameworks, build: buildOverview, sources: sources,
	}
	if err := run.Artifacts.Put(inventory); err != nil {
		return err
	}
	run.Report.Metadata = reportFromMetadata(inventory)
	return nil
}

func synthesizeLayout(ctx context.Context, snapshot rie.RepositorySnapshot) (Layout, error) {
	directories := map[string]struct{}{}
	files := map[string]struct{}{}
	maximumDepth := 0
	err := snapshot.ForEachEntry(func(entry rie.RepositoryEntry) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		cleaned := strings.Trim(entry.Path, "/")
		if cleaned == "" || cleaned == "." {
			return nil
		}
		depth := strings.Count(cleaned, "/") + 1
		if depth > maximumDepth {
			maximumDepth = depth
		}
		separator := strings.IndexByte(cleaned, '/')
		topLevel := cleaned
		if separator >= 0 {
			topLevel = cleaned[:separator]
		}
		if separator >= 0 || entry.IsDir {
			directories[topLevel] = struct{}{}
		} else {
			files[topLevel] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return Layout{}, err
	}
	return Layout{topLevelDirectories: sortedKeys(directories), topLevelFiles: sortedKeys(files), MaximumDepth: maximumDepth}, nil
}

func synthesizeLanguages(inventory languageengine.LanguageInventory) []Language {
	items := inventory.Items()
	result := make([]Language, 0, len(items))
	for _, item := range items {
		result = append(result, Language{Name: item.Name, FileCount: item.Count, Percentage: item.Percentage})
	}
	return result
}

func synthesizeFrameworks(inventory frameworkengine.FrameworkInventory) []Framework {
	result := make([]Framework, 0, len(inventory.Items()))
	for _, item := range inventory.Items() {
		locations := map[string]struct{}{}
		for _, itemEvidence := range item.Evidence() {
			locations[locationOf(itemEvidence.File)] = struct{}{}
		}
		result = append(result, Framework{Name: item.Name, Ecosystem: item.Ecosystem, locations: sortedKeys(locations)})
	}
	return result
}

func synthesizeBuild(inventory buildengine.BuildInventory) (BuildOverview, map[string]struct{}) {
	projectLocations := map[string]struct{}{}
	managerMap := map[string]*Technology{}
	for _, item := range inventory.PackageManagers() {
		projectLocations[item.Location] = struct{}{}
		mergeTechnology(managerMap, item.ID, item.Name, item.Location)
	}
	buildMap := map[string]*Technology{}
	for _, item := range inventory.BuildSystems() {
		projectLocations[item.Location] = struct{}{}
		mergeTechnology(buildMap, item.ID, item.Name, item.Location)
	}
	toolchainMap := map[string]*Toolchain{}
	for _, item := range inventory.Toolchains() {
		toolchain := toolchainMap[item.Tool]
		if toolchain == nil {
			toolchain = &Toolchain{Tool: item.Tool}
			toolchainMap[item.Tool] = toolchain
		}
		toolchain.constraints = appendUnique(toolchain.constraints, item.Constraint)
		toolchain.locations = appendUnique(toolchain.locations, item.Location)
	}
	overview := BuildOverview{LockFileCount: len(inventory.LockFiles())}
	for _, item := range managerMap {
		sort.Strings(item.locations)
		overview.packageManagers = append(overview.packageManagers, *item)
	}
	for _, item := range buildMap {
		sort.Strings(item.locations)
		overview.buildSystems = append(overview.buildSystems, *item)
	}
	for _, item := range toolchainMap {
		sort.Strings(item.constraints)
		sort.Strings(item.locations)
		overview.toolchains = append(overview.toolchains, *item)
	}
	sort.Slice(overview.packageManagers, func(i, j int) bool { return overview.packageManagers[i].ID < overview.packageManagers[j].ID })
	sort.Slice(overview.buildSystems, func(i, j int) bool { return overview.buildSystems[i].ID < overview.buildSystems[j].ID })
	sort.Slice(overview.toolchains, func(i, j int) bool { return overview.toolchains[i].Tool < overview.toolchains[j].Tool })
	return overview, projectLocations
}

func workspaceFacts(inventory buildengine.BuildInventory, memberThreshold int) (int, int, bool) {
	modules := map[string]struct{}{}
	thresholdReached := false
	workspaces := inventory.Workspaces()
	for _, workspace := range workspaces {
		members := workspace.Members()
		if len(members) >= memberThreshold {
			thresholdReached = true
		}
		for _, member := range members {
			modules[workspace.Location+"\x00"+member] = struct{}{}
		}
	}
	return len(workspaces), len(modules), thresholdReached
}

func mergeTechnology(items map[string]*Technology, id, name, location string) {
	item := items[id]
	if item == nil {
		item = &Technology{ID: id, Name: name}
		items[id] = item
	}
	item.locations = appendUnique(item.locations, location)
}

func appendUnique(items []string, value string) []string {
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

func sortedKeys(items map[string]struct{}) []string {
	result := make([]string, 0, len(items))
	for item := range items {
		result = append(result, item)
	}
	sort.Strings(result)
	return result
}

func locationOf(file string) string {
	location := path.Dir(file)
	if location == "" {
		return "."
	}
	return location
}

func reportFromMetadata(inventory RepositoryMetadata) rie.RepositoryMetadataSummary {
	repository := inventory.Repository()
	layout := inventory.Layout()
	report := rie.RepositoryMetadataSummary{
		Name: repository.Name, RootPath: repository.RootPath,
		Git:        rie.GitMetadata{Present: repository.Git.Present, CurrentBranch: repository.Git.CurrentBranch, DefaultBranch: repository.Git.DefaultBranch},
		Statistics: inventory.Statistics(),
		Layout:     rie.RepositoryLayout{TopLevelDirectories: layout.TopLevelDirectories(), TopLevelFiles: layout.TopLevelFiles(), MaximumDepth: layout.MaximumDepth},
		Monorepo:   inventory.Monorepo(), WorkspaceCount: inventory.WorkspaceCount(), DeclaredModuleCount: inventory.DeclaredModuleCount(),
		Languages: []rie.MetadataLanguage{}, Frameworks: []rie.MetadataFramework{},
		Build:           rie.MetadataBuildSummary{PackageManagers: []rie.MetadataTechnology{}, BuildSystems: []rie.MetadataTechnology{}, Toolchains: []rie.MetadataToolchain{}, LockFileCount: inventory.Build().LockFileCount},
		SourceArtifacts: inventory.SourceArtifacts(),
	}
	for _, item := range inventory.Languages() {
		report.Languages = append(report.Languages, rie.MetadataLanguage{Name: item.Name, FileCount: item.FileCount, Percentage: item.Percentage})
	}
	for _, item := range inventory.Frameworks() {
		report.Frameworks = append(report.Frameworks, rie.MetadataFramework{Name: item.Name, Ecosystem: item.Ecosystem, Locations: item.Locations()})
	}
	buildOverview := inventory.Build()
	for _, item := range buildOverview.PackageManagers() {
		report.Build.PackageManagers = append(report.Build.PackageManagers, rie.MetadataTechnology{ID: item.ID, Name: item.Name, Locations: item.Locations()})
	}
	for _, item := range buildOverview.BuildSystems() {
		report.Build.BuildSystems = append(report.Build.BuildSystems, rie.MetadataTechnology{ID: item.ID, Name: item.Name, Locations: item.Locations()})
	}
	for _, item := range buildOverview.Toolchains() {
		report.Build.Toolchains = append(report.Build.Toolchains, rie.MetadataToolchain{Tool: item.Tool, Constraints: item.Constraints(), Locations: item.Locations()})
	}
	return report
}
