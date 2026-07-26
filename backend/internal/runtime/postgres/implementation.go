package postgres

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	runtimeconfig "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/config"
	storagepostgres "github.com/AjayMunagala/software-engineering-platform/backend/internal/storage/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	ingestRole    = "platform_ingestor"
	readRole      = "platform_artifact_reader"
	retentionRole = "platform_retention_worker"
)

const sessionProofSQL = `
SELECT current_setting('server_version_num')::integer,
       current_setting('TimeZone'),
       current_setting('client_encoding'),
       current_setting('application_name'),
       current_user,
       session_user`

const compatibilityProofSQL = `
SELECT contract_key, schema_contract_version, minimum_adapter_major,
       maximum_adapter_major, migration_revision, published_at
FROM platform.runtime_compatibility
WHERE singleton_key = 1`

const privilegeProofSQL = `
SELECT has_schema_privilege(current_user, 'platform', 'USAGE'),
       has_schema_privilege(current_user, 'platform', 'CREATE'),
       has_schema_privilege(current_user, 'atlas_schema_revisions', 'USAGE'),
       has_table_privilege(current_user, 'platform.runtime_compatibility', 'SELECT'),
       has_table_privilege(current_user, 'platform.runtime_compatibility', 'INSERT'),
       has_table_privilege(current_user, 'platform.runtime_compatibility', 'UPDATE'),
       has_table_privilege(current_user, 'platform.runtime_compatibility', 'DELETE'),
       has_table_privilege(current_user, 'platform.repositories', 'INSERT'),
       has_table_privilege(current_user, 'platform.repositories', 'DELETE'),
       has_table_privilege(current_user, 'platform.artifact_payload_chunks', 'SELECT'),
       has_table_privilege(current_user, 'platform.artifact_envelopes', 'DELETE')`

type databasePool interface {
	storagepostgres.Database
	Ping(context.Context) error
	Close()
}

type poolConstructor func(context.Context, *pgxpool.Config) (databasePool, error)

type factory struct {
	newPool poolConstructor
	clock   clock
}

type poolPlan struct {
	capability Capability
	pool       runtimeconfig.PoolConfig
	role       string
}

type sessionProof struct {
	serverVersion   int
	timezone        string
	encoding        string
	applicationName string
	currentUser     string
	sessionUser     string
}

type privilegeProof struct {
	platformUsage       bool
	platformCreate      bool
	atlasUsage          bool
	compatibilityRead   bool
	compatibilityInsert bool
	compatibilityUpdate bool
	compatibilityDelete bool
	repositoryInsert    bool
	repositoryDelete    bool
	payloadChunkRead    bool
	artifactDelete      bool
}

// NewFactory creates the PostgreSQL runtime factory. Pool creation is lazy and
// occurs only when Open is called.
func NewFactory() Factory {
	return factory{newPool: defaultPoolConstructor, clock: systemClock{}}
}

func defaultPoolConstructor(ctx context.Context, config *pgxpool.Config) (databasePool, error) {
	return pgxpool.NewWithConfig(ctx, config)
}

func (value factory) Open(ctx context.Context, loaded runtimeconfig.LoadedConfiguration) (_ *Runtime, resultErr error) {
	if ctx == nil {
		return nil, newError(ErrorInvalidInput, "context", "", nil)
	}
	if err := ctx.Err(); err != nil {
		return nil, newError(ErrorCanceled, "context", "", err)
	}
	if value.newPool == nil || value.clock == nil {
		return nil, newError(ErrorInvalidInput, "factory", "", nil)
	}

	configuration := loaded.Config()
	database := configuration.Database()
	tlsConfiguration, err := loadTLSConfiguration(ctx, database.TLS(), database.Host(), value.clock.Now())
	if err != nil {
		return nil, err
	}
	plans, err := poolPlans(database.Pools())
	if err != nil {
		return nil, err
	}

	runtime := &Runtime{}
	defer func() {
		if resultErr != nil {
			runtime.Close()
		}
	}()

	var acceptedProof CompatibilityProof
	for _, plan := range plans {
		if err := ctx.Err(); err != nil {
			return nil, newError(ErrorCanceled, "pool-open", plan.capability, err)
		}
		secret, err := loaded.ResolveSecret(ctx, plan.pool.SecretReference())
		if err != nil {
			return nil, newError(ErrorUnavailable, "secret-resolve", plan.capability, err)
		}
		poolConfiguration, err := buildPoolConfiguration(database, plan, secret, tlsConfiguration)
		clear(secret)
		if err != nil {
			return nil, newError(ErrorPoolConfiguration, "pool-configure", plan.capability, err)
		}
		pool, err := value.newPool(ctx, poolConfiguration)
		if err != nil {
			return nil, newError(ErrorUnavailable, "pool-create", plan.capability, err)
		}
		runtime.pools = append(runtime.pools, ownedPool{capability: plan.capability, pool: pool})

		pingCtx, cancel := context.WithTimeout(ctx, database.Pools().PingTimeout())
		err = pool.Ping(pingCtx)
		cancel()
		if err != nil {
			return nil, newError(ErrorUnavailable, "pool-ping", plan.capability, err)
		}

		proof, err := verifyPool(ctx, pool, configuration, plan, value.clock.Now())
		if err != nil {
			return nil, err
		}
		if acceptedProof.contractKey == "" {
			acceptedProof = proof
		} else if !equalCompatibilityProof(acceptedProof, proof) {
			return nil, newError(ErrorSchemaIncompatible, "schema-consistency", plan.capability, nil)
		}

		adapter, err := storagepostgres.New(pool)
		if err != nil {
			return nil, newError(ErrorAdapterInvalid, "adapter-create", plan.capability, err)
		}
		ingest, read, retention := adapterCapabilities(adapter)
		switch plan.capability {
		case CapabilityCombined:
			runtime.ingest, runtime.read, runtime.retention = ingest, read, retention
		case CapabilityIngest:
			runtime.ingest = ingest
		case CapabilityRead:
			runtime.read = read
		case CapabilityRetention:
			runtime.retention = retention
		}
	}
	if runtime.ingest == nil || runtime.read == nil || runtime.retention == nil {
		return nil, newError(ErrorInternal, "capability-routing", "", nil)
	}
	runtime.proof = acceptedProof
	return runtime, nil
}

func poolPlans(pools runtimeconfig.PoolSetConfig) ([]poolPlan, error) {
	switch pools.Mode() {
	case runtimeconfig.PoolCombined:
		return []poolPlan{{CapabilityCombined, pools.Ingest(), ""}}, nil
	case runtimeconfig.PoolSeparate:
		return []poolPlan{
			{CapabilityIngest, pools.Ingest(), ingestRole},
			{CapabilityRead, pools.Read(), readRole},
			{CapabilityRetention, pools.Retention(), retentionRole},
		}, nil
	default:
		return nil, newError(ErrorPoolConfiguration, "pool-mode", "", nil)
	}
}

func buildPoolConfiguration(database runtimeconfig.DatabaseConfig, plan poolPlan, password []byte, tlsConfiguration *tls.Config) (*pgxpool.Config, error) {
	// ParseConfig establishes pgxpool's private invariants. Every connection
	// field is then overwritten from the already validated immutable model.
	configuration, err := pgxpool.ParseConfig("host=127.0.0.1 port=5432 user=runtime database=runtime sslmode=disable")
	if err != nil {
		return nil, err
	}
	configuration.ConnConfig.Host = database.Host()
	configuration.ConnConfig.Port = database.Port()
	configuration.ConnConfig.Database = database.Name()
	configuration.ConnConfig.User = plan.pool.User()
	configuration.ConnConfig.Password = string(password)
	configuration.ConnConfig.ConnectTimeout = database.ConnectTimeout()
	configuration.ConnConfig.TLSConfig = cloneTLSConfiguration(tlsConfiguration)
	configuration.ConnConfig.Fallbacks = nil
	expectTLS := tlsConfiguration != nil
	configuration.ConnConfig.AfterNetConnect = func(_ context.Context, _ *pgconn.Config, connection net.Conn) (net.Conn, error) {
		_, tlsUsed := connection.(*tls.Conn)
		if tlsUsed != expectTLS {
			return connection, errors.New("transport security policy mismatch")
		}
		return connection, nil
	}
	configuration.ConnConfig.RuntimeParams = map[string]string{
		"application_name": database.ApplicationName(),
		"client_encoding":  "UTF8",
		"timezone":         "UTC",
	}
	configuration.MaxConns = plan.pool.MaxConns()
	configuration.MinConns = 0
	configuration.MinIdleConns = plan.pool.MinIdleConns()
	configuration.MaxConnLifetime = database.Pools().MaxConnLifetime()
	configuration.MaxConnLifetimeJitter = database.Pools().MaxConnLifetimeJitter()
	configuration.MaxConnIdleTime = database.Pools().MaxConnIdleTime()
	configuration.HealthCheckPeriod = database.Pools().HealthCheckPeriod()
	configuration.PingTimeout = database.Pools().PingTimeout()
	if plan.role != "" {
		role := plan.role
		configuration.AfterConnect = func(ctx context.Context, connection *pgx.Conn) error {
			switch role {
			case ingestRole:
				_, executeErr := connection.Exec(ctx, "SET ROLE platform_ingestor")
				return executeErr
			case readRole:
				_, executeErr := connection.Exec(ctx, "SET ROLE platform_artifact_reader")
				return executeErr
			case retentionRole:
				_, executeErr := connection.Exec(ctx, "SET ROLE platform_retention_worker")
				return executeErr
			default:
				return errors.New("unsupported capability role")
			}
		}
	}
	return configuration, nil
}

func cloneTLSConfiguration(configuration *tls.Config) *tls.Config {
	if configuration == nil {
		return nil
	}
	return configuration.Clone()
}

func verifyPool(ctx context.Context, pool databasePool, configuration runtimeconfig.RuntimeConfig, plan poolPlan, now time.Time) (CompatibilityProof, error) {
	if err := verifySession(ctx, pool, configuration, plan); err != nil {
		return CompatibilityProof{}, err
	}
	proof, err := verifyCompatibility(ctx, pool, plan.capability, now)
	if err != nil {
		return CompatibilityProof{}, err
	}
	if err := verifyPrivileges(ctx, pool, plan); err != nil {
		return CompatibilityProof{}, err
	}
	return proof, nil
}

func verifySession(ctx context.Context, pool databasePool, configuration runtimeconfig.RuntimeConfig, plan poolPlan) error {
	var proof sessionProof
	err := pool.QueryRow(ctx, sessionProofSQL).Scan(
		&proof.serverVersion, &proof.timezone, &proof.encoding,
		&proof.applicationName, &proof.currentUser, &proof.sessionUser,
	)
	if err != nil {
		return newError(ErrorUnavailable, "session-query", plan.capability, err)
	}
	if proof.serverVersion/10000 != ExpectedPostgreSQLMajor {
		return newError(ErrorUnsupportedPostgres, "server-version", plan.capability, nil)
	}
	if !strings.EqualFold(proof.timezone, "UTC") || !strings.EqualFold(proof.encoding, "UTF8") ||
		proof.applicationName != configuration.Database().ApplicationName() || proof.sessionUser != plan.pool.User() {
		return newError(ErrorSessionInvalid, "session-settings", plan.capability, nil)
	}
	expectedUser := proof.sessionUser
	if plan.role != "" {
		expectedUser = plan.role
	}
	if proof.currentUser != expectedUser {
		return newError(ErrorSessionInvalid, "session-role", plan.capability, nil)
	}
	return nil
}

func verifyCompatibility(ctx context.Context, pool databasePool, capability Capability, now time.Time) (CompatibilityProof, error) {
	var proof CompatibilityProof
	err := pool.QueryRow(ctx, compatibilityProofSQL).Scan(
		&proof.contractKey, &proof.schemaVersion, &proof.minimumAdapterMajor,
		&proof.maximumAdapterMajor, &proof.migrationRevision, &proof.publishedAt,
	)
	if err != nil {
		return CompatibilityProof{}, newError(ErrorSchemaIncompatible, "schema-query", capability, err)
	}
	proof.publishedAt = proof.publishedAt.UTC()
	revision, revisionErr := strconv.ParseUint(proof.migrationRevision, 10, 64)
	if proof.contractKey != CompatibilityContractKey || proof.schemaVersion != SchemaContractVersion ||
		proof.minimumAdapterMajor > AdapterMajor || proof.maximumAdapterMajor < AdapterMajor ||
		len(proof.migrationRevision) != 12 || revisionErr != nil || revision == 0 ||
		proof.publishedAt.IsZero() || proof.publishedAt.After(now.UTC()) {
		return CompatibilityProof{}, newError(ErrorSchemaIncompatible, "schema-contract", capability, nil)
	}
	return proof, nil
}

func verifyPrivileges(ctx context.Context, pool databasePool, plan poolPlan) error {
	var proof privilegeProof
	err := pool.QueryRow(ctx, privilegeProofSQL).Scan(
		&proof.platformUsage, &proof.platformCreate, &proof.atlasUsage,
		&proof.compatibilityRead, &proof.compatibilityInsert, &proof.compatibilityUpdate,
		&proof.compatibilityDelete, &proof.repositoryInsert, &proof.repositoryDelete,
		&proof.payloadChunkRead, &proof.artifactDelete,
	)
	if err != nil {
		return newError(ErrorUnavailable, "privilege-query", plan.capability, err)
	}
	if !proof.platformUsage || proof.platformCreate || proof.atlasUsage || !proof.compatibilityRead ||
		proof.compatibilityInsert || proof.compatibilityUpdate || proof.compatibilityDelete {
		return newError(ErrorPrivilegeDenied, "privilege-common", plan.capability, nil)
	}
	valid := false
	switch plan.capability {
	case CapabilityCombined:
		valid = proof.repositoryInsert && proof.payloadChunkRead && proof.artifactDelete
	case CapabilityIngest:
		valid = proof.repositoryInsert && !proof.repositoryDelete
	case CapabilityRead:
		valid = proof.payloadChunkRead && !proof.repositoryInsert && !proof.artifactDelete
	case CapabilityRetention:
		valid = proof.artifactDelete && !proof.repositoryInsert
	}
	if !valid {
		return newError(ErrorPrivilegeDenied, "privilege-capability", plan.capability, nil)
	}
	return nil
}

func equalCompatibilityProof(left, right CompatibilityProof) bool {
	return left.contractKey == right.contractKey && left.schemaVersion == right.schemaVersion &&
		left.minimumAdapterMajor == right.minimumAdapterMajor && left.maximumAdapterMajor == right.maximumAdapterMajor &&
		left.migrationRevision == right.migrationRevision && left.publishedAt.Equal(right.publishedAt)
}

func loadTLSConfiguration(ctx context.Context, configuration runtimeconfig.TLSConfig, host string, now time.Time) (*tls.Config, error) {
	if err := ctx.Err(); err != nil {
		return nil, newError(ErrorCanceled, "tls-load", "", err)
	}
	if configuration.Mode() == runtimeconfig.TLSDisabled {
		return nil, nil
	}
	if configuration.Mode() != runtimeconfig.TLSVerifyFull {
		return nil, newError(ErrorTLSMaterial, "tls-mode", "", nil)
	}

	roots, systemErr := x509.SystemCertPool()
	if roots == nil {
		roots = x509.NewCertPool()
	}
	if systemErr != nil && configuration.RootCAPath() == "" {
		return nil, newError(ErrorTLSMaterial, "tls-system-roots", "", systemErr)
	}
	if configuration.RootCAPath() != "" {
		material, err := readTLSMaterial(ctx, configuration.RootCAPath())
		if err != nil {
			return nil, err
		}
		certificates, err := certificatesFromPEM(material)
		if err != nil {
			clear(material)
			return nil, newError(ErrorTLSMaterial, "tls-root-ca", "", err)
		}
		if !roots.AppendCertsFromPEM(material) {
			clear(material)
			return nil, newError(ErrorTLSMaterial, "tls-root-ca", "", nil)
		}
		clear(material)
		validCA := false
		for _, certificate := range certificates {
			if certificate.IsCA && !now.Before(certificate.NotBefore) && !now.After(certificate.NotAfter) {
				validCA = true
				break
			}
		}
		if !validCA {
			return nil, newError(ErrorTLSMaterial, "tls-root-validity", "", nil)
		}
	}

	serverName := configuration.ServerName()
	if serverName == "" {
		serverName = host
	}
	result := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		RootCAs:            roots,
		ServerName:         serverName,
		InsecureSkipVerify: false,
	}
	if configuration.ClientCertPath() != "" {
		certificatePEM, err := readTLSMaterial(ctx, configuration.ClientCertPath())
		if err != nil {
			return nil, err
		}
		keyPEM, err := readTLSMaterial(ctx, configuration.ClientKeyPath())
		if err != nil {
			clear(certificatePEM)
			return nil, err
		}
		if strings.Contains(string(keyPEM), "ENCRYPTED") {
			clear(certificatePEM)
			clear(keyPEM)
			return nil, newError(ErrorTLSMaterial, "tls-client-key", "", nil)
		}
		certificate, err := tls.X509KeyPair(certificatePEM, keyPEM)
		clear(certificatePEM)
		clear(keyPEM)
		if err != nil || len(certificate.Certificate) == 0 {
			return nil, newError(ErrorTLSMaterial, "tls-client-pair", "", err)
		}
		leaf, err := x509.ParseCertificate(certificate.Certificate[0])
		if err != nil || now.Before(leaf.NotBefore) || now.After(leaf.NotAfter) {
			return nil, newError(ErrorTLSMaterial, "tls-client-validity", "", err)
		}
		certificate.Leaf = leaf
		result.Certificates = []tls.Certificate{certificate}
	}
	return result, nil
}

func readTLSMaterial(ctx context.Context, path string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, newError(ErrorCanceled, "tls-read", "", err)
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maximumTLSMaterialBytes {
		return nil, newError(ErrorTLSMaterial, "tls-file", "", err)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, newError(ErrorTLSMaterial, "tls-file", "", err)
	}
	defer file.Close()
	material, err := io.ReadAll(io.LimitReader(file, maximumTLSMaterialBytes+1))
	if err != nil || len(material) == 0 || len(material) > maximumTLSMaterialBytes {
		clear(material)
		return nil, newError(ErrorTLSMaterial, "tls-file", "", err)
	}
	if err := ctx.Err(); err != nil {
		clear(material)
		return nil, newError(ErrorCanceled, "tls-read", "", err)
	}
	return material, nil
}

func certificatesFromPEM(material []byte) ([]*x509.Certificate, error) {
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(material) {
		return nil, errors.New("invalid certificate material")
	}
	var certificates []*x509.Certificate
	remaining := material
	for len(remaining) != 0 {
		block, rest := pem.Decode(remaining)
		remaining = rest
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		certificate, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		certificates = append(certificates, certificate)
	}
	if len(certificates) == 0 {
		return nil, errors.New("no certificates")
	}
	return certificates, nil
}
