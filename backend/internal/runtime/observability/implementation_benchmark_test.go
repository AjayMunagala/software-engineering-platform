package observability

import (
	"testing"
	"time"

	runtimehealth "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/health"
)

func BenchmarkMetricSnapshot(b *testing.B) {
	pools := make([]PoolSnapshot, 0, 3)
	for _, name := range []string{"ingest", "read", "retention"} {
		pool, _ := NewPoolSnapshot(PoolSnapshotParams{Name: name, Acquired: 2, Idle: 3, Total: 5, Maximum: 10, AcquireCount: 1000, AcquireDuration: time.Second, ConnectionsCreated: 10})
		pools = append(pools, pool)
	}
	snapshot, _ := NewRuntimeSnapshot(RuntimeSnapshotParams{ObservedAt: time.Now(), State: runtimehealth.StateReady, Liveness: runtimehealth.StatusHealthy, Readiness: runtimehealth.StatusHealthy, SchemaCompatible: true, Pools: pools})
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = buildMetricSnapshot(snapshot)
	}
}
