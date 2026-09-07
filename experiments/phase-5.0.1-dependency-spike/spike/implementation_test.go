package spike

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"runtime"
	"sort"
	"testing"
	"time"
)

func TestStableIDGoldenVectors(t *testing.T) {
	identity := NodeIdentity{Kind: NodePackage, Language: "Go", QualifiedName: "example.com/π/pkg", Path: "src\\pkg", Resolution: ResolvedLocal}
	node := nodeID(identity)
	edge := edgeID(edgeKey{graph: GraphPackage, kind: EdgeImports, from: node, to: "dependency-node-id/v1:sha256:target", resolution: ResolvedLocal})
	component := componentID(GraphPackage, []string{"a", "β"})
	cycles := cycleID(GraphPackage, component)

	want := []string{
		"dependency-node-id/v1:sha256:16c0f7392130f9bad33f66087e9c0b46c1f2823ba8cf7ea491187b35494c8a64",
		"dependency-edge-id/v1:sha256:bf8b464f5cbb5551b28068da1445e8e45359e2586f860ede189d9ac68b4c8ad1",
		"dependency-scc-id/v1:sha256:360e477842ffa8936cb211fa0ef8e621043e3f06105f680537a5d478d7f0a2f8",
		"dependency-cycle-id/v1:sha256:455fdfe1396a0c675860713ff4be826465b6eb83982e3bd7ecce66734c42eb8c",
	}
	got := []string{node, edge, component, cycles}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("golden vectors changed:\ngot  %#v\nwant %#v", got, want)
	}
}

func TestResultCanonicalJSONGolden(t *testing.T) {
	result, err := mustRunner(t, Config{Workers: 8}).Run(context.Background(), cyclicFixture())
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	got := sha256.Sum256(encoded)
	want := "5ee754fe619d0920f2473e06a04f6bd1372b768c565c82c65281c7b2e922b6dc"
	if fmt.Sprintf("%x", got) != want {
		t.Fatalf("canonical result SHA-256 = %x, want %s", got, want)
	}
}

func TestNormalizationIsDeterministicAcrossOrderAndWorkers(t *testing.T) {
	input := cyclicFixture()
	one := mustRunner(t, Config{Workers: 1})
	eight := mustRunner(t, Config{Workers: 8})
	want, err := one.Run(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}

	shuffled := cloneInput(input)
	rand.New(rand.NewSource(42)).Shuffle(len(shuffled.Nodes), func(i, j int) { shuffled.Nodes[i], shuffled.Nodes[j] = shuffled.Nodes[j], shuffled.Nodes[i] })
	rand.New(rand.NewSource(84)).Shuffle(len(shuffled.Edges), func(i, j int) { shuffled.Edges[i], shuffled.Edges[j] = shuffled.Edges[j], shuffled.Edges[i] })
	got, err := eight.Run(context.Background(), shuffled)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("one/eight-worker or input-order mismatch\none=%#v\neight=%#v", want, got)
	}
	a, _ := json.Marshal(want)
	b, _ := json.Marshal(got)
	if string(a) != string(b) {
		t.Fatal("canonical JSON differs")
	}
}

func TestAggregationEvidenceAndOmissionAreCanonical(t *testing.T) {
	a := packageNode("a")
	b := packageNode("b")
	evidence := []Evidence{
		{Artifact: "go-semantic-inventory", Version: "1.0.0", SourceID: "z", File: "z.go", Line: 2, Rule: "import"},
		{Artifact: "go-semantic-inventory", Version: "1.0.0", SourceID: "a", File: "a.go", Line: 1, Rule: "import"},
		{Artifact: "go-semantic-inventory", Version: "1.0.0", SourceID: "m", File: "m.go", Line: 3, Rule: "import"},
	}
	edges := []RawEdge{}
	for _, item := range evidence {
		edges = append(edges, RawEdge{Graph: GraphPackage, Kind: EdgeImports, From: a, To: b, Resolution: ResolvedLocal, Evidence: item})
	}
	edges = append(edges, edges[1])
	runner := mustRunner(t, Config{MaxEvidencePerEdge: 2})
	result, err := runner.Run(context.Background(), Input{Nodes: []NodeIdentity{a, b}, Edges: edges})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Edges) != 1 || result.Edges[0].Occurrences != 4 || result.Edges[0].OmittedEvidence != 1 {
		t.Fatalf("unexpected aggregation: %#v", result.Edges)
	}
	if result.Edges[0].Evidence[0].SourceID != "a" || result.Edges[0].Evidence[1].SourceID != "m" {
		t.Fatalf("evidence cap was not canonical: %#v", result.Edges[0].Evidence)
	}
}

func TestSCCAndCycleClassification(t *testing.T) {
	result, err := mustRunner(t, Config{}).Run(context.Background(), cyclicFixture())
	if err != nil {
		t.Fatal(err)
	}
	classes := map[GraphKind]string{}
	for _, cycle := range result.Cycles {
		classes[cycle.Graph] = cycle.Classification
	}
	if classes[GraphPackage] != "language_invalid" {
		t.Fatalf("package cycle: %q", classes[GraphPackage])
	}
	if classes[GraphFile] != "informational" {
		t.Fatalf("file cycle: %q", classes[GraphFile])
	}
	if classes[GraphModule] != "structural" {
		t.Fatalf("module cycle: %q", classes[GraphModule])
	}
	for _, cycle := range result.Cycles {
		if cycle.Rule == "" {
			t.Fatal("cycle classification omitted its deterministic rule")
		}
	}
}

func TestImpactForwardReverseAndLimits(t *testing.T) {
	a, b, c, d := packageNode("a"), packageNode("b"), packageNode("c"), packageNode("d")
	input := Input{Nodes: []NodeIdentity{a, b, c, d}, Edges: []RawEdge{
		raw(GraphPackage, a, b), raw(GraphPackage, a, c), raw(GraphPackage, b, d), raw(GraphPackage, c, d),
	}}
	runner := mustRunner(t, Config{})
	graph, err := runner.Run(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	forward, err := runner.Impact(context.Background(), graph, ImpactQuery{NodeID: nodeID(a), Graph: GraphPackage, Direction: Dependencies})
	if err != nil {
		t.Fatal(err)
	}
	if len(forward.Nodes) != 3 || forward.Nodes[0].Depth != 1 || forward.Nodes[2].Depth != 2 {
		t.Fatalf("forward=%#v", forward)
	}
	reverse, err := runner.Impact(context.Background(), graph, ImpactQuery{NodeID: nodeID(d), Graph: GraphPackage, Direction: Dependents})
	if err != nil {
		t.Fatal(err)
	}
	if len(reverse.Nodes) != 3 {
		t.Fatalf("reverse=%#v", reverse)
	}
	limited, err := runner.Impact(context.Background(), graph, ImpactQuery{NodeID: nodeID(a), Graph: GraphPackage, Direction: Dependencies, MaxNodes: 1, MaxDepth: 8})
	if err != nil {
		t.Fatal(err)
	}
	if !limited.Truncated || len(limited.Nodes) != 1 {
		t.Fatalf("limited=%#v", limited)
	}
}

func TestUnknownStatesNeverCreateLocalSCCEdges(t *testing.T) {
	a, b := packageNode("a"), packageNode("b")
	input := Input{Nodes: []NodeIdentity{a, b}, Edges: []RawEdge{
		{Graph: GraphPackage, Kind: EdgeImports, From: a, To: b, Resolution: Unresolved},
		{Graph: GraphPackage, Kind: EdgeImports, From: b, To: a, Resolution: Ambiguous},
	}}
	result, err := mustRunner(t, Config{}).Run(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Cycles) != 0 {
		t.Fatalf("unknown edges formed a local cycle: %#v", result.Cycles)
	}
}

func TestCancellationAtBoundedCheckpoint(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := mustRunner(t, Config{Workers: 1}).(*runner)
	r.checkpoint = func(processed int) {
		if processed >= checkpointInterval {
			cancel()
		}
	}
	nodes, edges := layeredFixture(2_100, 2_048)
	_, err := r.Run(ctx, Input{Nodes: nodes, Edges: edges})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

func TestSCCCancellationChecksHighFanoutEdges(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	nodes := make([]Node, 2_000)
	for index := range nodes {
		identity := packageNode(fmtID(index))
		nodes[index] = Node{ID: nodeID(identity), NodeIdentity: identity}
	}
	edges := make([]Edge, 0, len(nodes)-1)
	for index := 1; index < len(nodes); index++ {
		key := edgeKey{graph: GraphPackage, kind: EdgeImports, from: nodes[0].ID, to: nodes[index].ID, resolution: ResolvedLocal}
		edges = append(edges, Edge{ID: edgeID(key), Graph: GraphPackage, Kind: EdgeImports, FromID: key.from, ToID: key.to, Resolution: ResolvedLocal})
	}
	_, _, err := analyzeComponents(ctx, nodes, edges, func(processed int) {
		if processed >= checkpointInterval {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("SCC cancellation error=%v", err)
	}
}

func TestInvalidInputsFailClosed(t *testing.T) {
	if _, err := New(Config{Workers: 9}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("config error=%v", err)
	}
	runner := mustRunner(t, Config{})
	a, b := packageNode("a"), packageNode("b")
	_, err := runner.Run(context.Background(), Input{Nodes: []NodeIdentity{a}, Edges: []RawEdge{raw(GraphPackage, a, b)}})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("input error=%v", err)
	}
	_, err = runner.Impact(context.Background(), Result{}, ImpactQuery{NodeID: "missing", Graph: GraphPackage, Direction: Dependencies})
	if !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("query error=%v", err)
	}
}

func TestEdgeLimitUsesCanonicalOrder(t *testing.T) {
	a, b, c := packageNode("a"), packageNode("b"), packageNode("c")
	input := Input{Nodes: []NodeIdentity{a, b, c}, Edges: []RawEdge{raw(GraphPackage, c, a), raw(GraphPackage, a, c), raw(GraphPackage, a, b)}}
	one := mustRunner(t, Config{Workers: 1, MaxEdges: 2})
	eight := mustRunner(t, Config{Workers: 8, MaxEdges: 2})
	want, err := one.Run(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	input.Edges[0], input.Edges[2] = input.Edges[2], input.Edges[0]
	got, err := eight.Run(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if want.Statistics.OmittedEdges != 1 || !reflect.DeepEqual(want, got) {
		t.Fatalf("canonical edge omission mismatch: want=%#v got=%#v", want, got)
	}
}

func mustRunner(t *testing.T, config Config) Runner {
	t.Helper()
	value, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	return value
}
func packageNode(name string) NodeIdentity {
	return NodeIdentity{Kind: NodePackage, Language: "Go", QualifiedName: "example.com/" + name, Path: name, Resolution: ResolvedLocal}
}
func moduleNode(name string) NodeIdentity {
	return NodeIdentity{Kind: NodeModule, Language: "Go", QualifiedName: "example.com/" + name, Path: name, Resolution: ResolvedLocal}
}
func fileNode(name string) NodeIdentity {
	return NodeIdentity{Kind: NodeFile, Language: "Go", QualifiedName: name + ".go", Path: "pkg/" + name + ".go", Resolution: ResolvedLocal}
}
func raw(graph GraphKind, from, to NodeIdentity) RawEdge {
	return RawEdge{Graph: graph, Kind: EdgeImports, From: from, To: to, Resolution: ResolvedLocal, Evidence: Evidence{Artifact: "go-semantic-inventory", Version: "1.0.0", SourceID: from.QualifiedName + "->" + to.QualifiedName, Rule: "fixture"}}
}
func cyclicFixture() Input {
	m1, m2 := moduleNode("m1"), moduleNode("m2")
	p1, p2 := packageNode("p1"), packageNode("p2")
	f1, f2 := fileNode("f1"), fileNode("f2")
	return Input{Nodes: []NodeIdentity{m1, m2, p1, p2, f1, f2}, Edges: []RawEdge{raw(GraphModule, m1, m2), raw(GraphModule, m2, m1), raw(GraphPackage, p1, p2), raw(GraphPackage, p2, p1), raw(GraphFile, f1, f2), raw(GraphFile, f2, f1)}}
}
func cloneInput(input Input) Input {
	return Input{Nodes: append([]NodeIdentity(nil), input.Nodes...), Edges: append([]RawEdge(nil), input.Edges...)}
}
func layeredFixture(nodeCount, edgeCount int) ([]NodeIdentity, []RawEdge) {
	nodes := make([]NodeIdentity, nodeCount)
	for i := range nodes {
		nodes[i] = packageNode(fmtID(i))
	}
	edges := make([]RawEdge, 0, edgeCount)
	half := nodeCount / 2
	for i := 0; i < edgeCount; i++ {
		from := i % half
		to := half + ((i/half)*7919+from)%max(1, nodeCount-half)
		edges = append(edges, raw(GraphPackage, nodes[from], nodes[to]))
	}
	return nodes, edges
}
func fmtID(value int) string {
	const digits = "0123456789"
	buffer := []byte("n000000")
	for i := len(buffer) - 1; i >= 1; i-- {
		buffer[i] = digits[value%10]
		value /= 10
	}
	return string(buffer)
}

func TestReferenceSCCPartition(t *testing.T) {
	input := cyclicFixture()
	result, err := mustRunner(t, Config{}).Run(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	for _, graph := range []GraphKind{GraphModule, GraphPackage, GraphFile} {
		ids := []string{}
		for _, node := range result.Nodes {
			if nodeMatchesGraph(node, graph) {
				ids = append(ids, node.ID)
			}
		}
		sort.Strings(ids)
		members := []string{}
		for _, component := range result.Components {
			if component.Graph == graph {
				members = append(members, component.NodeIDs...)
			}
		}
		sort.Strings(members)
		if !reflect.DeepEqual(ids, members) {
			t.Fatalf("partition %s: nodes=%v members=%v", graph, ids, members)
		}
	}
}

func TestScaleGate100KNodes1MEdges(t *testing.T) {
	if os.Getenv("DIE_SPIKE_SCALE") != "1" {
		t.Skip("set DIE_SPIKE_SCALE=1 to run the bounded design-spike scale gate")
	}
	nodes, edges := layeredFixture(100_000, 1_000_000)
	runner := mustRunner(t, Config{Workers: 8, MaxEdges: 1_100_000, MaxEvidencePerEdge: 1})
	runtime.GC()
	done := make(chan struct{})
	peakResult := make(chan uint64, 1)
	go samplePeakHeap(done, peakResult)
	started := time.Now()
	result, err := runner.Run(context.Background(), Input{Nodes: nodes, Edges: edges})
	duration := time.Since(started)
	close(done)
	peak := <-peakResult
	if err != nil {
		t.Fatal(err)
	}
	if duration >= 30*time.Second {
		t.Fatalf("scale duration %s exceeds 30s", duration)
	}
	if peak >= 4<<30 {
		t.Fatalf("peak heap %d bytes exceeds 4 GiB spike safety ceiling", peak)
	}
	t.Logf("nodes=%d raw_edges=%d normalized_edges=%d duration=%s peak_heap_bytes=%d allocated_output_edges=%d", len(nodes), len(edges), len(result.Edges), duration, peak, result.Statistics.Edges)
	runtime.KeepAlive(result)
}

func samplePeakHeap(done <-chan struct{}, result chan<- uint64) {
	ticker := time.NewTicker(2 * time.Millisecond)
	defer ticker.Stop()
	var peak uint64
	for {
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		if stats.HeapAlloc > peak {
			peak = stats.HeapAlloc
		}
		select {
		case <-done:
			result <- peak
			return
		case <-ticker.C:
		}
	}
}
