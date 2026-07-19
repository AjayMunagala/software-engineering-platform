package language

import (
	"context"
	"math"
	"path"
	"sort"
	"strings"

	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
)

// ExtensionEngine identifies languages from normalized file extensions.
type ExtensionEngine struct {
	config Config
}

// New returns Language Engine with optional package-specific configuration.
func New(configs ...Config) Engine {
	config := DefaultConfig()
	if len(configs) > 0 {
		config = configs[0]
	}
	config.Extensions = normalizedMappings(config.Extensions)
	return ExtensionEngine{config: config}
}

func (ExtensionEngine) Name() string { return "language" }

func (ExtensionEngine) Version() string { return "0.3.0" }

func (ExtensionEngine) Description() string {
	return "Detects repository languages deterministically from file extensions"
}

// Execute detects languages in entries retained by Ignore Engine.
func (engine ExtensionEngine) Execute(ctx context.Context, run *rie.RunContext) error {
	if run == nil {
		return rie.ErrRunContextRequired
	}
	if _, completed := run.CompletedEngines["ignore"]; !completed {
		return ErrIgnoreRequired
	}
	if len(engine.config.Extensions) == 0 {
		return ErrNoExtensionMappings
	}

	counts := make(map[string]int)
	unknownFiles := 0
	for _, entry := range run.Entries {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if entry.IsDir {
			continue
		}
		extension := strings.ToLower(path.Ext(entry.Path))
		languageName, detected := engine.config.Extensions[extension]
		if !detected {
			unknownFiles++
			continue
		}
		counts[languageName]++
	}

	detectedFiles := 0
	for _, count := range counts {
		detectedFiles += count
	}
	items := make([]Detection, 0, len(counts))
	for name, count := range counts {
		percentage := 0.0
		if detectedFiles > 0 {
			percentage = math.Round((float64(count)/float64(detectedFiles))*10000) / 100
		}
		items = append(items, Detection{Name: name, FileCount: count, Percentage: percentage})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].FileCount == items[j].FileCount {
			return items[i].Name < items[j].Name
		}
		return items[i].FileCount > items[j].FileCount
	})

	run.Report.Languages = Summary{
		DetectedFiles: detectedFiles,
		UnknownFiles:  unknownFiles,
		Items:         items,
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
