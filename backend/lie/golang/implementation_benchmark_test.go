package golang_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	golang "github.com/AjayMunagala/software-engineering-platform/backend/lie/golang"
)

func benchmarkGoEngine(b *testing.B, fileCount int) {
	files := make(map[string]string, fileCount)
	for index := 0; index < fileCount; index++ {
		files[fmt.Sprintf("pkg%d/file%d.go", index%100, index)] = fmt.Sprintf("package pkg%d\nimport \"fmt\"\ntype Item%d struct{}\nfunc Value%d(){fmt.Println(%d)}\n", index%100, index, index, index)
	}
	snapshot, languages := artifacts(b, files)
	// Warm the operating-system file cache so the benchmark measures engine
	// analysis rather than antivirus work triggered by creating the fixture.
	for relativePath := range files {
		if _, err := os.ReadFile(filepath.Join(snapshot.RootPath(), filepath.FromSlash(relativePath))); err != nil {
			b.Fatal(err)
		}
	}
	config := golang.DefaultConfig()
	config.MaxWorkers = 8
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = analyzeInput(b, snapshot, languages, &config)
	}
}

func BenchmarkGoEngine10Files(b *testing.B)    { benchmarkGoEngine(b, 10) }
func BenchmarkGoEngine1000Files(b *testing.B)  { benchmarkGoEngine(b, 1_000) }
func BenchmarkGoEngine10000Files(b *testing.B) { benchmarkGoEngine(b, 10_000) }
