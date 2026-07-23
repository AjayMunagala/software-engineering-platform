package semantic

import (
	"context"
	"fmt"
	"testing"
)

func BenchmarkDeclarationReconciliation100Files(b *testing.B) {
	benchmarkDeclarationReconciliation(b, 100)
}

func BenchmarkDeclarationReconciliation1000Files(b *testing.B) {
	benchmarkDeclarationReconciliation(b, 1_000)
}

func benchmarkDeclarationReconciliation(b *testing.B, fileCount int) {
	b.Helper()
	files := map[string]string{"go.mod": "module example.com/benchmark\n\ngo 1.22\n"}
	for index := 0; index < fileCount; index++ {
		files[fmt.Sprintf("pkg/file%05d.go", index)] = fmt.Sprintf("package benchmark\nfunc Value%05d() {}\n", index)
	}
	input := prerequisites(b, files)
	engine, err := New()
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ReportMetric(float64(fileCount), "files/op")
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		inventory, err := engine.Resolve(context.Background(), input)
		if err != nil {
			b.Fatal(err)
		}
		if inventory.Statistics().PartialFiles != fileCount {
			b.Fatalf("partial files = %d, want %d", inventory.Statistics().PartialFiles, fileCount)
		}
		if inventory.Statistics().ResolvedDeclarations != fileCount {
			b.Fatalf("resolved declarations = %d, want %d", inventory.Statistics().ResolvedDeclarations, fileCount)
		}
	}
}
