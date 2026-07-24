package spike

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/AjayMunagala/software-engineering-platform/backend/lie"
	golang "github.com/AjayMunagala/software-engineering-platform/backend/lie/golang"
	"github.com/AjayMunagala/software-engineering-platform/backend/lie/golang/packageidentity"
	"github.com/AjayMunagala/software-engineering-platform/backend/lie/golang/semantic"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
	buildengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/build"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie/discovery"
	frameworkengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/framework"
	ignoreengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/ignore"
	languageengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/language"
	metadataengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/metadata"
	summaryengine "github.com/AjayMunagala/software-engineering-platform/backend/rie/summary"
	"github.com/jackc/pgx/v5"
)

const (
	manifestSchemaVersion       = "1.0.0"
	reportSchemaVersion         = "1.0.0"
	benchmarkSchema             = "persistence_spike"
	coreRepresentation          = "chunk-1m-core"
	singleByteaOOMEvidenceBytes = int64(635557163)
	absSchemaCeiling            = int64(8 << 30)
)

type languageInventoryView struct {
	Artifact        golang.Metadata         `json:"artifact"`
	SourceArtifacts []rie.ArtifactReference `json:"source_artifacts"`
	Files           []golang.GoFile         `json:"files"`
	Packages        []golang.GoPackage      `json:"packages"`
	Symbols         []golang.GoSymbol       `json:"symbols"`
	Diagnostics     []lie.Diagnostic        `json:"diagnostics"`
	Statistics      golang.ParseStatistics  `json:"statistics"`
}

type artifactValue struct {
	name    string
	version string
	codec   string
	value   any
}

func generateFixtures(ctx context.Context, input FixtureConfig) (FixtureManifest, error) {
	config, err := input.validate()
	if err != nil {
		return FixtureManifest{}, err
	}
	if err := os.MkdirAll(config.OutputDirectory, 0o700); err != nil {
		return FixtureManifest{}, fmt.Errorf("create fixture directory: %w", err)
	}

	run := rie.NewRunContext(config.RepositoryRoot, rie.DefaultConfig())
	pipeline := rie.New()
	engines := []rie.Engine{discovery.New(), ignoreengine.New(), languageengine.New()}
	if config.IncludeRIEReport {
		engines = append(engines, frameworkengine.New(), buildengine.New(), metadataengine.New(), summaryengine.New())
	}
	for _, engine := range engines {
		if err := pipeline.Register(engine); err != nil {
			return FixtureManifest{}, fmt.Errorf("register %s: %w", engine.Name(), err)
		}
	}
	if err := pipeline.Run(ctx, run); err != nil {
		return FixtureManifest{}, fmt.Errorf("run RIE: %w", err)
	}

	goEngine, err := golang.New()
	if err != nil {
		return FixtureManifest{}, fmt.Errorf("create Go engine: %w", err)
	}
	runner, err := lie.New(goEngine)
	if err != nil {
		return FixtureManifest{}, fmt.Errorf("create language runner: %w", err)
	}
	if _, err := runner.Run(ctx, run.Artifacts); err != nil {
		return FixtureManifest{}, fmt.Errorf("run Go language engine: %w", err)
	}
	snapshot, ok := rie.ArtifactAs[rie.RepositorySnapshot](run.Artifacts, rie.RepositorySnapshotArtifactName)
	if !ok {
		return FixtureManifest{}, fmt.Errorf("%w: repository snapshot missing", ErrFixtureIntegrity)
	}
	syntax, ok := golang.InventoryFrom(run.Artifacts)
	if !ok {
		return FixtureManifest{}, fmt.Errorf("%w: Go language inventory missing", ErrFixtureIntegrity)
	}
	identityEngine, err := packageidentity.New()
	if err != nil {
		return FixtureManifest{}, fmt.Errorf("create package identity engine: %w", err)
	}
	identities, err := identityEngine.Analyze(ctx, packageidentity.Input{Snapshot: snapshot, Syntax: syntax})
	if err != nil {
		return FixtureManifest{}, fmt.Errorf("analyze package identity: %w", err)
	}
	if err := run.Artifacts.Put(identities); err != nil {
		return FixtureManifest{}, fmt.Errorf("publish package identity: %w", err)
	}
	semanticConfig := semantic.DefaultConfig()
	semanticConfig.MaxWorkers = config.MaxWorkers
	integrator, err := semantic.NewIntegrator(semanticConfig)
	if err != nil {
		return FixtureManifest{}, fmt.Errorf("create semantic integrator: %w", err)
	}
	semanticInventory, err := integrator.Run(ctx, run.Artifacts)
	if err != nil {
		return FixtureManifest{}, fmt.Errorf("run semantic integrator: %w", err)
	}

	manifest := FixtureManifest{
		SchemaVersion: manifestSchemaVersion,
		Label:         config.Label, Repository: filepath.Base(config.RepositoryRoot),
		Commit: config.Commit, GeneratedAt: time.Now().UTC(),
	}
	if config.IncludeRIEReport {
		artifact, err := writeFixture(config, "rie-report", rie.SchemaVersion, "json/rie-report-v1", run.Report)
		if err != nil {
			return FixtureManifest{}, err
		}
		manifest.Artifacts = append(manifest.Artifacts, artifact)
	}
	var artifacts []artifactValue
	if !config.SemanticOnly {
		languageView := languageInventoryView{
			Artifact: syntax.Metadata(), SourceArtifacts: nonNil(syntax.SourceArtifacts()),
			Files: nonNil(syntax.Files()), Packages: nonNil(syntax.Packages()),
			Symbols: nonNil(syntax.Symbols()), Diagnostics: nonNil(syntax.Diagnostics()),
			Statistics: syntax.Statistics(),
		}
		artifacts = append(artifacts,
			artifactValue{name: golang.ArtifactName, version: golang.ArtifactVersion, codec: "json-stream/go-language-v1", value: languageView},
			artifactValue{name: packageidentity.ArtifactName, version: packageidentity.ArtifactVersion, codec: "json-stream/go-package-identity-v1", value: identities},
		)
	}
	artifacts = append(artifacts, artifactValue{
		name: semantic.ArtifactName, version: semantic.ArtifactVersion,
		codec: "json-stream/go-semantic-v1", value: semanticInventory,
	})
	for _, artifact := range artifacts {
		written, err := writeFixture(config, artifact.name, artifact.version, artifact.codec, artifact.value)
		if err != nil {
			return FixtureManifest{}, err
		}
		manifest.Artifacts = append(manifest.Artifacts, written)
	}
	sort.Slice(manifest.Artifacts, func(i, j int) bool {
		return manifest.Artifacts[i].ArtifactName < manifest.Artifacts[j].ArtifactName
	})
	manifestPath := filepath.Join(config.OutputDirectory, config.Label+".manifest.json")
	if err := writeJSONFile(manifestPath, manifest); err != nil {
		return FixtureManifest{}, err
	}
	return manifest, nil
}

func nonNil[T any](values []T) []T {
	if values == nil {
		return []T{}
	}
	return values
}

func writeFixture(config FixtureConfig, name, version, codec string, value any) (FixtureFile, error) {
	fileName := config.Label + "--" + name + ".artifact.json"
	path := filepath.Join(config.OutputDirectory, fileName)
	temporary := path + ".tmp"
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return FixtureFile{}, fmt.Errorf("create %s: %w", name, err)
	}
	hasher := sha256.New()
	writer := io.MultiWriter(file, hasher)
	encodeErr := streamArtifact(writer, value)
	closeErr := file.Close()
	if encodeErr != nil {
		_ = os.Remove(temporary)
		return FixtureFile{}, fmt.Errorf("encode %s: %w", name, encodeErr)
	}
	if closeErr != nil {
		_ = os.Remove(temporary)
		return FixtureFile{}, fmt.Errorf("close %s: %w", name, closeErr)
	}
	info, err := os.Stat(temporary)
	if err != nil {
		return FixtureFile{}, fmt.Errorf("stat %s: %w", name, err)
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return FixtureFile{}, fmt.Errorf("replace %s: %w", name, err)
	}
	if err := os.Rename(temporary, path); err != nil {
		return FixtureFile{}, fmt.Errorf("publish %s: %w", name, err)
	}
	return FixtureFile{
		Label: config.Label, Repository: filepath.Base(config.RepositoryRoot), Commit: config.Commit,
		ArtifactName: name, ArtifactVersion: version, Codec: codec,
		RelativePath: filepath.ToSlash(fileName), SizeBytes: info.Size(),
		SHA256: hex.EncodeToString(hasher.Sum(nil)),
	}, nil
}

func streamArtifact(writer io.Writer, value any) error {
	switch artifact := value.(type) {
	case languageInventoryView:
		return streamLanguageInventory(writer, artifact)
	case packageidentity.GoPackageIdentityInventory:
		return streamPackageIdentity(writer, artifact)
	case semantic.GoSemanticInventory:
		return streamSemanticInventory(writer, artifact)
	default:
		return json.NewEncoder(writer).Encode(value)
	}
}

func streamLanguageInventory(writer io.Writer, inventory languageInventoryView) error {
	fields := []streamField{
		{name: "artifact", value: func() any { return inventory.Artifact }},
		{name: "source_artifacts", value: func() any { return nonNil(inventory.SourceArtifacts) }},
		{name: "files", value: func() any { return nonNil(inventory.Files) }},
		{name: "packages", value: func() any { return nonNil(inventory.Packages) }},
		{name: "symbols", value: func() any { return nonNil(inventory.Symbols) }},
		{name: "diagnostics", value: func() any { return nonNil(inventory.Diagnostics) }},
		{name: "statistics", value: func() any { return inventory.Statistics }},
	}
	return streamObject(writer, fields)
}

func streamPackageIdentity(writer io.Writer, inventory packageidentity.GoPackageIdentityInventory) error {
	fields := []streamField{
		{name: "artifact", value: func() any { return inventory.Metadata() }},
		{name: "source_artifacts", value: func() any { return nonNil(inventory.SourceArtifacts()) }},
		{name: "contexts", value: func() any { return nonNil(inventory.Contexts()) }},
		{name: "modules", value: func() any { return nonNil(inventory.Modules()) }},
		{name: "proofs", value: func() any { return nonNil(inventory.Proofs()) }},
		{name: "diagnostics", value: func() any { return nonNil(inventory.Diagnostics()) }},
		{name: "statistics", value: func() any { return inventory.Statistics() }},
	}
	return streamObject(writer, fields)
}

func streamSemanticInventory(writer io.Writer, inventory semantic.GoSemanticInventory) error {
	fields := []streamField{
		{name: "artifact", value: func() any { return inventory.Metadata() }},
		{name: "source_artifacts", value: func() any { return nonNil(inventory.SourceArtifacts()) }},
		{name: "files", value: func() any { return nonNil(inventory.Files()) }},
		{name: "declarations", value: func() any { return nonNil(inventory.Declarations()) }},
		{name: "references", value: func() any { return nonNil(inventory.References()) }},
		{name: "receiver_bindings", value: func() any { return nonNil(inventory.ReceiverBindings()) }},
		{name: "import_bindings", value: func() any { return nonNil(inventory.ImportBindings()) }},
		{name: "type_relations", value: func() any { return nonNil(inventory.TypeRelations()) }},
		{name: "interface_satisfaction", value: func() any { return nonNil(inventory.InterfaceSatisfaction()) }},
		{name: "diagnostics", value: func() any { return nonNil(inventory.Diagnostics()) }},
		{name: "statistics", value: func() any { return inventory.Statistics() }},
	}
	return streamObject(writer, fields)
}

type streamField struct {
	name  string
	value func() any
}

func streamObject(writer io.Writer, fields []streamField) error {
	if _, err := io.WriteString(writer, "{"); err != nil {
		return err
	}
	encoder := json.NewEncoder(writer)
	for index, field := range fields {
		if index > 0 {
			if _, err := io.WriteString(writer, ","); err != nil {
				return err
			}
		}
		key, _ := json.Marshal(field.name)
		if _, err := writer.Write(key); err != nil {
			return err
		}
		if _, err := io.WriteString(writer, ":"); err != nil {
			return err
		}
		value := field.value()
		if err := encoder.Encode(value); err != nil {
			return err
		}
		value = nil
		runtime.GC()
	}
	_, err := io.WriteString(writer, "}")
	return err
}

func writeJSONFile(path string, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", filepath.Base(path), err)
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	return nil
}

type representation struct {
	name      string
	chunkSize int
}

var representations = []representation{
	{name: "single-bytea"},
	{name: "chunk-256k", chunkSize: 256 << 10},
	{name: "chunk-1m", chunkSize: 1 << 20},
	{name: "chunk-4m", chunkSize: 4 << 20},
}

func runBenchmark(ctx context.Context, input Config) (BenchmarkReport, error) {
	config, err := input.validate()
	if err != nil {
		return BenchmarkReport{}, err
	}
	connectionConfig, err := pgx.ParseConfig(config.ConnectionString)
	if err != nil {
		return BenchmarkReport{}, fmt.Errorf("parse connection: %w", err)
	}
	if !strings.HasPrefix(connectionConfig.Database, "platform_bench_") ||
		(connectionConfig.Host != "/var/run/postgresql" && connectionConfig.Host != "localhost" && connectionConfig.Host != "127.0.0.1") {
		return BenchmarkReport{}, fmt.Errorf("%w: database=%q host=%q", ErrUnsafeDatabase, connectionConfig.Database, connectionConfig.Host)
	}
	fixtures, err := loadFixtures(config.FixtureDirectory)
	if err != nil {
		return BenchmarkReport{}, err
	}
	connection, err := pgx.ConnectConfig(ctx, connectionConfig)
	if err != nil {
		return BenchmarkReport{}, fmt.Errorf("connect benchmark database: %w", err)
	}
	defer connection.Close(context.Background())
	if err := createBenchmarkSchema(ctx, connection); err != nil {
		return BenchmarkReport{}, err
	}
	environment, err := captureEnvironment(ctx, connection, config)
	if err != nil {
		return BenchmarkReport{}, err
	}
	report := BenchmarkReport{
		SchemaVersion: reportSchemaVersion, Status: "running", Environment: environment,
		Fixtures: fixtures, SelectedChunkBytes: 4 << 20,
		Notes: []string{
			"The database is disposable and peer-authenticated; no credential value was used.",
			"Fixture files were copied to the WSL ext4 filesystem before measurement.",
		},
	}
	allExact := true
	allDuplicates := true
	for _, fixture := range fixtures {
		path := filepath.Join(config.FixtureDirectory, filepath.FromSlash(fixture.RelativePath))
		for _, candidate := range representations {
			for iteration := 1; iteration <= config.Iterations; iteration++ {
				measurement, duplicateOK, err := measureRepresentation(ctx, connection, fixture, path, candidate, iteration)
				if err != nil {
					return BenchmarkReport{}, err
				}
				report.Measurements = append(report.Measurements, measurement)
				if !measurement.Supported {
					continue
				}
				allExact = allExact && measurement.DigestVerified
				allDuplicates = allDuplicates && duplicateOK
			}
		}
	}
	if _, err := connection.Exec(ctx, `TRUNCATE persistence_spike.payload_chunks, persistence_spike.payloads, persistence_spike.single_payloads CASCADE`); err != nil {
		return BenchmarkReport{}, fmt.Errorf("clear representation measurements: %w", err)
	}
	if err := stageCoreFixtures(ctx, connection, fixtures, config.FixtureDirectory); err != nil {
		return BenchmarkReport{}, err
	}
	publications, publicationState, err := measurePublications(ctx, connection, fixtures, config.Iterations, config.ConnectionString)
	if err != nil {
		return BenchmarkReport{}, err
	}
	report.Publications = publications
	correctness, plan, err := validateRelationalBehavior(ctx, connection, fixtures, publicationState, config.ConnectionString)
	if err != nil {
		return BenchmarkReport{}, err
	}
	correctness.ExactRoundTrips = allExact
	correctness.DuplicateStageIdempotent = allDuplicates
	correctness.AtomicPublicationVisible = publicationState.atomic
	report.MetadataQueryPlan = plan
	report.BackupRestore, correctness.BackupRestoreVerified, err = runBackupRestore(ctx, connectionConfig, fixtures)
	if err != nil {
		return BenchmarkReport{}, err
	}
	report.Correctness = correctness
	var largest int64
	for _, fixture := range fixtures {
		if fixture.SizeBytes > largest {
			largest = fixture.SizeBytes
		}
	}
	report.OperationalLimitBytes = operationalLimit(largest)
	if report.OperationalLimitBytes > absSchemaCeiling {
		return BenchmarkReport{}, fmt.Errorf("%w: derived operational limit exceeds schema ceiling", ErrBenchmarkIntegrity)
	}
	if !allCorrectness(correctness) {
		report.Status = "failed"
	} else {
		report.Status = "passed"
	}
	if err := writeJSONFile(config.OutputPath, report); err != nil {
		return BenchmarkReport{}, err
	}
	return report, nil
}

func loadFixtures(directory string) ([]FixtureFile, error) {
	manifestPaths, err := filepath.Glob(filepath.Join(directory, "*.manifest.json"))
	if err != nil || len(manifestPaths) == 0 {
		return nil, fmt.Errorf("%w: no fixture manifests", ErrFixtureIntegrity)
	}
	var fixtures []FixtureFile
	for _, manifestPath := range manifestPaths {
		encoded, err := os.ReadFile(manifestPath)
		if err != nil {
			return nil, fmt.Errorf("read manifest: %w", err)
		}
		var manifest FixtureManifest
		if err := json.Unmarshal(encoded, &manifest); err != nil {
			return nil, fmt.Errorf("decode manifest: %w", err)
		}
		if manifest.SchemaVersion != manifestSchemaVersion {
			return nil, fmt.Errorf("%w: manifest version %q", ErrFixtureIntegrity, manifest.SchemaVersion)
		}
		for _, fixture := range manifest.Artifacts {
			path, err := containedPath(directory, fixture.RelativePath)
			if err != nil {
				return nil, err
			}
			size, digest, err := fileIdentity(path)
			if err != nil {
				return nil, err
			}
			if size != fixture.SizeBytes || digest != fixture.SHA256 {
				return nil, fmt.Errorf("%w: %s", ErrFixtureIntegrity, fixture.RelativePath)
			}
			fixtures = append(fixtures, fixture)
		}
	}
	sort.Slice(fixtures, func(i, j int) bool {
		if fixtures[i].Label != fixtures[j].Label {
			return fixtures[i].Label < fixtures[j].Label
		}
		return fixtures[i].ArtifactName < fixtures[j].ArtifactName
	})
	return fixtures, nil
}

func containedPath(root, relative string) (string, error) {
	cleaned := filepath.Clean(filepath.FromSlash(relative))
	if filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: path escape", ErrFixtureIntegrity)
	}
	return filepath.Join(root, cleaned), nil
}

func fileIdentity(path string) (int64, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, "", fmt.Errorf("open fixture: %w", err)
	}
	defer file.Close()
	hasher := sha256.New()
	size, err := io.Copy(hasher, file)
	if err != nil {
		return 0, "", fmt.Errorf("hash fixture: %w", err)
	}
	return size, hex.EncodeToString(hasher.Sum(nil)), nil
}

func createBenchmarkSchema(ctx context.Context, connection *pgx.Conn) error {
	ddl := `
DROP SCHEMA IF EXISTS persistence_spike CASCADE;
CREATE SCHEMA persistence_spike;
CREATE TABLE persistence_spike.single_payloads (
  representation text NOT NULL,
  payload_digest bytea NOT NULL CHECK (octet_length(payload_digest)=32),
  payload_size bigint NOT NULL CHECK (payload_size>=0),
  payload_bytes bytea NOT NULL,
  PRIMARY KEY (representation,payload_digest),
  CHECK (octet_length(payload_bytes)=payload_size)
);
CREATE TABLE persistence_spike.payloads (
  representation text NOT NULL,
  payload_digest bytea NOT NULL CHECK (octet_length(payload_digest)=32),
  payload_size bigint NOT NULL CHECK (payload_size>=0 AND payload_size<=8589934592),
  chunk_size integer NOT NULL CHECK (chunk_size>0 AND chunk_size<=4194304),
  chunk_count integer NOT NULL CHECK (chunk_count>=0),
  PRIMARY KEY (representation,payload_digest),
  UNIQUE (representation,payload_digest,payload_size)
);
CREATE TABLE persistence_spike.payload_chunks (
  representation text NOT NULL,
  payload_digest bytea NOT NULL,
  chunk_ordinal integer NOT NULL CHECK (chunk_ordinal>=0),
  chunk_bytes bytea NOT NULL CHECK (octet_length(chunk_bytes)>0 AND octet_length(chunk_bytes)<=4194304),
  PRIMARY KEY (representation,payload_digest,chunk_ordinal),
  FOREIGN KEY (representation,payload_digest)
    REFERENCES persistence_spike.payloads(representation,payload_digest) ON DELETE CASCADE
);
CREATE TABLE persistence_spike.repositories (
  repository_id uuid PRIMARY KEY,
  current_scan_id uuid NULL,
  lifecycle_state text NOT NULL CHECK (lifecycle_state IN ('active','archived','purge_pending'))
);
CREATE TABLE persistence_spike.scans (
  scan_id uuid PRIMARY KEY,
  repository_id uuid NOT NULL REFERENCES persistence_spike.repositories(repository_id) ON DELETE RESTRICT,
  lifecycle_state text NOT NULL CHECK (lifecycle_state IN ('running','succeeded','failed','cancelled')),
  UNIQUE (repository_id,scan_id),
  UNIQUE (repository_id,scan_id,lifecycle_state)
);
CREATE TABLE persistence_spike.publications (
  scan_id uuid PRIMARY KEY,
  repository_id uuid NOT NULL,
  lifecycle_state text NOT NULL CHECK (lifecycle_state='succeeded'),
  artifact_set_digest bytea NOT NULL CHECK (octet_length(artifact_set_digest)=32),
  artifact_count integer NOT NULL CHECK (artifact_count>0 AND artifact_count<=256),
  published_at timestamptz NOT NULL,
  UNIQUE (repository_id,scan_id),
  FOREIGN KEY (repository_id,scan_id,lifecycle_state)
    REFERENCES persistence_spike.scans(repository_id,scan_id,lifecycle_state)
    ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED
);
ALTER TABLE persistence_spike.repositories
  ADD CONSTRAINT repositories_current_publication_fk
  FOREIGN KEY (repository_id,current_scan_id)
  REFERENCES persistence_spike.publications(repository_id,scan_id)
  ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED;
CREATE TABLE persistence_spike.artifact_envelopes (
  artifact_id uuid PRIMARY KEY,
  scan_id uuid NOT NULL REFERENCES persistence_spike.scans(scan_id) ON DELETE RESTRICT,
  artifact_name text NOT NULL,
  artifact_version text NOT NULL,
  representation text NOT NULL,
  payload_digest bytea NOT NULL,
  payload_size bigint NOT NULL,
  UNIQUE (scan_id,artifact_name),
  UNIQUE (scan_id,artifact_id),
  UNIQUE (artifact_id,payload_digest),
  FOREIGN KEY (scan_id) REFERENCES persistence_spike.publications(scan_id)
    ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED,
  FOREIGN KEY (representation,payload_digest,payload_size)
    REFERENCES persistence_spike.payloads(representation,payload_digest,payload_size) ON DELETE RESTRICT
);
CREATE TABLE persistence_spike.artifact_dependencies (
  scan_id uuid NOT NULL,
  consumer_artifact_id uuid NOT NULL,
  dependency_ordinal integer NOT NULL CHECK (dependency_ordinal>=0),
  source_artifact_id uuid NOT NULL,
  PRIMARY KEY (consumer_artifact_id,dependency_ordinal),
  FOREIGN KEY (scan_id,consumer_artifact_id)
    REFERENCES persistence_spike.artifact_envelopes(scan_id,artifact_id) ON DELETE RESTRICT,
  FOREIGN KEY (scan_id,source_artifact_id)
    REFERENCES persistence_spike.artifact_envelopes(scan_id,artifact_id) ON DELETE RESTRICT
);
CREATE TABLE persistence_spike.artifact_projections (
  projection_id uuid PRIMARY KEY,
  artifact_id uuid NOT NULL,
  representation text NOT NULL,
  source_payload_digest bytea NOT NULL,
  document jsonb NOT NULL,
  FOREIGN KEY (representation,source_payload_digest)
    REFERENCES persistence_spike.payloads(representation,payload_digest) ON DELETE RESTRICT,
  FOREIGN KEY (artifact_id,source_payload_digest)
    REFERENCES persistence_spike.artifact_envelopes(artifact_id,payload_digest) ON DELETE CASCADE
);
CREATE TABLE persistence_spike.audit_events (
  audit_event_id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  occurred_at timestamptz NOT NULL,
  event_type text NOT NULL,
  repository_id uuid NULL,
  scan_id uuid NULL,
  safe_details jsonb NOT NULL
);
CREATE INDEX scans_repository_time_idx ON persistence_spike.scans(repository_id,scan_id);
CREATE INDEX dependency_source_idx ON persistence_spike.artifact_dependencies(source_artifact_id,consumer_artifact_id);
`
	if _, err := connection.Exec(ctx, ddl); err != nil {
		return fmt.Errorf("create benchmark schema: %w", err)
	}
	return nil
}

func measureRepresentation(ctx context.Context, connection *pgx.Conn, fixture FixtureFile, path string, candidate representation, iteration int) (Measurement, bool, error) {
	if candidate.chunkSize == 0 && fixture.SizeBytes >= singleByteaOOMEvidenceBytes {
		return Measurement{
			Fixture: fixture.Label, Artifact: fixture.ArtifactName, Representation: candidate.name,
			Iteration: iteration, SizeBytes: fixture.SizeBytes, Supported: false,
			Error: "not repeated: the 635557163-byte single-bytea probe exhausted the 3.7-GiB runner and Linux terminated the PostgreSQL backend",
		}, true, nil
	}
	if candidate.chunkSize == 0 && fixture.SizeBytes > (1<<30)-8 {
		return Measurement{
			Fixture: fixture.Label, Artifact: fixture.ArtifactName, Representation: candidate.name,
			Iteration: iteration, SizeBytes: fixture.SizeBytes, Supported: false,
			Error: "exact payload exceeds PostgreSQL's one-field varlena limit",
		}, true, nil
	}
	if _, err := connection.Exec(ctx, `TRUNCATE persistence_spike.payload_chunks, persistence_spike.payloads, persistence_spike.single_payloads CASCADE`); err != nil {
		return Measurement{}, false, fmt.Errorf("reset %s: %w", candidate.name, err)
	}
	beforeLSN, err := currentLSN(ctx, connection)
	if err != nil {
		return Measurement{}, false, err
	}
	stageDuration, peakRSS, err := measured(func() error {
		return stageFixture(ctx, connection, fixture, path, candidate)
	})
	if err != nil {
		return Measurement{}, false, err
	}
	afterLSN, err := currentLSN(ctx, connection)
	if err != nil {
		return Measurement{}, false, err
	}
	walBytes, err := walDifference(ctx, connection, afterLSN, beforeLSN)
	if err != nil {
		return Measurement{}, false, err
	}
	duplicateStarted := time.Now()
	duplicateOK, err := duplicateStage(ctx, connection, fixture, candidate)
	duplicateDuration := time.Since(duplicateStarted)
	if err != nil {
		return Measurement{}, false, err
	}
	var verified bool
	readDuration, readPeakRSS, err := measured(func() error {
		var readErr error
		verified, readErr = readAndVerify(ctx, connection, fixture, candidate)
		return readErr
	})
	if err != nil {
		return Measurement{}, false, err
	}
	relationBytes, err := representationSize(ctx, connection, candidate)
	if err != nil {
		return Measurement{}, false, err
	}
	return Measurement{
		Fixture: fixture.Label, Artifact: fixture.ArtifactName, Representation: candidate.name,
		Iteration: iteration, SizeBytes: fixture.SizeBytes,
		ChunkCount:        chunkCount(fixture.SizeBytes, candidate.chunkSize),
		StageMilliseconds: milliseconds(stageDuration), ReadMilliseconds: milliseconds(readDuration),
		DuplicateMilliseconds: milliseconds(duplicateDuration),
		StageMiBPerSecond:     throughput(fixture.SizeBytes, stageDuration),
		ReadMiBPerSecond:      throughput(fixture.SizeBytes, readDuration),
		WALBytes:              walBytes, RelationBytes: relationBytes, PeakRSSBytes: peakRSS,
		ReadPeakRSSBytes: readPeakRSS, Supported: true, DigestVerified: verified,
	}, duplicateOK, nil
}

func stageFixture(ctx context.Context, connection *pgx.Conn, fixture FixtureFile, path string, candidate representation) error {
	digest, err := hex.DecodeString(fixture.SHA256)
	if err != nil || len(digest) != sha256.Size {
		return fmt.Errorf("%w: invalid fixture digest", ErrFixtureIntegrity)
	}
	transaction, err := connection.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin stage: %w", err)
	}
	defer transaction.Rollback(context.Background())
	if candidate.chunkSize == 0 {
		encoded, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read single payload: %w", err)
		}
		sum := sha256.Sum256(encoded)
		if int64(len(encoded)) != fixture.SizeBytes || hex.EncodeToString(sum[:]) != fixture.SHA256 {
			return fmt.Errorf("%w: single payload", ErrFixtureIntegrity)
		}
		if _, err := transaction.Exec(ctx, `INSERT INTO persistence_spike.single_payloads(representation,payload_digest,payload_size,payload_bytes) VALUES($1,$2,$3,$4)`, candidate.name, digest, fixture.SizeBytes, encoded); err != nil {
			return fmt.Errorf("insert single payload: %w", err)
		}
	} else {
		count := chunkCount(fixture.SizeBytes, candidate.chunkSize)
		if _, err := transaction.Exec(ctx, `INSERT INTO persistence_spike.payloads(representation,payload_digest,payload_size,chunk_size,chunk_count) VALUES($1,$2,$3,$4,$5)`, candidate.name, digest, fixture.SizeBytes, candidate.chunkSize, count); err != nil {
			return fmt.Errorf("insert payload metadata: %w", err)
		}
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open chunk payload: %w", err)
		}
		defer file.Close()
		hasher := sha256.New()
		buffer := make([]byte, candidate.chunkSize)
		var total int64
		for ordinal := 0; ; ordinal++ {
			read, readErr := io.ReadFull(file, buffer)
			if readErr != nil && readErr != io.ErrUnexpectedEOF && readErr != io.EOF {
				return fmt.Errorf("read chunk: %w", readErr)
			}
			if read > 0 {
				chunk := buffer[:read]
				total += int64(read)
				_, _ = hasher.Write(chunk)
				if _, err := transaction.Exec(ctx, `INSERT INTO persistence_spike.payload_chunks(representation,payload_digest,chunk_ordinal,chunk_bytes) VALUES($1,$2,$3,$4)`, candidate.name, digest, ordinal, chunk); err != nil {
					return fmt.Errorf("insert chunk %d: %w", ordinal, err)
				}
			}
			if readErr == io.ErrUnexpectedEOF || readErr == io.EOF {
				break
			}
		}
		if total != fixture.SizeBytes || hex.EncodeToString(hasher.Sum(nil)) != fixture.SHA256 {
			return fmt.Errorf("%w: chunk payload", ErrFixtureIntegrity)
		}
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit stage: %w", err)
	}
	return nil
}

func duplicateStage(ctx context.Context, connection *pgx.Conn, fixture FixtureFile, candidate representation) (bool, error) {
	digest, _ := hex.DecodeString(fixture.SHA256)
	var size int64
	query := `SELECT payload_size FROM persistence_spike.payloads WHERE representation=$1 AND payload_digest=$2`
	if candidate.chunkSize == 0 {
		query = `SELECT payload_size FROM persistence_spike.single_payloads WHERE representation=$1 AND payload_digest=$2`
	}
	if err := connection.QueryRow(ctx, query, candidate.name, digest).Scan(&size); err != nil {
		return false, fmt.Errorf("duplicate lookup: %w", err)
	}
	return size == fixture.SizeBytes, nil
}

func readAndVerify(ctx context.Context, connection *pgx.Conn, fixture FixtureFile, candidate representation) (bool, error) {
	digest, _ := hex.DecodeString(fixture.SHA256)
	hasher := sha256.New()
	var total int64
	if candidate.chunkSize == 0 {
		var encoded []byte
		if err := connection.QueryRow(ctx, `SELECT payload_bytes FROM persistence_spike.single_payloads WHERE representation=$1 AND payload_digest=$2`, candidate.name, digest).Scan(&encoded); err != nil {
			return false, fmt.Errorf("read single payload: %w", err)
		}
		total = int64(len(encoded))
		_, _ = hasher.Write(encoded)
	} else {
		rows, err := connection.Query(ctx, `SELECT chunk_bytes FROM persistence_spike.payload_chunks WHERE representation=$1 AND payload_digest=$2 ORDER BY chunk_ordinal`, candidate.name, digest)
		if err != nil {
			return false, fmt.Errorf("query chunks: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var chunk []byte
			if err := rows.Scan(&chunk); err != nil {
				return false, fmt.Errorf("scan chunk: %w", err)
			}
			total += int64(len(chunk))
			_, _ = hasher.Write(chunk)
		}
		if err := rows.Err(); err != nil {
			return false, fmt.Errorf("iterate chunks: %w", err)
		}
	}
	return total == fixture.SizeBytes && hex.EncodeToString(hasher.Sum(nil)) == fixture.SHA256, nil
}

func representationSize(ctx context.Context, connection *pgx.Conn, candidate representation) (int64, error) {
	var size int64
	query := `SELECT pg_total_relation_size('persistence_spike.single_payloads')`
	if candidate.chunkSize != 0 {
		query = `SELECT pg_total_relation_size('persistence_spike.payloads') + pg_total_relation_size('persistence_spike.payload_chunks')`
	}
	if err := connection.QueryRow(ctx, query).Scan(&size); err != nil {
		return 0, fmt.Errorf("measure relation size: %w", err)
	}
	return size, nil
}

func stageCoreFixtures(ctx context.Context, connection *pgx.Conn, fixtures []FixtureFile, directory string) error {
	for _, fixture := range fixtures {
		path := filepath.Join(directory, filepath.FromSlash(fixture.RelativePath))
		candidate := representation{name: coreRepresentation, chunkSize: 1 << 20}
		digest, _ := hex.DecodeString(fixture.SHA256)
		var existingSize int64
		err := connection.QueryRow(ctx, `SELECT payload_size FROM persistence_spike.payloads WHERE representation=$1 AND payload_digest=$2`, coreRepresentation, digest).Scan(&existingSize)
		if err == nil {
			if existingSize != fixture.SizeBytes {
				return fmt.Errorf("%w: core payload size conflict", ErrBenchmarkIntegrity)
			}
			continue
		}
		if err != pgx.ErrNoRows {
			return fmt.Errorf("lookup core fixture: %w", err)
		}
		if err := stageFixture(ctx, connection, fixture, path, candidate); err != nil {
			return fmt.Errorf("stage core fixture %s/%s: %w", fixture.Label, fixture.ArtifactName, err)
		}
	}
	return nil
}

type publicationState struct {
	scans     []string
	artifacts [][]string
	atomic    bool
}

func measurePublications(ctx context.Context, connection *pgx.Conn, fixtures []FixtureFile, iterations int, connectionString string) ([]PublicationMeasurement, publicationState, error) {
	reader, err := pgx.Connect(ctx, connectionString)
	if err != nil {
		return nil, publicationState{}, fmt.Errorf("connect publication reader: %w", err)
	}
	defer reader.Close(context.Background())
	var measurements []PublicationMeasurement
	state := publicationState{atomic: true}
	for iteration := 1; iteration <= iterations; iteration++ {
		beforeLSN, err := currentLSN(ctx, connection)
		if err != nil {
			return nil, publicationState{}, err
		}
		started := time.Now()
		measurement, scanID, artifactIDs, err := publishFixtureSet(ctx, connection, reader, fixtures, iteration == 1)
		measurement.DurationMS = milliseconds(time.Since(started))
		measurement.Iteration = iteration
		if err != nil {
			return nil, publicationState{}, err
		}
		afterLSN, err := currentLSN(ctx, connection)
		if err != nil {
			return nil, publicationState{}, err
		}
		measurement.WALBytes, err = walDifference(ctx, connection, afterLSN, beforeLSN)
		if err != nil {
			return nil, publicationState{}, err
		}
		state.atomic = state.atomic && !measurement.PartialVisible && measurement.CompleteVisible
		state.scans = append(state.scans, scanID)
		state.artifacts = append(state.artifacts, artifactIDs)
		measurements = append(measurements, measurement)
	}
	return measurements, state, nil
}

func publishFixtureSet(ctx context.Context, writer, reader *pgx.Conn, fixtures []FixtureFile, checkPartial bool) (PublicationMeasurement, string, []string, error) {
	repositoryID := newUUID()
	scanID := newUUID()
	transaction, err := writer.Begin(ctx)
	if err != nil {
		return PublicationMeasurement{}, "", nil, err
	}
	defer transaction.Rollback(context.Background())
	if _, err := transaction.Exec(ctx, `INSERT INTO persistence_spike.repositories(repository_id,lifecycle_state) VALUES($1,'active')`, repositoryID); err != nil {
		return PublicationMeasurement{}, "", nil, err
	}
	if _, err := transaction.Exec(ctx, `INSERT INTO persistence_spike.scans(scan_id,repository_id,lifecycle_state) VALUES($1,$2,'running')`, scanID, repositoryID); err != nil {
		return PublicationMeasurement{}, "", nil, err
	}
	if _, err := transaction.Exec(ctx, `UPDATE persistence_spike.scans SET lifecycle_state='succeeded' WHERE scan_id=$1`, scanID); err != nil {
		return PublicationMeasurement{}, "", nil, err
	}
	setHasher := sha256.New()
	for _, fixture := range fixtures {
		_, _ = io.WriteString(setHasher, fixture.ArtifactName+":"+fixture.SHA256+"\n")
	}
	if _, err := transaction.Exec(ctx, `INSERT INTO persistence_spike.publications(scan_id,repository_id,lifecycle_state,artifact_set_digest,artifact_count,published_at) VALUES($1,$2,'succeeded',$3,$4,clock_timestamp())`, scanID, repositoryID, setHasher.Sum(nil), len(fixtures)); err != nil {
		return PublicationMeasurement{}, "", nil, err
	}
	artifactIDs := make([]string, 0, len(fixtures))
	for _, fixture := range fixtures {
		artifactID := newUUID()
		artifactIDs = append(artifactIDs, artifactID)
		digest, _ := hex.DecodeString(fixture.SHA256)
		if _, err := transaction.Exec(ctx, `INSERT INTO persistence_spike.artifact_envelopes(artifact_id,scan_id,artifact_name,artifact_version,representation,payload_digest,payload_size) VALUES($1,$2,$3,$4,$5,$6,$7)`, artifactID, scanID, fixture.Label+"/"+fixture.ArtifactName, fixture.ArtifactVersion, coreRepresentation, digest, fixture.SizeBytes); err != nil {
			return PublicationMeasurement{}, "", nil, err
		}
		if len(artifactIDs) > 1 {
			if _, err := transaction.Exec(ctx, `INSERT INTO persistence_spike.artifact_dependencies(scan_id,consumer_artifact_id,dependency_ordinal,source_artifact_id) VALUES($1,$2,0,$3)`, scanID, artifactID, artifactIDs[len(artifactIDs)-2]); err != nil {
				return PublicationMeasurement{}, "", nil, err
			}
		}
	}
	firstDigest, _ := hex.DecodeString(fixtures[0].SHA256)
	if _, err := transaction.Exec(ctx, `INSERT INTO persistence_spike.artifact_projections(projection_id,artifact_id,representation,source_payload_digest,document) VALUES($1,$2,$3,$4,$5)`, newUUID(), artifactIDs[0], coreRepresentation, firstDigest, map[string]any{"artifact_count": len(fixtures)}); err != nil {
		return PublicationMeasurement{}, "", nil, err
	}
	if _, err := transaction.Exec(ctx, `UPDATE persistence_spike.repositories SET current_scan_id=$1 WHERE repository_id=$2`, scanID, repositoryID); err != nil {
		return PublicationMeasurement{}, "", nil, err
	}
	if _, err := transaction.Exec(ctx, `INSERT INTO persistence_spike.audit_events(occurred_at,event_type,repository_id,scan_id,safe_details) VALUES(clock_timestamp(),'scan-published',$1,$2,$3)`, repositoryID, scanID, map[string]any{"artifacts": len(fixtures)}); err != nil {
		return PublicationMeasurement{}, "", nil, err
	}
	partialVisible := false
	if checkPartial {
		var count int
		if err := reader.QueryRow(ctx, `SELECT count(*) FROM persistence_spike.publications WHERE scan_id=$1`, scanID).Scan(&count); err != nil {
			return PublicationMeasurement{}, "", nil, err
		}
		partialVisible = count != 0
	}
	if err := transaction.Commit(ctx); err != nil {
		return PublicationMeasurement{}, "", nil, fmt.Errorf("commit publication: %w", err)
	}
	var count int
	if err := reader.QueryRow(ctx, `SELECT count(*) FROM persistence_spike.publications WHERE scan_id=$1`, scanID).Scan(&count); err != nil {
		return PublicationMeasurement{}, "", nil, err
	}
	return PublicationMeasurement{
		Artifacts: len(fixtures), Dependencies: max(0, len(fixtures)-1),
		PartialVisible: partialVisible, CompleteVisible: count == 1,
	}, scanID, artifactIDs, nil
}

func validateRelationalBehavior(ctx context.Context, connection *pgx.Conn, fixtures []FixtureFile, state publicationState, connectionString string) (CorrectnessResults, string, error) {
	result := CorrectnessResults{}
	reader, err := pgx.Connect(ctx, connectionString)
	if err != nil {
		return result, "", err
	}
	defer reader.Close(context.Background())

	transaction, err := connection.Begin(ctx)
	if err != nil {
		return result, "", err
	}
	repoID, scanID := newUUID(), newUUID()
	if _, err = transaction.Exec(ctx, `INSERT INTO persistence_spike.repositories(repository_id,lifecycle_state) VALUES($1,'active')`, repoID); err == nil {
		_, err = transaction.Exec(ctx, `INSERT INTO persistence_spike.scans(scan_id,repository_id,lifecycle_state) VALUES($1,$2,'succeeded')`, scanID, repoID)
	}
	if err == nil {
		digest := sha256.Sum256([]byte("rollback"))
		_, err = transaction.Exec(ctx, `INSERT INTO persistence_spike.publications(scan_id,repository_id,lifecycle_state,artifact_set_digest,artifact_count,published_at) VALUES($1,$2,'succeeded',$3,1,clock_timestamp())`, scanID, repoID, digest[:])
	}
	if err != nil {
		transaction.Rollback(context.Background())
		return result, "", fmt.Errorf("prepare rollback test: %w", err)
	}
	var visible int
	if err := reader.QueryRow(ctx, `SELECT count(*) FROM persistence_spike.publications WHERE scan_id=$1`, scanID).Scan(&visible); err != nil {
		return result, "", err
	}
	if err := transaction.Rollback(ctx); err != nil {
		return result, "", err
	}
	var afterRollback int
	if err := reader.QueryRow(ctx, `SELECT count(*) FROM persistence_spike.publications WHERE scan_id=$1`, scanID).Scan(&afterRollback); err != nil {
		return result, "", err
	}
	result.RollbackInvisible = visible == 0 && afterRollback == 0

	if len(state.scans) < 2 || len(fixtures) < 2 {
		return result, "", fmt.Errorf("%w: relational validation requires two publications and fixtures", ErrBenchmarkIntegrity)
	}
	_, err = connection.Exec(ctx, `INSERT INTO persistence_spike.artifact_dependencies(scan_id,consumer_artifact_id,dependency_ordinal,source_artifact_id) VALUES($1,$2,999,$3)`, state.scans[0], state.artifacts[0][0], state.artifacts[1][0])
	result.CrossScanDependencyRejected = err != nil

	wrongDigest, _ := hex.DecodeString(fixtures[1].SHA256)
	_, err = connection.Exec(ctx, `INSERT INTO persistence_spike.artifact_projections(projection_id,artifact_id,representation,source_payload_digest,document) VALUES($1,$2,$3,$4,'{}')`, newUUID(), state.artifacts[0][0], coreRepresentation, wrongDigest)
	result.WrongProjectionDigestRejected = err != nil

	referencedDigest, _ := hex.DecodeString(fixtures[0].SHA256)
	_, err = connection.Exec(ctx, `DELETE FROM persistence_spike.payloads WHERE representation=$1 AND payload_digest=$2`, coreRepresentation, referencedDigest)
	result.ReferencedPayloadProtected = err != nil

	orphanBytes := []byte("unreferenced payload")
	orphanDigest := sha256.Sum256(orphanBytes)
	if _, err := connection.Exec(ctx, `INSERT INTO persistence_spike.payloads(representation,payload_digest,payload_size,chunk_size,chunk_count) VALUES('orphan',$1,$2,1048576,1)`, orphanDigest[:], len(orphanBytes)); err != nil {
		return result, "", fmt.Errorf("insert orphan: %w", err)
	}
	if _, err := connection.Exec(ctx, `INSERT INTO persistence_spike.payload_chunks(representation,payload_digest,chunk_ordinal,chunk_bytes) VALUES('orphan',$1,0,$2)`, orphanDigest[:], orphanBytes); err != nil {
		return result, "", fmt.Errorf("insert orphan chunk: %w", err)
	}
	if _, err := connection.Exec(ctx, `DELETE FROM persistence_spike.payloads WHERE representation='orphan' AND payload_digest=$1`, orphanDigest[:]); err != nil {
		return result, "", fmt.Errorf("delete orphan: %w", err)
	}
	var orphanChunks int
	if err := connection.QueryRow(ctx, `SELECT count(*) FROM persistence_spike.payload_chunks WHERE representation='orphan' AND payload_digest=$1`, orphanDigest[:]).Scan(&orphanChunks); err != nil {
		return result, "", err
	}
	result.UnreferencedPayloadCollected = orphanChunks == 0

	rows, err := connection.Query(ctx, `EXPLAIN (ANALYZE,BUFFERS,FORMAT TEXT) SELECT e.artifact_name,e.artifact_version FROM persistence_spike.publications p JOIN persistence_spike.artifact_envelopes e ON e.scan_id=p.scan_id WHERE p.scan_id=$1 ORDER BY e.artifact_name`, state.scans[0])
	if err != nil {
		return result, "", err
	}
	defer rows.Close()
	var planLines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return result, "", err
		}
		planLines = append(planLines, line)
	}
	plan := strings.Join(planLines, "\n")
	result.MetadataPlanAvoidedChunks = !strings.Contains(strings.ToLower(plan), "payload_chunks")
	return result, plan, nil
}

func runBackupRestore(ctx context.Context, connectionConfig *pgx.ConnConfig, fixtures []FixtureFile) (BackupRestoreResult, bool, error) {
	database := connectionConfig.Database
	if !strings.HasPrefix(database, "platform_bench_") {
		return BackupRestoreResult{}, false, ErrUnsafeDatabase
	}
	restoreDatabase := database + "_restore"
	dumpPath := filepath.Join(os.TempDir(), database+".dump")
	defer os.Remove(dumpPath)
	_ = runCommand(ctx, "dropdb", "--if-exists", "--force", restoreDatabase)
	started := time.Now()
	if output, err := commandCombined(ctx, "pg_dump", "--format=custom", "--file="+dumpPath, database); err != nil {
		return BackupRestoreResult{}, false, fmt.Errorf("pg_dump: %w: %s", err, output)
	}
	dumpDuration := time.Since(started)
	info, err := os.Stat(dumpPath)
	if err != nil {
		return BackupRestoreResult{}, false, err
	}
	started = time.Now()
	if output, err := commandCombined(ctx, "createdb", restoreDatabase); err != nil {
		return BackupRestoreResult{}, false, fmt.Errorf("createdb restore: %w: %s", err, output)
	}
	if output, err := commandCombined(ctx, "pg_restore", "--dbname="+restoreDatabase, dumpPath); err != nil {
		return BackupRestoreResult{}, false, fmt.Errorf("pg_restore: %w: %s", err, output)
	}
	restoreDuration := time.Since(started)
	restoredConfig := connectionConfig.Copy()
	restoredConfig.Database = restoreDatabase
	restored, err := pgx.ConnectConfig(ctx, restoredConfig)
	if err != nil {
		return BackupRestoreResult{}, false, err
	}
	verified := 0
	allVerified := true
	for _, fixture := range fixtures {
		ok, verifyErr := readAndVerify(ctx, restored, fixture, representation{name: coreRepresentation, chunkSize: 1 << 20})
		if verifyErr != nil || !ok {
			allVerified = false
			break
		}
		verified++
	}
	restored.Close(context.Background())
	if output, err := commandCombined(ctx, "dropdb", "--if-exists", "--force", restoreDatabase); err != nil {
		return BackupRestoreResult{}, false, fmt.Errorf("drop restore database: %w: %s", err, output)
	}
	return BackupRestoreResult{
		DumpBytes: info.Size(), DumpMS: milliseconds(dumpDuration),
		RestoreMS: milliseconds(restoreDuration), PayloadsVerified: verified,
	}, allVerified && verified == len(fixtures), nil
}

func captureEnvironment(ctx context.Context, connection *pgx.Conn, config Config) (BenchmarkEnvironment, error) {
	environment := BenchmarkEnvironment{
		RecordedAt: time.Now().UTC(), OS: commandOutput("sh", "-c", ". /etc/os-release && printf '%s' \"$PRETTY_NAME\""),
		Kernel: commandOutput("uname", "-a"), CPU: cpuModel(), LogicalCPUs: runtime.NumCPU(),
		RAM: memoryTotal(), Storage: commandOutput("lsblk", "-o", "NAME,ROTA,TYPE,SIZE,MOUNTPOINTS"),
		HostStorage: config.HostStorage, GoVersion: runtime.Version(),
		ClientBaselineRSSBytes: readRSS(),
		ClientConfiguration: map[string]string{
			"iterations": strconv.Itoa(config.Iterations), "connections": "2",
			"chunk_candidates": "single,256KiB,1MiB,4MiB", "authentication": "local peer",
		},
		PostgreSQLSettings: map[string]string{},
	}
	if err := connection.QueryRow(ctx, `SELECT version()`).Scan(&environment.PostgreSQLVersion); err != nil {
		return environment, err
	}
	for key, query := range map[string]string{
		"shared_buffers": `SHOW shared_buffers`, "max_connections": `SHOW max_connections`,
		"wal_level": `SHOW wal_level`, "synchronous_commit": `SHOW synchronous_commit`,
		"full_page_writes": `SHOW full_page_writes`, "wal_compression": `SHOW wal_compression`,
		"default_toast_compression": `SHOW default_toast_compression`, "timezone": `SHOW TimeZone`,
		"server_version": `SHOW server_version`,
	} {
		var value string
		if err := connection.QueryRow(ctx, query).Scan(&value); err != nil {
			return environment, err
		}
		environment.PostgreSQLSettings[key] = value
	}
	if err := connection.QueryRow(ctx, `SHOW server_encoding`).Scan(&environment.DatabaseEncoding); err != nil {
		return environment, err
	}
	if err := connection.QueryRow(ctx, `SHOW data_checksums`).Scan(&environment.DatabaseDataChecksums); err != nil {
		return environment, err
	}
	if err := connection.QueryRow(ctx, `SELECT datcollate FROM pg_database WHERE datname=current_database()`).Scan(&environment.DatabaseCollation); err != nil {
		return environment, err
	}
	return environment, nil
}

func currentLSN(ctx context.Context, connection *pgx.Conn) (string, error) {
	var lsn string
	if err := connection.QueryRow(ctx, `SELECT pg_current_wal_insert_lsn()::text`).Scan(&lsn); err != nil {
		return "", fmt.Errorf("read WAL LSN: %w", err)
	}
	return lsn, nil
}

func walDifference(ctx context.Context, connection *pgx.Conn, after, before string) (int64, error) {
	var bytes int64
	if err := connection.QueryRow(ctx, `SELECT pg_wal_lsn_diff($1::pg_lsn,$2::pg_lsn)::bigint`, after, before).Scan(&bytes); err != nil {
		return 0, fmt.Errorf("measure WAL: %w", err)
	}
	return bytes, nil
}

func measured(operation func() error) (time.Duration, uint64, error) {
	runtime.GC()
	debug.FreeOSMemory()
	done := make(chan struct{})
	result := make(chan uint64, 1)
	go sampleRSS(done, result)
	started := time.Now()
	err := operation()
	duration := time.Since(started)
	close(done)
	return duration, <-result, err
}

func sampleRSS(done <-chan struct{}, result chan<- uint64) {
	ticker := time.NewTicker(2 * time.Millisecond)
	defer ticker.Stop()
	var peak uint64
	for {
		if current := readRSS(); current > peak {
			peak = current
		}
		select {
		case <-done:
			result <- peak
			return
		case <-ticker.C:
		}
	}
}

func readRSS() uint64 {
	file, err := os.Open("/proc/self/status")
	if err != nil {
		return 0
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[0] == "VmRSS:" {
			value, _ := strconv.ParseUint(fields[1], 10, 64)
			return value << 10
		}
	}
	return 0
}

func cpuModel() string {
	file, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return "unavailable"
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "model name") {
			if _, value, ok := strings.Cut(line, ":"); ok {
				return strings.TrimSpace(value)
			}
		}
	}
	return "unavailable"
}

func memoryTotal() string {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return "unavailable"
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "MemTotal:") {
			return strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "MemTotal:"))
		}
	}
	return "unavailable"
}

func commandOutput(name string, arguments ...string) string {
	output, err := exec.Command(name, arguments...).CombinedOutput()
	if err != nil {
		return "unavailable: " + err.Error()
	}
	return strings.TrimSpace(string(output))
}

func commandCombined(ctx context.Context, name string, arguments ...string) (string, error) {
	output, err := exec.CommandContext(ctx, name, arguments...).CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func runCommand(ctx context.Context, name string, arguments ...string) error {
	_, err := commandCombined(ctx, name, arguments...)
	return err
}

func newUUID() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		panic(err)
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		value[0:4], value[4:6], value[6:8], value[8:10], value[10:16])
}

func chunkCount(size int64, chunkSize int) int {
	if size == 0 || chunkSize <= 0 {
		return 0
	}
	return int((size + int64(chunkSize) - 1) / int64(chunkSize))
}

func operationalLimit(largest int64) int64 {
	target := largest * 2
	minimum := int64(64 << 20)
	if target < minimum {
		target = minimum
	}
	value := int64(1)
	for value < target {
		value <<= 1
	}
	return value
}

func throughput(bytes int64, duration time.Duration) float64 {
	if duration <= 0 {
		return 0
	}
	return float64(bytes) / (1 << 20) / duration.Seconds()
}

func milliseconds(duration time.Duration) float64 {
	return float64(duration) / float64(time.Millisecond)
}

func max(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func allCorrectness(result CorrectnessResults) bool {
	return result.ExactRoundTrips && result.DuplicateStageIdempotent && result.RollbackInvisible &&
		result.AtomicPublicationVisible && result.CrossScanDependencyRejected &&
		result.WrongProjectionDigestRejected && result.ReferencedPayloadProtected &&
		result.UnreferencedPayloadCollected && result.MetadataPlanAvoidedChunks &&
		result.BackupRestoreVerified
}
