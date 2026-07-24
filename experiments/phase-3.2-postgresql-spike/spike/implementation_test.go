package spike

import "testing"

func TestSafeMachineName(t *testing.T) {
	for _, valid := range []string{"project", "open-telemetry", "kubernetes_1"} {
		if !safeMachineName(valid) {
			t.Fatalf("expected valid name %q", valid)
		}
	}
	for _, invalid := range []string{"", "../escape", "with space", "a/b"} {
		if safeMachineName(invalid) {
			t.Fatalf("expected invalid name %q", invalid)
		}
	}
}

func TestChunkCount(t *testing.T) {
	for _, test := range []struct {
		size  int64
		chunk int
		want  int
	}{{0, 1024, 0}, {1, 1024, 1}, {1024, 1024, 1}, {1025, 1024, 2}} {
		if got := chunkCount(test.size, test.chunk); got != test.want {
			t.Fatalf("chunkCount(%d, %d) = %d, want %d", test.size, test.chunk, got, test.want)
		}
	}
}

func TestOperationalLimit(t *testing.T) {
	if got := operationalLimit(1); got != 64<<20 {
		t.Fatalf("minimum operational limit = %d", got)
	}
	if got := operationalLimit(70 << 20); got != 256<<20 {
		t.Fatalf("rounded operational limit = %d", got)
	}
}
