package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func BenchmarkLoadLocalEnvironment(b *testing.B) {
	loader := NewLoader()
	request := NewLoadRequest(LoadRequestParams{Environment: localEnvironment()})
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := loader.Load(context.Background(), request); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLoadProductionEnvironment(b *testing.B) {
	loader := NewLoader()
	provider := staticSecretProvider{
		SecretDatabaseIngest:    []byte("ingest"),
		SecretDatabaseRead:      []byte("read"),
		SecretDatabaseRetention: []byte("retention"),
	}
	request := NewLoadRequest(LoadRequestParams{
		Environment:    productionEnvironment(),
		SecretProvider: provider,
	})
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := loader.Load(context.Background(), request); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStrictJSONDecode(b *testing.B) {
	data := []byte(`{
  "profile":"production",
  "startup":{"startup_timeout":"30s","drain_timeout":"30s"},
  "database":{
    "host":"db.example.com","port":5432,"name":"platform","connection_budget":12,
    "pools":{
      "ingest":{"user":"platform_ingest","max_conns":5,"min_idle_conns":1},
      "read":{"user":"platform_read","max_conns":5,"min_idle_conns":1},
      "retention":{"user":"platform_retention","max_conns":2,"min_idle_conns":0}
    },
    "tls":{"mode":"verify-full"}
  }
}`)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for range b.N {
		if _, err := decodeStrictConfiguration(data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLoad64KiBConfiguration(b *testing.B) {
	directory := b.TempDir()
	path := filepath.Join(directory, "runtime.json")
	prefix := `{"database":{"host":"127.0.0.1","name":"platform","user":"platform_local"}}`
	data := prefix + strings.Repeat(" ", (64<<10)-len(prefix))
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		b.Fatal(err)
	}
	loader := NewLoader()
	request := NewLoadRequest(LoadRequestParams{Environment: []string{
		"AEGIS_CONFIG_FILE=" + path,
		"AEGIS_DATABASE_PASSWORD=benchmark-placeholder",
	}})
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for range b.N {
		if _, err := loader.Load(context.Background(), request); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSafeView(b *testing.B) {
	loaded, err := NewLoader().Load(context.Background(), NewLoadRequest(LoadRequestParams{Environment: localEnvironment()}))
	if err != nil {
		b.Fatal(err)
	}
	configuration := loaded.Config()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = configuration.SafeView()
	}
}
