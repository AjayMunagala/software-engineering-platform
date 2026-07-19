package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkDiscoveryEngineScan(b *testing.B) {
	repository := b.TempDir()
	for i := 0; i < 1000; i++ {
		path := filepath.Join(repository, "pkg", fmt.Sprintf("package-%04d", i%50), fmt.Sprintf("file-%04d.go", i))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("package fixture"), 0o600); err != nil {
			b.Fatal(err)
		}
	}

	engine := New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := engine.Scan(repository); err != nil {
			b.Fatal(err)
		}
	}
}
