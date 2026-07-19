package discovery

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
)

// FileSystemEngine discovers repository entries using the local filesystem.
type FileSystemEngine struct {
	config Config
}

// New returns Discovery Engine with optional package-specific configuration.
func New(configs ...Config) Engine {
	config := DefaultConfig()
	if len(configs) > 0 {
		config = configs[0]
	}
	return FileSystemEngine{config: config}
}

// NewFileSystemScanner is retained as a compatibility alias for New.
func NewFileSystemScanner() Engine {
	return New()
}

func (FileSystemEngine) Name() string { return "discovery" }

func (FileSystemEngine) Version() string { return "0.1.0" }

func (FileSystemEngine) Description() string {
	return "Discovers repository identity and normalized local filesystem entries"
}

// Execute discovers repository entries and stores them for later engines.
func (engine FileSystemEngine) Execute(ctx context.Context, run *rie.RunContext) error {
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
	return nil
}

// Scan runs Discovery Engine by itself and returns a complete RIE report.
func (engine FileSystemEngine) Scan(repositoryPath string) (Report, error) {
	run := rie.NewRunContext(repositoryPath, rie.DefaultConfig())
	pipeline := rie.New()
	if err := pipeline.Register(engine); err != nil {
		return Report{}, err
	}
	err := pipeline.Run(context.Background(), run)
	return run.Report, err
}

func (engine FileSystemEngine) discover(ctx context.Context, repositoryPath string) (Report, []Entry, error) {
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
