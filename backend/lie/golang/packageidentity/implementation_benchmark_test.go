package packageidentity_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/AjayMunagala/software-engineering-platform/backend/lie/golang/packageidentity"
)

func BenchmarkPackageIdentity100Imports(b *testing.B) {
	benchmarkPackageIdentity(b, 100)
}

func BenchmarkPackageIdentity1000Imports(b *testing.B) {
	benchmarkPackageIdentity(b, 1_000)
}

func BenchmarkPackageIdentity10000Imports(b *testing.B) {
	benchmarkPackageIdentity(b, 10_000)
}

func benchmarkPackageIdentity(b *testing.B, packageCount int) {
	b.Helper()
	files := map[string]string{"go.mod": "module example.com/benchmark\n\ngo 1.22\n"}
	var imports strings.Builder
	imports.WriteString("package main\nimport (\n")
	for index := 0; index < packageCount; index++ {
		fmt.Fprintf(&imports, "_ \"example.net/dependency/p%05d\"\n", index)
	}
	imports.WriteString(")\nfunc main() {}\n")
	files["main.go"] = imports.String()
	_, snapshot, syntax := prerequisiteRoot(b, files)
	engine, err := packageidentity.New()
	if err != nil {
		b.Fatal(err)
	}
	input := packageidentity.Input{Snapshot: snapshot, Syntax: syntax}

	b.ReportAllocs()
	b.ReportMetric(float64(packageCount), "imports/op")
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		inventory, err := engine.Analyze(context.Background(), input)
		if err != nil {
			b.Fatal(err)
		}
		if len(inventory.Proofs()) != packageCount {
			b.Fatalf("proofs = %d, want %d", len(inventory.Proofs()), packageCount)
		}
	}
}
