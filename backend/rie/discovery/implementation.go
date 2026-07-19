package discovery

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
)

// fileSystemEngine discovers repository entries using the local filesystem.
type fileSystemEngine struct {
	config Config
}

// New returns Discovery Engine with optional package-specific configuration.
func New(configs ...Config) Engine {
	config := DefaultConfig()
	if len(configs) > 0 {
		config = configs[0]
	}
	return fileSystemEngine{config: config}
}

func (fileSystemEngine) Name() string { return "discovery" }

func (fileSystemEngine) Version() string { return "0.1.1" }

func (fileSystemEngine) Description() string {
	return "Discovers repository identity and normalized local filesystem entries"
}

// Execute discovers repository entries and stores them for later engines.
func (engine fileSystemEngine) Execute(ctx context.Context, run *rie.RunContext) error {
	if run == nil {
		return rie.ErrRunContextRequired
	}

	report, entries, err := engine.discover(ctx, run.RepositoryPath)
	if err != nil {
		return err
	}
	run.Entries = entries
	run.Report.Repository = report.Repository
	run.Report.Statistics = report.Statistics
	if run.Artifacts == nil {
		run.Artifacts = rie.NewArtifactStore()
	}
	currentBranch, defaultBranch := localGitBranches(report.Repository.RootPath, report.Repository.Git)
	inventory := newDiscoveryInventory(RepositoryIdentity{
		Name: report.Repository.Name, RootPath: report.Repository.RootPath, Git: report.Repository.Git,
		CurrentBranch: currentBranch, DefaultBranch: defaultBranch,
	}, report.Statistics)
	return run.Artifacts.Put(inventory)
}

// Scan runs Discovery Engine by itself and returns a complete RIE report.
func (engine fileSystemEngine) Scan(repositoryPath string) (Report, error) {
	run := rie.NewRunContext(repositoryPath, rie.DefaultConfig())
	pipeline := rie.New()
	if err := pipeline.Register(engine); err != nil {
		return Report{}, err
	}
	err := pipeline.Run(context.Background(), run)
	return run.Report, err
}

func localGitBranches(rootPath string, isGit bool) (string, string) {
	if !isGit {
		return "", ""
	}
	gitDirectory := filepath.Join(rootPath, ".git")
	info, err := os.Stat(gitDirectory)
	if err != nil || !info.IsDir() {
		return "", ""
	}
	current := symbolicRefName(filepath.Join(gitDirectory, "HEAD"), "refs/heads/")
	defaultBranch := symbolicRefName(filepath.Join(gitDirectory, "refs", "remotes", "origin", "HEAD"), "refs/remotes/origin/")
	return current, defaultBranch
}

func symbolicRefName(filePath, prefix string) string {
	file, err := os.Open(filePath)
	if err != nil {
		return ""
	}
	defer file.Close()
	scanner := bufio.NewScanner(io.LimitReader(file, 4096))
	if !scanner.Scan() {
		return ""
	}
	line := strings.TrimSpace(scanner.Text())
	if !strings.HasPrefix(line, "ref: "+prefix) {
		return ""
	}
	return strings.TrimPrefix(line, "ref: "+prefix)
}

func (engine fileSystemEngine) discover(ctx context.Context, repositoryPath string) (Report, []Entry, error) {
	if repositoryPath == "" {
		return Report{}, nil, ErrRepositoryPathRequired
	}

	rootPath, err := filepath.Abs(filepath.Clean(repositoryPath))
	if err != nil {
		return Report{}, nil, fmt.Errorf("resolve repository path: %w", err)
	}

	info, err := os.Stat(rootPath)
	if err != nil {
		return Report{}, nil, fmt.Errorf("inspect repository root: %w", err)
	}
	if !info.IsDir() {
		return Report{}, nil, fmt.Errorf("%w: %s", ErrRepositoryNotDirectory, rootPath)
	}

	report := Report{Repository: rie.Repository{Name: filepath.Base(rootPath), RootPath: rootPath}}
	entries := make([]Entry, 0, 256)

	err = filepath.WalkDir(rootPath, func(path string, entry fs.DirEntry, walkErr error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if walkErr != nil {
			return fmt.Errorf("walk %s: %w", path, walkErr)
		}
		if path == rootPath {
			return nil
		}

		relativePath, relErr := filepath.Rel(rootPath, path)
		if relErr != nil {
			return fmt.Errorf("make path relative %s: %w", path, relErr)
		}
		relativePath = filepath.ToSlash(relativePath)

		if entry.Name() == ".git" {
			report.Repository.Git = true
			if engine.config.ExcludeGitMetadata {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		entries = append(entries, Entry{Path: relativePath, IsDir: entry.IsDir()})
		if entry.IsDir() {
			report.Statistics.Folders++
		} else {
			report.Statistics.Files++
		}
		return nil
	})
	if err != nil {
		return Report{}, nil, err
	}

	return report, entries, nil
}
