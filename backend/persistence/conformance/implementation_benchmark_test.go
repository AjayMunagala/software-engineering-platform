package conformance

import "testing"

var benchmarkOperations []Operation

func BenchmarkScopeIsolationOperationCatalogue(b *testing.B) {
	b.ReportAllocs()
	for iteration := 0; iteration < b.N; iteration++ {
		operations := ScopeIsolationOperations()
		if len(operations) != 18 {
			b.Fatal("operation catalogue changed")
		}
		benchmarkOperations = operations
	}
}
