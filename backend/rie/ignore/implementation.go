package ignore

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
)

// ruleEngine applies ordered repository ignore rules to discovered entries.
type ruleEngine struct {
	config Config
}

// New returns Ignore Engine with optional package-specific configuration.
func New(configs ...Config) Engine {
	config := DefaultConfig()
	if len(configs) > 0 {
		config = configs[0]
	}
	return ruleEngine{config: config}
}

func (ruleEngine) Name() string { return "ignore" }

func (ruleEngine) Version() string { return "0.2.1" }

func (ruleEngine) Description() string {
	return "Loads ordered ignore rules and excludes matching repository entries"
}

// Execute filters Discovery Engine entries and recomputes public statistics.
func (engine ruleEngine) Execute(ctx context.Context, run *rie.RunContext) error {
	if run == nil {
		return rie.ErrRunContextRequired
	}
	if run.Report.Repository.RootPath == "" || run.Entries == nil {
		return ErrDiscoveryRequired
	}

	rules, sources := engine.loadRules(ctx, run)
	kept := make([]rie.RepositoryEntry, 0, len(run.Entries))
	summary := Summary{Rules: len(rules), Sources: sources}
	statistics := rie.Statistics{}

	for _, entry := range run.Entries {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		ignored := matchesRules(entry, rules)
		if !run.Config.ScanHidden && hasHiddenSegment(entry.Path) {
			ignored = true
		}
		if ignored {
			if entry.IsDir {
				summary.IgnoredFolders++
			} else {
				summary.IgnoredFiles++
			}
			continue
		}

		kept = append(kept, entry)
		if entry.IsDir {
			statistics.Folders++
		} else {
			statistics.Files++
		}
	}

	run.Entries = kept
	run.Report.Statistics = statistics
	run.Report.Ignore = summary
	if run.Artifacts == nil {
		run.Artifacts = rie.NewArtifactStore()
	}
	snapshot := rie.NewRepositorySnapshot(
		run.Report.Repository.RootPath, kept, statistics, run.Report.Warnings, engine.Version(),
	)
	return run.Artifacts.Put(snapshot)
}

func (engine ruleEngine) loadRules(ctx context.Context, run *rie.RunContext) ([]rule, []string) {
	ignoreFiles := engine.findIgnoreFiles(run.Entries)
	rules := make([]rule, 0, len(ignoreFiles)*8+len(run.Config.IgnorePatterns))
	sources := make([]string, 0, len(ignoreFiles)+1)

	for _, relativePath := range ignoreFiles {
		if ctx.Err() != nil {
			break
		}
		absolutePath := filepath.Join(run.Report.Repository.RootPath, filepath.FromSlash(relativePath))
		file, err := os.Open(absolutePath)
		if err != nil {
			run.Report.Warnings = append(run.Report.Warnings, rie.Diagnostic{
				Engine: engine.Name(), Code: "ignore_file_unreadable", Message: err.Error(), Path: relativePath,
			})
			continue
		}

		basePath := path.Dir(relativePath)
		if basePath == "." {
			basePath = ""
		}
		parsed, parseWarnings := parseRules(file, basePath, relativePath)
		_ = file.Close()
		rules = append(rules, parsed...)
		run.Report.Warnings = append(run.Report.Warnings, parseWarnings...)
		sources = append(sources, relativePath)
	}

	if len(run.Config.IgnorePatterns) > 0 {
		parsed, parseWarnings := parsePatternList(run.Config.IgnorePatterns, "", "config")
		rules = append(rules, parsed...)
		run.Report.Warnings = append(run.Report.Warnings, parseWarnings...)
		sources = append(sources, "config")
	}
	return rules, sources
}

func (engine ruleEngine) findIgnoreFiles(entries []rie.RepositoryEntry) []string {
	names := make(map[string]struct{}, len(engine.config.IgnoreFileNames))
	for _, name := range engine.config.IgnoreFileNames {
		names[name] = struct{}{}
	}
	files := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir {
			continue
		}
		if _, ok := names[path.Base(entry.Path)]; ok {
			files = append(files, entry.Path)
		}
	}
	sort.Slice(files, func(i, j int) bool {
		leftDepth := strings.Count(files[i], "/")
		rightDepth := strings.Count(files[j], "/")
		if leftDepth == rightDepth {
			return files[i] < files[j]
		}
		return leftDepth < rightDepth
	})
	return files
}

func parseRules(file *os.File, basePath, source string) ([]rule, []rie.Diagnostic) {
	patterns := make([]string, 0, 16)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		patterns = append(patterns, scanner.Text())
	}
	rules, warnings := parsePatternList(patterns, basePath, source)
	if err := scanner.Err(); err != nil {
		warnings = append(warnings, rie.Diagnostic{
			Engine: "ignore", Code: "ignore_file_read_failed", Message: err.Error(), Path: source,
		})
	}
	return rules, warnings
}

func parsePatternList(patterns []string, basePath, source string) ([]rule, []rie.Diagnostic) {
	rules := make([]rule, 0, len(patterns))
	warnings := make([]rie.Diagnostic, 0)
	for lineNumber, raw := range patterns {
		parsed, ok, err := compileRule(raw, basePath, source)
		if err != nil {
			warnings = append(warnings, rie.Diagnostic{
				Engine:  "ignore",
				Code:    "invalid_ignore_pattern",
				Message: fmt.Sprintf("line %d: %v", lineNumber+1, err),
				Path:    source,
			})
			continue
		}
		if ok {
			rules = append(rules, parsed)
		}
	}
	return rules, warnings
}

func compileRule(raw, basePath, source string) (rule, bool, error) {
	pattern := strings.TrimSpace(raw)
	if pattern == "" || strings.HasPrefix(pattern, "#") {
		return rule{}, false, nil
	}
	if strings.HasPrefix(pattern, `\#`) {
		pattern = pattern[1:]
	}

	negated := false
	if strings.HasPrefix(pattern, `\!`) {
		pattern = pattern[1:]
	} else if strings.HasPrefix(pattern, "!") {
		negated = true
		pattern = strings.TrimPrefix(pattern, "!")
	}
	pattern = strings.ReplaceAll(pattern, "\\", "/")
	directory := strings.HasSuffix(pattern, "/")
	pattern = strings.TrimSuffix(pattern, "/")
	anchored := strings.HasPrefix(pattern, "/")
	pattern = strings.TrimPrefix(pattern, "/")
	if pattern == "" {
		return rule{}, false, nil
	}

	expression, err := globExpression(pattern, anchored)
	if err != nil {
		return rule{}, false, err
	}
	matcher, err := regexp.Compile(expression)
	if err != nil {
		return rule{}, false, err
	}
	return rule{
		pattern: pattern, basePath: basePath, source: source,
		negated: negated, directory: directory, matcher: matcher,
	}, true, nil
}

func globExpression(pattern string, anchored bool) (string, error) {
	var expression strings.Builder
	if anchored || strings.Contains(pattern, "/") {
		expression.WriteString("^")
	} else {
		expression.WriteString(`(?:^|/)`)
	}

	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				if i+2 < len(pattern) && pattern[i+2] == '/' {
					expression.WriteString(`(?:.*/)?`)
					i += 2
				} else {
					expression.WriteString(".*")
					i++
				}
			} else {
				expression.WriteString(`[^/]*`)
			}
		case '?':
			expression.WriteString(`[^/]`)
		case '[':
			return "", fmt.Errorf("character classes are not supported yet: %q", pattern)
		default:
			expression.WriteString(regexp.QuoteMeta(string(pattern[i])))
		}
	}
	expression.WriteString(`$`)
	return expression.String(), nil
}

func matchesRules(entry rie.RepositoryEntry, rules []rule) bool {
	ignored := false
	for _, candidate := range rules {
		target, applies := pathWithinBase(entry.Path, candidate.basePath)
		if !applies || !ruleMatchesTarget(candidate, target, entry.IsDir) {
			continue
		}
		ignored = !candidate.negated
	}
	return ignored
}

func ruleMatchesTarget(candidate rule, target string, targetIsDir bool) bool {
	if candidate.matcher.MatchString(target) && (!candidate.directory || targetIsDir) {
		return true
	}
	for parent := path.Dir(target); parent != "." && parent != "/"; parent = path.Dir(parent) {
		if candidate.matcher.MatchString(parent) {
			return true
		}
	}
	return false
}

func pathWithinBase(relativePath, basePath string) (string, bool) {
	if basePath == "" {
		return relativePath, true
	}
	if relativePath == basePath {
		return "", true
	}
	prefix := basePath + "/"
	if !strings.HasPrefix(relativePath, prefix) {
		return "", false
	}
	return strings.TrimPrefix(relativePath, prefix), true
}

func hasHiddenSegment(relativePath string) bool {
	for _, segment := range strings.Split(relativePath, "/") {
		if strings.HasPrefix(segment, ".") && segment != "." && segment != ".." {
			return true
		}
	}
	return false
}
