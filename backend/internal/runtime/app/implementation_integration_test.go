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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	runtime, err := NewDefaultStarter().Start(ctx, request)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if runtime.State() != runtimehealth.StateReady || runtime.Readiness(ctx).Readiness().Status() != runtimehealth.StatusHealthy {
		runtime.Force()
		_, _ = runtime.Shutdown(context.Background())
		t.Fatalf("runtime did not become ready: state=%s", runtime.State())
	}
	work, err := runtime.Admit(ctx)
	if err != nil {
		t.Fatal(err)
	}
	work.Done()
	result, err := runtime.Shutdown(ctx)
	if err != nil || result.Outcome() != ShutdownGraceful || !result.ResourcesClosed() ||
		runtime.State() != runtimehealth.StateStopped {
		t.Fatalf("Shutdown() = (%#v, %v), state=%s", result, err, runtime.State())
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
