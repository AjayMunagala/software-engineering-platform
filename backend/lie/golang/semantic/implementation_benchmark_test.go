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

func BenchmarkInterfaceSatisfaction100Files(b *testing.B) {
	benchmarkInterfaceSatisfaction(b, 100)
}

func BenchmarkInterfaceSatisfaction1000Files(b *testing.B) {
	benchmarkInterfaceSatisfaction(b, 1_000)
}

func BenchmarkCandidateIntegration1000Files(b *testing.B) {
	const fileCount = 1_000
	files := map[string]string{"go.mod": "module example.com/integration\n\ngo 1.26\n"}
	for index := 0; index < fileCount; index++ {
		files[fmt.Sprintf("pkg/file%05d.go", index)] = fmt.Sprintf("package integration\ntype Worker%05d struct{}\nfunc (Worker%05d) Run() {}\n", index, index)
	}
	input := prerequisites(b, files)
	candidate, err := NewIntegrator()
	if err != nil {
		b.Fatal(err)
	}
	config := DefaultConfig()
	b.ReportAllocs()
	b.ReportMetric(fileCount, "files/op")
	b.ReportMetric(float64(config.MaxWorkers), "workers/op")
	b.ResetTimer()
	for b.Loop() {
		inventory, err := candidate.Run(context.Background(), semanticArtifactStore(b, input))
		if err != nil {
			b.Fatal(err)
		}
		if len(inventory.Files()) != fileCount {
			b.Fatalf("files = %d, want %d", len(inventory.Files()), fileCount)
		}
	}
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

func benchmarkInterfaceSatisfaction(b *testing.B, fileCount int) {
	b.Helper()
	files := map[string]string{"go.mod": "module example.com/interfaces\n\ngo 1.26\n"}
	for index := 0; index < fileCount; index++ {
		files[fmt.Sprintf("pkg/interface%05d.go", index)] = fmt.Sprintf("package interfaces\ntype Runner%05d interface { Run%05d() }\ntype Worker%05d struct{}\nfunc (Worker%05d) Run%05d() {}\nvar _ Runner%05d = Worker%05d{}\n", index, index, index, index, index, index, index)
	}
	input := prerequisites(b, files)
	engine, err := New()
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ReportMetric(float64(fileCount), "checks/op")
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		inventory, err := engine.Resolve(context.Background(), input)
		if err != nil {
			b.Fatal(err)
		}
		if len(inventory.InterfaceSatisfaction()) != fileCount {
			b.Fatalf("interface checks = %d, want %d", len(inventory.InterfaceSatisfaction()), fileCount)
		}
	}
}
