package config

import "time"

const (
	maximumConfigBytes = 1 << 20
	maximumSecretBytes = 64 << 10
	maximumTextBytes   = 256

	defaultStartupTimeout        = 30 * time.Second
	defaultDrainTimeout          = 30 * time.Second
	defaultForcedShutdownTimeout = 5 * time.Second
	defaultConnectTimeout        = 10 * time.Second
	defaultHealthTimeout         = 2 * time.Second
	defaultHealthInterval        = 15 * time.Second
	defaultCIHealthInterval      = 5 * time.Second
	defaultMetricsInterval       = 15 * time.Second
	defaultMaxConnLifetime       = time.Hour
	defaultMaxConnLifetimeJitter = 5 * time.Minute
	defaultMaxConnIdleTime       = 30 * time.Minute
	defaultPoolHealthPeriod      = time.Minute
	defaultPingTimeout           = 5 * time.Second
)

const (
	fieldProfile = "profile"

	fieldStartupTimeout = "startup.startup_timeout"
	fieldDrainTimeout   = "startup.drain_timeout"
	fieldForcedTimeout  = "startup.forced_shutdown_timeout"

	fieldLogLevel  = "logging.level"
	fieldLogFormat = "logging.format"
	fieldService   = "logging.service_name"

	fieldDBHost        = "database.host"
	fieldDBPort        = "database.port"
	fieldDBName        = "database.name"
	fieldDBUser        = "database.user"
	fieldDBApplication = "database.application_name"
	fieldDBConnect     = "database.connect_timeout"
	fieldDBBudget      = "database.connection_budget"

	fieldIngestUser = "database.pools.ingest.user"
	fieldIngestMax  = "database.pools.ingest.max_conns"
	fieldIngestMin  = "database.pools.ingest.min_idle_conns"
	fieldReadUser   = "database.pools.read.user"
	fieldReadMax    = "database.pools.read.max_conns"
	fieldReadMin    = "database.pools.read.min_idle_conns"
	fieldRetainUser = "database.pools.retention.user"
	fieldRetainMax  = "database.pools.retention.max_conns"
	fieldRetainMin  = "database.pools.retention.min_idle_conns"

	fieldMaxLifetime    = "database.pools.max_conn_lifetime"
	fieldLifetimeJitter = "database.pools.max_conn_lifetime_jitter"
	fieldMaxIdle        = "database.pools.max_conn_idle_time"
	fieldPoolHealth     = "database.pools.health_check_period"
	fieldPingTimeout    = "database.pools.ping_timeout"

	fieldTLSMode       = "database.tls.mode"
	fieldTLSRootCA     = "database.tls.root_ca_path"
	fieldTLSClientCert = "database.tls.client_cert_path"
	fieldTLSClientKey  = "database.tls.client_key_path"
	fieldTLSServerName = "database.tls.server_name"

	fieldHealthTimeout   = "health.check_timeout"
	fieldHealthInterval  = "health.check_interval"
	fieldHealthThreshold = "health.failure_threshold"

	fieldMetricsEnabled  = "observability.metrics_enabled"
	fieldMetricsInterval = "observability.collection_interval"
)

type valueType uint8

const (
	valueString valueType = iota + 1
	valueInteger
	valueBoolean
)

var fileFields = map[string]valueType{
	fieldProfile:        valueString,
	fieldStartupTimeout: valueString, fieldDrainTimeout: valueString,
	fieldForcedTimeout: valueString,
	fieldLogLevel:      valueString, fieldLogFormat: valueString, fieldService: valueString,
	fieldDBHost: valueString, fieldDBPort: valueInteger, fieldDBName: valueString,
	fieldDBUser: valueString, fieldDBApplication: valueString,
	fieldDBConnect: valueString, fieldDBBudget: valueInteger,
	fieldIngestUser: valueString, fieldIngestMax: valueInteger, fieldIngestMin: valueInteger,
	fieldReadUser: valueString, fieldReadMax: valueInteger, fieldReadMin: valueInteger,
	fieldRetainUser: valueString, fieldRetainMax: valueInteger, fieldRetainMin: valueInteger,
	fieldMaxLifetime: valueString, fieldLifetimeJitter: valueString,
	fieldMaxIdle: valueString, fieldPoolHealth: valueString, fieldPingTimeout: valueString,
	fieldTLSMode: valueString, fieldTLSRootCA: valueString,
	fieldTLSClientCert: valueString, fieldTLSClientKey: valueString,
	fieldTLSServerName: valueString,
	fieldHealthTimeout: valueString, fieldHealthInterval: valueString,
	fieldHealthThreshold: valueInteger,
	fieldMetricsEnabled:  valueBoolean, fieldMetricsInterval: valueString,
}

var environmentFields = map[string]string{
	"AEGIS_PROFILE":                           fieldProfile,
	"AEGIS_LOG_LEVEL":                         fieldLogLevel,
	"AEGIS_LOG_FORMAT":                        fieldLogFormat,
	"AEGIS_DATABASE_HOST":                     fieldDBHost,
	"AEGIS_DATABASE_PORT":                     fieldDBPort,
	"AEGIS_DATABASE_NAME":                     fieldDBName,
	"AEGIS_DATABASE_USER":                     fieldDBUser,
	"AEGIS_DATABASE_CONNECTION_BUDGET":        fieldDBBudget,
	"AEGIS_DATABASE_INGEST_MAX_CONNS":         fieldIngestMax,
	"AEGIS_DATABASE_INGEST_MIN_IDLE_CONNS":    fieldIngestMin,
	"AEGIS_DATABASE_INGEST_USER":              fieldIngestUser,
	"AEGIS_DATABASE_READ_MAX_CONNS":           fieldReadMax,
	"AEGIS_DATABASE_READ_MIN_IDLE_CONNS":      fieldReadMin,
	"AEGIS_DATABASE_READ_USER":                fieldReadUser,
	"AEGIS_DATABASE_RETENTION_MAX_CONNS":      fieldRetainMax,
	"AEGIS_DATABASE_RETENTION_MIN_IDLE_CONNS": fieldRetainMin,
	"AEGIS_DATABASE_RETENTION_USER":           fieldRetainUser,
	"AEGIS_DATABASE_CONNECT_TIMEOUT":          fieldDBConnect,
	"AEGIS_DATABASE_TLS_MODE":                 fieldTLSMode,
	"AEGIS_DATABASE_TLS_ROOT_CA_FILE":         fieldTLSRootCA,
	"AEGIS_DATABASE_TLS_CLIENT_CERT_FILE":     fieldTLSClientCert,
	"AEGIS_DATABASE_TLS_CLIENT_KEY_FILE":      fieldTLSClientKey,
	"AEGIS_DATABASE_TLS_SERVER_NAME":          fieldTLSServerName,
	"AEGIS_STARTUP_TIMEOUT":                   fieldStartupTimeout,
	"AEGIS_DRAIN_TIMEOUT":                     fieldDrainTimeout,
}

var secretEnvironmentFields = map[string]SecretReference{
	"AEGIS_DATABASE_PASSWORD":           SecretDatabaseCombined,
	"AEGIS_DATABASE_INGEST_PASSWORD":    SecretDatabaseIngest,
	"AEGIS_DATABASE_READ_PASSWORD":      SecretDatabaseRead,
	"AEGIS_DATABASE_RETENTION_PASSWORD": SecretDatabaseRetention,
}

var commandFields = map[string]string{
	"profile":                           fieldProfile,
	"log-level":                         fieldLogLevel,
	"log-format":                        fieldLogFormat,
	"database-host":                     fieldDBHost,
	"database-port":                     fieldDBPort,
	"database-name":                     fieldDBName,
	"database-user":                     fieldDBUser,
	"database-connection-budget":        fieldDBBudget,
	"database-ingest-user":              fieldIngestUser,
	"database-ingest-max-conns":         fieldIngestMax,
	"database-ingest-min-idle-conns":    fieldIngestMin,
	"database-read-user":                fieldReadUser,
	"database-read-max-conns":           fieldReadMax,
	"database-read-min-idle-conns":      fieldReadMin,
	"database-retention-user":           fieldRetainUser,
	"database-retention-max-conns":      fieldRetainMax,
	"database-retention-min-idle-conns": fieldRetainMin,
	"database-connect-timeout":          fieldDBConnect,
	"database-tls-mode":                 fieldTLSMode,
	"database-tls-root-ca-file":         fieldTLSRootCA,
	"database-tls-client-cert-file":     fieldTLSClientCert,
	"database-tls-client-key-file":      fieldTLSClientKey,
	"database-tls-server-name":          fieldTLSServerName,
	"startup-timeout":                   fieldStartupTimeout,
	"drain-timeout":                     fieldDrainTimeout,
}

func defaultValues(profile Profile) (map[string]string, map[string]SourceCategory) {
	logFormat := string(LogJSON)
	healthInterval := defaultHealthInterval
	tlsMode := string(TLSVerifyFull)
	budget := ""
	poolMax := ""
	poolMin := ""
	if profile == ProfileLocal || profile == ProfileCI {
		budget = "4"
		poolMax = "4"
		poolMin = "0"
		tlsMode = string(TLSDisabled)
	}
	if profile == ProfileLocal {
		logFormat = string(LogText)
	}
	if profile == ProfileCI {
		healthInterval = defaultCIHealthInterval
	}
	values := map[string]string{
		fieldProfile:        string(profile),
		fieldStartupTimeout: defaultStartupTimeout.String(),
		fieldDrainTimeout:   defaultDrainTimeout.String(),
		fieldForcedTimeout:  defaultForcedShutdownTimeout.String(),
		fieldLogLevel:       string(LogInfo), fieldLogFormat: logFormat,
		fieldService: "aegis-codemind",
		fieldDBHost:  "", fieldDBPort: "5432", fieldDBName: "", fieldDBUser: "",
		fieldDBApplication: "aegis-codemind-" + string(profile),
		fieldDBConnect:     defaultConnectTimeout.String(), fieldDBBudget: budget,
		fieldIngestUser: "", fieldIngestMax: poolMax, fieldIngestMin: poolMin,
		fieldReadUser: "", fieldReadMax: poolMax, fieldReadMin: poolMin,
		fieldRetainUser: "", fieldRetainMax: poolMax, fieldRetainMin: poolMin,
		fieldMaxLifetime:    defaultMaxConnLifetime.String(),
		fieldLifetimeJitter: defaultMaxConnLifetimeJitter.String(),
		fieldMaxIdle:        defaultMaxConnIdleTime.String(),
		fieldPoolHealth:     defaultPoolHealthPeriod.String(),
		fieldPingTimeout:    defaultPingTimeout.String(),
		fieldTLSMode:        tlsMode, fieldTLSRootCA: "", fieldTLSClientCert: "",
		fieldTLSClientKey: "", fieldTLSServerName: "",
		fieldHealthTimeout:  defaultHealthTimeout.String(),
		fieldHealthInterval: healthInterval.String(), fieldHealthThreshold: "3",
		fieldMetricsEnabled: "true", fieldMetricsInterval: defaultMetricsInterval.String(),
	}
	sources := make(map[string]SourceCategory, len(values))
	for field := range values {
		sources[field] = SourceDefault
	}
	return values, sources
}
