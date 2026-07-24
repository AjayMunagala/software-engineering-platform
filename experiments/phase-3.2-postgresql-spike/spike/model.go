package spike

import "time"

type FixtureManifest struct {
	SchemaVersion string        `json:"schema_version"`
	Label         string        `json:"label"`
	Repository    string        `json:"repository"`
	Commit        string        `json:"commit"`
	GeneratedAt   time.Time     `json:"generated_at"`
	Artifacts     []FixtureFile `json:"artifacts"`
}

type FixtureFile struct {
	Label           string `json:"label"`
	Repository      string `json:"repository"`
	Commit          string `json:"commit"`
	ArtifactName    string `json:"artifact_name"`
	ArtifactVersion string `json:"artifact_version"`
	Codec           string `json:"codec"`
	RelativePath    string `json:"relative_path"`
	SizeBytes       int64  `json:"size_bytes"`
	SHA256          string `json:"sha256"`
}

type BenchmarkEnvironment struct {
	RecordedAt             time.Time         `json:"recorded_at"`
	OS                     string            `json:"os"`
	Kernel                 string            `json:"kernel"`
	CPU                    string            `json:"cpu"`
	LogicalCPUs            int               `json:"logical_cpus"`
	RAM                    string            `json:"ram"`
	Storage                string            `json:"storage"`
	HostStorage            string            `json:"host_storage"`
	GoVersion              string            `json:"go_version"`
	ClientBaselineRSSBytes uint64            `json:"client_baseline_rss_bytes"`
	ClientConfiguration    map[string]string `json:"client_configuration"`
	PostgreSQLVersion      string            `json:"postgresql_version"`
	PostgreSQLSettings     map[string]string `json:"postgresql_settings"`
	DatabaseEncoding       string            `json:"database_encoding"`
	DatabaseCollation      string            `json:"database_collation"`
	DatabaseDataChecksums  string            `json:"database_data_checksums"`
}

type Measurement struct {
	Fixture               string  `json:"fixture"`
	Artifact              string  `json:"artifact"`
	Representation        string  `json:"representation"`
	Iteration             int     `json:"iteration"`
	SizeBytes             int64   `json:"size_bytes"`
	ChunkCount            int     `json:"chunk_count"`
	StageMilliseconds     float64 `json:"stage_ms"`
	ReadMilliseconds      float64 `json:"read_ms"`
	DuplicateMilliseconds float64 `json:"duplicate_ms"`
	StageMiBPerSecond     float64 `json:"stage_mib_per_second"`
	ReadMiBPerSecond      float64 `json:"read_mib_per_second"`
	WALBytes              int64   `json:"wal_bytes"`
	RelationBytes         int64   `json:"relation_bytes"`
	PeakRSSBytes          uint64  `json:"peak_rss_bytes"`
	ReadPeakRSSBytes      uint64  `json:"read_peak_rss_bytes"`
	Supported             bool    `json:"supported"`
	DigestVerified        bool    `json:"digest_verified"`
	Error                 string  `json:"error,omitempty"`
}

type PublicationMeasurement struct {
	Iteration       int     `json:"iteration"`
	Artifacts       int     `json:"artifacts"`
	Dependencies    int     `json:"dependencies"`
	DurationMS      float64 `json:"duration_ms"`
	WALBytes        int64   `json:"wal_bytes"`
	PartialVisible  bool    `json:"partial_visible"`
	CompleteVisible bool    `json:"complete_visible"`
}

type CorrectnessResults struct {
	ExactRoundTrips               bool `json:"exact_round_trips"`
	DuplicateStageIdempotent      bool `json:"duplicate_stage_idempotent"`
	RollbackInvisible             bool `json:"rollback_invisible"`
	AtomicPublicationVisible      bool `json:"atomic_publication_visible"`
	CrossScanDependencyRejected   bool `json:"cross_scan_dependency_rejected"`
	WrongProjectionDigestRejected bool `json:"wrong_projection_digest_rejected"`
	ReferencedPayloadProtected    bool `json:"referenced_payload_protected"`
	UnreferencedPayloadCollected  bool `json:"unreferenced_payload_collected"`
	MetadataPlanAvoidedChunks     bool `json:"metadata_plan_avoided_chunks"`
	BackupRestoreVerified         bool `json:"backup_restore_verified"`
}

type BackupRestoreResult struct {
	DumpBytes        int64   `json:"dump_bytes"`
	DumpMS           float64 `json:"dump_ms"`
	RestoreMS        float64 `json:"restore_ms"`
	PayloadsVerified int     `json:"payloads_verified"`
}

type BenchmarkReport struct {
	SchemaVersion         string                   `json:"schema_version"`
	Status                string                   `json:"status"`
	Environment           BenchmarkEnvironment     `json:"environment"`
	Fixtures              []FixtureFile            `json:"fixtures"`
	Measurements          []Measurement            `json:"measurements"`
	Publications          []PublicationMeasurement `json:"publications"`
	Correctness           CorrectnessResults       `json:"correctness"`
	BackupRestore         BackupRestoreResult      `json:"backup_restore"`
	MetadataQueryPlan     string                   `json:"metadata_query_plan"`
	OperationalLimitBytes int64                    `json:"operational_limit_bytes"`
	SelectedChunkBytes    int                      `json:"selected_chunk_bytes"`
	Notes                 []string                 `json:"notes"`
}
