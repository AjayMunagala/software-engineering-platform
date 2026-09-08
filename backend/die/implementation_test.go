package die

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

func testSource(id string) SourceIdentity {
	return SourceIdentity{"go-semantic-inventory", "1.0.0", id}
}
func testEvidence(id string) DependencyEvidence {
	return DependencyEvidence{Source: testSource(id), File: "pkg\\file.go", StartLine: 1, StartColumn: 2, Rule: "go-import", Value: "example.com/b"}
}
func testNode(name, path string) NodeCandidate {
	return NodeCandidate{Identity: NodeIdentity{NodePackage, "Go", name, path, ResolvedLocal}, Name: name, SourceIdentity: testSource(name), Evidence: []DependencyEvidence{testEvidence(name)}}
}
func newTestCore(t testing.TB, params ConfigParams) Core {
	t.Helper()
	cfg, err := NewConfig(params)
	if err != nil {
		t.Fatal(err)
	}
	core, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return core
}

func TestStableIDGoldenVectors(t *testing.T) {
	n := NodeIdentity{NodePackage, "Go", "example.com/π/pkg", "src\\pkg", ResolvedLocal}
	normalized, err := normalizeIdentity(n)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := nodeID(normalized), "dependency-node-id/v1:sha256:16c0f7392130f9bad33f66087e9c0b46c1f2823ba8cf7ea491187b35494c8a64"; got != want {
		t.Fatalf("node ID = %s", got)
	}
	if got, want := edgeID(GraphPackage, DependencyImports, nodeID(normalized), "dependency-node-id/v1:sha256:target", ResolvedLocal), "dependency-edge-id/v1:sha256:bf8b464f5cbb5551b28068da1445e8e45359e2586f860ede189d9ac68b4c8ad1"; got != want {
		t.Fatalf("edge ID = %s", got)
	}
	got := containmentID(ContainmentModulePackage, "dependency-node-id/v1:sha256:parent", "dependency-node-id/v1:sha256:child")
	const want = "dependency-containment-id/v1:sha256:3b47d635724cdb8ab06ff0319c2ded1e67d1289fc6a92df92a02a0c29f0c4334"
	if got != want {
		t.Fatalf("containment ID = %s", got)
	}
}

func TestNormalizeDeterministicAggregationAndImmutability(t *testing.T) {
	a, b := testNode("example.com/a", "a"), testNode("example.com/b", "b")
	dep := DependencyCandidate{GraphPackage, DependencyImports, a.Identity, b.Identity, ResolvedLocal, []DependencyEvidence{testEvidence("z"), testEvidence("a"), testEvidence("a")}}
	contain := ContainmentCandidate{ContainmentModulePackage, a.Identity, b.Identity, []DependencyEvidence{testEvidence("c")}}
	input := GraphInput{SourceArtifacts: []ArtifactReference{{"go-semantic-inventory", "1.0.0", ""}}, Nodes: []NodeCandidate{b, a}, Containment: []ContainmentCandidate{contain}, Dependencies: []DependencyCandidate{dep, dep}}
	core := newTestCore(t, ConfigParams{MaxEvidencePerItem: 1})
	first, err := core.Normalize(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	rand.New(rand.NewSource(42)).Shuffle(len(input.Nodes), func(i, j int) { input.Nodes[i], input.Nodes[j] = input.Nodes[j], input.Nodes[i] })
	second, err := core.Normalize(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.View(), second.View()) {
		t.Fatal("shuffled input changed output")
	}
	edges := first.Dependencies()
	if len(edges) != 1 || edges[0].Occurrences != 2 || len(edges[0].Evidence) != 1 || edges[0].OmittedEvidence != 1 {
		t.Fatalf("unexpected aggregation: %#v", edges)
	}
	nodes := first.Nodes()
	nodes[0].Name = "changed"
	nodes[0].Evidence[0].Rule = "changed"
	if reflect.DeepEqual(nodes, first.Nodes()) {
		t.Fatal("accessor mutation reached inventory")
	}
	b1, _ := json.Marshal(first)
	b2, _ := json.Marshal(second)
	if string(b1) != string(b2) {
		t.Fatal("JSON is not deterministic")
	}
}

func TestMaxNodesIsHardGate(t *testing.T) {
	core := newTestCore(t, ConfigParams{MaxNodes: 1})
	got, err := core.Normalize(context.Background(), GraphInput{Nodes: []NodeCandidate{testNode("a", "a"), testNode("b", "b")}})
	if ErrorKindOf(err) != ErrorLimitExceeded {
		t.Fatalf("expected limit error, got %v", err)
	}
	if got.ArtifactName() != "" || len(got.Nodes()) != 0 {
		t.Fatal("hard limit published a partial inventory")
	}
}
func TestMissingEndpointFailsIntegrity(t *testing.T) {
	a, b := testNode("a", "a"), testNode("b", "b")
	core := newTestCore(t, ConfigParams{})
	_, err := core.Normalize(context.Background(), GraphInput{Nodes: []NodeCandidate{a}, Dependencies: []DependencyCandidate{{GraphPackage, DependencyImports, a.Identity, b.Identity, ResolvedLocal, nil}}})
	if ErrorKindOf(err) != ErrorIntegrity {
		t.Fatalf("expected integrity error, got %v", err)
	}
}
func TestUnsafePathRejected(t *testing.T) {
	n := testNode("a", "../outside")
	_, err := newTestCore(t, ConfigParams{}).Normalize(context.Background(), GraphInput{Nodes: []NodeCandidate{n}})
	if ErrorKindOf(err) != ErrorInvalidInput {
		t.Fatalf("expected invalid input, got %v", err)
	}
}
func TestCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := newTestCore(t, ConfigParams{}).Normalize(ctx, GraphInput{})
	if !errors.Is(err, context.Canceled) || ErrorKindOf(err) != ErrorCanceled {
		t.Fatalf("expected cancellation, got %v", err)
	}
}
func TestEmptyInventoryUsesArrays(t *testing.T) {
	got, err := newTestCore(t, ConfigParams{}).Normalize(context.Background(), GraphInput{})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(got)
	if string(data) == "" || !containsAll(string(data), `"nodes":[]`, `"containment":[]`, `"dependencies":[]`) {
		t.Fatalf("unexpected JSON %s", data)
	}
}

func TestPublicContractAndBoundedCollections(t *testing.T) {
	a, b := testNode("a", "a"), testNode("b", "b")
	a.Evidence = []DependencyEvidence{testEvidence("z"), testEvidence("a")}
	input := GraphInput{
		SourceArtifacts: []ArtifactReference{{"z", "1.0.0", ""}, {"a", "1.0.0", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}},
		Nodes:           []NodeCandidate{a, a, b},
		Containment: []ContainmentCandidate{
			{ContainmentModulePackage, a.Identity, b.Identity, []DependencyEvidence{testEvidence("1"), testEvidence("2")}},
			{ContainmentPackageFile, a.Identity, b.Identity, nil},
		},
		Dependencies: []DependencyCandidate{
			{GraphPackage, DependencyImports, a.Identity, b.Identity, ResolvedLocal, nil},
			{GraphFile, DependencyReferences, a.Identity, b.Identity, ResolvedLocal, nil},
		},
		Diagnostics: []DiagnosticCandidate{
			{"z", GraphPackage, "z.go", 2, 1, "z", "z"},
			{"a", GraphPackage, "a.go", 1, 1, "a", "a"},
		},
	}
	core := newTestCore(t, ConfigParams{MaxEdges: 1, MaxEvidencePerItem: 1, MaxDiagnostics: 1})
	if core.Name() != EngineName || core.Version() != EngineVersion {
		t.Fatal("unexpected core metadata")
	}
	got, err := core.Normalize(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if got.ArtifactName() != ArtifactName || got.ArtifactVersion() != ArtifactVersion || got.Metadata().ContainmentIDSchemeVersion != "dependency-containment-id/v1" {
		t.Fatal("unexpected artifact metadata")
	}
	if len(got.SourceArtifacts()) != 2 || len(got.Containment()) != 1 || len(got.Diagnostics()) != 1 || len(got.StrongComponents()) != 0 || len(got.Cycles()) != 0 {
		t.Fatal("unexpected bounded collections")
	}
	stats := got.Statistics()
	if stats.OmittedContainment != 1 || stats.OmittedEdges != 1 || stats.OmittedDiagnostics != 1 || stats.OmittedEvidence != 2 {
		t.Fatalf("unexpected statistics %#v", stats)
	}
}

func TestValidationErrors(t *testing.T) {
	cases := []struct {
		name  string
		input GraphInput
		kind  ErrorKind
	}{
		{"bad source", GraphInput{SourceArtifacts: []ArtifactReference{{"", "1", ""}}}, ErrorInvalidInput},
		{"bad digest", GraphInput{SourceArtifacts: []ArtifactReference{{"a", "1", "ABC"}}}, ErrorInvalidInput},
		{"duplicate source", GraphInput{SourceArtifacts: []ArtifactReference{{"a", "1", ""}, {"a", "1", ""}}}, ErrorInvalidInput},
		{"bad node", GraphInput{Nodes: []NodeCandidate{{Identity: NodeIdentity{QualifiedName: "x"}, SourceIdentity: testSource("x")}}}, ErrorInvalidInput},
		{"bad source identity", GraphInput{Nodes: []NodeCandidate{{Identity: testNode("a", "a").Identity}}}, ErrorInvalidInput},
		{"conflicting node", GraphInput{Nodes: []NodeCandidate{testNode("a", "a"), func() NodeCandidate { n := testNode("a", "a"); n.Name = "other"; return n }()}}, ErrorIntegrity},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := newTestCore(t, ConfigParams{}).Normalize(context.Background(), tc.input)
			if ErrorKindOf(err) != tc.kind {
				t.Fatalf("got %v", err)
			}
		})
	}
	a, b := testNode("a", "a"), testNode("b", "b")
	more := []GraphInput{
		{Nodes: []NodeCandidate{a, b}, Containment: []ContainmentCandidate{{ContainmentKind("bad"), a.Identity, b.Identity, nil}}},
		{Nodes: []NodeCandidate{a, b}, Dependencies: []DependencyCandidate{{GraphKind("bad"), DependencyImports, a.Identity, b.Identity, ResolvedLocal, nil}}},
		{Nodes: []NodeCandidate{a}, Diagnostics: []DiagnosticCandidate{{"", GraphPackage, "", 0, 0, "", "bad"}}},
		{Nodes: []NodeCandidate{a}, Diagnostics: []DiagnosticCandidate{{"x", GraphPackage, "C:/secret", 0, 0, "", "bad"}}},
		{Nodes: []NodeCandidate{a}, Dependencies: []DependencyCandidate{{GraphPackage, DependencyImports, a.Identity, a.Identity, ResolvedLocal, []DependencyEvidence{{Source: testSource("x"), StartLine: -1, Rule: "r"}}}}},
	}
	for i, input := range more {
		if _, err := newTestCore(t, ConfigParams{}).Normalize(context.Background(), input); err == nil {
			t.Fatalf("case %d unexpectedly passed", i)
		}
	}
}

func TestConfigurationAndErrors(t *testing.T) {
	if DefaultConfig().MaxNodes() != DefaultMaxNodes {
		t.Fatal("bad default")
	}
	if _, err := NewConfig(ConfigParams{MaxNodes: MaximumMaxNodes + 1}); err == nil {
		t.Fatal("expected max node validation")
	}
	if _, err := NewConfig(ConfigParams{MaxEdges: MaximumMaxEdges + 1}); err == nil {
		t.Fatal("expected max edge validation")
	}
	if _, err := NewConfig(ConfigParams{MaxEvidencePerItem: MaximumMaxEvidencePerItem + 1}); err == nil {
		t.Fatal("expected max evidence validation")
	}
	if _, err := NewConfig(ConfigParams{MaxDiagnostics: MaximumMaxDiagnostics + 1}); err == nil {
		t.Fatal("expected max diagnostics validation")
	}
	if _, err := New(Config{}); err == nil {
		t.Fatal("expected constructor validation")
	}
	err := newError(ErrorInvalidInput, "code", "message", context.Canceled)
	if err.Error() != "message" {
		t.Fatal(err)
	}
	var typed *Error
	if !errors.As(err, &typed) || typed.Code() != "code" {
		t.Fatal("bad typed error")
	}
	if ErrorKindOf(errors.New("raw")) != ErrorInternal {
		t.Fatal("raw error kind")
	}
}
func containsAll(s string, values ...string) bool {
	for _, v := range values {
		if !contains(s, v) {
			return false
		}
	}
	return true
}
func contains(s, v string) bool {
	for i := 0; i+len(v) <= len(s); i++ {
		if s[i:i+len(v)] == v {
			return true
		}
	}
	return false
}

func FuzzNormalizeNeverPanics(f *testing.F) {
	f.Add("pkg", "pkg/file.go")
	f.Fuzz(func(t *testing.T, name, p string) {
		if len(name) > 256 || len(p) > 256 {
			return
		}
		core := newTestCore(t, ConfigParams{MaxNodes: 10})
		_, _ = core.Normalize(context.Background(), GraphInput{Nodes: []NodeCandidate{testNode(name, p)}})
	})
}

func TestScaleGate100KNodes1MEdges(t *testing.T) {
	if os.Getenv("DIE_CORE_SCALE") != "1" {
		t.Skip("set DIE_CORE_SCALE=1")
	}
	nodes := make([]NodeCandidate, 100_000)
	for i := range nodes {
		nodes[i] = testNode(fmt.Sprintf("example.com/p%06d", i), fmt.Sprintf("p/%06d", i))
	}
	edges := make([]DependencyCandidate, 1_000_000)
	for i := range edges {
		from := i % len(nodes)
		to := (from + 1 + i/len(nodes)) % len(nodes)
		edges[i] = DependencyCandidate{GraphPackage, DependencyImports, nodes[from].Identity, nodes[to].Identity, ResolvedLocal, nil}
	}
	core := newTestCore(t, ConfigParams{MaxNodes: 100_000, MaxEdges: 1_000_000})
	var peak atomic.Uint64
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				for old := peak.Load(); m.HeapAlloc > old && !peak.CompareAndSwap(old, m.HeapAlloc); old = peak.Load() {
				}
			}
		}
	}()
	start := time.Now()
	result, err := core.Normalize(context.Background(), GraphInput{Nodes: nodes, Dependencies: edges})
	close(done)
	if err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	if len(result.Nodes()) != 100_000 || len(result.Dependencies()) != 1_000_000 {
		t.Fatal("scale result incomplete")
	}
	if elapsed >= 30*time.Second {
		t.Fatalf("elapsed %s exceeds gate", elapsed)
	}
	if peak.Load() >= 2*1024*1024*1024 {
		t.Fatalf("peak heap %d exceeds 2 GiB", peak.Load())
	}
	t.Logf("elapsed=%s peak_live_heap=%d", elapsed, peak.Load())
}
