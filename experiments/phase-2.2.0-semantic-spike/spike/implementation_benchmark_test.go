package spike

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func BenchmarkSemanticFullRebuild100Files(benchmark *testing.B) {
	benchmarkSemanticFullRebuild(benchmark, 100)
}

func BenchmarkSemanticFullRebuild1000Files(benchmark *testing.B) {
	benchmarkSemanticFullRebuild(benchmark, 1000)
}

func BenchmarkSemanticFullRebuild10000Files(benchmark *testing.B) {
	benchmarkSemanticFullRebuild(benchmark, 10_000)
}

func benchmarkSemanticFullRebuild(benchmark *testing.B, fileCount int) {
	engine, err := New()
	if err != nil {
		benchmark.Fatal(err)
	}
	files := make([]SourceFile, fileCount)
	for index := range files {
		files[index] = SourceFile{
			Path:    fmt.Sprintf("pkg/file_%06d.go", index),
			Content: fmt.Sprintf("package fixture\ntype Type%06d struct { Value int }\nfunc (value Type%06d) Method() int { return value.Value }\n", index, index),
		}
	}
	input := Input{Files: files}
	benchmark.ReportAllocs()
	benchmark.ResetTimer()
	for range benchmark.N {
		result, runErr := engine.Run(context.Background(), input)
		if runErr != nil {
			benchmark.Fatal(runErr)
		}
		if result.ParseCount != fileCount {
			benchmark.Fatalf("parsed %d files, want %d", result.ParseCount, fileCount)
		}
	}
}

func BenchmarkRelationshipCancellationCheckpoint(benchmark *testing.B) {
	benchmark.ReportAllocs()
	for range benchmark.N {
		processed, err := processRelationships(context.Background(), 100_000, 256, nil)
		if err != nil || processed != 100_000 {
			benchmark.Fatalf("processed=%d err=%v", processed, err)
		}
	}
}

func BenchmarkASTCancellationAt1024Nodes(benchmark *testing.B) {
	var source strings.Builder
	source.WriteString("package fixture\nfunc F() {\n")
	for index := 0; index < 3000; index++ {
		fmt.Fprintf(&source, "var value%d = %d\n", index, index)
	}
	source.WriteString("}\n")
	file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", source.String(), parser.AllErrors)
	if err != nil {
		benchmark.Fatal(err)
	}
	benchmark.ReportAllocs()
	benchmark.ResetTimer()
	for range benchmark.N {
		ctx, cancel := context.WithCancel(context.Background())
		visited, walkErr := inspectWithContext(ctx, file, 1024, func(count int) {
			if count == 1024 {
				cancel()
			}
		})
		if walkErr == nil || visited != 1024 {
			benchmark.Fatalf("visited=%d err=%v", visited, walkErr)
		}
	}
}

func BenchmarkRelationshipCancellationAt256(benchmark *testing.B) {
	benchmark.ReportAllocs()
	for range benchmark.N {
		ctx, cancel := context.WithCancel(context.Background())
		processed, err := processRelationships(ctx, 100_000, 256, func(count int) {
			if count == 256 {
				cancel()
			}
		})
		if err == nil || processed != 256 {
			benchmark.Fatalf("processed=%d err=%v", processed, err)
		}
	}
}

func BenchmarkGoTypesPackage1000Files(benchmark *testing.B) {
	fileSet := token.NewFileSet()
	files := make([]*ast.File, 1000)
	for index := range files {
		source := fmt.Sprintf("package fixture\ntype Type%06d struct { Value int }\nfunc (value Type%06d) Method() int { return value.Value }\n", index, index)
		file, err := parser.ParseFile(fileSet, fmt.Sprintf("file_%06d.go", index), source, parser.AllErrors)
		if err != nil {
			benchmark.Fatal(err)
		}
		files[index] = file
	}
	benchmark.ReportAllocs()
	benchmark.ResetTimer()
	for range benchmark.N {
		if _, err := checkPackage(context.Background(), fileSet, "example.com/fixture", files, rejectingImporter{}); err != nil {
			benchmark.Fatal(err)
		}
	}
}
