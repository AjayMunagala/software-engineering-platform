package build

import (
	"bufio"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
)

// intelligenceEngine inventories build and package facts from RepositorySnapshot.
type intelligenceEngine struct{ config Config }

func New(configs ...Config) Engine {
	config := DefaultConfig()
	if len(configs) > 0 {
		config = configs[0]
	}
	return intelligenceEngine{config: config}
}

func (intelligenceEngine) Name() string { return "build-package" }

func (intelligenceEngine) Version() string { return "0.5.0" }

func (intelligenceEngine) Description() string {
	return "Deterministically discovers build systems, package managers, workspaces, lock files, and toolchains"
}

func (engine intelligenceEngine) Execute(ctx context.Context, run *rie.RunContext) error {
	if run == nil {
		return rie.ErrRunContextRequired
	}
	snapshot, available := rie.RepositorySnapshotFrom(run)
	if !available || snapshot.ArtifactVersion() != rie.RepositorySnapshotArtifactVersion {
		return ErrSnapshotRequired
	}
	if engine.config.MaxManifestSize <= 0 {
		return ErrInvalidManifestSize
	}
	if len(engine.config.Detectors) == 0 {
		return ErrNoDetectors
	}
	detectorIndex, err := indexDetectors(engine.config.Detectors)
	if err != nil {
		return err
	}
	if run.Artifacts == nil {
		run.Artifacts = rie.NewArtifactStore()
	}

	findings := make([]Finding, 0)
	contents := make(map[string][]byte)
	readErrors := make(map[string]error)
	warnedReads := make(map[string]struct{})
	err = snapshot.ForEachEntry(func(entry rie.RepositoryEntry) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir {
			return nil
		}
		for _, detector := range detectorIndex[path.Base(entry.Path)] {
			candidate := Candidate{Path: entry.Path}
			if detector.RequiresContent() {
				content, cached := contents[entry.Path]
				readErr := readErrors[entry.Path]
				if !cached && readErr == nil {
					absolute := filepath.Join(snapshot.RootPath(), filepath.FromSlash(entry.Path))
					content, readErr = readBounded(absolute, engine.config.MaxManifestSize)
					if readErr != nil {
						readErrors[entry.Path] = readErr
					} else {
						contents[entry.Path] = content
					}
				}
				if readErr != nil {
					if _, warned := warnedReads[entry.Path]; !warned {
						code := "build_manifest_unreadable"
						if errors.Is(readErr, ErrManifestTooLarge) {
							code = "build_manifest_too_large"
						}
						run.Report.Warnings = append(run.Report.Warnings, rie.Diagnostic{Engine: engine.Name(), Code: code, Message: readErr.Error(), Path: entry.Path})
						warnedReads[entry.Path] = struct{}{}
					}
					continue
				}
				candidate.Content = content
			}
			detected, err := detector.Detect(ctx, candidate)
			if err != nil {
				if contextErr := ctx.Err(); contextErr != nil {
					return contextErr
				}
				run.Report.Warnings = append(run.Report.Warnings, rie.Diagnostic{Engine: engine.Name(), Code: "build_manifest_invalid", Message: err.Error(), Path: entry.Path})
				continue
			}
			findings = append(findings, detected...)
		}
		return nil
	})
	if err != nil {
		return err
	}

	inventory := newBuildInventory(findings)
	run.Report.Warnings = append(run.Report.Warnings, managerConflictWarnings(inventory)...)
	if err := run.Artifacts.Put(inventory); err != nil {
		return err
	}
	run.Report.Build = reportFromInventory(inventory)
	return nil
}

func indexDetectors(detectors []Detector) (map[string][]Detector, error) {
	seen := make(map[string]struct{}, len(detectors))
	index := make(map[string][]Detector)
	for _, detector := range detectors {
		if detector == nil || strings.TrimSpace(detector.ID()) == "" {
			return nil, ErrInvalidDetector
		}
		if _, duplicate := seen[detector.ID()]; duplicate {
			return nil, ErrInvalidDetector
		}
		if len(detector.FileNames()) == 0 {
			return nil, ErrInvalidDetector
		}
		seen[detector.ID()] = struct{}{}
		for _, fileName := range detector.FileNames() {
			if fileName == "" || path.Base(fileName) != fileName {
				return nil, ErrInvalidDetector
			}
			index[fileName] = append(index[fileName], detector)
		}
	}
	return index, nil
}

func readBounded(filePath string, limit int64) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > limit {
		return nil, fmt.Errorf("%w: %s", ErrManifestTooLarge, filePath)
	}
	return content, nil
}

type manifestDetector struct {
	id              string
	fileNames       map[string]struct{}
	requiresContent bool
	detect          func(Candidate) ([]Finding, error)
}

func newManifestDetector(id string, requiresContent bool, detect func(Candidate) ([]Finding, error), names ...string) Detector {
	fileNames := make(map[string]struct{}, len(names))
	for _, name := range names {
		fileNames[name] = struct{}{}
	}
	return manifestDetector{id: id, fileNames: fileNames, requiresContent: requiresContent, detect: detect}
}

func (detector manifestDetector) ID() string { return detector.id }

func (detector manifestDetector) FileNames() []string {
	names := make([]string, 0, len(detector.fileNames))
	for name := range detector.fileNames {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (detector manifestDetector) RequiresContent() bool { return detector.requiresContent }

func (detector manifestDetector) Detect(ctx context.Context, candidate Candidate) ([]Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return detector.detect(candidate)
}

func defaultDetectors() []Detector {
	return []Detector{
		newManifestDetector("go-mod", true, detectGoMod, "go.mod"),
		newManifestDetector("go-work", true, detectGoWork, "go.work"),
		newManifestDetector("node-package", true, detectPackageJSON, "package.json"),
		newManifestDetector("maven", true, detectMaven, "pom.xml"),
		newManifestDetector("gradle-build", false, detectGradleBuild, "build.gradle", "build.gradle.kts"),
		newManifestDetector("gradle-settings", true, detectGradleSettings, "settings.gradle", "settings.gradle.kts"),
		newManifestDetector("python-requirements", false, detectRequirements, "requirements.txt"),
		newManifestDetector("python-project", true, detectPyProject, "pyproject.toml"),
		newManifestDetector("cargo", true, detectCargo, "Cargo.toml"),
		newManifestDetector("composer", true, detectComposer, "composer.json"),
		newManifestDetector("npm-lock", false, lockDetector("npm", "npm", "npm"), "package-lock.json", "npm-shrinkwrap.json"),
		newManifestDetector("pnpm-lock", false, lockDetector("pnpm", "pnpm", "pnpm"), "pnpm-lock.yaml"),
		newManifestDetector("yarn-lock", false, lockDetector("yarn", "Yarn", "yarn"), "yarn.lock"),
		newManifestDetector("bun-lock", false, lockDetector("bun", "Bun", "bun"), "bun.lock", "bun.lockb"),
		newManifestDetector("cargo-lock", false, lockDetector("cargo", "Cargo", "cargo"), "Cargo.lock"),
		newManifestDetector("poetry-lock", false, lockDetector("poetry", "Poetry", "poetry"), "poetry.lock"),
		newManifestDetector("uv-lock", false, lockDetector("uv", "uv", "uv"), "uv.lock"),
		newManifestDetector("composer-lock", false, lockDetector("composer", "Composer", "composer"), "composer.lock"),
		newManifestDetector("pnpm-workspace", true, detectPnpmWorkspace, "pnpm-workspace.yaml"),
	}
}

func locationOf(relativePath string) string {
	location := path.Dir(relativePath)
	if location == "" {
		return "."
	}
	return location
}

func evidence(file, rule, value string) rie.Evidence {
	return rie.Evidence{File: file, Rule: rule, Value: value}
}

func toolFindings(id, name, location string, itemEvidence rie.Evidence, kinds ...FindingKind) []Finding {
	items := make([]Finding, 0, len(kinds))
	for _, kind := range kinds {
		items = append(items, Finding{Kind: kind, ID: id, Name: name, Location: location, Evidence: itemEvidence})
	}
	return items
}

func detectGoMod(candidate Candidate) ([]Finding, error) {
	location := locationOf(candidate.Path)
	items := toolFindings("go-modules", "Go Modules", location, evidence(candidate.Path, "manifest.presence", "go.mod"), PackageManagerFinding)
	items = append(items, toolFindings("go-toolchain", "Go Toolchain", location, evidence(candidate.Path, "manifest.presence", "go.mod"), BuildSystemFinding)...)
	for _, line := range linesWithoutComments(string(candidate.Content), "//") {
		fields := strings.Fields(line)
		if len(fields) == 2 && (fields[0] == "go" || fields[0] == "toolchain") {
			tool := "Go"
			constraint := fields[1]
			items = append(items, Finding{Kind: ToolchainFinding, Name: tool, Location: location, Constraint: constraint, Evidence: evidence(candidate.Path, "go."+fields[0], constraint)})
		}
	}
	return items, nil
}

func detectGoWork(candidate Candidate) ([]Finding, error) {
	location := locationOf(candidate.Path)
	members := parseGoWorkMembers(string(candidate.Content))
	items := []Finding{{Kind: WorkspaceFinding, ID: "go-workspace", Name: "Go Workspace", Location: location, Members: members, Evidence: evidence(candidate.Path, "workspace.declaration", "go.work")}}
	for _, line := range linesWithoutComments(string(candidate.Content), "//") {
		fields := strings.Fields(line)
		if len(fields) == 2 && (fields[0] == "go" || fields[0] == "toolchain") {
			items = append(items, Finding{Kind: ToolchainFinding, Name: "Go", Location: location, Constraint: fields[1], Evidence: evidence(candidate.Path, "go."+fields[0], fields[1])})
		}
	}
	return items, nil
}

func parseGoWorkMembers(content string) []string {
	members := []string{}
	inBlock := false
	for _, line := range linesWithoutComments(content, "//") {
		line = strings.TrimSpace(line)
		if line == "use (" {
			inBlock = true
			continue
		}
		if inBlock && line == ")" {
			inBlock = false
			continue
		}
		if strings.HasPrefix(line, "use ") {
			members = appendUniqueStrings(members, strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "use ")), `"`))
			continue
		}
		if inBlock && line != "" {
			members = appendUniqueStrings(members, strings.Trim(line, `"`))
		}
	}
	sort.Strings(members)
	return members
}

type packageJSON struct {
	PackageManager string            `json:"packageManager"`
	Engines        map[string]string `json:"engines"`
	Workspaces     json.RawMessage   `json:"workspaces"`
}

func detectPackageJSON(candidate Candidate) ([]Finding, error) {
	var manifest packageJSON
	if err := json.Unmarshal(candidate.Content, &manifest); err != nil {
		return nil, err
	}
	location := locationOf(candidate.Path)
	items := []Finding{}
	if manifest.PackageManager != "" {
		manager, constraint, _ := strings.Cut(manifest.PackageManager, "@")
		if name := packageManagerName(manager); name != "" {
			items = append(items, toolFindings(manager, name, location, evidence(candidate.Path, "package_json.package_manager", manifest.PackageManager), PackageManagerFinding)...)
			if constraint != "" {
				items = append(items, Finding{Kind: ToolchainFinding, Name: name, Location: location, Constraint: constraint, Evidence: evidence(candidate.Path, "package_json.package_manager", manifest.PackageManager)})
			}
		}
	}
	engineNames := make([]string, 0, len(manifest.Engines))
	for name := range manifest.Engines {
		engineNames = append(engineNames, name)
	}
	sort.Strings(engineNames)
	for _, name := range engineNames {
		constraint := manifest.Engines[name]
		items = append(items, Finding{Kind: ToolchainFinding, Name: toolchainName(name), Location: location, Constraint: constraint, Evidence: evidence(candidate.Path, "package_json.engines", name+"="+constraint)})
	}
	members, err := parseNodeWorkspaces(manifest.Workspaces)
	if err != nil {
		return nil, err
	}
	if len(members) > 0 {
		items = append(items, Finding{Kind: WorkspaceFinding, ID: "node-workspace", Name: "Node Workspace", Location: location, Members: members, Evidence: evidence(candidate.Path, "package_json.workspaces", "workspaces")})
	}
	return items, nil
}

func packageManagerName(id string) string {
	switch strings.ToLower(id) {
	case "npm":
		return "npm"
	case "pnpm":
		return "pnpm"
	case "yarn":
		return "Yarn"
	case "bun":
		return "Bun"
	}
	return ""
}

func toolchainName(id string) string {
	switch strings.ToLower(id) {
	case "node", "nodejs":
		return "Node.js"
	case "python":
		return "Python"
	case "go":
		return "Go"
	case "rust":
		return "Rust"
	case "php":
		return "PHP"
	default:
		return id
	}
}

func parseNodeWorkspaces(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var direct []string
	if err := json.Unmarshal(raw, &direct); err == nil {
		sort.Strings(direct)
		return direct, nil
	}
	var object struct {
		Packages []string `json:"packages"`
	}
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, err
	}
	sort.Strings(object.Packages)
	return object.Packages, nil
}

type mavenProject struct {
	Modules []string `xml:"modules>module"`
}

func detectMaven(candidate Candidate) ([]Finding, error) {
	var manifest mavenProject
	if err := xml.Unmarshal(candidate.Content, &manifest); err != nil {
		return nil, err
	}
	location := locationOf(candidate.Path)
	ev := evidence(candidate.Path, "manifest.presence", "pom.xml")
	items := toolFindings("maven", "Maven", location, ev, PackageManagerFinding, BuildSystemFinding)
	if len(manifest.Modules) > 0 {
		items = append(items, Finding{Kind: WorkspaceFinding, ID: "maven-multi-module", Name: "Maven Multi-module", Location: location, Members: trimStrings(manifest.Modules), Evidence: evidence(candidate.Path, "maven.modules", "modules")})
	}
	return items, nil
}

func detectGradleBuild(candidate Candidate) ([]Finding, error) {
	return toolFindings("gradle", "Gradle", locationOf(candidate.Path), evidence(candidate.Path, "manifest.presence", path.Base(candidate.Path)), BuildSystemFinding), nil
}

func detectGradleSettings(candidate Candidate) ([]Finding, error) {
	location := locationOf(candidate.Path)
	items := toolFindings("gradle", "Gradle", location, evidence(candidate.Path, "manifest.presence", path.Base(candidate.Path)), BuildSystemFinding)
	members := parseGradleMembers(string(candidate.Content))
	if len(members) > 0 {
		items = append(items, Finding{Kind: WorkspaceFinding, ID: "gradle-multi-project", Name: "Gradle Multi-project", Location: location, Members: members, Evidence: evidence(candidate.Path, "gradle.include", "include")})
	}
	return items, nil
}

func parseGradleMembers(content string) []string {
	members := []string{}
	for _, line := range linesWithoutComments(content, "//") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "include") {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(line, "include"))
		rest = strings.Trim(rest, "()")
		for _, member := range strings.Split(rest, ",") {
			members = appendUniqueStrings(members, strings.Trim(strings.TrimSpace(member), `'"`))
		}
	}
	sort.Strings(members)
	return members
}

func detectRequirements(candidate Candidate) ([]Finding, error) {
	return toolFindings("pip", "pip", locationOf(candidate.Path), evidence(candidate.Path, "manifest.presence", "requirements.txt"), PackageManagerFinding), nil
}

func detectPyProject(candidate Candidate) ([]Finding, error) {
	location := locationOf(candidate.Path)
	section := ""
	items := []Finding{}
	seenManagers := map[string]bool{}
	for _, raw := range linesWithoutComments(string(candidate.Content), "#") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.Trim(line, "[]")
			continue
		}
		key, value, ok := splitAssignment(line)
		if !ok {
			continue
		}
		value = strings.Trim(value, `'"`)
		if section == "build-system" && key == "build-backend" {
			id := normalizeID(strings.Split(value, ".")[0])
			items = append(items, Finding{Kind: BuildSystemFinding, ID: id, Name: value, Location: location, Evidence: evidence(candidate.Path, "pyproject.build_backend", value)})
		}
		if section == "project" && key == "requires-python" {
			items = append(items, Finding{Kind: ToolchainFinding, Name: "Python", Location: location, Constraint: value, Evidence: evidence(candidate.Path, "pyproject.requires_python", value)})
		}
	}
	text := string(candidate.Content)
	for id, name := range map[string]string{"poetry": "Poetry", "uv": "uv"} {
		if strings.Contains(text, "[tool."+id+"]") && !seenManagers[id] {
			items = append(items, toolFindings(id, name, location, evidence(candidate.Path, "pyproject.tool", id), PackageManagerFinding)...)
			seenManagers[id] = true
		}
	}
	return items, nil
}

func detectCargo(candidate Candidate) ([]Finding, error) {
	location := locationOf(candidate.Path)
	ev := evidence(candidate.Path, "manifest.presence", "Cargo.toml")
	items := toolFindings("cargo", "Cargo", location, ev, PackageManagerFinding, BuildSystemFinding)
	section := ""
	for _, raw := range linesWithoutComments(string(candidate.Content), "#") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.Trim(line, "[]")
			continue
		}
		key, value, ok := splitAssignment(line)
		if !ok {
			continue
		}
		value = strings.Trim(value, `'"`)
		if section == "package" && key == "rust-version" {
			items = append(items, Finding{Kind: ToolchainFinding, Name: "Rust", Location: location, Constraint: value, Evidence: evidence(candidate.Path, "cargo.rust_version", value)})
		}
	}
	members := parseTOMLStringArray(string(candidate.Content), "workspace", "members")
	if strings.Contains(string(candidate.Content), "[workspace]") {
		items = append(items, Finding{Kind: WorkspaceFinding, ID: "cargo-workspace", Name: "Cargo Workspace", Location: location, Members: members, Evidence: evidence(candidate.Path, "cargo.workspace", "workspace")})
	}
	return items, nil
}

type composerManifest struct {
	Require map[string]string `json:"require"`
}

func detectComposer(candidate Candidate) ([]Finding, error) {
	var manifest composerManifest
	if err := json.Unmarshal(candidate.Content, &manifest); err != nil {
		return nil, err
	}
	location := locationOf(candidate.Path)
	items := toolFindings("composer", "Composer", location, evidence(candidate.Path, "manifest.presence", "composer.json"), PackageManagerFinding)
	if constraint := manifest.Require["php"]; constraint != "" {
		items = append(items, Finding{Kind: ToolchainFinding, Name: "PHP", Location: location, Constraint: constraint, Evidence: evidence(candidate.Path, "composer.require_php", constraint)})
	}
	return items, nil
}

func lockDetector(managerID, managerName, ruleValue string) func(Candidate) ([]Finding, error) {
	return func(candidate Candidate) ([]Finding, error) {
		location := locationOf(candidate.Path)
		ev := evidence(candidate.Path, "lockfile.presence", path.Base(candidate.Path))
		items := toolFindings(managerID, managerName, location, ev, PackageManagerFinding)
		items = append(items, Finding{Kind: LockFileFinding, PackageManagerID: managerID, Path: candidate.Path, Location: location, Evidence: ev, ID: ruleValue})
		return items, nil
	}
}

func detectPnpmWorkspace(candidate Candidate) ([]Finding, error) {
	location := locationOf(candidate.Path)
	members := []string{}
	inPackages := false
	for _, raw := range linesWithoutComments(string(candidate.Content), "#") {
		line := strings.TrimSpace(raw)
		if line == "packages:" {
			inPackages = true
			continue
		}
		if inPackages && strings.HasPrefix(line, "-") {
			members = appendUniqueStrings(members, strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "-")), `'"`))
			continue
		}
		if inPackages && line != "" {
			inPackages = false
		}
	}
	sort.Strings(members)
	ev := evidence(candidate.Path, "workspace.declaration", "pnpm-workspace.yaml")
	items := toolFindings("pnpm", "pnpm", location, ev, PackageManagerFinding)
	items = append(items, Finding{Kind: WorkspaceFinding, ID: "pnpm-workspace", Name: "pnpm Workspace", Location: location, Members: members, Evidence: ev})
	return items, nil
}

func linesWithoutComments(content, marker string) []string {
	lines := []string{}
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if index := strings.Index(line, marker); index >= 0 {
			line = line[:index]
		}
		lines = append(lines, line)
	}
	return lines
}

func splitAssignment(line string) (string, string, bool) {
	index := strings.Index(line, "=")
	if index <= 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:index]), strings.TrimSpace(line[index+1:]), true
}

func parseTOMLStringArray(content, wantedSection, wantedKey string) []string {
	section := ""
	collecting := false
	var value strings.Builder
	for _, raw := range linesWithoutComments(content, "#") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") && !collecting {
			section = strings.Trim(line, "[]")
			continue
		}
		if collecting {
			value.WriteString(line)
			if strings.Contains(line, "]") {
				break
			}
			continue
		}
		key, assigned, ok := splitAssignment(line)
		if section == wantedSection && ok && key == wantedKey {
			value.WriteString(assigned)
			if !strings.Contains(assigned, "]") {
				collecting = true
			} else {
				break
			}
		}
	}
	text := strings.Trim(value.String(), "[] ")
	if text == "" {
		return nil
	}
	items := []string{}
	for _, item := range strings.Split(text, ",") {
		items = appendUniqueStrings(items, strings.Trim(strings.TrimSpace(item), `'"`))
	}
	sort.Strings(items)
	return items
}

func trimStrings(values []string) []string {
	result := []string{}
	for _, value := range values {
		result = appendUniqueStrings(result, strings.TrimSpace(value))
	}
	sort.Strings(result)
	return result
}

func normalizeID(value string) string {
	value = strings.ToLower(value)
	var builder strings.Builder
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '-' {
			builder.WriteRune(character)
		} else if builder.Len() > 0 {
			builder.WriteByte('-')
		}
	}
	return strings.Trim(builder.String(), "-")
}

func managerConflictWarnings(inventory BuildInventory) []rie.Diagnostic {
	locations := make(map[string]map[string]struct{})
	for _, manager := range inventory.PackageManagers() {
		if !isNodeManager(manager.ID) {
			continue
		}
		if locations[manager.Location] == nil {
			locations[manager.Location] = make(map[string]struct{})
		}
		locations[manager.Location][manager.ID] = struct{}{}
	}
	warnings := []rie.Diagnostic{}
	for location, managers := range locations {
		if len(managers) < 2 {
			continue
		}
		names := make([]string, 0, len(managers))
		for manager := range managers {
			names = append(names, manager)
		}
		sort.Strings(names)
		warnings = append(warnings, rie.Diagnostic{Engine: "build-package", Code: "multiple_package_managers", Message: "multiple Node package managers detected: " + strings.Join(names, ", "), Path: location})
	}
	sort.Slice(warnings, func(i, j int) bool { return warnings[i].Path < warnings[j].Path })
	return warnings
}

func isNodeManager(id string) bool { return id == "npm" || id == "pnpm" || id == "yarn" || id == "bun" }

func reportFromInventory(inventory BuildInventory) rie.BuildSummary {
	report := rie.BuildSummary{PackageManagers: []rie.BuildTool{}, BuildSystems: []rie.BuildTool{}, Workspaces: []rie.BuildWorkspace{}, LockFiles: []rie.BuildLockFile{}, Toolchains: []rie.BuildToolchain{}}
	for _, item := range inventory.PackageManagers() {
		report.PackageManagers = append(report.PackageManagers, rie.BuildTool{ID: item.ID, Name: item.Name, Location: item.Location, Evidence: item.Evidence()})
	}
	for _, item := range inventory.BuildSystems() {
		report.BuildSystems = append(report.BuildSystems, rie.BuildTool{ID: item.ID, Name: item.Name, Location: item.Location, Evidence: item.Evidence()})
	}
	for _, item := range inventory.Workspaces() {
		report.Workspaces = append(report.Workspaces, rie.BuildWorkspace{ID: item.ID, Kind: item.Kind, Location: item.Location, Members: item.Members(), Evidence: item.Evidence()})
	}
	for _, item := range inventory.LockFiles() {
		report.LockFiles = append(report.LockFiles, rie.BuildLockFile{PackageManagerID: item.PackageManagerID, Path: item.Path, Location: item.Location, Evidence: item.Evidence()})
	}
	for _, item := range inventory.Toolchains() {
		report.Toolchains = append(report.Toolchains, rie.BuildToolchain{Tool: item.Tool, Constraint: item.Constraint, Location: item.Location, Evidence: item.Evidence()})
	}
	return report
}
