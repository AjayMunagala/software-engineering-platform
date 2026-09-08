package golang

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AjayMunagala/software-engineering-platform/backend/die"
	"github.com/AjayMunagala/software-engineering-platform/backend/die/conformance"
	"github.com/AjayMunagala/software-engineering-platform/backend/lie"
	syntax "github.com/AjayMunagala/software-engineering-platform/backend/lie/golang"
	identity "github.com/AjayMunagala/software-engineering-platform/backend/lie/golang/packageidentity"
	"github.com/AjayMunagala/software-engineering-platform/backend/lie/golang/semantic"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie/discovery"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie/ignore"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie/language"
)

func TestCoreConformance(t *testing.T) {
	if _, err := conformance.Run(t, conformance.Config{Factory: die.New}); err != nil {
		t.Fatal(err)
	}
}
func fixture(t testing.TB, files map[string]string, workers int) InputParams {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	run := rie.NewRunContext(root, rie.DefaultConfig())
	pipeline := rie.New()
	for _, e := range []rie.Engine{discovery.New(), ignore.New(), language.New()} {
		if err := pipeline.Register(e); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	if err := pipeline.Run(ctx, run); err != nil {
		t.Fatal(err)
	}
	snap, ok := rie.ArtifactAs[rie.RepositorySnapshot](run.Artifacts, rie.RepositorySnapshotArtifactName)
	if !ok {
		t.Fatal("snapshot")
	}
	langs, ok := rie.ArtifactAs[language.LanguageInventory](run.Artifacts, language.LanguageInventoryArtifactName)
	if !ok {
		t.Fatal("languages")
	}
	cfg := syntax.DefaultConfig()
	cfg.MaxWorkers = workers
	ge, err := syntax.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	art, err := ge.Analyze(ctx, lie.Input{Snapshot: snap, Languages: langs})
	if err != nil {
		t.Fatal(err)
	}
	syn := art.(syntax.GoLanguageInventory)
	ie, err := identity.New()
	if err != nil {
		t.Fatal(err)
	}
	ids, err := ie.Analyze(ctx, identity.Input{Snapshot: snap, Syntax: syn})
	if err != nil {
		t.Fatal(err)
	}
	se, err := semantic.New()
	if err != nil {
		t.Fatal(err)
	}
	sem, err := se.Resolve(ctx, semantic.Input{Snapshot: snap, Syntax: syn, PackageIdentities: ids})
	if err != nil {
		t.Fatal(err)
	}
	return InputParams{snap, syn, ids, sem}
}
func configured(t testing.TB, p ConfigParams) Engine {
	t.Helper()
	c, e := NewConfig(p)
	if e != nil {
		t.Fatal(e)
	}
	v, e := New(c)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func inputs(t testing.TB, p InputParams) Inputs {
	t.Helper()
	i, e := NewInputs(context.Background(), p)
	if e != nil {
		t.Fatal(e)
	}
	return i
}
func marshal(t testing.TB, v any) []byte {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func generous() *budget {
	return &budget{maxRecords: MaximumInputRecords, maxEvidence: MaximumInputEvidence}
}
func extracted(t testing.TB, p InputParams) facts {
	t.Helper()
	f, e := extract(context.Background(), p, generous())
	if e != nil {
		t.Fatal(e)
	}
	return f
}
func workspace() map[string]string {
	return map[string]string{
		"go.work":    "go 1.26\nuse (\n./app\n./lib\n)\n",
		"app/go.mod": "module example.com/app\ngo 1.26\nrequire example.com/lib v0.0.0\nreplace example.com/lib => ../lib\n",
		"app/a.go":   "package app\nimport (\"example.com/lib\"; \"fmt\")\nfunc A(){fmt.Println(lib.Value()); B()}\n",
		"app/b.go":   "package app\nfunc B(){}\n",
		"lib/go.mod": "module example.com/lib\ngo 1.26\n",
		"lib/值.go":   "package lib\nfunc Value()int{return 1}\n",
	}
}

func TestReleasedArtifacts(t *testing.T) {
	var canonical []byte
	for _, workers := range []int{1, 8} {
		p := fixture(t, workspace(), workers)
		in := inputs(t, p)
		before := marshal(t, p)
		e := configured(t, ConfigParams{})
		out, err := e.Analyze(context.Background(), in)
		if err != nil {
			t.Fatal(err)
		}
		if e.Name() == "" || e.Description() == "" || e.Version() != Version {
			t.Fatal("metadata")
		}
		data := marshal(t, out)
		if canonical == nil {
			canonical = data
		} else if !bytes.Equal(canonical, data) {
			t.Fatal("worker determinism")
		}
		if !bytes.Equal(before, marshal(t, p)) {
			t.Fatal("mutated prerequisites")
		}
		if bytes.Contains(data, []byte("AppData")) || bytes.Contains(data, []byte("Temp")) {
			t.Fatal("source root leakage")
		}
		graphs := map[die.GraphKind]int{}
		refs := 0
		for _, edge := range out.Dependencies() {
			graphs[edge.Graph]++
			if edge.Kind == die.DependencyReferences {
				refs++
			}
		}
		if graphs[die.GraphModule] == 0 || graphs[die.GraphPackage] == 0 || graphs[die.GraphFile] == 0 || refs == 0 {
			t.Fatalf("missing graphs %v refs=%d", graphs, refs)
		}
		if len(out.StrongComponents()) != 0 || len(out.Cycles()) != 0 {
			t.Fatal("unauthorized algorithms")
		}
		nodes := out.Nodes()
		nodes[0].Name = "mutated"
		if !bytes.Equal(data, marshal(t, out)) {
			t.Fatal("output not immutable")
		}
		for j := 0; j < 3; j++ {
			again, err := e.Analyze(context.Background(), in)
			if err != nil || !bytes.Equal(data, marshal(t, again)) {
				t.Fatal("repeat mismatch", err)
			}
		}
		t.Logf("workers=%d nodes=%d edges=%d sha256=%x", workers, len(out.Nodes()), len(out.Dependencies()), sha256.Sum256(data))
	}
}
func TestEmptyAndAliases(t *testing.T) {
	e := configured(t, ConfigParams{})
	p := fixture(t, map[string]string{"README.md": "notes"}, 1)
	out, err := e.Analyze(context.Background(), inputs(t, p))
	if err != nil || len(out.Nodes()) != 0 {
		t.Fatal("empty", err)
	}
	p = fixture(t, map[string]string{"go.mod": "module example.com/test\ngo 1.26\n", "a.go": "package test\nimport (\"fmt\"; f \"fmt\"; _ \"fmt\"; . \"fmt\"; _ \"example.net/unavailable\")\nfunc A(){fmt.Println();f.Println();Println()}\n"}, 1)
	out, err = e.Analyze(context.Background(), inputs(t, p))
	if err != nil {
		t.Fatal(err)
	}
	occ := uint64(0)
	states := map[die.ResolutionState]bool{}
	for _, v := range out.Dependencies() {
		if v.Graph == die.GraphPackage {
			occ += v.Occurrences
			states[v.Resolution] = true
		}
	}
	if occ != 5 || !states[die.External] {
		t.Fatalf("alias loss: %d %v", occ, states)
	}
}
func TestBoundaryGoldenVectors(t *testing.T) {
	for _, v := range [][4]string{
		{"pkg:π", "ctx:工作", "example.com/β", "73a932f02665c68184b80cd9d36f2a138061eaced6fdd3fc65e928b26009ce52"},
		{"pkg:a", "", "fmt", "c397d0371aeaace5958aeed411a75c6cbac46885a4fbd6f41260fcbf6a09d6f2"},
		{"ab", "c", "d", "73a9c0c2ba1156f5aa5d662f0955bdc5cbcb50abac1696119c5df348122be620"},
		{"a", "bc", "d", "4ce6c7abf4f42427511275958983b3b3b7f0be80b5f87f1b960d5e1d5e03334e"},
	} {
		if got := boundaryName(v[0], v[1], v[2]); got != BoundaryNameScheme+":sha256:"+v[3] {
			t.Fatal(got)
		}
	}
}
func TestInputErrorsAndLimits(t *testing.T) {
	if _, err := NewInputs(nil, InputParams{}); err == nil {
		t.Fatal("nil")
	}
	if _, err := NewInputs(context.Background(), InputParams{}); err == nil {
		t.Fatal("zero")
	}
	if _, err := New(Config{}); err == nil {
		t.Fatal("zero config")
	}
	for _, c := range []ConfigParams{{MaxInputRecords: MaximumInputRecords + 1}, {MaxInputEvidence: MaximumInputEvidence + 1}} {
		if _, err := NewConfig(c); err == nil {
			t.Fatal("config max")
		}
	}
	c, _ := NewConfig(ConfigParams{})
	if c.MaxInputRecords() != DefaultMaxInputRecords || c.MaxInputEvidence() != DefaultMaxInputEvidence || c.CoreConfig() != die.DefaultConfig() {
		t.Fatal("defaults")
	}
	p := fixture(t, workspace(), 1)
	in := inputs(t, p)
	e := configured(t, ConfigParams{})
	for _, ctx := range []context.Context{nil, func() context.Context { c, cancel := context.WithCancel(context.Background()); cancel(); return c }(), func() context.Context {
		c, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		defer cancel()
		return c
	}()} {
		out, err := e.Analyze(ctx, in)
		if err == nil || out.ArtifactName() != "" {
			t.Fatal("cancel must return no artifact")
		}
	}
	if _, err := e.Analyze(context.Background(), Inputs{}); err == nil {
		t.Fatal("invalid inputs")
	}
	tiny, _ := die.NewConfig(die.ConfigParams{MaxNodes: 1})
	for _, cfg := range []ConfigParams{{MaxInputRecords: 1}, {MaxInputEvidence: 1}, {CoreConfig: tiny}} {
		out, err := configured(t, cfg).Analyze(context.Background(), in)
		var typed *Error
		if !errors.As(err, &typed) || typed.Kind() != die.ErrorLimitExceeded || out.ArtifactName() != "" {
			t.Fatalf("budget %v", err)
		}
	}
	for _, err := range []error{errors.New("SECRET"), context.Canceled, context.DeadlineExceeded, fail(die.ErrorIntegrity, "fixed")} {
		safe := safeError(err)
		if strings.Contains(safe.Error(), "SECRET") {
			t.Fatal("leak")
		}
		var typed *Error
		if !errors.As(safe, &typed) || typed.Code() == "" {
			t.Fatal("safe error")
		}
	}
	if !errors.Is(safeError(context.Canceled), context.Canceled) {
		t.Fatal("lost cancellation")
	}
	if strings.Contains(fmt.Sprintf("%v %+v %#v", in, in, in), "example.com") {
		t.Fatal("input formatting")
	}
}

func TestFactValidation(t *testing.T) {
	p := fixture(t, workspace(), 1)
	cases := map[string]func(*facts){
		"duplicate file":    func(f *facts) { f.files = append(f.files, f.files[0]) },
		"path escape":       func(f *facts) { f.files[0].Path = "../secret.go" },
		"package join":      func(f *facts) { f.files[0].PackageID = "missing" },
		"file status":       func(f *facts) { f.files[0].Status = 0 },
		"digest":            func(f *facts) { f.files[0].ContentDigest = "bad" },
		"package duplicate": func(f *facts) { f.packages = append(f.packages, f.packages[0]) },
		"membership":        func(f *facts) { f.packages[0].FileIDs = append(f.packages[0].FileIDs, "missing") },
		"module duplicate":  func(f *facts) { f.modules = append(f.modules, f.modules[0]) },
		"context invalid":   func(f *facts) { f.contexts[0].Kind = 0 },
		"context module":    func(f *facts) { f.contexts[0].MainModuleIDs = append(f.contexts[0].MainModuleIDs, "missing") },
		"proof duplicate":   func(f *facts) { f.proofs = append(f.proofs, f.proofs[0]) },
		"proof importer":    func(f *facts) { f.proofs[0].ImportingPackageID = "missing" },
		"proof context":     func(f *facts) { f.proofs[0].ResolutionContextID = "missing" },
		"proof kind":        func(f *facts) { f.proofs[0].Kinds = []identity.ProofKind{0} },
		"proof target":      func(f *facts) { f.proofs[0].TargetPackageID = "missing" },
		"candidate":         func(f *facts) { f.proofs[0].CandidatePackageIDs = []string{"missing"} },
		"semantic file":     func(f *facts) { f.semanticFiles[0].FileID = "missing" },
		"semantic digest":   func(f *facts) { f.semanticFiles[0].ContentDigest = strings.Repeat("1", 64) },
		"declaration":       func(f *facts) { f.declarations[0].FileID = "missing" },
		"binding duplicate": func(f *facts) { f.imports = append(f.imports, f.imports[0]) },
		"binding path":      func(f *facts) { f.imports[0].ImportPath = "different" },
		"binding proof":     func(f *facts) { f.imports[0].PackageIdentityProofID = "missing" },
		"reference":         func(f *facts) { f.references[0].FileID = "missing" },
		"reference target":  func(f *facts) { f.references[0].TargetDeclarationID = "missing" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			f := extracted(t, p)
			change(&f)
			if _, err := translate(context.Background(), p.RepositorySnapshot, f, generous(), 10000); err == nil {
				t.Fatal("accepted inconsistent input")
			}
		})
	}
}
func TestShuffledAndConcurrent(t *testing.T) {
	p := fixture(t, workspace(), 1)
	f := extracted(t, p)
	core, _ := die.New(die.DefaultConfig())
	g, err := translate(context.Background(), p.RepositorySnapshot, f, generous(), 10000)
	if err != nil {
		t.Fatal(err)
	}
	out, err := core.Normalize(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	want := marshal(t, out)
	rng := rand.New(rand.NewSource(5))
	for i := 0; i < 20; i++ {
		rng.Shuffle(len(f.files), func(i, j int) { f.files[i], f.files[j] = f.files[j], f.files[i] })
		rng.Shuffle(len(f.proofs), func(i, j int) { f.proofs[i], f.proofs[j] = f.proofs[j], f.proofs[i] })
		rng.Shuffle(len(f.references), func(i, j int) { f.references[i], f.references[j] = f.references[j], f.references[i] })
		g, err = translate(context.Background(), p.RepositorySnapshot, f, generous(), 10000)
		if err != nil {
			t.Fatal(err)
		}
		out, err = core.Normalize(context.Background(), g)
		if err != nil || !bytes.Equal(want, marshal(t, out)) {
			t.Fatal("shuffle", err)
		}
	}
	e := configured(t, ConfigParams{})
	in := inputs(t, p)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := e.Analyze(context.Background(), in)
			if err != nil || !bytes.Equal(want, marshal(t, out)) {
				t.Error("concurrency", err)
			}
		}()
	}
	wg.Wait()
}
func TestResolutionAndMissing(t *testing.T) {
	p := fixture(t, workspace(), 1)
	f := extracted(t, p)
	x, err := validate(context.Background(), p.RepositorySnapshot, f, generous())
	if err != nil {
		t.Fatal(err)
	}
	var file syntax.GoFile
	var im syntax.GoImport
	var binding semantic.ImportBinding
	for _, candidate := range f.files {
		for _, v := range candidate.Imports {
			b := x.bindings[ikey(candidate.ID, v.Path, v.AliasKind.String(), v.Location)]
			if b.Status == semantic.ResolutionResolved {
				file, im, binding = candidate, v, b
			}
		}
	}
	if binding.ID == "" {
		t.Fatal("no resolved fixture")
	}
	proofs := x.byImport[file.PackageID+"\x00"+im.Path]
	for _, v := range []struct {
		s    semantic.ResolutionStatus
		want die.ResolutionState
	}{{semantic.ResolutionAmbiguous, die.Ambiguous}, {semantic.ResolutionUnresolved, die.Unresolved}, {semantic.ResolutionPartial, die.Unresolved}} {
		b := binding
		b.Status = v.s
		b.TargetPackageID = ""
		got, _, _, err := resolve(file, im, b, true, proofs, x)
		if err != nil || got != v.want {
			t.Fatal("state", got, err)
		}
	}
	got, _, _, err := resolve(file, im, binding, false, proofs, x)
	if err != nil || got != die.Unresolved {
		t.Fatal("missing")
	}
	stale := append([]identity.PackageIdentityProof{}, proofs...)
	stale[0].Status = identity.ProofStale
	got, _, _, err = resolve(file, im, binding, true, stale, x)
	if err != nil || got != die.Stale {
		t.Fatal("stale")
	}
	sf := x.sf[file.ID]
	sf.Status = semantic.SemanticFilePartial
	x.sf[file.ID] = sf
	got, _, _, err = resolve(file, im, binding, true, proofs, x)
	if err != nil || got != die.ResolvedLocal {
		t.Fatal("partial is not stale")
	}
	f.imports = nil
	f.omitted = true
	g, err := translate(context.Background(), p.RepositorySnapshot, f, generous(), 10000)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range g.Dependencies {
		if e.Kind == die.DependencyImports && e.Resolution == die.ResolvedLocal {
			t.Fatal("guessed missing binding")
		}
	}
	for _, p := range []string{"/secret", "C:\\secret", "../secret", "a/../b", "a\x00b"} {
		if _, ok := relative(p); ok {
			t.Fatal("unsafe", p)
		}
	}
	if got, ok := relative("a\\b"); !ok || got != "a/b" {
		t.Fatal("normalization")
	}
	if !reflect.DeepEqual(f.files, extracted(t, p).files) {
		t.Fatal("translate mutates facts")
	}
}
func FuzzBoundaryAndPaths(f *testing.F) {
	f.Add("pkg:π", "", "example.com/β")
	f.Add("a", "b", "../x")
	f.Fuzz(func(t *testing.T, a, b, c string) {
		if len(a)+len(b)+len(c) > 4096 {
			t.Skip()
		}
		v := boundaryName(a, b, c)
		if v != boundaryName(a, b, c) || len(v) != len(BoundaryNameScheme)+8+64 {
			t.Fatal("identity")
		}
		p, ok := relative(c)
		if ok && (strings.HasPrefix(p, "/") || strings.Contains(p, "\\") || strings.Contains(p, ":")) {
			t.Fatal("path")
		}
		lim := uint64(len(a) + 1)
		bgt := budget{maxRecords: lim}
		if err := bgt.add(lim, false); err != nil {
			t.Fatal(err)
		}
		if err := bgt.add(1, false); err == nil {
			t.Fatal("overflow")
		}
	})
}

// This context deterministically cancels at successive public checkpoints, without sleeps.
type checkpointContext struct {
	context.Context
	left int
}

func resolve(file syntax.GoFile, im syntax.GoImport, b semantic.ImportBinding, exists bool, proofs []identity.PackageIdentityProof, x index) (die.ResolutionState, string, string, error) {
	return resolveContext(context.Background(), file, im, b, exists, proofs, x)
}

func (c *checkpointContext) Err() error {
	c.left--
	if c.left <= 0 {
		return context.Canceled
	}
	return nil
}
func TestCancellationCheckpoints(t *testing.T) {
	p := fixture(t, workspace(), 1)
	in := inputs(t, p)
	e := configured(t, ConfigParams{})
	passed := false
	for n := 1; n < 500; n++ {
		ctx := &checkpointContext{context.Background(), n}
		out, err := e.Analyze(ctx, in)
		if err == nil {
			passed = true
			break
		}
		if !errors.Is(err, context.Canceled) || out.ArtifactName() != "" {
			t.Fatal(n, err)
		}
	}
	if !passed {
		t.Fatal("checkpoints unbounded")
	}
}
func TestResolutionProofMatrix(t *testing.T) {
	p := fixture(t, workspace(), 1)
	f := extracted(t, p)
	x, err := validate(context.Background(), p.RepositorySnapshot, f, generous())
	if err != nil {
		t.Fatal(err)
	}
	var file syntax.GoFile
	var im syntax.GoImport
	var binding semantic.ImportBinding
	for _, v := range f.files {
		for _, i := range v.Imports {
			b := x.bindings[ikey(v.ID, i.Path, i.AliasKind.String(), i.Location)]
			if b.Status == semantic.ResolutionResolved {
				file, im, binding = v, i, b
			}
		}
	}
	proofs := x.byImport[file.PackageID+"\x00"+im.Path]
	b := binding
	b.Status = semantic.ResolutionExternal
	b.TargetPackageID = ""
	external := append([]identity.PackageIdentityProof{}, proofs...)
	for i := range external {
		external[i].Status = identity.ProofExternal
		external[i].TargetPackageID = ""
		external[i].Kinds = []identity.ProofKind{identity.ProofStandardLibrary}
	}
	state, _, _, err := resolve(file, im, b, true, external, x)
	if err != nil || state != die.StandardLibrary {
		t.Fatal("explicit stdlib", state, err)
	}
	if _, _, _, err = resolve(file, im, b, true, nil, x); err == nil {
		t.Fatal("unproved external")
	}
	if _, _, _, err = resolve(file, im, b, true, proofs, x); err == nil {
		t.Fatal("contradictory external")
	}
	bad := binding
	bad.PackageIdentityProofID = "missing"
	if _, _, _, err = resolve(file, im, bad, true, proofs, x); err == nil {
		t.Fatal("unproved local")
	}
	x.omitted = true
	state, _, _, err = resolve(file, im, bad, true, proofs, x)
	if err != nil || state != die.Unresolved {
		t.Fatal("omitted proof")
	}
	state, _, _, err = resolve(file, im, b, true, nil, x)
	if err != nil || state != die.Unresolved {
		t.Fatal("omitted external proof")
	}
	x.omitted = false
	bad = binding
	bad.Status = semantic.ResolutionAmbiguous
	if _, _, _, err = resolve(file, im, bad, true, proofs, x); err == nil {
		t.Fatal("unexpected target")
	}
	other := append([]identity.PackageIdentityProof{}, proofs...)
	other[0].Status = identity.ProofAmbiguous
	if _, _, _, err = resolve(file, im, binding, true, other, x); err == nil {
		t.Fatal("conflict")
	}
	contexts := append(append([]identity.PackageIdentityProof{}, external...), external[0])
	contexts[len(contexts)-1].ResolutionContextID = "another"
	_, _, contextID, err := resolve(file, im, b, true, contexts, x)
	if err != nil || contextID != "" {
		t.Fatal("arbitrary context")
	}
	target := x.packages[binding.TargetPackageID].FileIDs[0]
	sf := x.sf[target]
	sf.Status = semantic.SemanticFileFailed
	x.sf[target] = sf
	state, _, _, err = resolve(file, im, binding, true, proofs, x)
	if err != nil || state != die.Unresolved {
		t.Fatal("unverified target")
	}
	sf.Status = semantic.SemanticFileStale
	x.sf[target] = sf
	state, _, _, err = resolve(file, im, binding, true, proofs, x)
	if err != nil || state != die.Stale {
		t.Fatal("stale target")
	}
	x.staleLocations[locationKey(im.Location)] = true
	state, _, _, err = resolve(file, im, binding, true, proofs, x)
	if err != nil || state != die.Stale {
		t.Fatal("stale proof diagnostic")
	}
}
func TestNestedAndUnmanaged(t *testing.T) {
	for _, sources := range []map[string]string{
		{"a.go": "package a\nfunc A(){}\n"},
		{"go.mod": "module example.com/root\ngo 1.26\n", "a/b/go.mod": "module example.com/nested\ngo 1.26\n", "a/b/a.go": "package nested\n", "a/bc/a.go": "package root\n"},
		{"go.mod": "module example.com/app\ngo 1.26\nrequire example.com/dep v1.0.0\n", "a.go": "package app\nimport _ \"example.com/dep/pkg\"\n", "vendor/modules.txt": "# example.com/dep v1.0.0\nexample.com/dep/pkg\n", "vendor/example.com/dep/pkg/a.go": "package pkg\n"},
		{"go.mod": "module example.com/broken\ngo 1.26\n", "a.go": "package broken\nfunc ("},
	} {
		p := fixture(t, sources, 1)
		out, err := configured(t, ConfigParams{}).Analyze(context.Background(), inputs(t, p))
		if err != nil {
			t.Fatal(err)
		}
		nodes := map[string]die.DependencyNode{}
		for _, n := range out.Nodes() {
			nodes[n.ID] = n
		}
		for _, c := range out.Containment() {
			if c.Kind == die.ContainmentModulePackage && nodes[c.ChildID].RepositoryPath == "a/bc" && nodes[c.ParentID].RepositoryPath != "" {
				t.Fatal("prefix ownership")
			}
		}
	}
}
func TestGeneratedBudgets(t *testing.T) {
	p := fixture(t, workspace(), 1)
	f := extracted(t, p)
	for _, evidence := range []bool{false, true} {
		failed := 0
		for limit := uint64(1); limit < 100; limit++ {
			b := generous()
			if evidence {
				b.maxEvidence = limit
			} else {
				b.maxRecords = limit
			}
			_, err := translate(context.Background(), p.RepositorySnapshot, f, b, 1000)
			if err != nil {
				failed++
			}
		}
		if failed == 0 {
			t.Fatal("no budget failures")
		}
	}
	c, _ := die.New(die.DefaultConfig())
	_, err := c.Normalize(context.Background(), die.GraphInput{Nodes: []die.NodeCandidate{{}}})
	var typed *Error
	if !errors.As(safeError(err), &typed) || typed.Kind() != die.ErrorInvalidInput {
		t.Fatal("core error translation")
	}
	b := builder{budget: &budget{}, nodes: map[die.NodeIdentity]bool{}}
	b.diag("fixed", "", "", "")
	b.diag("fixed", "", "", "")
	if b.err == nil || len(b.g.Diagnostics) != 0 {
		t.Fatal("diagnostic bound")
	}
}

func FuzzProofJoins(f *testing.F) {
	p := fixture(f, workspace(), 1)
	f.Add(uint8(1), "missing", uint16(10))
	f.Add(uint8(3), "", uint16(1000))
	f.Fuzz(func(t *testing.T, s uint8, target string, limit uint16) {
		if len(target) > 256 {
			t.Skip()
		}
		facts := extracted(t, p)
		facts.imports[0].Status = semantic.ResolutionStatus(s)
		facts.imports[0].TargetPackageID = target
		b := generous()
		b.maxRecords = uint64(limit) + 1
		g, err := translate(context.Background(), p.RepositorySnapshot, facts, b, 1000)
		if err != nil {
			var typed *Error
			if !errors.As(err, &typed) {
				t.Fatal("unsafe error")
			}
			return
		}
		core, _ := die.New(die.DefaultConfig())
		if _, err := core.Normalize(context.Background(), g); err != nil {
			t.Fatal("invalid translated graph", err)
		}
	})
}

func TestReleasedVendorEvidence(t *testing.T) {
	p := fixture(t, map[string]string{
		"go.mod":                          "module example.com/app\ngo 1.26\nrequire example.com/dep v1.0.0\n",
		"a.go":                            "package app\nimport _ \"example.com/dep/pkg\"\n",
		"vendor/modules.txt":              "# example.com/dep v1.0.0\nexample.com/dep/pkg\n",
		"vendor/example.com/dep/pkg/a.go": "package pkg\n",
	}, 1)
	found := false
	for _, proof := range p.PackageIdentity.Proofs() {
		for _, kind := range proof.Kinds {
			if kind == identity.ProofVendor && proof.Status == identity.ProofResolved {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("fixture must supply resolved vendor proof")
	}
	out, err := configured(t, ConfigParams{}).Analyze(context.Background(), inputs(t, p))
	if err != nil {
		t.Fatal(err)
	}
	resolved := false
	for _, edge := range out.Dependencies() {
		if edge.Graph == die.GraphPackage && edge.Resolution == die.ResolvedLocal {
			resolved = true
		}
	}
	if resolved {
		t.Fatal("adapter overrode unresolved released semantic binding")
	}
	f := extracted(t, p)
	f.imports[0].LocalName = "wrong"
	if _, err := translate(context.Background(), p.RepositorySnapshot, f, generous(), 10000); err == nil {
		t.Fatal("alias contradiction accepted")
	}
	// Controlled facts exercise an explicitly selected vendor context; the released
	// fixture above intentionally has no such selection. No artifact is mutated.
	f = extracted(t, p)
	for _, proof := range f.proofs {
		if proof.Status == identity.ProofResolved {
			f.proofs = []identity.PackageIdentityProof{proof}
			f.imports[0].Status = semantic.ResolutionResolved
			f.imports[0].TargetPackageID = proof.TargetPackageID
			f.imports[0].PackageIdentityProofID = proof.ID
			break
		}
	}
	g, err := translate(context.Background(), p.RepositorySnapshot, f, generous(), 10000)
	if err != nil {
		t.Fatal(err)
	}
	resolved = false
	for _, edge := range g.Dependencies {
		if edge.Graph == die.GraphPackage && edge.Resolution == die.ResolvedLocal {
			resolved = true
		}
	}
	if !resolved {
		t.Fatal("explicit vendor selection not retained")
	}
}
