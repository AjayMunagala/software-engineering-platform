package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	runtimehealth "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/health"
)

type testSource struct {
	snapshot RuntimeSnapshot
	calls    atomic.Int64
}

func (source *testSource) ObservabilitySnapshot(context.Context) RuntimeSnapshot {
	source.calls.Add(1)
	return source.snapshot
}

type testSink struct {
	mutex     sync.Mutex
	snapshots []MetricSnapshot
	err       error
	wait      bool
}

func (sink *testSink) Export(ctx context.Context, snapshot MetricSnapshot) error {
	if sink.wait {
		<-ctx.Done()
		return ctx.Err()
	}
	if sink.err != nil {
		return sink.err
	}
	sink.mutex.Lock()
	sink.snapshots = append(sink.snapshots, snapshot)
	sink.mutex.Unlock()
	return nil
}

func TestStructuredJSONEventUsesBoundedFields(t *testing.T) {
	buffer := &bytes.Buffer{}
	service := newTestService(t, buffer, nil)
	err := service.Event(context.Background(), EventParams{
		Level: LevelInfo, Component: ComponentRuntime, Event: EventReady, Outcome: OutcomeSuccess,
		Duration: 1500 * time.Microsecond, CorrelationID: "startup-1", ErrorKind: "", RuntimeState: runtimehealth.StateReady, Count: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(buffer.Bytes(), &value); err != nil {
		t.Fatalf("invalid JSON log: %v\n%s", err, buffer.String())
	}
	for _, key := range []string{"timestamp", "level", "service", "component", "event", "outcome", "duration_ms", "correlation_id", "runtime_state", "count"} {
		if _, found := value[key]; !found {
			t.Errorf("missing field %q", key)
		}
	}
	for _, forbidden := range []string{"password", "dsn", "sql", "payload", "path", "driver"} {
		if strings.Contains(strings.ToLower(buffer.String()), forbidden) {
			t.Fatalf("forbidden field in log: %s", buffer.String())
		}
	}
}

func TestEventValidationAndClosedService(t *testing.T) {
	service := newTestService(t, &bytes.Buffer{}, nil)
	invalid := []EventParams{
		{},
		{Level: LevelInfo, Component: "database-host", Event: EventReady, Outcome: OutcomeSuccess},
		{Level: LevelInfo, Component: ComponentRuntime, Event: EventReady, Outcome: OutcomeSuccess, CorrelationID: "bad value"},
		{Level: LevelInfo, Component: ComponentRuntime, Event: EventReady, Outcome: OutcomeSuccess, ErrorKind: strings.Repeat("x", 65)},
		{Level: LevelInfo, Component: ComponentRuntime, Event: EventReady, Outcome: OutcomeSuccess, Count: -1},
	}
	for _, event := range invalid {
		if CodeOf(service.Event(context.Background(), event)) != ErrorInvalidInput {
			t.Fatalf("event accepted: %#v", event)
		}
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	valid := EventParams{Level: LevelInfo, Component: ComponentRuntime, Event: EventReady, Outcome: OutcomeSuccess}
	if CodeOf(service.Event(canceled, valid)) != ErrorCanceled {
		t.Fatal("canceled event was accepted")
	}
	if err := service.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if CodeOf(service.Event(context.Background(), valid)) != ErrorClosed {
		t.Fatal("closed service accepted event")
	}
}

func TestMetricSnapshotIsDeterministicAndDetached(t *testing.T) {
	pool, err := NewPoolSnapshot(PoolSnapshotParams{Name: "read", Acquired: 1, Idle: 2, Total: 3, Maximum: 5, AcquireCount: 9, AcquireDuration: time.Second, EmptyAcquireCount: 2, EmptyAcquireWait: time.Millisecond, ConnectionsCreated: 4, ConnectionsDestroyedIdle: 1, ConnectionsDestroyedLifetime: 2})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := NewRuntimeSnapshot(RuntimeSnapshotParams{ObservedAt: time.Now(), State: runtimehealth.StateReady, Liveness: runtimehealth.StatusHealthy, Readiness: runtimehealth.StatusHealthy, InFlight: 4, SchemaCompatible: true, HealthDuration: time.Millisecond, Pools: []PoolSnapshot{pool}})
	if err != nil {
		t.Fatal(err)
	}
	metrics := buildMetricSnapshot(snapshot)
	if len(metrics.Metrics()) != 27 {
		t.Fatalf("metric count = %d", len(metrics.Metrics()))
	}
	copyPools := snapshot.Pools()
	copyPools[0] = PoolSnapshot{}
	if snapshot.Pools()[0].Name() != "read" {
		t.Fatal("pool view was mutable")
	}
	copyMetrics := metrics.Metrics()
	copyMetrics[0].labels[0].value = "changed"
	if metrics.Metrics()[0].Labels()[0].Value() != "liveness" {
		t.Fatal("metric labels were mutable")
	}
	if metrics.Metrics()[0].Name() != "aegis_runtime_health" || metrics.Metrics()[2].Name() != "aegis_runtime_in_flight" {
		t.Fatal("metric order changed")
	}
}

func TestCollectionExportsAndStops(t *testing.T) {
	sink := &testSink{}
	service := newTestService(t, &bytes.Buffer{}, sink)
	source := &testSource{snapshot: validSnapshot(t)}
	if err := service.Start(context.Background(), source); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return service.Statistics().Exports() == 1 })
	if CodeOf(service.Start(context.Background(), source)) != ErrorAlreadyStarted {
		t.Fatal("second start succeeded")
	}
	if err := service.StopCollection(context.Background()); err != nil {
		t.Fatal(err)
	}
	count := source.calls.Load()
	time.Sleep(20 * time.Millisecond)
	if source.calls.Load() != count {
		t.Fatal("collection continued after stop")
	}
	if err := service.StopCollection(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := service.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	statistics := service.Statistics()
	if statistics.Collections() != 1 || statistics.Exports() != 1 || statistics.Failures() != 0 || statistics.Dropped() != 0 {
		t.Fatalf("statistics = %#v", statistics)
	}
}

func TestDisabledMetricsStartsWithoutCollection(t *testing.T) {
	config, err := NewConfig(ConfigParams{ServiceName: "aegis", Level: LevelInfo, Format: FormatText, MetricsEnabled: false, Interval: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(config, &bytes.Buffer{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	source := &testSource{snapshot: validSnapshot(t)}
	if err := service.Start(context.Background(), source); err != nil {
		t.Fatal(err)
	}
	if err := service.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if source.calls.Load() != 0 {
		t.Fatal("disabled metrics collected")
	}
}

func TestExportFailureIsIsolatedAndCounted(t *testing.T) {
	sink := &testSink{err: errors.New("private exporter detail")}
	buffer := &bytes.Buffer{}
	service := newTestService(t, buffer, sink)
	if err := service.Start(context.Background(), &testSource{snapshot: validSnapshot(t)}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return service.Statistics().Failures() == 1 })
	if err := service.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buffer.String(), "private exporter detail") || !strings.Contains(buffer.String(), "export_failure") {
		t.Fatalf("unsafe or missing failure log: %s", buffer.String())
	}
}

func TestSlowSinkIsBoundedByExportTimeout(t *testing.T) {
	sink := &testSink{wait: true}
	service := newTestService(t, &bytes.Buffer{}, sink)
	started := time.Now()
	if err := service.Start(context.Background(), &testSource{snapshot: validSnapshot(t)}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return service.Statistics().Failures() == 1 })
	if time.Since(started) > time.Second {
		t.Fatal("slow exporter was not bounded")
	}
	if err := service.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestConfigAndModelValidation(t *testing.T) {
	invalidConfigs := []ConfigParams{
		{}, {ServiceName: "bad service", Level: LevelInfo, Format: FormatJSON, Interval: time.Second},
		{ServiceName: "aegis", Level: "trace", Format: FormatJSON, Interval: time.Second},
		{ServiceName: "aegis", Level: LevelInfo, Format: "xml", Interval: time.Second},
		{ServiceName: "aegis", Level: LevelInfo, Format: FormatJSON, Interval: time.Millisecond},
		{ServiceName: "aegis", Level: LevelInfo, Format: FormatJSON, Interval: time.Second, ExportTimeout: 2 * time.Second},
	}
	for _, params := range invalidConfigs {
		if _, err := NewConfig(params); CodeOf(err) != ErrorInvalidInput {
			t.Fatalf("config accepted: %#v", params)
		}
	}
	if _, err := NewService(Config{}, nil, nil); CodeOf(err) != ErrorInvalidInput {
		t.Fatal("invalid service accepted")
	}
	if _, err := NewPoolSnapshot(PoolSnapshotParams{Name: "other"}); CodeOf(err) != ErrorInvalidInput {
		t.Fatal("invalid pool accepted")
	}
	if _, err := NewPoolSnapshot(PoolSnapshotParams{Name: "read", Total: 2, Maximum: 1}); CodeOf(err) != ErrorInvalidInput {
		t.Fatal("invalid counts accepted")
	}
	if _, err := NewRuntimeSnapshot(RuntimeSnapshotParams{}); CodeOf(err) != ErrorInvalidInput {
		t.Fatal("invalid runtime accepted")
	}
	pool, _ := NewPoolSnapshot(PoolSnapshotParams{Name: "read"})
	params := RuntimeSnapshotParams{ObservedAt: time.Now(), State: runtimehealth.StateReady, Liveness: runtimehealth.StatusHealthy, Readiness: runtimehealth.StatusHealthy, Pools: []PoolSnapshot{pool, pool}}
	if _, err := NewRuntimeSnapshot(params); CodeOf(err) != ErrorInvalidInput {
		t.Fatal("duplicate pool accepted")
	}
}

func TestNilAndCanceledLifecycleInputs(t *testing.T) {
	var service *service
	if CodeOf(service.Event(context.Background(), EventParams{})) != ErrorInvalidInput || CodeOf(service.Start(context.Background(), &testSource{})) != ErrorInvalidInput || CodeOf(service.Close(context.Background())) != ErrorInvalidInput {
		t.Fatal("nil service accepted")
	}
	valid := newTestService(t, &bytes.Buffer{}, nil)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if CodeOf(valid.Start(canceled, &testSource{})) != ErrorCanceled {
		t.Fatal("canceled start accepted")
	}
	if CodeOf(valid.Start(context.Background(), nil)) != ErrorInvalidInput || CodeOf(valid.StopCollection(nil)) != ErrorInvalidInput || CodeOf(valid.Close(nil)) != ErrorInvalidInput {
		t.Fatal("invalid lifecycle input accepted")
	}
}

func TestPublicAccessorsAndStableErrors(t *testing.T) {
	config, err := NewConfig(ConfigParams{ServiceName: "aegis", Level: LevelWarn, Format: FormatText, MetricsEnabled: false, Interval: 2 * time.Second, ExportTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if config.ServiceName() != "aegis" || config.Level() != LevelWarn || config.Format() != FormatText || config.MetricsEnabled() || config.Interval() != 2*time.Second || config.ExportTimeout() != time.Second {
		t.Fatalf("config accessors changed: %#v", config)
	}
	pool, err := NewPoolSnapshot(PoolSnapshotParams{Name: "ingest", Acquired: 1, Idle: 2, Constructing: 1, Total: 4, Maximum: 8, AcquireCount: 5, AcquireDuration: time.Second, EmptyAcquireCount: 6, EmptyAcquireWait: time.Millisecond, ConnectionsCreated: 7, ConnectionsDestroyedIdle: 8, ConnectionsDestroyedLifetime: 9})
	if err != nil {
		t.Fatal(err)
	}
	if pool.Name() != "ingest" || pool.Acquired() != 1 || pool.Idle() != 2 || pool.Constructing() != 1 || pool.Total() != 4 || pool.Maximum() != 8 || pool.AcquireCount() != 5 || pool.AcquireDuration() != time.Second || pool.EmptyAcquireCount() != 6 || pool.EmptyAcquireWait() != time.Millisecond || pool.ConnectionsCreated() != 7 || pool.ConnectionsDestroyedIdle() != 8 || pool.ConnectionsDestroyedLifetime() != 9 {
		t.Fatalf("pool accessors changed: %#v", pool)
	}
	observedAt := time.Now().UTC()
	snapshot, err := NewRuntimeSnapshot(RuntimeSnapshotParams{ObservedAt: observedAt, State: runtimehealth.StateDraining, Liveness: runtimehealth.StatusHealthy, Readiness: runtimehealth.StatusUnhealthy, InFlight: 3, HealthDuration: 2 * time.Millisecond, Pools: []PoolSnapshot{pool}})
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.ObservedAt().Equal(observedAt) || snapshot.State() != runtimehealth.StateDraining || snapshot.Liveness() != runtimehealth.StatusHealthy || snapshot.Readiness() != runtimehealth.StatusUnhealthy || snapshot.InFlight() != 3 || snapshot.SchemaCompatible() || snapshot.HealthDuration() != 2*time.Millisecond {
		t.Fatalf("runtime accessors changed: %#v", snapshot)
	}
	metricSnapshot := buildMetricSnapshot(snapshot)
	metric := metricSnapshot.Metrics()[0]
	if metricSnapshot.ObservedAt().IsZero() || metric.Name() == "" || metric.Type() != MetricGauge || metric.Value() != 1 || metric.Labels()[0].Name() != "kind" {
		t.Fatalf("metric accessors changed: %#v", metric)
	}
	unknown, _ := NewRuntimeSnapshot(RuntimeSnapshotParams{ObservedAt: observedAt, State: runtimehealth.StateNew, Liveness: runtimehealth.StatusUnknown, Readiness: runtimehealth.StatusUnknown})
	if buildMetricSnapshot(unknown).Metrics()[4].Labels()[1].Value() != "unknown" {
		t.Fatal("unknown health outcome changed")
	}

	canceled := newError(ErrorInternal, "step", context.Canceled)
	if CodeOf(canceled) != ErrorCanceled || !errors.Is(canceled, context.Canceled) || !strings.Contains(canceled.Error(), "canceled") {
		t.Fatalf("canceled error = %v", canceled)
	}
	timed := newError(ErrorInternal, "step", context.DeadlineExceeded)
	if CodeOf(timed) != ErrorTimeout || !errors.Is(timed, context.DeadlineExceeded) {
		t.Fatalf("timeout error = %v", timed)
	}
	var nilFailure *Error
	if nilFailure.Code() != ErrorInternal || nilFailure.Error() == "" || nilFailure.Unwrap() != nil || CodeOf(errors.New("foreign")) != ErrorInternal {
		t.Fatal("nil/foreign error contract changed")
	}
	if (&Error{code: ErrorInternal, step: "safe"}).Unwrap() != nil {
		t.Fatal("unsafe error cause escaped")
	}
}

func TestDiscardSinkAndAllLogLevels(t *testing.T) {
	buffer := &bytes.Buffer{}
	service := newTestService(t, buffer, nil)
	for _, level := range []Level{LevelDebug, LevelInfo, LevelWarn, LevelError} {
		if err := service.Event(context.Background(), EventParams{Level: level, Component: ComponentHealth, Event: EventHealthLost, Outcome: OutcomeError, ErrorKind: "database_unavailable"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := service.Start(context.Background(), &testSource{snapshot: validSnapshot(t)}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return service.Statistics().Exports() == 1 })
	if err := service.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if lines := strings.Count(strings.TrimSpace(buffer.String()), "\n") + 1; lines < 4 {
		t.Fatalf("expected all log levels, got %d: %s", lines, buffer.String())
	}
}

func newTestService(t *testing.T, output *bytes.Buffer, sink Sink) Service {
	t.Helper()
	config, err := NewConfig(ConfigParams{ServiceName: "aegis-runtime", Level: LevelDebug, Format: FormatJSON, MetricsEnabled: true, Interval: time.Second, ExportTimeout: 25 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(config, output, sink)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func validSnapshot(t *testing.T) RuntimeSnapshot {
	t.Helper()
	snapshot, err := NewRuntimeSnapshot(RuntimeSnapshotParams{ObservedAt: time.Now(), State: runtimehealth.StateReady, Liveness: runtimehealth.StatusHealthy, Readiness: runtimehealth.StatusHealthy, SchemaCompatible: true})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition was not reached")
}
