package spike

import (
	"crypto/sha256"
	"testing"
)

func BenchmarkOneMiBChunkDigest(b *testing.B) {
	payload := make([]byte, 1<<20)
	b.SetBytes(int64(len(payload)))
	b.ReportAllocs()
	for range b.N {
		_ = sha256.Sum256(payload)
	}
}
