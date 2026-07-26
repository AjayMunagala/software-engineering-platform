package config

import (
	"context"
	"encoding/json"
	"time"
)

type Profile string

const (
	ProfileLocal      Profile = "local"
	ProfileCI         Profile = "ci"
	ProfileStaging    Profile = "staging"
	ProfileProduction Profile = "production"
)

type TLSMode string

const (
	TLSDisabled   TLSMode = "disabled"
	TLSVerifyFull TLSMode = "verify-full"
)

type LogLevel string

const (
	LogDebug LogLevel = "debug"
	LogInfo  LogLevel = "info"
	LogWarn  LogLevel = "warn"
	LogError LogLevel = "error"
)

type LogFormat string

const (
	LogText LogFormat = "text"
	LogJSON LogFormat = "json"
)

type PoolMode string

const (
	PoolCombined PoolMode = "combined"
	PoolSeparate PoolMode = "separate"
)

type SourceCategory string

const (
	SourceDefault     SourceCategory = "default"
	SourceFile        SourceCategory = "file"
	SourceEnvironment SourceCategory = "environment"
	SourceCommandLine SourceCategory = "command_line"
)

// SecretReference names a secret without containing its value.
type SecretReference string

const (
	SecretDatabaseCombined  SecretReference = "database-combined-password"
	SecretDatabaseIngest    SecretReference = "database-ingest-password"
	SecretDatabaseRead      SecretReference = "database-read-password"
	SecretDatabaseRetention SecretReference = "database-retention-password"
	SecretTLSClientKey      SecretReference = "tls-client-key-passphrase"
)

const redactedMarker = "[REDACTED]"

type StartupConfig struct {
	startupTimeout        time.Duration
	drainTimeout          time.Duration
	forcedShutdownTimeout time.Duration
}

func (value StartupConfig) StartupTimeout() time.Duration        { return value.startupTimeout }
func (value StartupConfig) DrainTimeout() time.Duration          { return value.drainTimeout }
func (value StartupConfig) ForcedShutdownTimeout() time.Duration { return value.forcedShutdownTimeout }

type LoggingConfig struct {
	level       LogLevel
	format      LogFormat
	serviceName string
}

func (value LoggingConfig) Level() LogLevel     { return value.level }
func (value LoggingConfig) Format() LogFormat   { return value.format }
func (value LoggingConfig) ServiceName() string { return value.serviceName }

type PoolConfig struct {
	user         string
	maxConns     int32
	minIdleConns int32
	secret       SecretReference
}

func (value PoolConfig) User() string                     { return value.user }
func (value PoolConfig) MaxConns() int32                  { return value.maxConns }
func (value PoolConfig) MinIdleConns() int32              { return value.minIdleConns }
func (value PoolConfig) SecretReference() SecretReference { return value.secret }

type PoolSetConfig struct {
	mode                  PoolMode
	ingest                PoolConfig
	read                  PoolConfig
	retention             PoolConfig
	maxConnLifetime       time.Duration
	maxConnLifetimeJitter time.Duration
	maxConnIdleTime       time.Duration
	healthCheckPeriod     time.Duration
	pingTimeout           time.Duration
}

func (value PoolSetConfig) Mode() PoolMode                       { return value.mode }
func (value PoolSetConfig) Ingest() PoolConfig                   { return value.ingest }
func (value PoolSetConfig) Read() PoolConfig                     { return value.read }
func (value PoolSetConfig) Retention() PoolConfig                { return value.retention }
func (value PoolSetConfig) MaxConnLifetime() time.Duration       { return value.maxConnLifetime }
func (value PoolSetConfig) MaxConnLifetimeJitter() time.Duration { return value.maxConnLifetimeJitter }
func (value PoolSetConfig) MaxConnIdleTime() time.Duration       { return value.maxConnIdleTime }
func (value PoolSetConfig) HealthCheckPeriod() time.Duration     { return value.healthCheckPeriod }
func (value PoolSetConfig) PingTimeout() time.Duration           { return value.pingTimeout }

type TLSConfig struct {
	mode           TLSMode
	rootCAPath     string
	clientCertPath string
	clientKeyPath  string
	serverName     string
}

func (value TLSConfig) Mode() TLSMode          { return value.mode }
func (value TLSConfig) RootCAPath() string     { return value.rootCAPath }
func (value TLSConfig) ClientCertPath() string { return value.clientCertPath }
func (value TLSConfig) ClientKeyPath() string  { return value.clientKeyPath }
func (value TLSConfig) ServerName() string     { return value.serverName }

type DatabaseConfig struct {
	host             string
	port             uint16
	name             string
	user             string
	applicationName  string
	connectTimeout   time.Duration
	connectionBudget int32
	pools            PoolSetConfig
	tls              TLSConfig
}

func (value DatabaseConfig) Host() string                  { return value.host }
func (value DatabaseConfig) Port() uint16                  { return value.port }
func (value DatabaseConfig) Name() string                  { return value.name }
func (value DatabaseConfig) User() string                  { return value.user }
func (value DatabaseConfig) ApplicationName() string       { return value.applicationName }
func (value DatabaseConfig) ConnectTimeout() time.Duration { return value.connectTimeout }
func (value DatabaseConfig) ConnectionBudget() int32       { return value.connectionBudget }
func (value DatabaseConfig) Pools() PoolSetConfig          { return value.pools }
func (value DatabaseConfig) TLS() TLSConfig                { return value.tls }

type HealthConfig struct {
	checkTimeout     time.Duration
	checkInterval    time.Duration
	failureThreshold int
}

func (value HealthConfig) CheckTimeout() time.Duration  { return value.checkTimeout }
func (value HealthConfig) CheckInterval() time.Duration { return value.checkInterval }
func (value HealthConfig) FailureThreshold() int        { return value.failureThreshold }

type ObservabilityConfig struct {
	metricsEnabled     bool
	collectionInterval time.Duration
}

func (value ObservabilityConfig) MetricsEnabled() bool              { return value.metricsEnabled }
func (value ObservabilityConfig) CollectionInterval() time.Duration { return value.collectionInterval }

// RuntimeConfig is the immutable, secret-free runtime configuration.
type RuntimeConfig struct {
	profile       Profile
	startup       StartupConfig
	logging       LoggingConfig
	database      DatabaseConfig
	health        HealthConfig
	observability ObservabilityConfig
	sources       map[string]SourceCategory
}

func (value RuntimeConfig) Profile() Profile                   { return value.profile }
func (value RuntimeConfig) Startup() StartupConfig             { return value.startup }
func (value RuntimeConfig) Logging() LoggingConfig             { return value.logging }
func (value RuntimeConfig) Database() DatabaseConfig           { return value.database }
func (value RuntimeConfig) Health() HealthConfig               { return value.health }
func (value RuntimeConfig) Observability() ObservabilityConfig { return value.observability }
func (value RuntimeConfig) Source(field string) SourceCategory { return value.sources[field] }

// Sources returns a detached source map containing no secret values.
func (value RuntimeConfig) Sources() map[string]SourceCategory {
	result := make(map[string]SourceCategory, len(value.sources))
	for field, source := range value.sources {
		result[field] = source
	}
	return result
}

// RequiredSecrets returns the deterministic references required by the
// selected profile. It never indicates secret length or content.
func (value RuntimeConfig) RequiredSecrets() []SecretReference {
	if value.database.pools.mode == PoolSeparate {
		return []SecretReference{
			SecretDatabaseIngest,
			SecretDatabaseRead,
			SecretDatabaseRetention,
		}
	}
	return []SecretReference{SecretDatabaseCombined}
}

// SafeView is a detached, JSON-safe configuration view. Secret fields are
// represented only by their schema-owned reference and the redaction marker.
type SafeView struct {
	ContractVersion string                     `json:"contract_version"`
	Profile         Profile                    `json:"profile"`
	Startup         SafeStartupView            `json:"startup"`
	Logging         SafeLoggingView            `json:"logging"`
	Database        SafeDatabaseView           `json:"database"`
	Health          SafeHealthView             `json:"health"`
	Observability   SafeObservabilityView      `json:"observability"`
	Sources         map[string]SourceCategory  `json:"sources"`
	Secrets         map[SecretReference]string `json:"secrets"`
}

type SafeStartupView struct {
	StartupTimeout        string `json:"startup_timeout"`
	DrainTimeout          string `json:"drain_timeout"`
	ForcedShutdownTimeout string `json:"forced_shutdown_timeout"`
}

type SafeLoggingView struct {
	Level       LogLevel  `json:"level"`
	Format      LogFormat `json:"format"`
	ServiceName string    `json:"service_name"`
}

type SafePoolView struct {
	User         string          `json:"user"`
	MaxConns     int32           `json:"max_conns"`
	MinIdleConns int32           `json:"min_idle_conns"`
	Secret       SecretReference `json:"secret_reference"`
}

type SafePoolSetView struct {
	Mode                  PoolMode     `json:"mode"`
	Ingest                SafePoolView `json:"ingest"`
	Read                  SafePoolView `json:"read"`
	Retention             SafePoolView `json:"retention"`
	MaxConnLifetime       string       `json:"max_conn_lifetime"`
	MaxConnLifetimeJitter string       `json:"max_conn_lifetime_jitter"`
	MaxConnIdleTime       string       `json:"max_conn_idle_time"`
	HealthCheckPeriod     string       `json:"health_check_period"`
	PingTimeout           string       `json:"ping_timeout"`
}

type SafeTLSView struct {
	Mode           TLSMode `json:"mode"`
	RootCAPath     string  `json:"root_ca_path,omitempty"`
	ClientCertPath string  `json:"client_cert_path,omitempty"`
	ClientKeyPath  string  `json:"client_key_path,omitempty"`
	ServerName     string  `json:"server_name,omitempty"`
}

type SafeDatabaseView struct {
	Host             string          `json:"host"`
	Port             uint16          `json:"port"`
	Name             string          `json:"name"`
	User             string          `json:"user,omitempty"`
	ApplicationName  string          `json:"application_name"`
	ConnectTimeout   string          `json:"connect_timeout"`
	ConnectionBudget int32           `json:"connection_budget"`
	Pools            SafePoolSetView `json:"pools"`
	TLS              SafeTLSView     `json:"tls"`
}

type SafeHealthView struct {
	CheckTimeout     string `json:"check_timeout"`
	CheckInterval    string `json:"check_interval"`
	FailureThreshold int    `json:"failure_threshold"`
}

type SafeObservabilityView struct {
	MetricsEnabled     bool   `json:"metrics_enabled"`
	CollectionInterval string `json:"collection_interval"`
}

func (value RuntimeConfig) SafeView() SafeView {
	pools := value.database.pools
	view := SafeView{
		ContractVersion: ContractVersion,
		Profile:         value.profile,
		Startup: SafeStartupView{
			StartupTimeout:        value.startup.startupTimeout.String(),
			DrainTimeout:          value.startup.drainTimeout.String(),
			ForcedShutdownTimeout: value.startup.forcedShutdownTimeout.String(),
		},
		Logging: SafeLoggingView{
			Level:       value.logging.level,
			Format:      value.logging.format,
			ServiceName: value.logging.serviceName,
		},
		Database: SafeDatabaseView{
			Host:             value.database.host,
			Port:             value.database.port,
			Name:             value.database.name,
			User:             value.database.user,
			ApplicationName:  value.database.applicationName,
			ConnectTimeout:   value.database.connectTimeout.String(),
			ConnectionBudget: value.database.connectionBudget,
			Pools: SafePoolSetView{
				Mode:                  pools.mode,
				Ingest:                safePoolView(pools.ingest),
				Read:                  safePoolView(pools.read),
				Retention:             safePoolView(pools.retention),
				MaxConnLifetime:       pools.maxConnLifetime.String(),
				MaxConnLifetimeJitter: pools.maxConnLifetimeJitter.String(),
				MaxConnIdleTime:       pools.maxConnIdleTime.String(),
				HealthCheckPeriod:     pools.healthCheckPeriod.String(),
				PingTimeout:           pools.pingTimeout.String(),
			},
			TLS: SafeTLSView{
				Mode:           value.database.tls.mode,
				RootCAPath:     value.database.tls.rootCAPath,
				ClientCertPath: value.database.tls.clientCertPath,
				ClientKeyPath:  value.database.tls.clientKeyPath,
				ServerName:     value.database.tls.serverName,
			},
		},
		Health: SafeHealthView{
			CheckTimeout:     value.health.checkTimeout.String(),
			CheckInterval:    value.health.checkInterval.String(),
			FailureThreshold: value.health.failureThreshold,
		},
		Observability: SafeObservabilityView{
			MetricsEnabled:     value.observability.metricsEnabled,
			CollectionInterval: value.observability.collectionInterval.String(),
		},
		Sources: value.Sources(),
		Secrets: make(map[SecretReference]string),
	}
	for _, reference := range value.RequiredSecrets() {
		view.Secrets[reference] = redactedMarker
	}
	return view
}

func safePoolView(value PoolConfig) SafePoolView {
	return SafePoolView{
		User:         value.user,
		MaxConns:     value.maxConns,
		MinIdleConns: value.minIdleConns,
		Secret:       value.secret,
	}
}

func (value RuntimeConfig) String() string {
	encoded, err := json.Marshal(value.SafeView())
	if err != nil {
		return "{\"contract_version\":\"" + ContractVersion + "\"}"
	}
	return string(encoded)
}

// LoadedConfiguration keeps the selected provider private while exposing the
// immutable ordinary configuration.
type LoadedConfiguration struct {
	configuration RuntimeConfig
	provider      SecretProvider
}

func (value LoadedConfiguration) Config() RuntimeConfig { return value.configuration }

// ResolveSecret delegates to the one selected provider and returns detached
// bytes. Callers must overwrite the returned slice immediately after use.
func (value LoadedConfiguration) ResolveSecret(ctx context.Context, reference SecretReference) ([]byte, error) {
	if ctx == nil {
		return nil, newError(ErrorInvalidInput, "context", nil)
	}
	if !value.configuration.requiresSecret(reference) {
		return nil, newError(ErrorInvalidInput, "secret_reference", nil)
	}
	if value.provider == nil {
		return nil, newError(ErrorSecretUnavailable, "secret_provider", nil)
	}
	secret, err := value.provider.Resolve(ctx, reference)
	if err != nil {
		return nil, newError(ErrorSecretUnavailable, "secret", err)
	}
	if len(secret) == 0 || len(secret) > maximumSecretBytes {
		clear(secret)
		return nil, newError(ErrorSecretUnavailable, "secret", nil)
	}
	result := append([]byte(nil), secret...)
	clear(secret)
	return result, nil
}

func (value RuntimeConfig) requiresSecret(reference SecretReference) bool {
	for _, required := range value.RequiredSecrets() {
		if required == reference {
			return true
		}
	}
	return false
}
