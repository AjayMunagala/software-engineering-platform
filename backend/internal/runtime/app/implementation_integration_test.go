package app

import (
	"context"
	"os"
	"testing"
	"time"

	runtimeconfig "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/config"
	runtimehealth "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/health"
)

func TestDisposablePostgreSQLApplicationRuntime(t *testing.T) {
	if os.Getenv("AEGIS_RUNTIME_POSTGRES_INTEGRATION") != "1" {
		t.Skip("set only by the disposable PostgreSQL runtime harness")
	}
	request := runtimeconfig.NewLoadRequest(runtimeconfig.LoadRequestParams{
		Environment: []string{
			"AEGIS_PROFILE=ci",
			"AEGIS_DATABASE_HOST=127.0.0.1",
			"AEGIS_DATABASE_PORT=" + requiredEnvironment(t, "AEGIS_RUNTIME_POSTGRES_PORT"),
			"AEGIS_DATABASE_NAME=" + requiredEnvironment(t, "AEGIS_RUNTIME_POSTGRES_DATABASE"),
			"AEGIS_DATABASE_USER=" + requiredEnvironment(t, "AEGIS_RUNTIME_POSTGRES_USER"),
		},
		SecretProvider: disposableSecretProvider{},
	})
	// Repeated independent instances prove that no lifecycle, health, pool, or
	// observability state leaks across start/ready/drain/stop cycles.
	for cycle := 0; cycle < 25; cycle++ {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		runtime, err := NewDefaultStarter().Start(ctx, request)
		if err != nil {
			cancel()
			t.Fatalf("cycle %d Start() error = %v", cycle, err)
		}
		if runtime.State() != runtimehealth.StateReady || runtime.Readiness(ctx).Readiness().Status() != runtimehealth.StatusHealthy {
			runtime.Force()
			_, _ = runtime.Shutdown(context.Background())
			cancel()
			t.Fatalf("cycle %d did not become ready: state=%s", cycle, runtime.State())
		}
		work, err := runtime.Admit(ctx)
		if err != nil {
			cancel()
			t.Fatalf("cycle %d Admit() error = %v", cycle, err)
		}
		work.Done()
		result, err := runtime.Shutdown(ctx)
		cancel()
		if err != nil || result.Outcome() != ShutdownGraceful || !result.ResourcesClosed() ||
			runtime.State() != runtimehealth.StateStopped || runtime.InFlight() != 0 {
			t.Fatalf("cycle %d Shutdown() = (%#v, %v), state=%s in_flight=%d", cycle, result, err, runtime.State(), runtime.InFlight())
		}
	}
}

type disposableSecretProvider struct{}

func (disposableSecretProvider) Resolve(ctx context.Context, _ runtimeconfig.SecretReference) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []byte("disposable-cluster-only"), nil
}

func requiredEnvironment(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("required integration setting %s is absent", name)
	}
	return value
}
