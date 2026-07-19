package framework

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
	languageengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/language"
)

// manifestEngine detects frameworks from declared manifest dependencies.
type manifestEngine struct {
	config        Config
	manifestNames map[string]struct{}
}

// New returns Framework Engine with optional package-specific configuration.
func New(configs ...Config) Engine {
	config := DefaultConfig()
	if len(configs) > 0 {
		config = configs[0]
	}
	names := make(map[string]struct{}, len(config.ManifestNames))
	for _, name := range config.ManifestNames {
		name = strings.ToLower(strings.TrimSpace(name))
		if name != "" {
			names[name] = struct{}{}
		}
	}
	return manifestEngine{config: config, manifestNames: names}
}

func (manifestEngine) Name() string { return "framework" }

func (manifestEngine) Version() string { return "0.4.2" }

func (manifestEngine) Description() string {
	return "Detects frameworks deterministically from supported dependency manifests"
}

// Execute inspects recognized manifests retained by Ignore Engine.
func (engine manifestEngine) Execute(ctx context.Context, run *rie.RunContext) error {
	if run == nil {
		return rie.ErrRunContextRequired
	}
	if _, available := languageengine.InventoryFrom(run); !available {
		return ErrLanguageRequired
	}
	snapshot, available := rie.RepositorySnapshotFrom(run)
	if !available {
		return ErrSnapshotRequired
	}
	if len(engine.manifestNames) == 0 {
		return ErrNoManifestNames
	}
	if engine.config.MaxManifestSize <= 0 {
		return ErrInvalidManifestLimit
	}
	if run.Artifacts == nil {
		run.Artifacts = rie.NewArtifactStore()
	}

	candidates := engine.manifestCandidates(snapshot)
	detected := make(map[string]map[string]rie.Evidence)
	manifestsInspected := 0
	for _, relativePath := range candidates {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		absolutePath := filepath.Join(snapshot.RootPath(), filepath.FromSlash(relativePath))
		content, err := readManifest(absolutePath, engine.config.MaxManifestSize)
		if err != nil {
			code := "manifest_unreadable"
			if errors.Is(err, ErrManifestTooLarge) {
				code = "manifest_too_large"
			}
			run.Report.Warnings = append(run.Report.Warnings, rie.Diagnostic{
				Engine: engine.Name(), Code: code, Message: err.Error(), Path: relativePath,
			})
			continue
		}
		manifestsInspected++

		frameworks, err := detectFrameworks(path.Base(relativePath), content)
		if err != nil {
			run.Report.Warnings = append(run.Report.Warnings, rie.Diagnostic{
				Engine: engine.Name(), Code: "manifest_invalid", Message: err.Error(), Path: relativePath,
			})
			continue
		}
		for _, framework := range frameworks {
			key := framework.ecosystem + "\x00" + framework.name
			if detected[key] == nil {
				detected[key] = make(map[string]rie.Evidence)
			}
			evidence := rie.Evidence{File: relativePath, Rule: framework.rule, Value: framework.value}
			evidenceKey := evidence.File + "\x00" + evidence.Rule + "\x00" + evidence.Value
			detected[key][evidenceKey] = evidence
		}
	}

	items := frameworkItems(detected)
	inventory := newFrameworkInventory(items, FrameworkSummary{ManifestsInspected: manifestsInspected})
	if err := run.Artifacts.Put(inventory); err != nil {
		return err
	}
	reportItems := make([]rie.Framework, 0, len(items))
	for _, item := range items {
		reportItems = append(reportItems, rie.Framework{
			Name: item.Name, Ecosystem: item.Ecosystem, Evidence: item.Evidence(),
		})
	}
	run.Report.Frameworks = rie.FrameworkSummary{
		ManifestsInspected: manifestsInspected, Items: reportItems,
	}
	return nil
}

func (engine manifestEngine) manifestCandidates(snapshot rie.RepositorySnapshot) []string {
	candidates := make([]string, 0)
	seen := make(map[string]struct{})
	_ = snapshot.ForEachEntry(func(entry rie.RepositoryEntry) error {
		if entry.IsDir {
			return nil
		}
		if _, supported := engine.manifestNames[strings.ToLower(path.Base(entry.Path))]; !supported {
			return nil
		}
		if _, duplicate := seen[entry.Path]; !duplicate {
			seen[entry.Path] = struct{}{}
			candidates = append(candidates, entry.Path)
		}
		return nil
	})
	sort.Strings(candidates)
	return candidates
}

func readManifest(filePath string, limit int64) ([]byte, error) {
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

func detectFrameworks(fileName string, content []byte) ([]detection, error) {
	switch strings.ToLower(fileName) {
	case "package.json":
		return detectNodeFrameworks(content)
	case "go.mod":
		return detectGoFrameworks(content), nil
	case "pom.xml":
		return detectMavenFrameworks(content)
	case "cargo.toml":
		return detectCargoFrameworks(content), nil
	case "composer.json":
		return detectComposerFrameworks(content)
	case "requirements.txt":
		return detectPythonFrameworks(content), nil
	default:
		return nil, nil
	}
}

type dependencyManifest struct {
	Dependencies     map[string]json.RawMessage `json:"dependencies"`
	DevDependencies  map[string]json.RawMessage `json:"devDependencies"`
	PeerDependencies map[string]json.RawMessage `json:"peerDependencies"`
	Require          map[string]json.RawMessage `json:"require"`
	RequireDev       map[string]json.RawMessage `json:"require-dev"`
}

func detectNodeFrameworks(content []byte) ([]detection, error) {
	var manifest dependencyManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		return nil, err
	}
	dependencies := mergeDependencyNames(manifest.Dependencies, manifest.DevDependencies, manifest.PeerDependencies)
	rules := map[string]detection{
		"react":            {name: "React", ecosystem: "Node.js", rule: "node.dependency"},
		"redux":            {name: "Redux", ecosystem: "Node.js", rule: "node.dependency"},
		"@reduxjs/toolkit": {name: "Redux", ecosystem: "Node.js", rule: "node.dependency"},
		"next":             {name: "Next.js", ecosystem: "Node.js", rule: "node.dependency"},
		"vue":              {name: "Vue", ecosystem: "Node.js", rule: "node.dependency"},
		"@angular/core":    {name: "Angular", ecosystem: "Node.js", rule: "node.dependency"},
		"express":          {name: "Express", ecosystem: "Node.js", rule: "node.dependency"},
		"@nestjs/core":     {name: "NestJS", ecosystem: "Node.js", rule: "node.dependency"},
		"svelte":           {name: "Svelte", ecosystem: "Node.js", rule: "node.dependency"},
	}
	return matchedDependencies(dependencies, rules), nil
}

func detectComposerFrameworks(content []byte) ([]detection, error) {
	var manifest dependencyManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		return nil, err
	}
	dependencies := mergeDependencyNames(manifest.Require, manifest.RequireDev)
	rules := map[string]detection{
		"laravel/framework":        {name: "Laravel", ecosystem: "PHP", rule: "composer.require"},
		"symfony/framework-bundle": {name: "Symfony", ecosystem: "PHP", rule: "composer.require"},
	}
	return matchedDependencies(dependencies, rules), nil
}

func mergeDependencyNames(groups ...map[string]json.RawMessage) map[string]struct{} {
	dependencies := make(map[string]struct{})
	for _, group := range groups {
		for name := range group {
			dependencies[strings.ToLower(name)] = struct{}{}
		}
	}
	return dependencies
}

func matchedDependencies(dependencies map[string]struct{}, rules map[string]detection) []detection {
	results := make([]detection, 0)
	for dependency, framework := range rules {
		if _, present := dependencies[dependency]; !present {
			continue
		}
		framework.value = dependency
		results = append(results, framework)
	}
	return results
}

func detectGoFrameworks(content []byte) []detection {
	text := uncommentLines(string(content), "//")
	rules := map[string]detection{
		"github.com/gin-gonic/gin": {name: "Gin", ecosystem: "Go", rule: "go.require"},
		"github.com/labstack/echo": {name: "Echo", ecosystem: "Go", rule: "go.require"},
		"github.com/gofiber/fiber": {name: "Fiber", ecosystem: "Go", rule: "go.require"},
		"github.com/go-chi/chi":    {name: "Chi", ecosystem: "Go", rule: "go.require"},
	}
	return matchedText(text, rules)
}

type mavenProject struct {
	Dependencies []mavenDependency `xml:"dependencies>dependency"`
}

type mavenDependency struct {
	GroupID    string `xml:"groupId"`
	ArtifactID string `xml:"artifactId"`
}

func detectMavenFrameworks(content []byte) ([]detection, error) {
	var project mavenProject
	if err := xml.Unmarshal(content, &project); err != nil {
		return nil, err
	}
	results := make([]detection, 0)
	seen := make(map[string]struct{})
	for _, dependency := range project.Dependencies {
		group := strings.TrimSpace(dependency.GroupID)
		artifact := strings.TrimSpace(dependency.ArtifactID)
		coordinate := group + ":" + artifact
		var framework detection
		if group == "org.springframework.boot" || strings.HasPrefix(artifact, "spring-boot-") {
			framework = detection{name: "Spring Boot", ecosystem: "Java", rule: "maven.dependency", value: coordinate}
		} else if strings.HasPrefix(group, "org.springframework") {
			framework = detection{name: "Spring Framework", ecosystem: "Java", rule: "maven.dependency", value: coordinate}
		} else {
			continue
		}
		key := framework.name + "\x00" + framework.value
		if _, duplicate := seen[key]; !duplicate {
			seen[key] = struct{}{}
			results = append(results, framework)
		}
	}
	return results, nil
}

func detectCargoFrameworks(content []byte) []detection {
	text := uncommentLines(string(content), "#")
	rules := map[string]detection{
		"actix-web": {name: "Actix Web", ecosystem: "Rust", rule: "cargo.dependency"},
		"axum":      {name: "Axum", ecosystem: "Rust", rule: "cargo.dependency"},
		"rocket":    {name: "Rocket", ecosystem: "Rust", rule: "cargo.dependency"},
	}
	results := make([]detection, 0)
	seen := make(map[string]struct{})
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		name := ""
		if separator := strings.Index(line, "="); separator > 0 {
			name = strings.Trim(strings.TrimSpace(line[:separator]), `"'`)
		} else if strings.HasPrefix(line, "[dependencies.") && strings.HasSuffix(line, "]") {
			name = strings.TrimSuffix(strings.TrimPrefix(line, "[dependencies."), "]")
		}
		framework, matches := rules[name]
		if matches {
			key := framework.ecosystem + "\x00" + framework.name
			if _, duplicate := seen[key]; !duplicate {
				seen[key] = struct{}{}
				framework.value = name
				results = append(results, framework)
			}
		}
	}
	return results
}

func detectPythonFrameworks(content []byte) []detection {
	rules := map[string]detection{
		"django":  {name: "Django", ecosystem: "Python", rule: "python.requirement"},
		"flask":   {name: "Flask", ecosystem: "Python", rule: "python.requirement"},
		"fastapi": {name: "FastAPI", ecosystem: "Python", rule: "python.requirement"},
	}
	results := make([]detection, 0)
	seen := make(map[string]struct{})
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		line := strings.TrimSpace(strings.SplitN(scanner.Text(), "#", 2)[0])
		name := strings.ToLower(line)
		if index := strings.IndexAny(name, "<>=!~[ ;"); index >= 0 {
			name = name[:index]
		}
		framework, matches := rules[name]
		if matches {
			key := framework.ecosystem + "\x00" + framework.name
			if _, duplicate := seen[key]; !duplicate {
				seen[key] = struct{}{}
				framework.value = name
				results = append(results, framework)
			}
		}
	}
	return results
}

func uncommentLines(text, marker string) string {
	lines := strings.Split(text, "\n")
	for index, line := range lines {
		if comment := strings.Index(line, marker); comment >= 0 {
			lines[index] = line[:comment]
		}
	}
	return strings.Join(lines, "\n")
}

func matchedText(text string, rules map[string]detection) []detection {
	results := make([]detection, 0)
	for token, framework := range rules {
		if strings.Contains(text, token) {
			framework.value = token
			results = append(results, framework)
		}
	}
	return results
}

func frameworkItems(detected map[string]map[string]rie.Evidence) []FrameworkItem {
	items := make([]FrameworkItem, 0, len(detected))
	for key, evidenceSet := range detected {
		parts := strings.SplitN(key, "\x00", 2)
		evidence := make([]rie.Evidence, 0, len(evidenceSet))
		for _, itemEvidence := range evidenceSet {
			evidence = append(evidence, itemEvidence)
		}
		sort.Slice(evidence, func(i, j int) bool {
			if evidence[i].File != evidence[j].File {
				return evidence[i].File < evidence[j].File
			}
			if evidence[i].Rule != evidence[j].Rule {
				return evidence[i].Rule < evidence[j].Rule
			}
			return evidence[i].Value < evidence[j].Value
		})
		items = append(items, FrameworkItem{Name: parts[1], Ecosystem: parts[0], evidence: evidence})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Name == items[j].Name {
			return items[i].Ecosystem < items[j].Ecosystem
		}
		return items[i].Name < items[j].Name
	})
	return items
}
