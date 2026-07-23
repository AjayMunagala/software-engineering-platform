package semantic

import (
	"context"
	"fmt"
	"testing"
)

func BenchmarkSourceVerification100Files(b *testing.B) {
	benchmarkSourceVerification(b, 100)
}

func BenchmarkSourceVerification1000Files(b *testing.B) {
	benchmarkSourceVerification(b, 1_000)
}

func benchmarkSourceVerification(b *testing.B, fileCount int) {
	b.Helper()
	files := map[string]string{"go.mod": "module example.com/benchmark\n\ngo 1.22\n"}
	for index := 0; index < fileCount; index++ {
		files[fmt.Sprintf("pkg/file%05d.go", index)] = "package benchmark\nfunc Value() {}\n"
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
	}
}
