package die

import (
	"context"
	"fmt"
	"testing"
)

func BenchmarkNormalize100KNodes1MEdges(b *testing.B) {
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
	core := newTestCore(b, ConfigParams{MaxNodes: 100_000, MaxEdges: 1_000_000})
	input := GraphInput{Nodes: nodes, Dependencies: edges}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := core.Normalize(context.Background(), input); err != nil {
			b.Fatal(err)
		}
	}
}
