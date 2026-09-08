package golang

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/AjayMunagala/software-engineering-platform/backend/die"
)

func scaleSources(n int) map[string]string {
	sources := map[string]string{"go.mod": "module example.com/scale\ngo 1.26\n"}
	for i := 0; i < n; i++ {
		sources[fmt.Sprintf("f%05d.go", i)] = fmt.Sprintf("package scale\nimport \"fmt\"\nfunc F%d(){fmt.Println()}\n", i)
	}
	return sources
}

// All released-engine work and filesystem setup precede ResetTimer.
func BenchmarkAdapter(b *testing.B) {
	for _, n := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			p := fixture(b, scaleSources(n), 8)
			in := inputs(b, p)
			engine := configured(b, ConfigParams{})
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := engine.Analyze(context.Background(), in); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
func BenchmarkStages(b *testing.B) {
	p := fixture(b, scaleSources(1000), 8)
	f := extracted(b, p)
	graph, err := translate(context.Background(), p.RepositorySnapshot, f, generous(), 100000)
	if err != nil {
		b.Fatal(err)
	}
	core, _ := die.New(die.DefaultConfig())
	b.Run("extract", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := extract(context.Background(), p, generous()); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("validate_translate", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := translate(context.Background(), p.RepositorySnapshot, f, generous(), 100000); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("normalize", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := core.Normalize(context.Background(), graph); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// Opt-in sampled peak includes live prerequisite artifacts; allocation delta does not.
func TestScaleMemory(t *testing.T) {
	if os.Getenv("DIE_ADAPTER_MEMORY_TEST") != "1" {
		t.Skip("opt-in scale measurement")
	}
	p := fixture(t, scaleSources(10000), 8)
	in := inputs(t, p)
	e := configured(t, ConfigParams{})
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	peak := before.HeapAlloc
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				if m.HeapAlloc > peak {
					peak = m.HeapAlloc
				}
			}
		}
	}()
	start := time.Now()
	out, err := e.Analyze(context.Background(), in)
	elapsed := time.Since(start)
	runtime.ReadMemStats(&after)
	close(done)
	wg.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if after.HeapAlloc > peak {
		peak = after.HeapAlloc
	}
	runtime.KeepAlive(p)
	t.Logf("files=10000 imports=10000 nodes=%d edges=%d diagnostics=%d omittedEvidence=%d elapsed=%s baselineHeap=%d sampledPeakHeap=%d allocatedBytes=%d allocations=%d", len(out.Nodes()), len(out.Dependencies()), len(out.Diagnostics()), out.Statistics().OmittedEvidence, elapsed, before.HeapAlloc, peak, after.TotalAlloc-before.TotalAlloc, after.Mallocs-before.Mallocs)
}
