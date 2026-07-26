package health

import (
	"context"
	"testing"
	"time"
)

func BenchmarkReadiness(b *testing.B) {
	configuration, _ := NewConfig(ConfigParams{
		CheckTimeout: time.Second, CheckInterval: time.Second,
		FailureThreshold: 3, Clock: fixedClock{now: time.Unix(1, 0)},
	})
	monitor, _ := NewMonitor(checkerFunc(func(context.Context) error { return nil }), configuration)
	for _, state := range []RuntimeState{StateLoading, StateValidating, StateConnecting, StateCompatibilityChecking, StateReady} {
		_ = monitor.Transition(state)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		_ = monitor.Readiness(context.Background())
	}
}
