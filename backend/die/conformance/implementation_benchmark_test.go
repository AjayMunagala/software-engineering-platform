package conformance

import "testing"

func BenchmarkHarnessConstruction(b *testing.B) {
	for range b.N {
		_ = Config{}
	}
}
