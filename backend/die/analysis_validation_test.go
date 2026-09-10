package die

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"runtime"
	"sort"
	"sync/atomic"
	"testing"
	"time"
)

// A deterministic checkpoint trigger makes cancellation sites reproducible;
// unlike sleeping goroutines it does not depend on host scheduling.
type analysisCheckpointContext struct {
	context.Context
	calls, at int
	trigger   time.Time
}

func (c *analysisCheckpointContext) Err() error {
	c.calls++
	if c.at > 0 && c.calls >= c.at {
		if c.trigger.IsZero() {
			c.trigger = time.Now()
		}
		return context.Canceled
	}
	return nil
}

func TestAnalysisEveryCancellationCheckpoint(t *testing.T) {
	inv, ids := analysisFixture(t, 4, [][2]int{{0, 1}, {1, 2}, {2, 0}, {0, 3}, {3, 3}})
	inv.view.Containment = []ContainmentEdge{{ID: containmentID(ContainmentModulePackage, ids[0], ids[1]), Kind: ContainmentModulePackage, ParentID: ids[0], ChildID: ids[1], Evidence: []DependencyEvidence{testEvidence("c")}}}
	inv.view.Nodes[0].Evidence = []DependencyEvidence{testEvidence("n")}
	inv.view.Dependencies[0].Evidence = []DependencyEvidence{testEvidence("e")}
	a := analysisDefault(t)
	inv, e := a.Analyze(context.Background(), inv)
	if e != nil {
		t.Fatal(e)
	}
	calls := []func(context.Context) error{
		func(ctx context.Context) error {
			out, e := a.Analyze(ctx, inv)
			if e != nil && out.view.Artifact.Name != "" {
				t.Fatal("partial artifact")
			}
			return e
		},
		func(ctx context.Context) error {
			out, e := a.DirectDependencies(ctx, inv, nodeQ(t, ids[0], 1, ""))
			if e != nil && out.view.InputDigest != "" {
				t.Fatal("partial page")
			}
			return e
		},
		func(ctx context.Context) error {
			out, e := a.Impact(ctx, inv, impactQ(t, ids[:1], Dependencies, 64, 0, 0))
			if e != nil && out.view.InputDigest != "" {
				t.Fatal("partial impact")
			}
			return e
		},
	}
	total := 0
	var max time.Duration
	for _, call := range calls {
		ctx := &analysisCheckpointContext{Context: context.Background()}
		if e := call(ctx); e != nil {
			t.Fatal(e)
		}
		total += ctx.calls
		for checkpoint := 1; checkpoint <= ctx.calls; checkpoint++ {
			c := &analysisCheckpointContext{Context: context.Background(), at: checkpoint}
			if e := call(c); !errors.Is(e, context.Canceled) {
				t.Fatalf("checkpoint %d: %v", checkpoint, e)
			}
			elapsed := time.Since(c.trigger)
			if elapsed > max {
				max = elapsed
			}
		}
	}
	t.Logf("cancellation_checkpoints=%d maximum_observed_return_latency_ns=%d", total, max.Nanoseconds())
}

func TestAnalysisProjectionAndAuxiliaryCaps(t *testing.T) {
	input := GraphInput{}
	for _, kind := range []NodeKind{NodeModule, NodePackage, NodeFile} {
		for j := 0; j < 2; j++ {
			n := testNode(fmt.Sprintf("%s%d", kind, j), fmt.Sprintf("%s%d", kind, j))
			n.Identity.Kind = kind
			input.Nodes = append(input.Nodes, n)
		}
	}
	for j := 0; j < 6; j += 2 {
		g := GraphKind(input.Nodes[j].Identity.Kind)
		for _, p := range [][2]int{{j, j + 1}, {j + 1, j}} {
			for _, kind := range []DependencyKind{DependencyImports, DependencyReferences} {
				input.Dependencies = append(input.Dependencies, DependencyCandidate{Graph: g, Kind: kind, From: input.Nodes[p[0]].Identity, To: input.Nodes[p[1]].Identity, Resolution: ResolvedLocal, Evidence: []DependencyEvidence{testEvidence("e")}})
			}
		}
	}
	input.Dependencies = append(input.Dependencies, DependencyCandidate{Graph: GraphFile, Kind: DependencyImports, From: input.Nodes[4].Identity, To: input.Nodes[2].Identity, Resolution: ResolvedLocal})
	input.Containment = []ContainmentCandidate{{Kind: ContainmentModulePackage, Parent: input.Nodes[0].Identity, Child: input.Nodes[2].Identity, Evidence: []DependencyEvidence{testEvidence("c")}}}
	inv, e := newTestCore(t, ConfigParams{}).Normalize(context.Background(), input)
	if e != nil {
		t.Fatal(e)
	}
	a := analysisDefault(t)
	out, e := a.Analyze(context.Background(), inv)
	if e != nil {
		t.Fatal(e)
	}
	if len(out.Cycles()) != 3 || len(out.StrongComponents()) != 3 {
		t.Fatal("projection mixed")
	}
	for _, s := range out.Analysis().Graphs {
		if s.EligibleNodes != 2 || s.EligibleEdges != 4 || s.Components != 1 || s.CyclicComponents != 1 {
			t.Fatal(s)
		}
		if (s.Graph == GraphFile) != s.TopologyLimited {
			t.Fatal(s)
		}
	}
	boundaryRoot := nodeID(input.Nodes[2].Identity)
	q, e := NewNodeQuery(NodeQueryParams{NodeID: boundaryRoot, Graph: GraphFile})
	if e != nil {
		t.Fatal(e)
	}
	page, e := a.DirectDependents(context.Background(), inv, q)
	if e != nil || len(page.View().Neighbors) != 1 {
		t.Fatal(page, e)
	}
	imp, e := NewImpactQuery(ImpactQueryParams{NodeIDs: []string{nodeID(input.Nodes[4].Identity)}, Graph: GraphFile, Direction: Dependencies})
	if e != nil {
		t.Fatal(e)
	}
	result, e := a.Impact(context.Background(), inv, imp)
	if e != nil || len(result.View().BoundaryNodeIDs) != 1 || len(result.View().Reached) != 1 {
		t.Fatal(result, e)
	}
	small := a.config
	small.p.MaxInputEvidence = 1
	if _, e := (&graphAnalyzer{small}).Analyze(context.Background(), inv); ErrorKindOf(e) != ErrorLimitExceeded {
		t.Fatal(e)
	}
	small = a.config
	small.p.MaxInputAuxRecords = 1
	if _, e := (&graphAnalyzer{small}).Analyze(context.Background(), out); ErrorKindOf(e) != ErrorLimitExceeded {
		t.Fatal(e)
	}
	for _, mutate := range []func(*DependencyInventoryView){func(v *DependencyInventoryView) { v.Containment[0].ID = "bad" }, func(v *DependencyInventoryView) { v.Containment = append(v.Containment, v.Containment[0]) }, func(v *DependencyInventoryView) { v.Dependencies[0].Resolution = "bad" }, func(v *DependencyInventoryView) { v.Dependencies[0].Occurrences = 0 }, func(v *DependencyInventoryView) { v.Nodes[0].Kind = "bad" }} {
		v := cloneView(inv.view)
		mutate(&v)
		if _, e := a.Analyze(context.Background(), DependencyInventory{v}); ErrorKindOf(e) != ErrorIntegrity {
			t.Fatal(e)
		}
	}
	for _, tc := range []struct {
		value uint64
		n     int
		limit uint64
	}{{^uint64(0), 1, 1}, {1, 1, 1}, {0, -1, 1}} {
		v := tc.value
		if e := countAnalysis(&v, tc.n, tc.limit); e == nil {
			t.Fatal("counter overflow")
		}
	}
	empty, ids := analysisFixture(t, 1, nil)
	p, e := a.DirectDependencies(context.Background(), empty, nodeQ(t, ids[0], 0, ""))
	if e != nil || len(p.View().Neighbors) != 0 || p.View().HasMore {
		t.Fatal(p, e)
	}
	if _, e := a.DirectDependencies(context.Background(), empty, nodeQ(t, boundaryRoot, 0, "")); e == nil {
		t.Fatal("unknown root")
	}
}

func TestAnalysisDeterminismEvidence(t *testing.T) {
	input := GraphInput{}
	for i := 0; i < 64; i++ {
		input.Nodes = append(input.Nodes, testNode(fmt.Sprintf("fixture/π%d", i), fmt.Sprintf("pkg/%d", i)))
	}
	for i := 0; i < 256; i++ {
		from := i % 64
		to := (from + 1 + i/64) % 64
		input.Dependencies = append(input.Dependencies, DependencyCandidate{Graph: GraphPackage, Kind: DependencyImports, From: input.Nodes[from].Identity, To: input.Nodes[to].Identity, Resolution: ResolvedLocal, Evidence: []DependencyEvidence{testEvidence("e")}})
	}
	var expected []byte
	var inputDigest string
	for seed := int64(0); seed < 8; seed++ {
		r := rand.New(rand.NewSource(seed))
		r.Shuffle(len(input.Nodes), func(i, j int) { input.Nodes[i], input.Nodes[j] = input.Nodes[j], input.Nodes[i] })
		r.Shuffle(len(input.Dependencies), func(i, j int) {
			input.Dependencies[i], input.Dependencies[j] = input.Dependencies[j], input.Dependencies[i]
		})
		inv, e := newTestCore(t, ConfigParams{}).Normalize(context.Background(), input)
		if e != nil {
			t.Fatal(e)
		}
		a := analysisDefault(t)
		out, e := a.Analyze(context.Background(), inv)
		if e != nil {
			t.Fatal(e)
		}
		ids := []string{}
		for _, n := range inv.Nodes() {
			ids = append(ids, n.ID)
		}
		sort.Strings(ids)
		page, e := a.DirectDependencies(context.Background(), inv, nodeQ(t, ids[0], 1, ""))
		if e != nil {
			t.Fatal(e)
		}
		impact, e := a.Impact(context.Background(), inv, impactQ(t, ids[:2], Dependents, 3, 16, 32))
		if e != nil {
			t.Fatal(e)
		}
		b, e := json.Marshal([]any{out, page, impact})
		if e != nil {
			t.Fatal(e)
		}
		if seed == 0 {
			expected = b
			inputDigest = out.Analysis().InputDigest
		} else if string(b) != string(expected) {
			t.Fatal("shuffle mismatch")
		}
	}
	sum := sha256.Sum256(expected)
	t.Logf("analysis_fixture nodes=64 edges=256 shuffles=8 input_digest=%s combined_output_sha256=%s", inputDigest, hex.EncodeToString(sum[:]))
}

func TestAnalysisScaleCharacterization(t *testing.T) {
	if os.Getenv("DIE_ANALYSIS_SCALE") != "1" {
		t.Skip("opt-in 100k/1m memory characterization")
	}
	// Exact accepted core benchmark topology, node names, paths, and evidence.
	nodes := make([]NodeCandidate, 100000)
	for i := range nodes {
		nodes[i] = testNode(fmt.Sprintf("example.com/p%06d", i), fmt.Sprintf("p/%06d", i))
	}
	edges := make([]DependencyCandidate, 1000000)
	for i := range edges {
		from := i % len(nodes)
		to := (from + 1 + i/len(nodes)) % len(nodes)
		edges[i] = DependencyCandidate{GraphPackage, DependencyImports, nodes[from].Identity, nodes[to].Identity, ResolvedLocal, nil}
	}
	inv, e := newTestCore(t, ConfigParams{MaxNodes: 100000, MaxEdges: 1000000}).Normalize(context.Background(), GraphInput{Nodes: nodes, Dependencies: edges})
	if e != nil {
		t.Fatal(e)
	}
	nodes = nil
	edges = nil
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	var peak atomic.Uint64
	peak.Store(before.HeapAlloc)
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				for old := peak.Load(); m.HeapAlloc > old; old = peak.Load() {
					if peak.CompareAndSwap(old, m.HeapAlloc) {
						break
					}
				}
			}
		}
	}()
	started := time.Now()
	a := analysisDefault(t)
	out, e := a.Analyze(context.Background(), inv)
	elapsed := time.Since(started)
	close(done)
	<-stopped
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	if after.HeapAlloc > peak.Load() {
		peak.Store(after.HeapAlloc)
	}
	if e != nil {
		t.Fatal(e)
	}
	if len(out.view.StrongComponents) != 1 || len(out.view.Cycles) != 1 {
		t.Fatal("scale partition")
	}
	t.Logf("analysis_scale nodes=100000 edges=1000000 gomaxprocs=%d duration_ns=%d retained_input_heap_bytes=%d sampled_peak_heap_bytes=%d allocated_bytes=%d input_digest=%s", runtime.GOMAXPROCS(0), elapsed.Nanoseconds(), before.HeapAlloc, peak.Load(), after.TotalAlloc-before.TotalAlloc, out.Analysis().InputDigest)
	runtime.KeepAlive(inv)
	runtime.KeepAlive(out)
}

func BenchmarkAnalysis(b *testing.B) {
	for _, n := range []int{100, 1000, 10000} {
		pairs := make([][2]int, n*4)
		for i := range pairs {
			pairs[i] = [2]int{i % n, (i%n + 1 + i/n) % n}
		}
		inv, ids := analysisFixture(b, n, pairs)
		a := analysisDefault(b)
		b.Run(fmt.Sprintf("IndexAndDigest/%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				if _, e := a.index(context.Background(), inv); e != nil {
					b.Fatal(e)
				}
			}
		})
		b.Run(fmt.Sprintf("SCCAndPublication/%d", n), func(b *testing.B) {
			x, e := a.index(context.Background(), inv)
			if e != nil {
				b.Fatal(e)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				for g, s := range x.summary {
					s.Components = 0
					s.CyclicComponents = 0
					x.summary[g] = s
				}
				if _, e := a.analyzeIndex(context.Background(), inv, x); e != nil {
					b.Fatal(e)
				}
			}
		})
		b.Run(fmt.Sprintf("Analyze/%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				if _, e := a.Analyze(context.Background(), inv); e != nil {
					b.Fatal(e)
				}
			}
		})
		b.Run(fmt.Sprintf("Direct/%d", n), func(b *testing.B) {
			q := nodeQ(b, ids[0], 2, "")
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if _, e := a.DirectDependencies(context.Background(), inv, q); e != nil {
					b.Fatal(e)
				}
			}
		})
		b.Run(fmt.Sprintf("Impact/%d", n), func(b *testing.B) {
			q := impactQ(b, ids[:1], Dependencies, 16, 1000, 10000)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if _, e := a.Impact(context.Background(), inv, q); e != nil {
					b.Fatal(e)
				}
			}
		})
		b.Run(fmt.Sprintf("Fingerprint/%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				if _, e := analysisFingerprint(context.Background(), inv.view); e != nil {
					b.Fatal(e)
				}
			}
		})
		b.Run(fmt.Sprintf("OutputClone/%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				v := cloneView(inv.view)
				runtime.KeepAlive(v)
			}
		})
	}
}

func TestAnalysisFingerprintMatchesIndependentSerialization(t *testing.T) {
	inv, _ := analysisFixture(t, 4, [][2]int{{0, 1}, {1, 2}, {2, 3}})
	got, e := analysisFingerprint(context.Background(), inv.view)
	if e != nil {
		t.Fatal(e)
	}
	// Compare record-streaming against the released JSON representation, separately
	// from the independently frozen byte vectors, to catch future field omission.
	b, e := json.Marshal(inv)
	if e != nil {
		t.Fatal(e)
	}
	prefix := append([]byte{0, 0, 0, 0, 0, 0, 0, 28}, []byte("dependency-analysis-input/v1")...)
	prefix = append(prefix, b...)
	prefix = append(prefix, '\n')
	sum := sha256.Sum256(prefix)
	if got != AnalysisDigestScheme+":sha256:"+hex.EncodeToString(sum[:]) {
		t.Fatal("base serialization drift")
	}
	out, e := analysisDefault(t).Analyze(context.Background(), inv)
	if e != nil {
		t.Fatal(e)
	}
	got2, e := analysisFingerprint(context.Background(), out.view)
	if e != nil || !reflect.DeepEqual(got, got2) {
		t.Fatal("derived fields entered digest")
	}
}

func TestAnalysisBoundaryPromotesAtLaterDepth(t *testing.T) {
	inv, ids := analysisFixture(t, 4, [][2]int{{0, 2}, {2, 1}, {1, 3}})
	e := DependencyEdge{Graph: GraphPackage, Kind: DependencyImports, FromNodeID: ids[0], ToNodeID: ids[1], Resolution: Ambiguous, Occurrences: 1}
	e.ID = edgeID(e.Graph, e.Kind, e.FromNodeID, e.ToNodeID, e.Resolution)
	inv.view.Dependencies = append(inv.view.Dependencies, e)
	r, err := analysisDefault(t).Impact(context.Background(), inv, impactQ(t, ids[:1], Dependencies, 64, 4, 0))
	if err != nil {
		t.Fatal(err)
	}
	w := r.View()
	if w.VisitedNodes != 4 || len(w.BoundaryNodeIDs) != 0 || len(w.Reached) != 3 || w.Truncated {
		t.Fatal(w)
	}
	for _, v := range w.Reached {
		if v.NodeID == ids[1] && v.Depth != 2 || v.NodeID == ids[3] && v.Depth != 3 {
			t.Fatal(w)
		}
	}
}
