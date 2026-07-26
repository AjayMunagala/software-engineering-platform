package postgres

import (
	"fmt"
	"testing"
)

func BenchmarkAdapterManifestDigest(b *testing.B) {
	parts := make([]string, 256)
	for index := range parts {
		parts[index] = fmt.Sprintf("artifact-%03d@1.0.0", index)
	}
	b.ReportAllocs()
	for iteration := 0; iteration < b.N; iteration++ {
		_ = digestParts(parts...)
	}
}

func BenchmarkAdapterChunkCount(b *testing.B) {
	b.ReportAllocs()
	for iteration := 0; iteration < b.N; iteration++ {
		_ = chunkCount(4 << 30)
	}
}
