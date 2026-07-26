package app

import (
	"context"
	"testing"
)

func BenchmarkZeroWorkShutdown(b *testing.B) {
	b.StopTimer()
	b.ReportAllocs()
	for iteration := 0; iteration < b.N; iteration++ {
		runtime := readyTestRuntime(b)
		b.StartTimer()
		if _, err := runtime.Shutdown(context.Background()); err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
	}
}
