package semantic

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/AjayMunagala/software-engineering-platform/backend/lie"
	"github.com/AjayMunagala/software-engineering-platform/backend/lie/golang"
	"github.com/AjayMunagala/software-engineering-platform/backend/lie/golang/packageidentity"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
)

type engine struct{ config Config }

func (*engine) Name() string         { return "go-semantic" }
func (*engine) Version() string      { return engineVersion }
func (*engine) Language() string     { return "Go" }
func (*engine) ArtifactName() string { return ArtifactName }
func (*engine) Description() string {
	return "Verifies snapshot-authorized Go source and produces a bounded semantic candidate artifact"
}

func (engine *engine) Resolve(ctx context.Context, input Input) (GoSemanticInventory, error) {
	if ctx == nil {
		return GoSemanticInventory{}, ErrContextRequired
	}
	if err := ctx.Err(); err != nil {
		return GoSemanticInventory{}, err
	}
	if err := validateInput(input); err != nil {
		return GoSemanticInventory{}, err
	}

	syntaxFiles := input.Syntax.Files()
	candidatePaths := make([]string, 0, len(syntaxFiles))
	for _, file := range syntaxFiles {
		if file.Status == golang.FileStatusParsed {
			candidatePaths = append(candidatePaths, file.Path)
		}
	}
	reader, err := newSourceReader(input.Snapshot.RootPath(), candidatePaths)
	if err != nil {
		return GoSemanticInventory{}, err
	}
	defer reader.close()

	outcomes := make([]fileOutcome, len(syntaxFiles))
	jobs := make(chan int)
	workerCount := min(engine.config.MaxWorkers, max(1, len(syntaxFiles)))
	var workers sync.WaitGroup
	workers.Add(workerCount)
	for range workerCount {
		go func() {
			defer workers.Done()
			for index := range jobs {
				if ctx.Err() != nil {
					return
				}
				outcomes[index] = engine.verifyFile(ctx, reader, syntaxFiles[index])
			}
		}()
	}
	for index := range syntaxFiles {
		select {
		case jobs <- index:
		case <-ctx.Done():
			close(jobs)
			workers.Wait()
			return GoSemanticInventory{}, ctx.Err()
		}
	}
	close(jobs)
	workers.Wait()
	if err := ctx.Err(); err != nil {
		return GoSemanticInventory{}, err
	}

	files, diagnostics, statistics := collectOutcomes(outcomes)
	sortDiagnostics(diagnostics)
	diagnostics, omitted := limitDiagnostics(diagnostics, engine.config.MaxDiagnosticsPerFile, engine.config.MaxDiagnostics)
	statistics.Diagnostics = len(diagnostics)
	statistics.OmittedDiagnostics = omitted
	if err := ctx.Err(); err != nil {
		return GoSemanticInventory{}, err
	}
	return newInventory(files, diagnostics, statistics), nil
}

func validateInput(input Input) error {
	snapshotMetadata := input.Snapshot.Metadata()
	if input.Snapshot.RootPath() == "" || snapshotMetadata.Name == "" {
		return ErrMissingRepositorySnapshot
	}
	if input.Snapshot.ArtifactName() != rie.RepositorySnapshotArtifactName ||
		input.Snapshot.ArtifactVersion() != rie.RepositorySnapshotArtifactVersion ||
		snapshotMetadata.Name != rie.RepositorySnapshotArtifactName ||
		snapshotMetadata.Version != rie.RepositorySnapshotArtifactVersion {
		return ErrIncompatibleRepositorySnapshot
	}

	syntaxMetadata := input.Syntax.Metadata()
	if syntaxMetadata.Name == "" {
		return ErrMissingGoLanguageInventory
	}
	if input.Syntax.ArtifactName() != golang.ArtifactName || input.Syntax.ArtifactVersion() != golang.ArtifactVersion ||
		syntaxMetadata.Name != golang.ArtifactName || syntaxMetadata.Version != golang.ArtifactVersion {
		return ErrIncompatibleGoInventory
	}

	identityMetadata := input.PackageIdentities.Metadata()
	if identityMetadata.Name == "" {
		return ErrMissingPackageIdentityInventory
	}
	if input.PackageIdentities.ArtifactName() != packageidentity.ArtifactName ||
		input.PackageIdentities.ArtifactVersion() != packageidentity.ArtifactVersion ||
		identityMetadata.Name != packageidentity.ArtifactName || identityMetadata.Version != packageidentity.ArtifactVersion {
		return ErrIncompatiblePackageIdentity
	}

	snapshotReference := rie.ArtifactReference{Name: rie.RepositorySnapshotArtifactName, Version: rie.RepositorySnapshotArtifactVersion}
	syntaxReference := rie.ArtifactReference{Name: golang.ArtifactName, Version: golang.ArtifactVersion}
	if !hasArtifactReference(input.Syntax.SourceArtifacts(), snapshotReference) ||
		!hasExactArtifactReferences(input.PackageIdentities.SourceArtifacts(), []rie.ArtifactReference{snapshotReference, syntaxReference}) {
		return ErrArtifactProvenanceMismatch
	}

	entries := make(map[string]bool)
	for _, entry := range input.Snapshot.Entries() {
		if _, duplicate := entries[entry.Path]; duplicate {
			return fmt.Errorf("%w: duplicate snapshot entry %s", ErrArtifactProvenanceMismatch, entry.Path)
		}
		entries[entry.Path] = entry.IsDir
	}
	fileIDs := make(map[string]struct{})
	filePaths := make(map[string]struct{})
	for _, file := range input.Syntax.Files() {
		isDirectory, exists := entries[file.Path]
		if file.ID == "" || file.Path == "" || !exists || isDirectory {
			return fmt.Errorf("%w: Go file %s is absent from RepositorySnapshot", ErrArtifactProvenanceMismatch, file.Path)
		}
		if _, duplicate := fileIDs[file.ID]; duplicate {
			return fmt.Errorf("%w: duplicate Go file ID %s", ErrArtifactProvenanceMismatch, file.ID)
		}
		if _, duplicate := filePaths[file.Path]; duplicate {
			return fmt.Errorf("%w: duplicate Go file path %s", ErrArtifactProvenanceMismatch, file.Path)
		}
		fileIDs[file.ID] = struct{}{}
		filePaths[file.Path] = struct{}{}
	}

	packageIDs := make(map[string]struct{})
	for _, pkg := range input.Syntax.Packages() {
		packageIDs[pkg.ID] = struct{}{}
	}
	for _, proof := range input.PackageIdentities.Proofs() {
		if _, exists := packageIDs[proof.ImportingPackageID]; !exists {
			return fmt.Errorf("%w: proof %s imports from unknown package %s", ErrArtifactProvenanceMismatch, proof.ID, proof.ImportingPackageID)
		}
		if proof.TargetPackageID != "" {
			if _, exists := packageIDs[proof.TargetPackageID]; !exists {
				return fmt.Errorf("%w: proof %s targets unknown package %s", ErrArtifactProvenanceMismatch, proof.ID, proof.TargetPackageID)
			}
		}
		for _, candidate := range proof.CandidatePackageIDs {
			if _, exists := packageIDs[candidate]; !exists {
				return fmt.Errorf("%w: proof %s names unknown candidate %s", ErrArtifactProvenanceMismatch, proof.ID, candidate)
			}
		}
	}
	return nil
}

func hasArtifactReference(values []rie.ArtifactReference, expected rie.ArtifactReference) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func hasExactArtifactReferences(values, expected []rie.ArtifactReference) bool {
	if len(values) != len(expected) {
		return false
	}
	remaining := make(map[rie.ArtifactReference]int, len(expected))
	for _, value := range expected {
		remaining[value]++
	}
	for _, value := range values {
		if remaining[value] == 0 {
			return false
		}
		remaining[value]--
	}
	return true
}

type fileOutcome struct {
	path        string
	file        SemanticFile
	diagnostics []lie.Diagnostic
}

func (engine *engine) verifyFile(ctx context.Context, reader *sourceReader, source golang.GoFile) fileOutcome {
	outcome := fileOutcome{
		path:        source.Path,
		file:        SemanticFile{FileID: source.ID, PackageID: source.PackageID},
		diagnostics: []lie.Diagnostic{},
	}
	switch source.Status {
	case golang.FileStatusFailed:
		outcome.file.Status = SemanticFileFailed
		outcome.diagnostics = append(outcome.diagnostics, semanticDiagnostic(lie.SeverityWarning, "semantic_prerequisite_file_failed", "Phase 2.1 did not produce a parsed source file", source.Path))
		return outcome
	case golang.FileStatusSkipped:
		outcome.file.Status = SemanticFileSkipped
		outcome.diagnostics = append(outcome.diagnostics, semanticDiagnostic(lie.SeverityWarning, "semantic_prerequisite_file_skipped", "Phase 2.1 skipped this source file", source.Path))
		return outcome
	case golang.FileStatusParsed:
	default:
		outcome.file.Status = SemanticFileFailed
		outcome.diagnostics = append(outcome.diagnostics, semanticDiagnostic(lie.SeverityError, "semantic_prerequisite_file_invalid", "Phase 2.1 supplied an unknown file status", source.Path))
		return outcome
	}
	if source.SizeBytes > engine.config.MaxSourceFileSize {
		outcome.file.Status = SemanticFileSkipped
		outcome.diagnostics = append(outcome.diagnostics, semanticDiagnostic(lie.SeverityWarning, "semantic_source_oversized", fmt.Sprintf("source exceeds the configured %d-byte limit", engine.config.MaxSourceFileSize), source.Path))
		return outcome
	}
	if source.ContentDigest == "" {
		outcome.file.Status = SemanticFileFailed
		outcome.diagnostics = append(outcome.diagnostics, semanticDiagnostic(lie.SeverityError, "semantic_digest_missing", "Phase 2.1 source digest is missing", source.Path))
		return outcome
	}
	if err := ctx.Err(); err != nil {
		return outcome
	}
	data, err := reader.readFile(source.Path, engine.config.MaxSourceFileSize)
	if err != nil {
		outcome.file.Status = SemanticFileFailed
		code, severity := "semantic_source_unreadable", lie.SeverityWarning
		switch {
		case errors.Is(err, ErrSourceOutsideRoot):
			code, severity = "semantic_source_outside_root", lie.SeverityError
		case errors.Is(err, ErrSourceMissing):
			code = "semantic_source_missing"
		case errors.Is(err, ErrSourceOversized):
			code = "semantic_source_oversized"
			outcome.file.Status = SemanticFileSkipped
		}
		outcome.diagnostics = append(outcome.diagnostics, semanticDiagnostic(severity, code, err.Error(), source.Path))
		return outcome
	}
	if err := ctx.Err(); err != nil {
		return outcome
	}
	digestBytes := sha256.Sum256(data)
	digest := fmt.Sprintf("sha256:%x", digestBytes)
	if digest != source.ContentDigest {
		outcome.file.Status = SemanticFileStale
		outcome.diagnostics = append(outcome.diagnostics, semanticDiagnostic(lie.SeverityWarning, "semantic_digest_mismatch", "source digest no longer matches GoLanguageInventory", source.Path))
		return outcome
	}
	outcome.file.Status = SemanticFilePartial
	outcome.file.ContentDigest = digest
	return outcome
}

func collectOutcomes(outcomes []fileOutcome) ([]SemanticFile, []lie.Diagnostic, SemanticStatistics) {
	sort.Slice(outcomes, func(i, j int) bool {
		if outcomes[i].path != outcomes[j].path {
			return outcomes[i].path < outcomes[j].path
		}
		return outcomes[i].file.FileID < outcomes[j].file.FileID
	})
	files := make([]SemanticFile, 0, len(outcomes))
	diagnostics := make([]lie.Diagnostic, 0)
	statistics := emptyStatistics()
	statistics.CandidateFiles = len(outcomes)
	for _, outcome := range outcomes {
		files = append(files, outcome.file)
		diagnostics = append(diagnostics, outcome.diagnostics...)
		switch outcome.file.Status {
		case SemanticFileResolved:
			statistics.ResolvedFiles++
		case SemanticFilePartial:
			statistics.PartialFiles++
		case SemanticFileFailed:
			statistics.FailedFiles++
		case SemanticFileStale:
			statistics.StaleFiles++
		case SemanticFileSkipped:
			statistics.SkippedFiles++
		}
	}
	return files, diagnostics, statistics
}

func semanticDiagnostic(severity lie.Severity, code, message, file string) lie.Diagnostic {
	location := &lie.SourceRange{File: file}
	if file == "" {
		location = nil
	}
	return lie.Diagnostic{Engine: "go-semantic", Severity: severity, Code: code, Message: message, Location: location}
}

func sortDiagnostics(values []lie.Diagnostic) {
	sort.Slice(values, func(i, j int) bool { return diagnosticKey(values[i]) < diagnosticKey(values[j]) })
}

func diagnosticKey(value lie.Diagnostic) string {
	file, start, end := "", 0, 0
	if value.Location != nil {
		file, start, end = value.Location.File, value.Location.Start.Offset, value.Location.End.Offset
	}
	return fmt.Sprintf("%s#%012d#%012d#%d#%s#%s", file, start, end, value.Severity, value.Code, value.Message)
}

func limitDiagnostics(values []lie.Diagnostic, maximumPerFile, maximum int) ([]lie.Diagnostic, int) {
	unique := make([]lie.Diagnostic, 0, len(values))
	for _, value := range values {
		if len(unique) > 0 && diagnosticKey(unique[len(unique)-1]) == diagnosticKey(value) {
			continue
		}
		unique = append(unique, value)
	}
	perFile := make(map[string]int)
	kept := make([]lie.Diagnostic, 0, len(unique))
	omitted := 0
	for _, value := range unique {
		file := ""
		if value.Location != nil {
			file = value.Location.File
		}
		if perFile[file] >= maximumPerFile {
			omitted++
			continue
		}
		perFile[file]++
		kept = append(kept, value)
	}
	if omitted == 0 && len(kept) <= maximum {
		return kept, 0
	}
	ordinaryLimit := maximum - 1
	if len(kept) > ordinaryLimit {
		omitted += len(kept) - ordinaryLimit
		kept = kept[:ordinaryLimit]
	}
	kept = append(kept, semanticDiagnostic(lie.SeverityWarning, "semantic_diagnostic_limit", fmt.Sprintf("%d diagnostics omitted", omitted), ""))
	return kept, omitted
}

type sourceReader struct {
	root        *os.Root
	canonical   string
	directories map[string]sourceDirectory
}

type sourceDirectory struct {
	root     *os.Root
	absolute string
	symlinks map[string]bool
	err      error
}

func newSourceReader(root string, candidates []string) (*sourceReader, error) {
	absRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return nil, fmt.Errorf("%w: resolve repository root: %v", ErrInvalidRepositoryRoot, err)
	}
	canonicalRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return nil, fmt.Errorf("%w: resolve repository root: %v", ErrInvalidRepositoryRoot, err)
	}
	rootHandle, err := os.OpenRoot(canonicalRoot)
	if err != nil {
		return nil, fmt.Errorf("%w: open repository root: %v", ErrInvalidRepositoryRoot, err)
	}
	reader := &sourceReader{root: rootHandle, canonical: canonicalRoot, directories: make(map[string]sourceDirectory)}
	for _, candidate := range candidates {
		cleanPath := filepath.Clean(filepath.FromSlash(candidate))
		if !safeRelativePath(candidate, cleanPath) {
			continue
		}
		directoryName := filepath.Dir(cleanPath)
		if _, exists := reader.directories[directoryName]; exists {
			continue
		}
		absoluteDirectory, resolveErr := filepath.EvalSymlinks(filepath.Join(canonicalRoot, directoryName))
		directory := sourceDirectory{absolute: absoluteDirectory, symlinks: map[string]bool{}, err: resolveErr}
		if resolveErr == nil && !withinRoot(canonicalRoot, absoluteDirectory) {
			directory.err = ErrSourceOutsideRoot
		}
		if directory.err == nil {
			directory.root, directory.err = rootHandle.OpenRoot(directoryName)
		}
		if directory.err == nil {
			entries, readErr := os.ReadDir(absoluteDirectory)
			if readErr != nil {
				directory.err = readErr
			} else {
				for _, entry := range entries {
					directory.symlinks[entry.Name()] = entry.Type()&os.ModeSymlink != 0
				}
			}
		}
		reader.directories[directoryName] = directory
	}
	return reader, nil
}

func (reader *sourceReader) readFile(relativePath string, maximumBytes int64) ([]byte, error) {
	cleanPath := filepath.Clean(filepath.FromSlash(relativePath))
	if !safeRelativePath(relativePath, cleanPath) {
		return nil, fmt.Errorf("%w: %s", ErrSourceOutsideRoot, relativePath)
	}
	directory := reader.directories[filepath.Dir(cleanPath)]
	if directory.root == nil && directory.err == nil {
		return nil, fmt.Errorf("%w: %s was not authorized by the repository snapshot", ErrSourceUnreadable, relativePath)
	}
	if directory.err != nil {
		if errors.Is(directory.err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrSourceMissing, relativePath)
		}
		if errors.Is(directory.err, ErrSourceOutsideRoot) || errors.Is(directory.err, os.ErrPermission) || errors.Is(directory.err, os.ErrInvalid) {
			return nil, fmt.Errorf("%w: %s", ErrSourceOutsideRoot, relativePath)
		}
		return nil, fmt.Errorf("%w: %s: %v", ErrSourceUnreadable, relativePath, directory.err)
	}
	baseName := filepath.Base(cleanPath)
	if directory.symlinks[baseName] {
		resolved, err := filepath.EvalSymlinks(filepath.Join(directory.absolute, baseName))
		if err != nil || !withinRoot(reader.canonical, resolved) {
			return nil, fmt.Errorf("%w: %s", ErrSourceOutsideRoot, relativePath)
		}
	}
	flags := os.O_RDONLY
	if runtime.GOOS == "windows" {
		flags |= 0x08000000
	}
	file, err := directory.root.OpenFile(baseName, flags, 0)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrSourceMissing, relativePath)
		}
		return nil, fmt.Errorf("%w: %s: %v", ErrSourceUnreadable, relativePath, err)
	}
	defer file.Close()
	information, err := file.Stat()
	if err != nil || !information.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: %s is not a regular file", ErrSourceUnreadable, relativePath)
	}
	data, err := io.ReadAll(io.LimitReader(file, maximumBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrSourceUnreadable, relativePath, err)
	}
	if int64(len(data)) > maximumBytes {
		return nil, fmt.Errorf("%w: %s exceeds %d bytes", ErrSourceOversized, relativePath, maximumBytes)
	}
	return data, nil
}

func (reader *sourceReader) close() {
	for _, directory := range reader.directories {
		if directory.root != nil {
			_ = directory.root.Close()
		}
	}
	if reader.root != nil {
		_ = reader.root.Close()
	}
}

func safeRelativePath(original, clean string) bool {
	return original != "" && clean != "." && !filepath.IsAbs(clean) && filepath.VolumeName(clean) == "" && clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator))
}

func withinRoot(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}
