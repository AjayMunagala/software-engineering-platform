package config

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestLoadLocalDefaultsAndImmutability(t *testing.T) {
	environment := localEnvironment()
	request := NewLoadRequest(LoadRequestParams{Environment: environment})
	environment[0] = "AEGIS_DATABASE_HOST=malicious.example"

	loaded, err := NewLoader().Load(context.Background(), request)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	configuration := loaded.Config()
	if configuration.Profile() != ProfileLocal {
		t.Fatalf("Profile() = %q", configuration.Profile())
	}
	if got := configuration.Database().Host(); got != "127.0.0.1" {
		t.Fatalf("Host() = %q", got)
	}
	if configuration.Database().Pools().Mode() != PoolCombined {
		t.Fatalf("Mode() = %q", configuration.Database().Pools().Mode())
	}
	if configuration.Database().Pools().Ingest().User() != "platform_local" {
		t.Fatalf("combined user was not propagated")
	}
	if configuration.Database().TLS().Mode() != TLSDisabled {
		t.Fatalf("TLS mode = %q", configuration.Database().TLS().Mode())
	}

	sources := configuration.Sources()
	sources[fieldDBHost] = SourceDefault
	if configuration.Source(fieldDBHost) != SourceEnvironment {
		t.Fatal("Sources returned mutable internal state")
	}
	required := configuration.RequiredSecrets()
	required[0] = SecretDatabaseRead
	if configuration.RequiredSecrets()[0] != SecretDatabaseCombined {
		t.Fatal("RequiredSecrets returned mutable internal state")
	}
}

func TestSourcePrecedence(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "runtime.json")
	data := `{
  "profile": "local",
  "logging": {"level": "debug", "service_name": "file-service"},
  "database": {"host": "127.0.0.2", "name": "from_file", "user": "file_user"}
}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	environment := []string{
		"AEGIS_CONFIG_FILE=" + path,
		"AEGIS_LOG_LEVEL=warn",
		"AEGIS_DATABASE_HOST=127.0.0.3",
		"AEGIS_DATABASE_NAME=from_environment",
		"AEGIS_DATABASE_USER=environment_user",
		"AEGIS_DATABASE_PASSWORD=secret",
	}
	arguments := []string{"--log-level=error", "--database-host", "127.0.0.4"}
	loaded, err := NewLoader().Load(context.Background(), NewLoadRequest(LoadRequestParams{
		Environment: environment,
		Arguments:   arguments,
	}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	configuration := loaded.Config()
	if configuration.Logging().Level() != LogError {
		t.Fatalf("level = %q", configuration.Logging().Level())
	}
	if configuration.Database().Host() != "127.0.0.4" {
		t.Fatalf("host = %q", configuration.Database().Host())
	}
	if configuration.Database().Name() != "from_environment" {
		t.Fatalf("name = %q", configuration.Database().Name())
	}
	if err := os.WriteFile(path, []byte(`{"logging":{"service_name":"changed"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if configuration.Logging().ServiceName() != "file-service" {
		t.Fatal("configuration changed after source-file mutation")
	}
	if configuration.Source(fieldLogLevel) != SourceCommandLine ||
		configuration.Source(fieldDBName) != SourceEnvironment ||
		configuration.Source(fieldForcedTimeout) != SourceDefault {
		t.Fatalf("unexpected sources: %#v", configuration.Sources())
	}
}

func TestCIDefaults(t *testing.T) {
	environment := append(localEnvironment(), "AEGIS_PROFILE=ci")
	loaded, err := NewLoader().Load(context.Background(), NewLoadRequest(LoadRequestParams{Environment: environment}))
	if err != nil {
		t.Fatal(err)
	}
	configuration := loaded.Config()
	if configuration.Profile() != ProfileCI || configuration.Logging().Format() != LogJSON ||
		configuration.Health().CheckInterval().String() != "5s" || configuration.Database().ConnectionBudget() != 4 {
		t.Fatalf("unexpected CI defaults: %s", configuration.String())
	}
}

func TestStrictConfigurationFile(t *testing.T) {
	tests := []struct {
		name string
		data string
		code ErrorCode
	}{
		{name: "unknown", data: `{"database":{"unknown":"x"}}`, code: ErrorUnknownField},
		{name: "duplicate", data: `{"profile":"local","profile":"ci"}`, code: ErrorDuplicateField},
		{name: "wrong type", data: `{"database":{"port":"5432"}}`, code: ErrorInvalidValue},
		{name: "array", data: `[]`, code: ErrorInvalidValue},
		{name: "trailing", data: `{} {}`, code: ErrorInvalidValue},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "runtime.json")
			if err := os.WriteFile(path, []byte(test.data), 0o600); err != nil {
				t.Fatal(err)
			}
			environment := append(localEnvironment(), "AEGIS_CONFIG_FILE="+path)
			_, err := NewLoader().Load(context.Background(), NewLoadRequest(LoadRequestParams{Environment: environment}))
			if CodeOf(err) != test.code {
				t.Fatalf("CodeOf(error) = %q, want %q; error = %v", CodeOf(err), test.code, err)
			}
		})
	}
}

func TestEnvironmentAndArgumentRejections(t *testing.T) {
	tests := []struct {
		name        string
		environment []string
		arguments   []string
		code        ErrorCode
	}{
		{name: "unknown environment", environment: append(localEnvironment(), "AEGIS_UNKNOWN=value"), code: ErrorUnknownField},
		{name: "duplicate environment", environment: append(localEnvironment(), "AEGIS_DATABASE_HOST=127.0.0.2"), code: ErrorDuplicateField},
		{name: "unknown argument", environment: localEnvironment(), arguments: []string{"--database-password=secret"}, code: ErrorUnknownField},
		{name: "duplicate argument", environment: localEnvironment(), arguments: []string{"--log-level=info", "--log-level=warn"}, code: ErrorDuplicateField},
		{name: "positional argument", environment: localEnvironment(), arguments: []string{"runtime.json"}, code: ErrorInvalidInput},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewLoader().Load(context.Background(), NewLoadRequest(LoadRequestParams{
				Environment: test.environment,
				Arguments:   test.arguments,
			}))
			if CodeOf(err) != test.code {
				t.Fatalf("CodeOf(error) = %q, want %q; error = %v", CodeOf(err), test.code, err)
			}
		})
	}
}

func TestProductionSeparatePools(t *testing.T) {
	provider := staticSecretProvider{
		SecretDatabaseIngest:    []byte("ingest-secret"),
		SecretDatabaseRead:      []byte("read-secret"),
		SecretDatabaseRetention: []byte("retention-secret"),
	}
	environment := []string{
		"AEGIS_PROFILE=production",
		"AEGIS_DATABASE_HOST=db.example.com",
		"AEGIS_DATABASE_NAME=platform",
		"AEGIS_DATABASE_CONNECTION_BUDGET=12",
		"AEGIS_DATABASE_INGEST_USER=platform_ingest",
		"AEGIS_DATABASE_INGEST_MAX_CONNS=5",
		"AEGIS_DATABASE_INGEST_MIN_IDLE_CONNS=1",
		"AEGIS_DATABASE_READ_USER=platform_read",
		"AEGIS_DATABASE_READ_MAX_CONNS=5",
		"AEGIS_DATABASE_READ_MIN_IDLE_CONNS=1",
		"AEGIS_DATABASE_RETENTION_USER=platform_retention",
		"AEGIS_DATABASE_RETENTION_MAX_CONNS=2",
		"AEGIS_DATABASE_RETENTION_MIN_IDLE_CONNS=0",
	}
	loaded, err := NewLoader().Load(context.Background(), NewLoadRequest(LoadRequestParams{
		Environment:    environment,
		SecretProvider: provider,
	}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	configuration := loaded.Config()
	if configuration.Database().Pools().Mode() != PoolSeparate {
		t.Fatalf("Mode() = %q", configuration.Database().Pools().Mode())
	}
	if configuration.Database().TLS().Mode() != TLSVerifyFull {
		t.Fatalf("TLS mode = %q", configuration.Database().TLS().Mode())
	}
	if got := configuration.Database().Pools().Retention().User(); got != "platform_retention" {
		t.Fatalf("retention user = %q", got)
	}
	if !reflect.DeepEqual(configuration.RequiredSecrets(), []SecretReference{
		SecretDatabaseIngest, SecretDatabaseRead, SecretDatabaseRetention,
	}) {
		t.Fatalf("RequiredSecrets() = %#v", configuration.RequiredSecrets())
	}
}

func TestProductionRejectsUnsafeConfiguration(t *testing.T) {
	base := productionEnvironment()
	tests := []struct {
		name   string
		mutate func([]string) []string
		code   ErrorCode
	}{
		{name: "disabled TLS", mutate: func(env []string) []string { return replaceEnvironment(env, "AEGIS_DATABASE_TLS_MODE", "disabled") }, code: ErrorUnsupported},
		{name: "text logging", mutate: func(env []string) []string { return append(env, "AEGIS_LOG_FORMAT=text") }, code: ErrorUnsupported},
		{name: "debug logging", mutate: func(env []string) []string { return append(env, "AEGIS_LOG_LEVEL=debug") }, code: ErrorUnsupported},
		{name: "combined user", mutate: func(env []string) []string { return append(env, "AEGIS_DATABASE_USER=combined") }, code: ErrorUnsupported},
		{name: "duplicate role", mutate: func(env []string) []string {
			return replaceEnvironment(env, "AEGIS_DATABASE_READ_USER", "platform_ingest")
		}, code: ErrorConflictingValue},
		{name: "budget exceeded", mutate: func(env []string) []string { return replaceEnvironment(env, "AEGIS_DATABASE_CONNECTION_BUDGET", "8") }, code: ErrorConflictingValue},
	}
	provider := staticSecretProvider{
		SecretDatabaseIngest: []byte("ingest"), SecretDatabaseRead: []byte("read"), SecretDatabaseRetention: []byte("retention"),
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewLoader().Load(context.Background(), NewLoadRequest(LoadRequestParams{
				Environment: test.mutate(append([]string(nil), base...)), SecretProvider: provider,
			}))
			if CodeOf(err) != test.code {
				t.Fatalf("CodeOf(error) = %q, want %q; error = %v", CodeOf(err), test.code, err)
			}
		})
	}
}

func TestSecretBoundariesAndRedaction(t *testing.T) {
	secret := "do-not-log-this-value"
	environment := []string{
		"AEGIS_DATABASE_HOST=127.0.0.1",
		"AEGIS_DATABASE_NAME=platform",
		"AEGIS_DATABASE_USER=platform_local",
		"AEGIS_DATABASE_PASSWORD=" + secret,
	}
	loaded, err := NewLoader().Load(context.Background(), NewLoadRequest(LoadRequestParams{Environment: environment}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	encoded, err := json.Marshal(loaded.Config().SafeView())
	if err != nil {
		t.Fatal(err)
	}
	for _, output := range []string{string(encoded), loaded.Config().String()} {
		if strings.Contains(output, secret) {
			t.Fatalf("secret leaked in safe output: %s", output)
		}
		if !strings.Contains(output, redactedMarker) {
			t.Fatalf("redaction marker missing: %s", output)
		}
	}
	resolved, err := loaded.ResolveSecret(context.Background(), SecretDatabaseCombined)
	if err != nil {
		t.Fatal(err)
	}
	if string(resolved) != secret {
		t.Fatal("resolved secret differs")
	}
	clear(resolved)

	_, err = NewLoader().Load(context.Background(), NewLoadRequest(LoadRequestParams{
		Environment:    environment,
		SecretProvider: staticSecretProvider{SecretDatabaseCombined: []byte("other")},
	}))
	if CodeOf(err) != ErrorSecretAmbiguous {
		t.Fatalf("CodeOf(ambiguous error) = %q", CodeOf(err))
	}
}

func TestMissingSecretAndSafeError(t *testing.T) {
	environment := localEnvironment()
	environment = environment[:len(environment)-1]
	_, err := NewLoader().Load(context.Background(), NewLoadRequest(LoadRequestParams{Environment: environment}))
	if CodeOf(err) != ErrorSecretUnavailable {
		t.Fatalf("CodeOf(error) = %q; error = %v", CodeOf(err), err)
	}
	if strings.Contains(err.Error(), "platform_local") {
		t.Fatalf("error leaked supplied data: %v", err)
	}
}

func TestLocalTLSBoundaryAndDurationUnits(t *testing.T) {
	tests := []struct {
		name        string
		environment []string
		code        ErrorCode
	}{
		{name: "remote disabled TLS", environment: replaceEnvironment(localEnvironment(), "AEGIS_DATABASE_HOST", "db.example.com"), code: ErrorConflictingValue},
		{name: "duration without unit", environment: append(localEnvironment(), "AEGIS_STARTUP_TIMEOUT=30"), code: ErrorInvalidValue},
		{name: "inconsistent combined pool", environment: append(localEnvironment(), "AEGIS_DATABASE_READ_MAX_CONNS=3"), code: ErrorConflictingValue},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewLoader().Load(context.Background(), NewLoadRequest(LoadRequestParams{Environment: test.environment}))
			if CodeOf(err) != test.code {
				t.Fatalf("CodeOf(error) = %q, want %q; error = %v", CodeOf(err), test.code, err)
			}
		})
	}
}

func TestCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewLoader().Load(ctx, NewLoadRequest(LoadRequestParams{Environment: localEnvironment()}))
	if !errors.Is(err, context.Canceled) || CodeOf(err) != ErrorCanceled {
		t.Fatalf("error = %v, code = %q", err, CodeOf(err))
	}
}

func TestConfigFileMustBeAbsolute(t *testing.T) {
	environment := append(localEnvironment(), "AEGIS_CONFIG_FILE=runtime.json")
	_, err := NewLoader().Load(context.Background(), NewLoadRequest(LoadRequestParams{Environment: environment}))
	if CodeOf(err) != ErrorInvalidValue {
		t.Fatalf("CodeOf(error) = %q; error = %v", CodeOf(err), err)
	}
}

func TestImmutableModelGetterContract(t *testing.T) {
	request := NewLoadRequest(LoadRequestParams{Environment: localEnvironment(), Arguments: []string{"--log-format=text"}})
	if len(request.Environment()) != 4 || len(request.Arguments()) != 1 {
		t.Fatal("request getters returned unexpected values")
	}
	environment := request.Environment()
	environment[0] = "changed"
	arguments := request.Arguments()
	arguments[0] = "changed"
	if request.Environment()[0] == "changed" || request.Arguments()[0] == "changed" {
		t.Fatal("request getters returned shared slices")
	}

	loaded, err := NewLoader().Load(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	configuration := loaded.Config()
	startup := configuration.Startup()
	if startup.StartupTimeout() <= 0 || startup.DrainTimeout() <= 0 || startup.ForcedShutdownTimeout() <= 0 {
		t.Fatal("startup getters returned invalid durations")
	}
	logging := configuration.Logging()
	if logging.Format() != LogText || logging.ServiceName() != "aegis-codemind" {
		t.Fatal("logging getters returned unexpected values")
	}
	database := configuration.Database()
	if database.Port() != 5432 || database.User() != "platform_local" || database.ApplicationName() == "" ||
		database.ConnectTimeout() <= 0 || database.ConnectionBudget() != 4 {
		t.Fatal("database getters returned unexpected values")
	}
	pools := database.Pools()
	for _, pool := range []PoolConfig{pools.Ingest(), pools.Read(), pools.Retention()} {
		if pool.MaxConns() != 4 || pool.MinIdleConns() != 0 || pool.SecretReference() != SecretDatabaseCombined {
			t.Fatal("pool getters returned unexpected values")
		}
	}
	if pools.MaxConnLifetime() <= 0 || pools.MaxConnLifetimeJitter() <= 0 || pools.MaxConnIdleTime() <= 0 ||
		pools.HealthCheckPeriod() <= 0 || pools.PingTimeout() <= 0 {
		t.Fatal("pool duration getters returned invalid values")
	}
	tls := database.TLS()
	if tls.RootCAPath() != "" || tls.ClientCertPath() != "" || tls.ClientKeyPath() != "" || tls.ServerName() != "" {
		t.Fatal("unexpected TLS paths")
	}
	health := configuration.Health()
	if health.CheckTimeout() <= 0 || health.CheckInterval() <= 0 || health.FailureThreshold() != 3 {
		t.Fatal("health getters returned invalid values")
	}
	observability := configuration.Observability()
	if !observability.MetricsEnabled() || observability.CollectionInterval() <= 0 {
		t.Fatal("observability getters returned invalid values")
	}
}

func TestErrorContract(t *testing.T) {
	if CodeOf(nil) != "" || CodeOf(errors.New("raw")) != ErrorInternal {
		t.Fatal("CodeOf returned an unexpected category")
	}
	if CodeOf(context.Canceled) != ErrorCanceled || CodeOf(context.DeadlineExceeded) != ErrorCanceled {
		t.Fatal("context errors were not classified")
	}
	failure := newError(ErrorInvalidValue, "database.host", errors.New("sensitive cause"))
	var typed *Error
	if !errors.As(failure, &typed) || typed.Code() != ErrorInvalidValue || typed.Field() != "database.host" {
		t.Fatal("typed error contract failed")
	}
	if strings.Contains(failure.Error(), "sensitive") {
		t.Fatal("wrapped cause leaked")
	}
	if (&Error{}).Error() == "" || (*Error)(nil).Error() == "" || (*Error)(nil).Code() != ErrorInternal || (*Error)(nil).Field() != "configuration" {
		t.Fatal("nil error contract failed")
	}
	if newError(ErrorCode("invalid-code"), "unsafe field!", nil).(*Error).Code() != ErrorInternal {
		t.Fatal("unknown error code was not normalized")
	}
	canceled := newError(ErrorInternal, "configuration", context.Canceled)
	if !errors.Is(canceled, context.Canceled) || !errors.Is(newError(ErrorInternal, "configuration", context.DeadlineExceeded), context.DeadlineExceeded) {
		t.Fatal("context unwrapping failed")
	}
}

func TestTLSFilesAndServerName(t *testing.T) {
	directory := t.TempDir()
	rootCA := filepath.Join(directory, "root-ca.pem")
	clientCert := filepath.Join(directory, "client.pem")
	clientKey := filepath.Join(directory, "client-key.pem")
	for _, path := range []string{rootCA, clientCert, clientKey} {
		if err := os.WriteFile(path, []byte("test fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(directory, "runtime.json")
	data := `{
  "database": {
    "host": "127.0.0.1",
    "name": "platform",
    "user": "platform_local",
    "tls": {
      "mode": "verify-full",
      "root_ca_path": ` + strconv.Quote(rootCA) + `,
      "client_cert_path": ` + strconv.Quote(clientCert) + `,
      "client_key_path": ` + strconv.Quote(clientKey) + `,
      "server_name": "db.example.com"
    }
  }
}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := NewLoader().Load(context.Background(), NewLoadRequest(LoadRequestParams{Environment: []string{
		"AEGIS_CONFIG_FILE=" + path,
		"AEGIS_DATABASE_PASSWORD=secret",
	}}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	tls := loaded.Config().Database().TLS()
	if tls.RootCAPath() != rootCA || tls.ClientCertPath() != clientCert || tls.ClientKeyPath() != clientKey || tls.ServerName() != "db.example.com" {
		t.Fatal("TLS file getters returned unexpected values")
	}
}

func TestUnixClientKeyPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission contract")
	}
	path := filepath.Join(t.TempDir(), "client-key.pem")
	if err := os.WriteFile(path, []byte("fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validateTLSPath(fieldTLSClientKey, path, ProfileProduction); CodeOf(err) != ErrorUnsupported {
		t.Fatalf("CodeOf(error) = %q; error = %v", CodeOf(err), err)
	}
}

func TestValidationBranches(t *testing.T) {
	tests := []struct {
		name  string
		field string
		value string
		code  ErrorCode
	}{
		{name: "profile", field: fieldProfile, value: "invalid", code: ErrorInvalidProfile},
		{name: "log level", field: fieldLogLevel, value: "trace", code: ErrorInvalidValue},
		{name: "log format", field: fieldLogFormat, value: "xml", code: ErrorInvalidValue},
		{name: "database name", field: fieldDBName, value: "bad name", code: ErrorInvalidValue},
		{name: "application name", field: fieldDBApplication, value: strings.Repeat("a", 65), code: ErrorInvalidValue},
		{name: "host URL", field: fieldDBHost, value: "postgresql://db", code: ErrorInvalidValue},
		{name: "port", field: fieldDBPort, value: "0", code: ErrorInvalidValue},
		{name: "startup overflow", field: fieldStartupTimeout, value: "11m", code: ErrorInvalidValue},
		{name: "jitter conflict", field: fieldLifetimeJitter, value: "2h", code: ErrorConflictingValue},
		{name: "TLS mode", field: fieldTLSMode, value: "require", code: ErrorInvalidValue},
		{name: "local budget", field: fieldDBBudget, value: "5", code: ErrorUnsupported},
		{name: "client pair", field: fieldTLSClientCert, value: filepath.Join(t.TempDir(), "missing.pem"), code: ErrorConflictingValue},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			values, sources := defaultValues(ProfileLocal)
			values[fieldDBHost] = "127.0.0.1"
			values[fieldDBName] = "platform"
			values[fieldDBUser] = "platform_local"
			values[test.field] = test.value
			_, err := buildRuntimeConfig(values, sources)
			if CodeOf(err) != test.code {
				t.Fatalf("CodeOf(error) = %q, want %q; error = %v", CodeOf(err), test.code, err)
			}
		})
	}
}

func TestConfigFileReadFailures(t *testing.T) {
	directory := t.TempDir()
	tests := []struct {
		name string
		path string
		code ErrorCode
	}{
		{name: "missing", path: filepath.Join(directory, "missing.json"), code: ErrorFileRead},
		{name: "directory", path: directory, code: ErrorInvalidValue},
	}
	invalidUTF8 := filepath.Join(directory, "invalid.json")
	if err := os.WriteFile(invalidUTF8, []byte{0xff}, 0o600); err != nil {
		t.Fatal(err)
	}
	tests = append(tests, struct {
		name, path string
		code       ErrorCode
	}{name: "invalid UTF-8", path: invalidUTF8, code: ErrorInvalidValue})
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			environment := append(localEnvironment(), "AEGIS_CONFIG_FILE="+test.path)
			_, err := NewLoader().Load(context.Background(), NewLoadRequest(LoadRequestParams{Environment: environment}))
			if CodeOf(err) != test.code {
				t.Fatalf("CodeOf(error) = %q, want %q; error = %v", CodeOf(err), test.code, err)
			}
		})
	}
}

func TestSecretResolutionRejections(t *testing.T) {
	loaded, err := NewLoader().Load(context.Background(), NewLoadRequest(LoadRequestParams{Environment: localEnvironment()}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loaded.ResolveSecret(nil, SecretDatabaseCombined); CodeOf(err) != ErrorInvalidInput {
		t.Fatalf("nil context code = %q", CodeOf(err))
	}
	if _, err := loaded.ResolveSecret(context.Background(), SecretDatabaseRead); CodeOf(err) != ErrorInvalidInput {
		t.Fatalf("unexpected reference code = %q", CodeOf(err))
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := loaded.ResolveSecret(ctx, SecretDatabaseCombined); CodeOf(err) != ErrorCanceled {
		t.Fatalf("canceled secret code = %q", CodeOf(err))
	}
}

func FuzzStrictJSONNeverPanics(f *testing.F) {
	for _, seed := range []string{
		`{}`,
		`{"profile":"local"}`,
		`{"profile":"local","profile":"ci"}`,
		`{"database":{"port":5432}}`,
		`{"database":{"unknown":true}}`,
		`[]`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		_, _ = decodeStrictConfiguration([]byte(input))
	})
}

func FuzzEnvironmentAndValidationNeverPanics(f *testing.F) {
	f.Add("127.0.0.1", "30s")
	f.Add("db.example.com", "1m")
	f.Add("postgresql://invalid", "30")
	f.Fuzz(func(t *testing.T, host, startupTimeout string) {
		environment := []string{
			"AEGIS_DATABASE_HOST=" + host,
			"AEGIS_DATABASE_NAME=platform",
			"AEGIS_DATABASE_USER=platform_local",
			"AEGIS_DATABASE_PASSWORD=fuzz-placeholder",
			"AEGIS_STARTUP_TIMEOUT=" + startupTimeout,
		}
		_, _ = NewLoader().Load(context.Background(), NewLoadRequest(LoadRequestParams{Environment: environment}))
	})
}

type staticSecretProvider map[SecretReference][]byte

func (provider staticSecretProvider) Resolve(ctx context.Context, reference SecretReference) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	value, ok := provider[reference]
	if !ok {
		return nil, errors.New("missing")
	}
	return append([]byte(nil), value...), nil
}

func localEnvironment() []string {
	return []string{
		"AEGIS_DATABASE_HOST=127.0.0.1",
		"AEGIS_DATABASE_NAME=platform",
		"AEGIS_DATABASE_USER=platform_local",
		"AEGIS_DATABASE_PASSWORD=local-secret",
	}
}

func productionEnvironment() []string {
	return []string{
		"AEGIS_PROFILE=production",
		"AEGIS_DATABASE_HOST=db.example.com",
		"AEGIS_DATABASE_NAME=platform",
		"AEGIS_DATABASE_CONNECTION_BUDGET=12",
		"AEGIS_DATABASE_INGEST_USER=platform_ingest",
		"AEGIS_DATABASE_INGEST_MAX_CONNS=5",
		"AEGIS_DATABASE_INGEST_MIN_IDLE_CONNS=1",
		"AEGIS_DATABASE_READ_USER=platform_read",
		"AEGIS_DATABASE_READ_MAX_CONNS=5",
		"AEGIS_DATABASE_READ_MIN_IDLE_CONNS=1",
		"AEGIS_DATABASE_RETENTION_USER=platform_retention",
		"AEGIS_DATABASE_RETENTION_MAX_CONNS=2",
		"AEGIS_DATABASE_RETENTION_MIN_IDLE_CONNS=0",
	}
}

func replaceEnvironment(environment []string, name, value string) []string {
	prefix := name + "="
	for index, entry := range environment {
		if strings.HasPrefix(entry, prefix) {
			environment[index] = prefix + value
			return environment
		}
	}
	return append(environment, prefix+value)
}
