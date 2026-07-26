package postgres

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	runtimeconfig "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/config"
	"github.com/AjayMunagala/software-engineering-platform/backend/persistence"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var fixedNow = time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)

type fixedClock struct{ now time.Time }

func (value fixedClock) Now() time.Time { return value.now }

type staticSecrets map[runtimeconfig.SecretReference][]byte

func (values staticSecrets) Resolve(ctx context.Context, reference runtimeconfig.SecretReference) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	value, found := values[reference]
	if !found {
		return nil, errors.New("secret unavailable")
	}
	return append([]byte(nil), value...), nil
}

type fakePool struct {
	user         string
	role         string
	application  string
	pingErr      error
	sessionErr   error
	compatErr    error
	privilegeErr error
	privileges   privilegeProof
	closed       bool
	closeOrder   *[]string
	mu           *sync.Mutex
}

type nilStatisticsPool struct{ *fakePool }

func (*nilStatisticsPool) Stat() *pgxpool.Stat { return nil }

func (pool *fakePool) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return nil, errors.New("not used")
}

func (pool *fakePool) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("not used")
}

func (pool *fakePool) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("not used")
}

func (pool *fakePool) QueryRow(_ context.Context, query string, _ ...any) pgx.Row {
	switch {
	case strings.Contains(query, "server_version_num"):
		return fakeRow{values: []any{180002, "UTC", "UTF8", pool.application, pool.role, pool.user}, err: pool.sessionErr}
	case strings.Contains(query, "has_schema_privilege"):
		proof := pool.privileges
		return fakeRow{values: []any{
			proof.platformUsage, proof.platformCreate, proof.atlasUsage,
			proof.compatibilityRead, proof.compatibilityInsert, proof.compatibilityUpdate,
			proof.compatibilityDelete, proof.repositoryInsert, proof.repositoryDelete,
			proof.payloadChunkRead, proof.artifactDelete,
		}, err: pool.privilegeErr}
	case strings.Contains(query, compatibilityRelation):
		return fakeRow{values: []any{CompatibilityContractKey, SchemaContractVersion, 1, 1, "202607260008", fixedNow.Add(-time.Hour)}, err: pool.compatErr}
	default:
		return fakeRow{err: errors.New("unexpected query")}
	}
}

func (pool *fakePool) Ping(context.Context) error { return pool.pingErr }

func (pool *fakePool) Close() {
	pool.closed = true
	if pool.closeOrder != nil {
		pool.mu.Lock()
		*pool.closeOrder = append(*pool.closeOrder, pool.user)
		pool.mu.Unlock()
	}
}

type fakeRow struct {
	values []any
	err    error
}

type countingPool struct {
	*fakePool
	closed *int
	mutex  *sync.Mutex
}

func (pool *countingPool) Close() {
	pool.fakePool.Close()
	pool.mutex.Lock()
	*pool.closed++
	pool.mutex.Unlock()
}

func (row fakeRow) Scan(destinations ...any) error {
	if row.err != nil {
		return row.err
	}
	if len(destinations) != len(row.values) {
		return errors.New("scan arity")
	}
	for index, destination := range destinations {
		switch target := destination.(type) {
		case *string:
			*target = row.values[index].(string)
		case *int:
			*target = row.values[index].(int)
		case *bool:
			*target = row.values[index].(bool)
		case *time.Time:
			*target = row.values[index].(time.Time)
		default:
			return errors.New("unsupported scan destination")
		}
	}
	return nil
}

func TestOpenCombinedOwnsOnePool(t *testing.T) {
	loaded := loadLocalConfiguration(t)
	var captured *pgxpool.Config
	pool := validFakePool("runtime_local", "runtime_local", false)
	value := factory{
		clock: fixedClock{fixedNow},
		newPool: func(_ context.Context, configuration *pgxpool.Config) (databasePool, error) {
			captured = configuration
			pool.application = configuration.ConnConfig.RuntimeParams["application_name"]
			return pool, nil
		},
	}

	runtime, err := value.Open(context.Background(), loaded)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if runtime.PoolCount() != 1 || runtime.Ingest() == nil || runtime.Read() == nil || runtime.Retention() == nil {
		t.Fatalf("unexpected runtime capabilities: pools=%d", runtime.PoolCount())
	}
	if _, broad := runtime.Ingest().(persistence.Port); broad {
		t.Fatal("narrow ingest capability exposed the complete persistence port")
	}
	if captured.ConnConfig.Host != "127.0.0.1" || captured.ConnConfig.User != "runtime_local" || captured.ConnConfig.TLSConfig != nil {
		t.Fatalf("pool configuration did not preserve validated local settings")
	}
	if runtime.Compatibility().SchemaVersion() != SchemaContractVersion {
		t.Fatalf("schema version = %q", runtime.Compatibility().SchemaVersion())
	}
	if err := runtime.Check(context.Background()); err != nil {
		t.Fatalf("opaque runtime Check() error = %v", err)
	}
	runtime.Close()
	runtime.Close()
	if !pool.closed {
		t.Fatal("owned pool was not closed")
	}
	if CodeOf(runtime.Check(context.Background())) != ErrorUnavailable {
		t.Fatal("closed runtime remained healthy")
	}
}

func TestOpenSeparateOwnsThreeCapabilityPools(t *testing.T) {
	loaded := loadProductionConfiguration(t)
	var configurations []*pgxpool.Config
	var pools []*fakePool
	value := factory{
		clock: fixedClock{fixedNow},
		newPool: func(_ context.Context, configuration *pgxpool.Config) (databasePool, error) {
			role := roleForUser(configuration.ConnConfig.User)
			pool := validFakePool(configuration.ConnConfig.User, role, true)
			pool.application = configuration.ConnConfig.RuntimeParams["application_name"]
			configurations = append(configurations, configuration)
			pools = append(pools, pool)
			return pool, nil
		},
	}

	runtime, err := value.Open(context.Background(), loaded)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if runtime.PoolCount() != 3 {
		t.Fatalf("PoolCount() = %d, want 3", runtime.PoolCount())
	}
	users := []string{configurations[0].ConnConfig.User, configurations[1].ConnConfig.User, configurations[2].ConnConfig.User}
	if !reflect.DeepEqual(users, []string{"runtime_ingest", "runtime_read", "runtime_retention"}) {
		t.Fatalf("construction order = %v", users)
	}
	for _, configuration := range configurations {
		if configuration.ConnConfig.TLSConfig == nil || configuration.ConnConfig.TLSConfig.InsecureSkipVerify {
			t.Fatal("verify-full TLS was not applied")
		}
		if configuration.MinConns != 0 {
			t.Fatalf("MinConns = %d, want 0", configuration.MinConns)
		}
	}
	runtime.Close()
	for _, pool := range pools {
		if !pool.closed {
			t.Fatal("capability pool was not closed")
		}
	}
}

func TestOpenClosesReadThenIngestWhenRetentionCreationFails(t *testing.T) {
	loaded := loadProductionConfiguration(t)
	var order []string
	var mutex sync.Mutex
	created := 0
	value := factory{
		clock: fixedClock{fixedNow},
		newPool: func(_ context.Context, configuration *pgxpool.Config) (databasePool, error) {
			created++
			if created == 3 {
				return nil, errors.New("retention connection contains secret text")
			}
			pool := validFakePool(configuration.ConnConfig.User, roleForUser(configuration.ConnConfig.User), true)
			pool.application = configuration.ConnConfig.RuntimeParams["application_name"]
			pool.closeOrder = &order
			pool.mu = &mutex
			return pool, nil
		},
	}

	runtime, err := value.Open(context.Background(), loaded)
	if runtime != nil || CodeOf(err) != ErrorUnavailable {
		t.Fatalf("Open() = (%v, %v)", runtime, err)
	}
	want := []string{"runtime_read", "runtime_ingest"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("cleanup order = %v, want %v", order, want)
	}
	if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "connection") {
		t.Fatalf("error leaked driver detail: %v", err)
	}
}

func TestThousandOpenCloseCyclesReleaseEveryPool(t *testing.T) {
	loaded := loadLocalConfiguration(t)
	created := 0
	closed := 0
	var mutex sync.Mutex
	value := factory{
		clock: fixedClock{fixedNow},
		newPool: func(_ context.Context, configuration *pgxpool.Config) (databasePool, error) {
			created++
			pool := validFakePool(configuration.ConnConfig.User, configuration.ConnConfig.User, false)
			pool.application = configuration.ConnConfig.RuntimeParams["application_name"]
			return &countingPool{fakePool: pool, closed: &closed, mutex: &mutex}, nil
		},
	}
	for iteration := 0; iteration < 1000; iteration++ {
		runtime, err := value.Open(context.Background(), loaded)
		if err != nil {
			t.Fatalf("Open() iteration %d: %v", iteration, err)
		}
		runtime.Close()
		runtime.Close()
	}
	if created != 1000 || closed != 1000 {
		t.Fatalf("resource accounting created=%d closed=%d", created, closed)
	}
}

func TestOpenClosesCreatedPoolWhenCompatibilityFails(t *testing.T) {
	loaded := loadLocalConfiguration(t)
	pool := validFakePool("runtime_local", "runtime_local", false)
	pool.compatErr = errors.New("relation platform.runtime_compatibility missing at host")
	value := factory{
		clock: fixedClock{fixedNow},
		newPool: func(_ context.Context, configuration *pgxpool.Config) (databasePool, error) {
			pool.application = configuration.ConnConfig.RuntimeParams["application_name"]
			return pool, nil
		},
	}

	runtime, err := value.Open(context.Background(), loaded)
	if runtime != nil || CodeOf(err) != ErrorSchemaIncompatible || !pool.closed {
		t.Fatalf("Open() = (%v, %v), closed=%v", runtime, err, pool.closed)
	}
	if strings.Contains(err.Error(), "relation") || strings.Contains(err.Error(), "host") {
		t.Fatalf("error leaked database detail: %v", err)
	}
}

func TestOpenRejectsInvalidPrivilegeBoundary(t *testing.T) {
	loaded := loadProductionConfiguration(t)
	value := factory{
		clock: fixedClock{fixedNow},
		newPool: func(_ context.Context, configuration *pgxpool.Config) (databasePool, error) {
			pool := validFakePool(configuration.ConnConfig.User, roleForUser(configuration.ConnConfig.User), true)
			pool.application = configuration.ConnConfig.RuntimeParams["application_name"]
			if configuration.ConnConfig.User == "runtime_read" {
				pool.privileges.repositoryInsert = true
			}
			return pool, nil
		},
	}

	runtime, err := value.Open(context.Background(), loaded)
	if runtime != nil || CodeOf(err) != ErrorPrivilegeDenied {
		t.Fatalf("Open() = (%v, %v)", runtime, err)
	}
}

func TestOpenHonorsCancellation(t *testing.T) {
	loaded := loadLocalConfiguration(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runtime, err := NewFactory().Open(ctx, loaded)
	if runtime != nil || !errors.Is(err, context.Canceled) || CodeOf(err) != ErrorCanceled {
		t.Fatalf("Open() = (%v, %v)", runtime, err)
	}
}

func TestLoadTLSConfigurationRejectsExpiredCAWithoutLeakingPath(t *testing.T) {
	path := writeCA(t, fixedNow.Add(-48*time.Hour), fixedNow.Add(-24*time.Hour))
	loaded := loadProductionConfigurationWithCA(t, path)
	_, err := loadTLSConfiguration(context.Background(), loaded.Config().Database().TLS(), "db.internal.example", fixedNow)
	if CodeOf(err) != ErrorTLSMaterial {
		t.Fatalf("loadTLSConfiguration() error = %v", err)
	}
	if strings.Contains(err.Error(), path) {
		t.Fatalf("error leaked certificate path: %v", err)
	}
}

func TestFactoryAndModelBoundaryCases(t *testing.T) {
	if _, err := (factory{}).Open(context.Background(), runtimeconfig.LoadedConfiguration{}); CodeOf(err) != ErrorInvalidInput {
		t.Fatalf("zero factory error = %v", err)
	}
	if _, err := poolPlans(runtimeconfig.PoolSetConfig{}); CodeOf(err) != ErrorPoolConfiguration {
		t.Fatalf("zero pool set error = %v", err)
	}
	var runtime *Runtime
	if runtime.Ingest() != nil || runtime.Read() != nil || runtime.Retention() != nil || runtime.PoolCount() != 0 ||
		runtime.Compatibility().ContractKey() != "" {
		t.Fatal("nil runtime getters are not safe")
	}
	runtime.Close()
	proof := CompatibilityProof{
		contractKey: CompatibilityContractKey, schemaVersion: SchemaContractVersion,
		minimumAdapterMajor: 1, maximumAdapterMajor: 2,
		migrationRevision: "202607260008", publishedAt: fixedNow,
	}
	if proof.ContractKey() != CompatibilityContractKey || proof.MinimumAdapterMajor() != 1 ||
		proof.MaximumAdapterMajor() != 2 || proof.MigrationRevision() != "202607260008" ||
		!proof.PublishedAt().Equal(fixedNow) {
		t.Fatal("compatibility proof accessors changed values")
	}
}

func TestOpenClassifiesPingTimeoutAndClosesPool(t *testing.T) {
	loaded := loadLocalConfiguration(t)
	pool := validFakePool("runtime_local", "runtime_local", false)
	pool.pingErr = context.DeadlineExceeded
	value := factory{
		clock:   fixedClock{fixedNow},
		newPool: func(context.Context, *pgxpool.Config) (databasePool, error) { return pool, nil },
	}
	runtime, err := value.Open(context.Background(), loaded)
	if runtime != nil || CodeOf(err) != ErrorTimeout || !errors.Is(err, context.DeadlineExceeded) || !pool.closed {
		t.Fatalf("Open() = (%v, %v), closed=%v", runtime, err, pool.closed)
	}
}

func TestDetachedPoolStatisticsAndUnsupportedProviders(t *testing.T) {
	if (*Runtime)(nil).Statistics() != nil {
		t.Fatal("nil runtime returned statistics")
	}
	runtime := &Runtime{pools: []ownedPool{
		{capability: CapabilityRead, pool: &fakePool{}},
		{capability: CapabilityIngest, pool: &nilStatisticsPool{fakePool: &fakePool{}}},
	}}
	if values := runtime.Statistics(); len(values) != 0 {
		t.Fatalf("unsupported statistics = %#v", values)
	}
	statistics := PoolStatistics{
		capability: CapabilityRetention, acquired: 1, idle: 2, constructing: 3, total: 6, maximum: 9,
		acquireCount: 10, acquireDuration: time.Second, emptyAcquireCount: 11, emptyAcquireWait: 2 * time.Second,
		connectionsCreated: 12, connectionsDestroyedIdle: 13, connectionsDestroyedLifetime: 14,
	}
	if statistics.Capability() != CapabilityRetention || statistics.Acquired() != 1 || statistics.Idle() != 2 ||
		statistics.Constructing() != 3 || statistics.Total() != 6 || statistics.Maximum() != 9 ||
		statistics.AcquireCount() != 10 || statistics.AcquireDuration() != time.Second ||
		statistics.EmptyAcquireCount() != 11 || statistics.EmptyAcquireWait() != 2*time.Second ||
		statistics.ConnectionsCreated() != 12 || statistics.ConnectionsDestroyedIdle() != 13 ||
		statistics.ConnectionsDestroyedLifetime() != 14 {
		t.Fatalf("statistics accessors changed: %#v", statistics)
	}
}

func TestSessionCompatibilityAndPrivilegeRejections(t *testing.T) {
	loaded := loadLocalConfiguration(t)
	tests := []struct {
		name string
		edit func(*fakePool)
		code ErrorCode
	}{
		{"unsupported server", func(pool *fakePool) { pool.sessionErr = errors.New("server query") }, ErrorUnavailable},
		{"invalid settings", func(pool *fakePool) { pool.application = "wrong" }, ErrorSessionInvalid},
		{"privilege query", func(pool *fakePool) { pool.privilegeErr = errors.New("permission") }, ErrorUnavailable},
		{"common privilege", func(pool *fakePool) { pool.privileges.platformCreate = true }, ErrorPrivilegeDenied},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pool := validFakePool("runtime_local", "runtime_local", false)
			pool.application = loaded.Config().Database().ApplicationName()
			test.edit(pool)
			value := factory{clock: fixedClock{fixedNow}, newPool: func(context.Context, *pgxpool.Config) (databasePool, error) {
				return pool, nil
			}}
			runtime, err := value.Open(context.Background(), loaded)
			if runtime != nil || CodeOf(err) != test.code || !pool.closed {
				t.Fatalf("Open() = (%v, %v), closed=%v", runtime, err, pool.closed)
			}
		})
	}
}

func TestTransportPolicyRejectsTLSMismatch(t *testing.T) {
	loaded := loadLocalConfiguration(t)
	configuration, err := buildPoolConfiguration(
		loaded.Config().Database(),
		poolPlan{capability: CapabilityCombined, pool: loaded.Config().Database().Pools().Ingest()},
		[]byte("temporary"), nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	if _, err := configuration.ConnConfig.AfterNetConnect(context.Background(), &configuration.ConnConfig.Config, left); err != nil {
		t.Fatalf("plain transport rejected for disabled TLS: %v", err)
	}
	tlsConnection := tls.Client(left, &tls.Config{InsecureSkipVerify: true}) // test-only unconnected wrapper
	if _, err := configuration.ConnConfig.AfterNetConnect(context.Background(), &configuration.ConnConfig.Config, tlsConnection); err == nil {
		t.Fatal("TLS transport accepted for disabled-TLS policy")
	}
}

func TestTLSMaterialBoundaryCases(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := loadTLSConfiguration(ctx, runtimeconfig.TLSConfig{}, "host", fixedNow); CodeOf(err) != ErrorCanceled {
		t.Fatalf("canceled TLS load error = %v", err)
	}
	if _, err := loadTLSConfiguration(context.Background(), runtimeconfig.TLSConfig{}, "host", fixedNow); CodeOf(err) != ErrorTLSMaterial {
		t.Fatalf("zero TLS config error = %v", err)
	}
	if cloneTLSConfiguration(nil) != nil {
		t.Fatal("nil TLS configuration was not preserved")
	}

	path := writeCA(t, fixedNow.Add(-time.Hour), fixedNow.Add(time.Hour))
	material, err := readTLSMaterial(context.Background(), path)
	if err != nil || len(material) == 0 {
		t.Fatalf("readTLSMaterial() = (%d, %v)", len(material), err)
	}
	if certificates, err := certificatesFromPEM(material); err != nil || len(certificates) != 1 {
		t.Fatalf("certificatesFromPEM() = (%d, %v)", len(certificates), err)
	}
	if _, err := certificatesFromPEM([]byte("not a certificate")); err == nil {
		t.Fatal("invalid PEM was accepted")
	}
	if _, err := readTLSMaterial(context.Background(), filepath.Join(t.TempDir(), "missing.pem")); CodeOf(err) != ErrorTLSMaterial {
		t.Fatalf("missing TLS material error = %v", err)
	}
	largePath := filepath.Join(t.TempDir(), "large.pem")
	if err := os.WriteFile(largePath, make([]byte, maximumTLSMaterialBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readTLSMaterial(context.Background(), largePath); CodeOf(err) != ErrorTLSMaterial {
		t.Fatalf("large TLS material error = %v", err)
	}
}

func TestLoadTLSConfigurationAcceptsValidClientCertificate(t *testing.T) {
	caPath, certificatePath, keyPath := writeTLSBundle(t, fixedNow.Add(-time.Hour), fixedNow.Add(time.Hour))
	loaded := loadProductionConfigurationWithTLS(t, caPath, certificatePath, keyPath)
	configuration, err := loadTLSConfiguration(
		context.Background(), loaded.Config().Database().TLS(), loaded.Config().Database().Host(), fixedNow,
	)
	if err != nil {
		t.Fatalf("loadTLSConfiguration() error = %v", err)
	}
	if configuration == nil || configuration.ServerName != "db.internal.example" ||
		configuration.MinVersion != tls.VersionTLS12 || configuration.InsecureSkipVerify ||
		len(configuration.Certificates) != 1 || configuration.Certificates[0].Leaf == nil {
		t.Fatal("valid client TLS material was not retained safely")
	}
}

func TestLoadTLSConfigurationRejectsInvalidClientMaterial(t *testing.T) {
	caPath, certificatePath, keyPath := writeTLSBundle(t, fixedNow.Add(-time.Hour), fixedNow.Add(time.Hour))
	if err := os.WriteFile(keyPath, []byte("-----BEGIN ENCRYPTED PRIVATE KEY-----\ninvalid\n-----END ENCRYPTED PRIVATE KEY-----\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded := loadProductionConfigurationWithTLS(t, caPath, certificatePath, keyPath)
	_, err := loadTLSConfiguration(context.Background(), loaded.Config().Database().TLS(), "db.internal.example", fixedNow)
	if CodeOf(err) != ErrorTLSMaterial || !strings.Contains(err.Error(), "tls-client-key") {
		t.Fatalf("encrypted client key error = %v", err)
	}

	caPath, certificatePath, keyPath = writeTLSBundle(t, fixedNow.Add(-48*time.Hour), fixedNow.Add(-24*time.Hour))
	// Replace only the CA with a currently valid trust root so the expired
	// client-leaf check is reached.
	validCA := writeCA(t, fixedNow.Add(-time.Hour), fixedNow.Add(time.Hour))
	loaded = loadProductionConfigurationWithTLS(t, validCA, certificatePath, keyPath)
	_, err = loadTLSConfiguration(context.Background(), loaded.Config().Database().TLS(), "db.internal.example", fixedNow)
	if CodeOf(err) != ErrorTLSMaterial {
		t.Fatalf("expired client certificate error = %v (original CA %s)", err, caPath)
	}
}

func TestStableErrorContract(t *testing.T) {
	if CodeOf(nil) != "" || CodeOf(errors.New("driver")) != ErrorInternal || newError("bad", "BAD STEP", "bad", nil).Error() != "postgres-runtime: internal: postgres-runtime" {
		t.Fatal("stable error normalization failed")
	}
	err := newError(ErrorUnavailable, "pool-ping", CapabilityRead, context.DeadlineExceeded)
	var failure *Error
	if !errors.As(err, &failure) || failure.Code() != ErrorTimeout || failure.Step() != "pool-ping" ||
		failure.Capability() != CapabilityRead || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("stable error accessors failed: %v", err)
	}
	var nilFailure *Error
	if nilFailure.Error() != "postgres-runtime: internal: postgres-runtime" || nilFailure.Code() != ErrorInternal ||
		nilFailure.Step() != "postgres-runtime" || nilFailure.Capability() != "" || nilFailure.Unwrap() != nil {
		t.Fatal("nil error accessors are not safe")
	}
	if newError(ErrorSchemaIncompatible, "schema", "", nil).(*Error).HealthReason() != "schema_incompatible" ||
		newError(ErrorPrivilegeDenied, "privilege", "", nil).(*Error).HealthReason() != "privilege_incompatible" ||
		newError(ErrorUnavailable, "database", "", nil).(*Error).HealthReason() != "database_unavailable" {
		t.Fatal("health reason mapping is unstable")
	}
	if CodeOf(context.Canceled) != ErrorCanceled || CodeOf(context.DeadlineExceeded) != ErrorTimeout {
		t.Fatal("context failures were not classified")
	}
	if (systemClock{}).Now().Location() != time.UTC {
		t.Fatal("system clock did not return UTC")
	}
}

func validFakePool(user, role string, _ bool) *fakePool {
	privileges := privilegeProof{
		platformUsage:     true,
		compatibilityRead: true,
	}
	switch role {
	case ingestRole:
		privileges.repositoryInsert = true
	case readRole:
		privileges.payloadChunkRead = true
	case retentionRole:
		privileges.artifactDelete = true
	default:
		privileges.repositoryInsert = true
		privileges.payloadChunkRead = true
		privileges.artifactDelete = true
	}
	return &fakePool{user: user, role: role, privileges: privileges, mu: &sync.Mutex{}}
}

func roleForUser(user string) string {
	switch user {
	case "runtime_ingest":
		return ingestRole
	case "runtime_read":
		return readRole
	case "runtime_retention":
		return retentionRole
	default:
		return user
	}
}

func loadLocalConfiguration(t testing.TB) runtimeconfig.LoadedConfiguration {
	t.Helper()
	provider := staticSecrets{runtimeconfig.SecretDatabaseCombined: []byte("local-test-password")}
	return loadConfiguration(t, []string{
		"AEGIS_PROFILE=local",
		"AEGIS_DATABASE_HOST=127.0.0.1",
		"AEGIS_DATABASE_NAME=aegis_runtime_test",
		"AEGIS_DATABASE_USER=runtime_local",
	}, provider)
}

func loadProductionConfiguration(t testing.TB) runtimeconfig.LoadedConfiguration {
	t.Helper()
	return loadProductionConfigurationWithCA(t, writeCA(t, fixedNow.Add(-time.Hour), fixedNow.Add(24*time.Hour)))
}

func loadProductionConfigurationWithCA(t testing.TB, caPath string) runtimeconfig.LoadedConfiguration {
	t.Helper()
	return loadProductionConfigurationWithTLS(t, caPath, "", "")
}

func loadProductionConfigurationWithTLS(t testing.TB, caPath, clientCertificatePath, clientKeyPath string) runtimeconfig.LoadedConfiguration {
	t.Helper()
	provider := staticSecrets{
		runtimeconfig.SecretDatabaseIngest:    []byte("ingest-test-password"),
		runtimeconfig.SecretDatabaseRead:      []byte("read-test-password"),
		runtimeconfig.SecretDatabaseRetention: []byte("retention-test-password"),
	}
	environment := []string{
		"AEGIS_PROFILE=production",
		"AEGIS_DATABASE_HOST=db.internal.example",
		"AEGIS_DATABASE_NAME=aegis_runtime_test",
		"AEGIS_DATABASE_CONNECTION_BUDGET=9",
		"AEGIS_DATABASE_INGEST_USER=runtime_ingest",
		"AEGIS_DATABASE_INGEST_MAX_CONNS=3",
		"AEGIS_DATABASE_INGEST_MIN_IDLE_CONNS=0",
		"AEGIS_DATABASE_READ_USER=runtime_read",
		"AEGIS_DATABASE_READ_MAX_CONNS=3",
		"AEGIS_DATABASE_READ_MIN_IDLE_CONNS=0",
		"AEGIS_DATABASE_RETENTION_USER=runtime_retention",
		"AEGIS_DATABASE_RETENTION_MAX_CONNS=3",
		"AEGIS_DATABASE_RETENTION_MIN_IDLE_CONNS=0",
		"AEGIS_DATABASE_TLS_MODE=verify-full",
		"AEGIS_DATABASE_TLS_ROOT_CA_FILE=" + caPath,
		"AEGIS_DATABASE_TLS_SERVER_NAME=db.internal.example",
	}
	if clientCertificatePath != "" {
		environment = append(environment,
			"AEGIS_DATABASE_TLS_CLIENT_CERT_FILE="+clientCertificatePath,
			"AEGIS_DATABASE_TLS_CLIENT_KEY_FILE="+clientKeyPath,
		)
	}
	return loadConfiguration(t, environment, provider)
}

func loadConfiguration(t testing.TB, environment []string, provider runtimeconfig.SecretProvider) runtimeconfig.LoadedConfiguration {
	t.Helper()
	request := runtimeconfig.NewLoadRequest(runtimeconfig.LoadRequestParams{Environment: environment, SecretProvider: provider})
	loaded, err := runtimeconfig.NewLoader().Load(context.Background(), request)
	if err != nil {
		t.Fatalf("configuration Load() error = %v", err)
	}
	return loaded
}

func writeCA(t testing.TB, notBefore, notAfter time.Time) string {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Aegis Runtime Test CA"},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, publicKey, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "root-ca.pem")
	material := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(path, material, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeTLSBundle(t testing.TB, notBefore, notAfter time.Time) (string, string, string) {
	t.Helper()
	caPublic, caPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(11), Subject: pkix.Name{CommonName: "Aegis Client Test CA"},
		NotBefore: notBefore, NotAfter: notAfter, IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, caPublic, caPrivate)
	if err != nil {
		t.Fatal(err)
	}
	caCertificate, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	clientPublic, clientPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	clientTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(12), Subject: pkix.Name{CommonName: "Aegis Runtime Client"},
		NotBefore: notBefore, NotAfter: notAfter, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	clientDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, caCertificate, clientPublic, caPrivate)
	if err != nil {
		t.Fatal(err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(clientPrivate)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	caPath := filepath.Join(directory, "ca.pem")
	certificatePath := filepath.Join(directory, "client.pem")
	keyPath := filepath.Join(directory, "client-key.pem")
	files := []struct {
		path       string
		blockType  string
		material   []byte
		permission os.FileMode
	}{
		{caPath, "CERTIFICATE", caDER, 0o600},
		{certificatePath, "CERTIFICATE", clientDER, 0o600},
		{keyPath, "PRIVATE KEY", privateDER, 0o600},
	}
	for _, file := range files {
		if err := os.WriteFile(file.path, pem.EncodeToMemory(&pem.Block{Type: file.blockType, Bytes: file.material}), file.permission); err != nil {
			t.Fatal(err)
		}
	}
	return caPath, certificatePath, keyPath
}
