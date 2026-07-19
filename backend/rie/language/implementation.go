package language

import (
	"context"
	"math"
	"path"
	"sort"
	"strings"

	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
)

// extensionEngine identifies languages from normalized file extensions.
type extensionEngine struct {
	config Config
}

// New returns Language Engine with optional package-specific configuration.
func New(configs ...Config) Engine {
	config := DefaultConfig()
	if len(configs) > 0 {
		config = configs[0]
	}
	config.Extensions = normalizedMappings(config.Extensions)
	return extensionEngine{config: config}
}

func (extensionEngine) Name() string { return "language" }

func (extensionEngine) Version() string { return "0.3.2" }

func (extensionEngine) Description() string {
	return "Detects repository languages deterministically from file extensions"
}

// Execute detects languages in entries retained by Ignore Engine.
func (engine extensionEngine) Execute(ctx context.Context, run *rie.RunContext) error {
	if run == nil {
		return rie.ErrRunContextRequired
	}
	snapshot, available := rie.RepositorySnapshotFrom(run)
	if !available {
		return ErrIgnoreRequired
	}
	if len(engine.config.Extensions) == 0 {
		return ErrNoExtensionMappings
	}
	if run.Artifacts == nil {
		run.Artifacts = rie.NewArtifactStore()
	}

	counts := make(map[string]int)
	unknownFiles := 0
	err := snapshot.ForEachEntry(func(entry rie.RepositoryEntry) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if entry.IsDir {
			return nil
		}
		extension := strings.ToLower(path.Ext(entry.Path))
		languageName, detected := engine.config.Extensions[extension]
		if !detected {
			unknownFiles++
			return nil
		}
		counts[languageName]++
		return nil
	})
	if err != nil {
		return err
	}

	detectedFiles := 0
	for _, count := range counts {
		detectedFiles += count
	}
	items := make([]LanguageItem, 0, len(counts))
	for name, count := range counts {
		percentage := 0.0
		if detectedFiles > 0 {
			percentage = math.Round((float64(count)/float64(detectedFiles))*10000) / 100
		}
		items = append(items, LanguageItem{Name: name, Count: count, Percentage: percentage})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Name < items[j].Name
		}
		return items[i].Count > items[j].Count
	})

	inventory := newLanguageInventory(items, LanguageSummary{
		DetectedFiles: detectedFiles, UnknownFiles: unknownFiles,
	})
	if err := run.Artifacts.Put(inventory); err != nil {
		return err
	}
	reportItems := make([]rie.Language, 0, len(items))
	for _, item := range items {
		reportItems = append(reportItems, rie.Language{
			Name: item.Name, FileCount: item.Count, Percentage: item.Percentage,
		})
	}
	run.Report.Languages = rie.LanguageSummary{
		DetectedFiles: detectedFiles, UnknownFiles: unknownFiles, Items: reportItems,
	}
	return nil
}

func normalizedMappings(mappings map[string]string) map[string]string {
	normalized := make(map[string]string, len(mappings))
	for extension, name := range mappings {
		extension = strings.ToLower(strings.TrimSpace(extension))
		name = strings.TrimSpace(name)
		if extension == "" || name == "" {
			continue
		}
		if !strings.HasPrefix(extension, ".") {
			extension = "." + extension
		}
		normalized[extension] = name
	}
	return normalized
}
