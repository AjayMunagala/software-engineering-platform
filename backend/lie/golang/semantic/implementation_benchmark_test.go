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

func BenchmarkReceiverAndTypeBinding100Files(b *testing.B) {
	benchmarkReceiverAndTypeBinding(b, 100)
}

func BenchmarkReceiverAndTypeBinding1000Files(b *testing.B) {
	benchmarkReceiverAndTypeBinding(b, 1_000)
}

func BenchmarkReferencesAndImports100Files(b *testing.B) {
	benchmarkReferencesAndImports(b, 100)
}

func BenchmarkReferencesAndImports1000Files(b *testing.B) {
	benchmarkReferencesAndImports(b, 1_000)
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

func benchmarkReceiverAndTypeBinding(b *testing.B, fileCount int) {
	b.Helper()
	files := map[string]string{"go.mod": "module example.com/binding\n\ngo 1.22\n"}
	for index := 0; index < fileCount; index++ {
		name := fmt.Sprintf("Type%05d", index)
		files[fmt.Sprintf("pkg/type%05d.go", index)] = fmt.Sprintf("package binding\ntype %s struct { Next *%s }\nfunc (value *%s) Method(input %s) {}\n", name, name, name, name)
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
		if inventory.Statistics().ReceiverBindings != fileCount {
			b.Fatalf("receiver bindings = %d, want %d", inventory.Statistics().ReceiverBindings, fileCount)
		}
		if inventory.Statistics().TypeRelations < fileCount {
			b.Fatalf("type relations = %d, want at least %d", inventory.Statistics().TypeRelations, fileCount)
		}
	}
}

func benchmarkReferencesAndImports(b *testing.B, fileCount int) {
	b.Helper()
	files := map[string]string{
		"go.mod":           "module example.com/references\n\ngo 1.26\n",
		"shared/shared.go": "package shared\nfunc Value() int { return 1 }\n",
	}
	for index := 0; index < fileCount; index++ {
		files[fmt.Sprintf("app/file%05d.go", index)] = fmt.Sprintf("package app\nimport \"example.com/references/shared\"\nfunc Run%05d() int { return shared.Value() }\n", index)
	}
	input := prerequisites(b, files)
	engine, err := New()
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ReportMetric(float64(fileCount+1), "files/op")
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		inventory, err := engine.Resolve(context.Background(), input)
		if err != nil {
			b.Fatal(err)
		}
		if len(inventory.ImportBindings()) != fileCount {
			b.Fatalf("import bindings = %d, want %d", len(inventory.ImportBindings()), fileCount)
		}
		if inventory.Statistics().ReferencesByStatus[ResolutionResolved.String()] < fileCount {
			b.Fatalf("resolved references = %d, want at least %d", inventory.Statistics().ReferencesByStatus[ResolutionResolved.String()], fileCount)
		}
	}
}
