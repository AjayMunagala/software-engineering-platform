package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	runtimeconfig "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/config"
)

// TestDisposablePostgreSQLRuntime is intentionally opt-in. The companion
// tests/validate.sh harness creates and migrates a disposable PostgreSQL 18
// cluster before enabling it. No persistent or personal database is used.
func TestDisposablePostgreSQLRuntime(t *testing.T) {
	if os.Getenv("AEGIS_RUNTIME_POSTGRES_INTEGRATION") != "1" {
		t.Skip("set only by the disposable PostgreSQL runtime harness")
	}
	environment := []string{
		"AEGIS_PROFILE=ci",
		"AEGIS_DATABASE_HOST=127.0.0.1",
		"AEGIS_DATABASE_PORT=" + requiredIntegrationEnvironment(t, "AEGIS_RUNTIME_POSTGRES_PORT"),
		"AEGIS_DATABASE_NAME=" + requiredIntegrationEnvironment(t, "AEGIS_RUNTIME_POSTGRES_DATABASE"),
		"AEGIS_DATABASE_USER=" + requiredIntegrationEnvironment(t, "AEGIS_RUNTIME_POSTGRES_USER"),
	}
	provider := integrationSecretProvider{}
	request := runtimeconfig.NewLoadRequest(runtimeconfig.LoadRequestParams{
		Environment:    environment,
		SecretProvider: provider,
	})
	loaded, err := runtimeconfig.NewLoader().Load(context.Background(), request)
	if err != nil {
		t.Fatalf("configuration load: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	runtime, err := NewFactory().Open(ctx, loaded)
	if err != nil {
		t.Fatalf("runtime open: %v", err)
	}
	if runtime.PoolCount() != 1 || runtime.Compatibility().SchemaVersion() != SchemaContractVersion ||
		runtime.Compatibility().MigrationRevision() != "202607260008" {
		runtime.Close()
		t.Fatalf("unexpected compatibility proof: pools=%d schema=%q migration=%q",
			runtime.PoolCount(), runtime.Compatibility().SchemaVersion(), runtime.Compatibility().MigrationRevision())
	}
	runtime.Close()
	runtime.Close()
}

func TestDisposablePostgreSQLSeparateTLSRuntime(t *testing.T) {
	if os.Getenv("AEGIS_RUNTIME_POSTGRES_INTEGRATION") != "1" {
		t.Skip("set only by the disposable PostgreSQL runtime harness")
	}
	environment := []string{
		"AEGIS_PROFILE=production",
		"AEGIS_DATABASE_HOST=127.0.0.1",
		"AEGIS_DATABASE_PORT=" + requiredIntegrationEnvironment(t, "AEGIS_RUNTIME_POSTGRES_PORT"),
		"AEGIS_DATABASE_NAME=" + requiredIntegrationEnvironment(t, "AEGIS_RUNTIME_POSTGRES_DATABASE"),
		"AEGIS_DATABASE_CONNECTION_BUDGET=9",
		"AEGIS_DATABASE_INGEST_USER=" + requiredIntegrationEnvironment(t, "AEGIS_RUNTIME_POSTGRES_INGEST_USER"),
		"AEGIS_DATABASE_INGEST_MAX_CONNS=3",
		"AEGIS_DATABASE_INGEST_MIN_IDLE_CONNS=0",
		"AEGIS_DATABASE_READ_USER=" + requiredIntegrationEnvironment(t, "AEGIS_RUNTIME_POSTGRES_READ_USER"),
		"AEGIS_DATABASE_READ_MAX_CONNS=3",
		"AEGIS_DATABASE_READ_MIN_IDLE_CONNS=0",
		"AEGIS_DATABASE_RETENTION_USER=" + requiredIntegrationEnvironment(t, "AEGIS_RUNTIME_POSTGRES_RETENTION_USER"),
		"AEGIS_DATABASE_RETENTION_MAX_CONNS=3",
		"AEGIS_DATABASE_RETENTION_MIN_IDLE_CONNS=0",
		"AEGIS_DATABASE_TLS_MODE=verify-full",
		"AEGIS_DATABASE_TLS_ROOT_CA_FILE=" + requiredIntegrationEnvironment(t, "AEGIS_RUNTIME_POSTGRES_ROOT_CA"),
		"AEGIS_DATABASE_TLS_SERVER_NAME=runtime.test",
	}
	request := runtimeconfig.NewLoadRequest(runtimeconfig.LoadRequestParams{
		Environment:    environment,
		SecretProvider: integrationSecretProvider{},
	})
	loaded, err := runtimeconfig.NewLoader().Load(context.Background(), request)
	if err != nil {
		t.Fatalf("configuration load: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	runtime, err := NewFactory().Open(ctx, loaded)
	if err != nil {
		t.Fatalf("runtime open: %v", err)
	}
	if runtime.PoolCount() != 3 || runtime.Ingest() == nil || runtime.Read() == nil || runtime.Retention() == nil {
		runtime.Close()
		t.Fatalf("separate TLS runtime is incomplete: pools=%d", runtime.PoolCount())
	}
	runtime.Close()
}

type integrationSecretProvider struct{}

func (integrationSecretProvider) Resolve(ctx context.Context, reference runtimeconfig.SecretReference) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []byte("disposable-cluster-only"), nil
}

func requiredIntegrationEnvironment(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("required integration setting %s is absent", name)
	}
	return value
}
