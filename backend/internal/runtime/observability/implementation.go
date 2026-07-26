package observability

import (
	"context"
	"io"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	runtimehealth "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/health"
)

type service struct {
	config Config
	logger *slog.Logger
	sink   Sink

	mutex   sync.Mutex
	started bool
	closed  bool
	cancel  context.CancelFunc
	done    chan struct{}

	collections atomic.Uint64
	exports     atomic.Uint64
	failures    atomic.Uint64
	dropped     atomic.Uint64
}

type discardSink struct{}

func (discardSink) Export(context.Context, MetricSnapshot) error { return nil }

func newService(config Config, output io.Writer, sink Sink) (Service, error) {
	if output == nil || !safeIdentifier(config.serviceName, maximumServiceNameBytes) || !validLevel(config.level) || !validFormat(config.format) {
		return nil, newError(ErrorInvalidInput, "service", nil)
	}
	if sink == nil {
		sink = discardSink{}
	}
	options := &slog.HandlerOptions{Level: slogLevel(config.level), ReplaceAttr: replaceLogAttribute}
	var handler slog.Handler
	if config.format == FormatJSON {
		handler = slog.NewJSONHandler(output, options)
	} else {
		handler = slog.NewTextHandler(output, options)
	}
	return &service{config: config, logger: slog.New(handler), sink: sink}, nil
}

func (value *service) Event(ctx context.Context, event EventParams) error {
	if value == nil || ctx == nil || !validEvent(event) {
		return newError(ErrorInvalidInput, "event", nil)
	}
	if err := ctx.Err(); err != nil {
		return newError(ErrorCanceled, "event", err)
	}
	value.mutex.Lock()
	closed := value.closed
	value.mutex.Unlock()
	if closed {
		return newError(ErrorClosed, "event", nil)
	}
	attributes := []any{
		"service", value.config.serviceName,
		"component", string(event.Component),
		"event", string(event.Event),
		"outcome", string(event.Outcome),
	}
	if event.Duration > 0 {
		attributes = append(attributes, "duration_ms", float64(event.Duration)/float64(time.Millisecond))
	}
	if event.CorrelationID != "" {
		attributes = append(attributes, "correlation_id", event.CorrelationID)
	}
	if event.ErrorKind != "" {
		attributes = append(attributes, "error_kind", event.ErrorKind)
	}
	if event.RuntimeState != "" {
		attributes = append(attributes, "runtime_state", string(event.RuntimeState))
	}
	if event.Count != 0 {
		attributes = append(attributes, "count", event.Count)
	}
	value.logger.Log(ctx, slogLevel(event.Level), string(event.Event), attributes...)
	return nil
}

func (value *service) Start(parent context.Context, source Source) error {
	if value == nil || parent == nil || source == nil {
		return newError(ErrorInvalidInput, "start", nil)
	}
	if err := parent.Err(); err != nil {
		return newError(ErrorCanceled, "start", err)
	}
	value.mutex.Lock()
	defer value.mutex.Unlock()
	if value.closed {
		return newError(ErrorClosed, "start", nil)
	}
	if value.started {
		return newError(ErrorAlreadyStarted, "start", nil)
	}
	value.started = true
	if !value.config.metricsEnabled {
		return nil
	}
	collectorContext, cancel := context.WithCancel(context.WithoutCancel(parent))
	value.cancel = cancel
	value.done = make(chan struct{})
	go value.collect(collectorContext, source)
	return nil
}

func (value *service) collect(ctx context.Context, source Source) {
	defer close(value.done)
	value.export(ctx, source)
	ticker := time.NewTicker(value.config.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			value.export(ctx, source)
		}
	}
}

func (value *service) export(parent context.Context, source Source) {
	startedAt := time.Now()
	snapshot := buildMetricSnapshot(source.ObservabilitySnapshot(parent))
	value.collections.Add(1)
	exportContext, cancel := context.WithTimeout(parent, value.config.exportTimeout)
	err := value.sink.Export(exportContext, snapshot)
	cancel()
	if err != nil {
		value.failures.Add(1)
		value.dropped.Add(1)
		outcome := OutcomeError
		if exportContext.Err() != nil {
			outcome = OutcomeTimeout
		}
		_ = value.Event(context.Background(), EventParams{Level: LevelWarn, Component: ComponentObservability, Event: EventExport, Outcome: outcome, Duration: time.Since(startedAt), ErrorKind: "export_failure"})
		return
	}
	value.exports.Add(1)
}

func (value *service) StopCollection(ctx context.Context) error {
	if value == nil || ctx == nil {
		return newError(ErrorInvalidInput, "stop", nil)
	}
	value.mutex.Lock()
	if !value.started || value.cancel == nil {
		value.mutex.Unlock()
		return nil
	}
	cancel, done := value.cancel, value.done
	value.cancel = nil
	value.done = nil
	value.mutex.Unlock()
	cancel()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return newError(ErrorCanceled, "stop", ctx.Err())
	}
}

func (value *service) Close(ctx context.Context) error {
	if value == nil || ctx == nil {
		return newError(ErrorInvalidInput, "close", nil)
	}
	if err := value.StopCollection(ctx); err != nil {
		return err
	}
	value.mutex.Lock()
	value.closed = true
	value.mutex.Unlock()
	return nil
}

func (value *service) Statistics() Statistics {
	if value == nil {
		return Statistics{}
	}
	return Statistics{value.collections.Load(), value.exports.Load(), value.failures.Load(), value.dropped.Load()}
}

func NewPoolSnapshot(params PoolSnapshotParams) (PoolSnapshot, error) {
	if !validPoolName(params.Name) || params.Acquired < 0 || params.Idle < 0 || params.Constructing < 0 || params.Total < 0 || params.Maximum < 0 ||
		params.AcquireCount < 0 || params.AcquireDuration < 0 || params.EmptyAcquireCount < 0 || params.EmptyAcquireWait < 0 ||
		params.ConnectionsCreated < 0 || params.ConnectionsDestroyedIdle < 0 || params.ConnectionsDestroyedLifetime < 0 ||
		params.Total > params.Maximum || params.Acquired+params.Idle+params.Constructing > params.Total {
		return PoolSnapshot{}, newError(ErrorInvalidInput, "pool-snapshot", nil)
	}
	return PoolSnapshot{params.Name, params.Acquired, params.Idle, params.Constructing, params.Total, params.Maximum,
		params.AcquireCount, params.AcquireDuration, params.EmptyAcquireCount, params.EmptyAcquireWait,
		params.ConnectionsCreated, params.ConnectionsDestroyedIdle, params.ConnectionsDestroyedLifetime}, nil
}

func NewRuntimeSnapshot(params RuntimeSnapshotParams) (RuntimeSnapshot, error) {
	if params.ObservedAt.IsZero() || !validRuntimeState(params.State) || !validHealthStatus(params.Liveness) || !validHealthStatus(params.Readiness) || params.InFlight < 0 || params.HealthDuration < 0 {
		return RuntimeSnapshot{}, newError(ErrorInvalidInput, "runtime-snapshot", nil)
	}
	pools := slices.Clone(params.Pools)
	slices.SortFunc(pools, func(left, right PoolSnapshot) int { return strings.Compare(left.name, right.name) })
	for index, pool := range pools {
		if !validPoolName(pool.name) || (index > 0 && pools[index-1].name == pool.name) {
			return RuntimeSnapshot{}, newError(ErrorInvalidInput, "runtime-pools", nil)
		}
	}
	return RuntimeSnapshot{params.ObservedAt.UTC(), params.State, params.Liveness, params.Readiness, params.InFlight, params.SchemaCompatible, params.HealthDuration, pools}, nil
}

func buildMetricSnapshot(snapshot RuntimeSnapshot) MetricSnapshot {
	metrics := make([]Metric, 0, 16+len(snapshot.pools)*12)
	metrics = append(metrics,
		metric("aegis_runtime_health", MetricGauge, healthyValue(snapshot.liveness), Label{"kind", "liveness"}),
		metric("aegis_runtime_health", MetricGauge, healthyValue(snapshot.readiness), Label{"kind", "readiness"}),
		metric("aegis_runtime_in_flight", MetricGauge, float64(snapshot.inFlight)),
		metric("aegis_schema_compatibility", MetricGauge, booleanValue(snapshot.schemaCompatible)),
		metric("aegis_health_check_seconds", MetricHistogram, snapshot.healthDuration.Seconds(), Label{"kind", "readiness"}, Label{"outcome", healthOutcome(snapshot.readiness)}),
	)
	for _, state := range allRuntimeStates() {
		metrics = append(metrics, metric("aegis_runtime_state", MetricGauge, booleanValue(snapshot.state == state), Label{"state", string(state)}))
	}
	for _, pool := range snapshot.pools {
		for _, connection := range []struct {
			name  string
			value int32
		}{
			{"acquired", pool.acquired}, {"idle", pool.idle}, {"constructing", pool.constructing}, {"total", pool.total}, {"max", pool.maximum},
		} {
			metrics = append(metrics, metric("aegis_db_pool_connections", MetricGauge, float64(connection.value), Label{"pool", pool.name}, Label{"state", connection.name}))
		}
		metrics = append(metrics,
			metric("aegis_db_pool_acquire_total", MetricCounter, float64(pool.acquireCount), Label{"pool", pool.name}, Label{"outcome", "success"}),
			metric("aegis_db_pool_acquire_seconds_total", MetricCounter, pool.acquireDuration.Seconds(), Label{"pool", pool.name}),
			metric("aegis_db_pool_empty_wait_total", MetricCounter, float64(pool.emptyAcquireCount), Label{"pool", pool.name}),
			metric("aegis_db_pool_empty_wait_seconds_total", MetricCounter, pool.emptyAcquireWait.Seconds(), Label{"pool", pool.name}),
			metric("aegis_db_pool_connections_created_total", MetricCounter, float64(pool.connectionsCreated), Label{"pool", pool.name}),
			metric("aegis_db_pool_connections_destroyed_total", MetricCounter, float64(pool.connectionsDestroyedIdle), Label{"pool", pool.name}, Label{"reason", "idle"}),
			metric("aegis_db_pool_connections_destroyed_total", MetricCounter, float64(pool.connectionsDestroyedLifetime), Label{"pool", pool.name}, Label{"reason", "lifetime"}),
		)
	}
	return MetricSnapshot{snapshot.observedAt, metrics}
}

func metric(name string, kind MetricType, value float64, labels ...Label) Metric {
	return Metric{name: name, kind: kind, value: value, labels: slices.Clone(labels)}
}

func validEvent(value EventParams) bool {
	return validLevel(value.Level) && validComponent(value.Component) && validEventName(value.Event) && validOutcome(value.Outcome) && value.Duration >= 0 && value.Count >= 0 &&
		(value.CorrelationID == "" || safeIdentifier(value.CorrelationID, 64)) && (value.ErrorKind == "" || safeIdentifier(value.ErrorKind, 64)) &&
		(value.RuntimeState == "" || validRuntimeState(value.RuntimeState))
}

func safeIdentifier(value string, maximum int) bool {
	if value == "" || len(value) > maximum {
		return false
	}
	for _, character := range value {
		if !unicode.IsLetter(character) && !unicode.IsDigit(character) && character != '-' && character != '_' && character != '.' {
			return false
		}
	}
	return true
}

func validComponent(value Component) bool {
	return value == ComponentRuntime || value == ComponentConfiguration || value == ComponentPostgreSQL || value == ComponentHealth || value == ComponentObservability
}
func validEventName(value EventName) bool {
	return value == EventStartup || value == EventReady || value == EventDraining || value == EventStopping || value == EventStopped || value == EventHealthLost || value == EventHealthReady || value == EventExport
}
func validOutcome(value Outcome) bool {
	return value == OutcomeSuccess || value == OutcomeError || value == OutcomeTimeout || value == OutcomeCanceled || value == OutcomeForced
}
func validPoolName(value string) bool {
	return value == "combined" || value == "ingest" || value == "read" || value == "retention"
}
func validHealthStatus(value runtimehealth.Status) bool {
	return value == runtimehealth.StatusHealthy || value == runtimehealth.StatusUnhealthy || value == runtimehealth.StatusUnknown
}
func validRuntimeState(value runtimehealth.RuntimeState) bool {
	return slices.Contains(allRuntimeStates(), value)
}

func allRuntimeStates() []runtimehealth.RuntimeState {
	return []runtimehealth.RuntimeState{runtimehealth.StateNew, runtimehealth.StateLoading, runtimehealth.StateValidating, runtimehealth.StateConnecting, runtimehealth.StateCompatibilityChecking, runtimehealth.StateReady, runtimehealth.StateDraining, runtimehealth.StateStopping, runtimehealth.StateStopped, runtimehealth.StateFailed}
}

func slogLevel(value Level) slog.Level {
	switch value {
	case LevelDebug:
		return slog.LevelDebug
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func replaceLogAttribute(_ []string, attribute slog.Attr) slog.Attr {
	if attribute.Key == slog.TimeKey {
		attribute.Key = "timestamp"
	}
	return attribute
}

func healthyValue(value runtimehealth.Status) float64 {
	return booleanValue(value == runtimehealth.StatusHealthy)
}
func booleanValue(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
func healthOutcome(value runtimehealth.Status) string {
	if value == runtimehealth.StatusHealthy {
		return "success"
	}
	if value == runtimehealth.StatusUnhealthy {
		return "error"
	}
	return "unknown"
}
