package golang

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/AjayMunagala/software-engineering-platform/backend/lie"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie/language"
)

const engineVersion = "1.0.0"

type engine struct{ config Config }

// New returns the deterministic Go Language Engine.
func New(configs ...Config) (lie.Engine, error) {
	config := DefaultConfig()
	if len(configs) > 1 {
		return nil, fmt.Errorf("%w: at most one Go configuration is accepted", lie.ErrInvalidConfig)
	}
	if len(configs) == 1 {
		config = configs[0]
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &engine{config: config}, nil
}

func (*engine) Name() string         { return "golang" }
func (*engine) Version() string      { return engineVersion }
func (*engine) Language() string     { return "Go" }
func (*engine) ArtifactName() string { return ArtifactName }
func (*engine) Description() string {
	return "Deterministically parses authorized Go source files with the Go standard library parser"
}

func (engine *engine) Analyze(ctx context.Context, input lie.Input) (lie.LanguageArtifact, error) {
	if ctx == nil {
		return nil, lie.ErrContextRequired
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entries := input.Snapshot.Entries()
	allGoCandidates := make([]rie.RepositoryEntry, 0)
	for _, entry := range entries {
		if !entry.IsDir && strings.EqualFold(path.Ext(entry.Path), ".go") {
			allGoCandidates = append(allGoCandidates, entry)
		}
	}
	if err := verifyLanguageInventory(input.Languages, len(allGoCandidates)); err != nil {
		return nil, err
	}

	candidates := allGoCandidates
	if !engine.config.IncludeTests {
		candidates = make([]rie.RepositoryEntry, 0, len(allGoCandidates))
		for _, entry := range allGoCandidates {
			if !isGoTestFile(entry.Path) {
				candidates = append(candidates, entry)
			}
		}
	}
	sources := []rie.ArtifactReference{
		{Name: input.Snapshot.ArtifactName(), Version: input.Snapshot.ArtifactVersion()},
		{Name: input.Languages.ArtifactName(), Version: input.Languages.ArtifactVersion()},
	}
	if len(candidates) == 0 {
		return newGoLanguageInventory(sources, []GoFile{}, []GoPackage{}, []GoSymbol{}, []lie.Diagnostic{}, emptyStatistics()), nil
	}

	reader, err := newSafeReader(input.Snapshot.RootPath(), candidates)
	if err != nil {
		return nil, err
	}
	defer reader.close()
	outcomes := make([]fileOutcome, len(candidates))
	workerCount := min(engine.config.MaxWorkers, len(candidates))
	jobs := make(chan int)
	var workers sync.WaitGroup
	workers.Add(workerCount)
	for range workerCount {
		go func() {
			defer workers.Done()
			for index := range jobs {
				if ctx.Err() != nil {
					return
				}
				outcomes[index] = engine.processFile(ctx, reader, candidates[index])
			}
		}()
	}
	for index := range candidates {
		select {
		case jobs <- index:
		case <-ctx.Done():
			close(jobs)
			workers.Wait()
			return nil, ctx.Err()
		}
	}
	close(jobs)
	workers.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	files, packages, symbols, diagnostics, statistics := collect(outcomes, engine.config.MaxDiagnostics)
	return newGoLanguageInventory(sources, files, packages, symbols, diagnostics, statistics), nil
}

func verifyLanguageInventory(inventory language.LanguageInventory, expected int) error {
	found := false
	detected := 0
	for _, item := range inventory.Items() {
		if !strings.EqualFold(item.Name, "Go") {
			continue
		}
		if found {
			return fmt.Errorf("%w: duplicate Go records", lie.ErrLanguageInventoryMismatch)
		}
		found = true
		detected = item.Count
	}
	if detected != expected {
		return fmt.Errorf("%w: RepositorySnapshot contains %d .go files but LanguageInventory reports %d", lie.ErrLanguageInventoryMismatch, expected, detected)
	}
	return nil
}

func (engine *engine) processFile(ctx context.Context, reader *safeReader, entry rie.RepositoryEntry) fileOutcome {
	file := GoFile{ID: fileID(entry.Path), Path: entry.Path, IsTest: isGoTestFile(entry.Path), Imports: []GoImport{}}
	data, err := reader.readFile(entry.Path, engine.config.MaxSourceFileSize)
	if err != nil {
		file.Status = FileStatusFailed
		code := "go_source_unreadable"
		severity := lie.SeverityWarning
		if errors.Is(err, ErrSourceMissing) {
			code = "go_source_missing"
		} else if errors.Is(err, ErrSourceOversized) {
			file.Status = FileStatusSkipped
			code = "go_source_oversized"
		} else if errors.Is(err, ErrSourceOutsideRoot) {
			code = "go_source_outside_root"
			severity = lie.SeverityError
		}
		return fileOutcome{file: file, diagnostics: []lie.Diagnostic{{Engine: engine.Name(), Severity: severity, Code: code, Message: err.Error(), Location: rangePointer(entry.Path)}}}
	}
	file.SizeBytes = int64(len(data))
	digest := sha256.Sum256(data)
	file.ContentDigest = fmt.Sprintf("sha256:%x", digest)
	if err := ctx.Err(); err != nil {
		return fileOutcome{file: file}
	}
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, entry.Path, data, parser.SkipObjectResolution)
	if err != nil {
		file.Status = FileStatusFailed
		return fileOutcome{file: file, diagnostics: []lie.Diagnostic{{Engine: engine.Name(), Severity: lie.SeverityWarning, Code: "go_parse_error", Message: err.Error(), Location: rangePointer(entry.Path)}}}
	}
	file.Status = FileStatusParsed
	file.PackageName = parsed.Name.Name
	file.PackageID = packageID(path.Dir(entry.Path), file.PackageName)
	file.Imports = extractImports(fileSet, entry.Path, parsed)
	symbols, diagnostics := extractSymbols(fileSet, entry.Path, file.PackageID, file.ID, parsed)
	return fileOutcome{file: file, symbols: symbols, diagnostics: diagnostics}
}

type safeReader struct {
	root        *os.Root
	canonical   string
	directories map[string]directoryRoot
}

type directoryRoot struct {
	root     *os.Root
	absolute string
	symlinks map[string]bool
	err      error
}

func newSafeReader(root string, candidates []rie.RepositoryEntry) (*safeReader, error) {
	absRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return nil, fmt.Errorf("%w: resolve repository root: %v", ErrSourceUnreadable, err)
	}
	canonicalRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return nil, fmt.Errorf("%w: resolve repository root: %v", ErrSourceUnreadable, err)
	}
	rootHandle, err := os.OpenRoot(canonicalRoot)
	if err != nil {
		return nil, fmt.Errorf("%w: open repository root: %v", ErrSourceUnreadable, err)
	}
	reader := &safeReader{root: rootHandle, canonical: canonicalRoot, directories: make(map[string]directoryRoot)}
	for _, candidate := range candidates {
		cleanPath := filepath.Clean(filepath.FromSlash(candidate.Path))
		if candidate.Path == "" || cleanPath == "." || filepath.IsAbs(cleanPath) || filepath.VolumeName(cleanPath) != "" || cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
			continue
		}
		directory := filepath.Dir(cleanPath)
		if _, exists := reader.directories[directory]; exists {
			continue
		}
		absoluteDirectory, resolveErr := filepath.EvalSymlinks(filepath.Join(canonicalRoot, directory))
		record := directoryRoot{absolute: absoluteDirectory, symlinks: map[string]bool{}, err: resolveErr}
		if resolveErr == nil && !within(canonicalRoot, absoluteDirectory) {
			record.err = ErrSourceOutsideRoot
		}
		if record.err == nil {
			record.root, record.err = rootHandle.OpenRoot(directory)
		}
		if record.err == nil {
			entries, readErr := os.ReadDir(absoluteDirectory)
			if readErr != nil {
				record.err = readErr
			} else {
				for _, entry := range entries {
					record.symlinks[entry.Name()] = entry.Type()&os.ModeSymlink != 0
				}
			}
		}
		reader.directories[directory] = record
	}
	return reader, nil
}

func (reader *safeReader) readFile(relativePath string, maximumBytes int64) ([]byte, error) {
	nativePath := filepath.FromSlash(relativePath)
	cleanPath := filepath.Clean(nativePath)
	if relativePath == "" || cleanPath == "." || filepath.IsAbs(cleanPath) || filepath.VolumeName(cleanPath) != "" || cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("%w: %s", ErrSourceOutsideRoot, relativePath)
	}
	directory := reader.directories[filepath.Dir(cleanPath)]
	if directory.err != nil {
		if errors.Is(directory.err, ErrSourceOutsideRoot) {
			return nil, fmt.Errorf("%w: %s", ErrSourceOutsideRoot, relativePath)
		}
		if errors.Is(directory.err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrSourceMissing, relativePath)
		}
		if errors.Is(directory.err, os.ErrPermission) || errors.Is(directory.err, os.ErrInvalid) {
			return nil, fmt.Errorf("%w: %s: %v", ErrSourceOutsideRoot, relativePath, directory.err)
		}
		return nil, fmt.Errorf("%w: %s: %v", ErrSourceUnreadable, relativePath, directory.err)
	}
	if directory.root == nil {
		return nil, fmt.Errorf("%w: %s", ErrSourceUnreadable, relativePath)
	}
	baseName := filepath.Base(cleanPath)
	if directory.symlinks[baseName] {
		resolvedTarget, resolveErr := filepath.EvalSymlinks(filepath.Join(directory.absolute, baseName))
		if resolveErr != nil {
			if errors.Is(resolveErr, os.ErrNotExist) {
				return nil, fmt.Errorf("%w: %s", ErrSourceMissing, relativePath)
			}
			return nil, fmt.Errorf("%w: %s: %v", ErrSourceUnreadable, relativePath, resolveErr)
		}
		if !within(reader.canonical, resolvedTarget) {
			return nil, fmt.Errorf("%w: %s", ErrSourceOutsideRoot, relativePath)
		}
	}
	openFlags := os.O_RDONLY
	if runtime.GOOS == "windows" {
		openFlags |= 0x08000000 // FILE_FLAG_SEQUENTIAL_SCAN
	}
	file, err := directory.root.OpenFile(baseName, openFlags, 0)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrSourceMissing, relativePath)
		}
		if errors.Is(err, os.ErrPermission) || errors.Is(err, os.ErrInvalid) {
			return nil, fmt.Errorf("%w: %s: %v", ErrSourceOutsideRoot, relativePath, err)
		}
		return nil, fmt.Errorf("%w: %s: %v", ErrSourceUnreadable, relativePath, err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maximumBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrSourceUnreadable, relativePath, err)
	}
	if int64(len(data)) > maximumBytes {
		return nil, fmt.Errorf("%w: %s grew beyond %d bytes", ErrSourceOversized, relativePath, maximumBytes)
	}
	return data, nil
}

func (reader *safeReader) close() {
	for _, directory := range reader.directories {
		if directory.root != nil {
			_ = directory.root.Close()
		}
	}
	_ = reader.root.Close()
}

func within(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	return err == nil && !filepath.IsAbs(relative) && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

type fileOutcome struct {
	file        GoFile
	symbols     []GoSymbol
	diagnostics []lie.Diagnostic
}

func collect(outcomes []fileOutcome, maximumDiagnostics int) ([]GoFile, []GoPackage, []GoSymbol, []lie.Diagnostic, ParseStatistics) {
	sort.Slice(outcomes, func(i, j int) bool { return outcomes[i].file.Path < outcomes[j].file.Path })
	files := make([]GoFile, 0, len(outcomes))
	symbols := make([]GoSymbol, 0)
	diagnostics := make([]lie.Diagnostic, 0)
	packageFiles := make(map[string]map[string][]string)
	statistics := emptyStatistics()
	statistics.CandidateFiles = len(outcomes)
	for _, outcome := range outcomes {
		file := outcome.file
		files = append(files, file)
		symbols = append(symbols, outcome.symbols...)
		diagnostics = append(diagnostics, outcome.diagnostics...)
		switch file.Status {
		case FileStatusParsed:
			statistics.ParsedFiles++
			statistics.ParsedBytes += file.SizeBytes
			statistics.Imports += len(file.Imports)
			directory := path.Dir(file.Path)
			if packageFiles[directory] == nil {
				packageFiles[directory] = make(map[string][]string)
			}
			packageFiles[directory][file.PackageName] = append(packageFiles[directory][file.PackageName], file.ID)
		case FileStatusFailed:
			statistics.FailedFiles++
		case FileStatusSkipped:
			statistics.SkippedFiles++
		}
	}
	packages := buildPackages(packageFiles, &diagnostics)
	statistics.Packages = len(packages)
	sort.Slice(symbols, func(i, j int) bool {
		if symbols[i].Location.File != symbols[j].Location.File {
			return symbols[i].Location.File < symbols[j].Location.File
		}
		if symbols[i].Location.Start.Offset != symbols[j].Location.Start.Offset {
			return symbols[i].Location.Start.Offset < symbols[j].Location.Start.Offset
		}
		return symbols[i].ID < symbols[j].ID
	})
	for _, symbol := range symbols {
		statistics.SymbolsByKind[symbol.Kind.String()]++
	}
	sortDiagnostics(diagnostics)
	diagnostics, statistics.OmittedDiagnostics = limitDiagnostics(diagnostics, maximumDiagnostics)
	statistics.Diagnostics = len(diagnostics)
	return files, packages, symbols, diagnostics, statistics
}

func buildPackages(grouped map[string]map[string][]string, diagnostics *[]lie.Diagnostic) []GoPackage {
	directories := make([]string, 0, len(grouped))
	for directory := range grouped {
		directories = append(directories, directory)
	}
	sort.Strings(directories)
	packages := make([]GoPackage, 0)
	for _, directory := range directories {
		names := make([]string, 0, len(grouped[directory]))
		regular := make([]string, 0)
		for name := range grouped[directory] {
			names = append(names, name)
			if !strings.HasSuffix(name, "_test") {
				regular = append(regular, name)
			}
		}
		sort.Strings(names)
		sort.Strings(regular)
		if len(regular) > 1 {
			*diagnostics = append(*diagnostics, lie.Diagnostic{Engine: "golang", Severity: lie.SeverityWarning, Code: "go_directory_package_conflict", Message: fmt.Sprintf("directory %s contains multiple regular package declarations (%s)", directory, strings.Join(regular, ", ")), Location: rangePointer(directory)})
		}
		for _, name := range names {
			ids := grouped[directory][name]
			sort.Strings(ids)
			packages = append(packages, GoPackage{ID: packageID(directory, name), Name: name, Directory: directory, FileIDs: ids})
		}
	}
	return packages
}

func extractImports(fileSet *token.FileSet, relativePath string, file *ast.File) []GoImport {
	imports := make([]GoImport, 0, len(file.Imports))
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		alias, aliasKind := "", ImportAliasDefault
		if spec.Name != nil {
			alias = spec.Name.Name
			switch alias {
			case "_":
				aliasKind = ImportAliasBlank
			case ".":
				aliasKind = ImportAliasDot
			default:
				aliasKind = ImportAliasNamed
			}
		}
		imports = append(imports, GoImport{Path: importPath, Alias: alias, AliasKind: aliasKind, Location: sourceRange(fileSet, relativePath, spec.Pos(), spec.End())})
	}
	return imports
}

func extractSymbols(fileSet *token.FileSet, relativePath, packageIdentifier, fileIdentifier string, file *ast.File) ([]GoSymbol, []lie.Diagnostic) {
	symbols := make([]GoSymbol, 0)
	diagnostics := make([]lie.Diagnostic, 0)
	for _, declaration := range file.Decls {
		switch typed := declaration.(type) {
		case *ast.FuncDecl:
			kind := SymbolKindFunction
			receiverBase, pointerReceiver, genericReceiver := "", false, false
			if typed.Recv != nil {
				kind = SymbolKindMethod
				if len(typed.Recv.List) > 0 {
					receiverBase, pointerReceiver, genericReceiver = receiverDetails(typed.Recv.List[0].Type)
				}
				if receiverBase == "" {
					diagnostics = append(diagnostics, lie.Diagnostic{Engine: "golang", Severity: lie.SeverityWarning, Code: "go_receiver_unsupported", Message: fmt.Sprintf("method %s has an unsupported receiver shape", typed.Name.Name), Location: pointerToRange(sourceRange(fileSet, relativePath, typed.Pos(), typed.End()))})
				}
			}
			location := sourceRange(fileSet, relativePath, typed.Pos(), typed.End())
			symbols = append(symbols, newSymbol(relativePath, packageIdentifier, fileIdentifier, kind, typed.Name.Name, location, receiverBase, pointerReceiver, genericReceiver))
		case *ast.GenDecl:
			for _, specification := range typed.Specs {
				switch spec := specification.(type) {
				case *ast.TypeSpec:
					kind := SymbolKind(0)
					switch spec.Type.(type) {
					case *ast.StructType:
						kind = SymbolKindStruct
					case *ast.InterfaceType:
						kind = SymbolKindInterface
					}
					if kind != 0 {
						location := sourceRange(fileSet, relativePath, spec.Pos(), spec.End())
						symbols = append(symbols, newSymbol(relativePath, packageIdentifier, fileIdentifier, kind, spec.Name.Name, location, "", false, false))
					}
				case *ast.ValueSpec:
					kind := SymbolKind(0)
					if typed.Tok == token.CONST {
						kind = SymbolKindConstant
					} else if typed.Tok == token.VAR {
						kind = SymbolKindVariable
					}
					for _, name := range spec.Names {
						if kind != 0 {
							location := sourceRange(fileSet, relativePath, name.Pos(), name.End())
							symbols = append(symbols, newSymbol(relativePath, packageIdentifier, fileIdentifier, kind, name.Name, location, "", false, false))
						}
					}
				}
			}
		}
	}
	return symbols, diagnostics
}

func receiverDetails(expression ast.Expr) (base string, pointer, generic bool) {
	for expression != nil {
		switch current := expression.(type) {
		case *ast.ParenExpr:
			expression = current.X
		case *ast.StarExpr:
			pointer = true
			expression = current.X
		case *ast.IndexExpr:
			generic = true
			expression = current.X
		case *ast.IndexListExpr:
			generic = true
			expression = current.X
		case *ast.Ident:
			return current.Name, pointer, generic
		default:
			return "", pointer, generic
		}
	}
	return "", pointer, generic
}

func newSymbol(relativePath, packageIdentifier, fileIdentifier string, kind SymbolKind, name string, location lie.SourceRange, receiverBase string, pointerReceiver, genericReceiver bool) GoSymbol {
	return GoSymbol{ID: fmt.Sprintf("go:symbol:%s#%d:%s:%s", relativePath, location.Start.Offset, kind.String(), name), Kind: kind, Name: name, PackageID: packageIdentifier, FileID: fileIdentifier, Exported: ast.IsExported(name), ReceiverBase: receiverBase, PointerReceiver: pointerReceiver, GenericReceiver: genericReceiver, Location: location}
}

func sourceRange(fileSet *token.FileSet, relativePath string, start, end token.Pos) lie.SourceRange {
	startPosition, endPosition := fileSet.Position(start), fileSet.Position(end)
	return lie.SourceRange{File: relativePath, Start: lie.Position{Offset: startPosition.Offset, Line: startPosition.Line, Column: startPosition.Column}, End: lie.Position{Offset: endPosition.Offset, Line: endPosition.Line, Column: endPosition.Column}}
}

func pointerToRange(value lie.SourceRange) *lie.SourceRange { return &value }
func rangePointer(file string) *lie.SourceRange             { return &lie.SourceRange{File: file} }
func fileID(relativePath string) string                     { return "go:file:" + relativePath }
func packageID(directory, name string) string {
	return fmt.Sprintf("go:package:%s#%s", directory, name)
}
func isGoTestFile(relativePath string) bool {
	return strings.HasSuffix(strings.ToLower(path.Base(relativePath)), "_test.go")
}

func emptyStatistics() ParseStatistics { return ParseStatistics{SymbolsByKind: map[string]int{}} }

func sortDiagnostics(diagnostics []lie.Diagnostic) {
	sort.Slice(diagnostics, func(i, j int) bool {
		left, right := diagnostics[i], diagnostics[j]
		leftFile, rightFile, leftOffset, rightOffset := "", "", 0, 0
		if left.Location != nil {
			leftFile, leftOffset = left.Location.File, left.Location.Start.Offset
		}
		if right.Location != nil {
			rightFile, rightOffset = right.Location.File, right.Location.Start.Offset
		}
		if leftFile != rightFile {
			return leftFile < rightFile
		}
		if leftOffset != rightOffset {
			return leftOffset < rightOffset
		}
		if left.Code != right.Code {
			return left.Code < right.Code
		}
		return left.Message < right.Message
	})
}

func limitDiagnostics(diagnostics []lie.Diagnostic, maximum int) ([]lie.Diagnostic, int) {
	if len(diagnostics) <= maximum {
		return diagnostics, 0
	}
	omitted := len(diagnostics) - maximum + 1
	limited := append([]lie.Diagnostic(nil), diagnostics[:maximum-1]...)
	limited = append(limited, lie.Diagnostic{Engine: "golang", Severity: lie.SeverityWarning, Code: "go_diagnostic_limit", Message: fmt.Sprintf("%d additional diagnostics were omitted", omitted)})
	return limited, omitted
}
