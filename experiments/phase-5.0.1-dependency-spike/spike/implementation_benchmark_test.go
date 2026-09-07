package spike

import (
	"context"
	"errors"
	"runtime"
	"testing"
)

func BenchmarkBuildAndSCC100KNodes1MEdges(b *testing.B) {
	nodes, edges := layeredFixture(100_000, 1_000_000)
	runner, err := New(Config{Workers: 8, MaxEdges: 1_100_000, MaxEvidencePerEdge: 1})
	if err != nil {
		b.Fatal(err)
	}
	b.ReportMetric(float64(len(nodes)), "nodes/op")
	b.ReportMetric(float64(len(edges)), "raw_edges/op")
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		result, err := runner.Run(context.Background(), Input{Nodes: nodes, Edges: edges})
		if err != nil {
			b.Fatal(err)
		}
		runtime.KeepAlive(result)
	}
}

func BenchmarkCancelAt1024Edges(b *testing.B) {
	nodes, edges := layeredFixture(1_000, 100_000)
	b.ReportAllocs()
	for index := 0; index < b.N; index++ {
		ctx, cancel := context.WithCancel(context.Background())
		candidate, _ := New(Config{Workers: 1, MaxEdges: 110_000})
		concrete := candidate.(*runner)
		concrete.checkpoint = func(processed int) {
			if processed >= checkpointInterval {
				cancel()
			}
		}
		_, err := candidate.Run(ctx, Input{Nodes: nodes, Edges: edges})
		if !errors.Is(err, context.Canceled) {
			b.Fatalf("cancellation error=%v", err)
		}
	}
}

func BenchmarkImpact100KNodes1MEdges(b *testing.B) {
	result := impactBenchmarkResult(100_000, 10)
	runner, err := New(Config{Workers: 8, MaxEdges: 1_100_000, MaxEvidencePerEdge: 1, MaxTraversalNodes: 100_000, MaxTraversalDepth: 100_000})
	if err != nil {
		b.Fatal(err)
	}
	query := ImpactQuery{NodeID: result.Nodes[0].ID, Graph: GraphPackage, Direction: Dependencies, MaxDepth: 100_000, MaxNodes: 100_000}
	b.ReportMetric(float64(len(result.Nodes)), "reachable_nodes/op")
	b.ReportMetric(float64(len(result.Edges)), "traversable_edges/op")
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		impact, err := runner.Impact(context.Background(), result, query)
		if err != nil {
			b.Fatal(err)
		}
		runtime.KeepAlive(impact)
	}
}

func impactBenchmarkResult(nodeCount, fanout int) Result {
	nodes := make([]Node, nodeCount)
	for index := range nodes {
		identity := packageNode(fmtID(index))
		nodes[index] = Node{ID: nodeID(identity), NodeIdentity: identity}
	}
	edges := make([]Edge, 0, nodeCount*fanout)
	for source := 0; source < nodeCount; source++ {
		for offset := 1; offset <= fanout; offset++ {
			target := source + offset
			if target >= nodeCount {
				break
			}
			key := edgeKey{graph: GraphPackage, kind: EdgeImports, from: nodes[source].ID, to: nodes[target].ID, resolution: ResolvedLocal}
			edges = append(edges, Edge{ID: edgeID(key), Graph: GraphPackage, Kind: EdgeImports, FromID: key.from, ToID: key.to, Resolution: ResolvedLocal, Occurrences: 1})
		}
	}
	return Result{Nodes: nodes, Edges: edges}
}

func BenchmarkDeterminism10KNodes100KEdges(b *testing.B) {
	nodes, edges := layeredFixture(10_000, 100_000)
	for _, workers := range []int{1, 8} {
		b.Run(fmtID(workers)+"-workers", func(b *testing.B) {
			runner, _ := New(Config{Workers: workers, MaxEdges: 110_000, MaxEvidencePerEdge: 1})
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				result, err := runner.Run(context.Background(), Input{Nodes: nodes, Edges: edges})
				if err != nil {
					b.Fatal(err)
				}
				runtime.KeepAlive(result)
			}
		})
	}
}
