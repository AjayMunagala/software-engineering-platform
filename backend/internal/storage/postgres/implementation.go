package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/AjayMunagala/software-engineering-platform/backend/persistence"
)

// Adapter maps the neutral persistence contract to the accepted PostgreSQL
// schema. It owns no pool lifecycle, migrations, credentials, or artifact
// serialization.
type Adapter struct {
	database Database
	config   Config
}

func New(database Database, configs ...Config) (*Adapter, error) {
	if database == nil || len(configs) > 1 {
		return nil, ErrInvalidConfig
	}
	config := DefaultConfig()
	if len(configs) == 1 {
		config = configs[0].withDefaults()
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &Adapter{database: database, config: config}, nil
}

func (adapter *Adapter) RegisterRepository(ctx context.Context, request persistence.RegisterRepositoryRequest) (persistence.RepositoryRecord, error) {
	const operation = "register-repository"
	if err := validateIDs(request.Scope().ScopeID(), string(request.RepositoryID())); err != nil {
		return persistence.RepositoryRecord{}, invalid(operation)
	}
	now := adapter.config.Clock.Now().UTC()
	digest := digestParts(request.DisplayName(), request.Source().Kind(), request.Source().FingerprintScheme(), request.Source().Fingerprint().String())
	tx, err := adapter.database.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return persistence.RepositoryRecord{}, failure(operation, err)
	}
	defer rollback(ctx, tx)
	tag, err := tx.Exec(ctx, `
		INSERT INTO platform.repositories (
			repository_id, security_scope_id, idempotency_key, registration_digest,
			display_name, source_kind, source_fingerprint_scheme, source_fingerprint,
			lifecycle_state, created_at, updated_at
		) VALUES ($1::uuid,$2::uuid,$3,$4,$5,$6,$7,$8,'active',$9,$9)
		ON CONFLICT DO NOTHING`, string(request.RepositoryID()), request.Scope().ScopeID(), string(request.RequestID()), digest,
		request.DisplayName(), request.Source().Kind(), request.Source().FingerprintScheme(), digestBytes(request.Source().Fingerprint()), now)
	if err != nil {
		return persistence.RepositoryRecord{}, failure(operation, err)
	}
	record, storedRequest, storedDigest, err := repositoryByID(ctx, tx, request.Scope().ScopeID(), request.RepositoryID())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) && tag.RowsAffected() == 0 {
			var sameScope int
			lookupErr := tx.QueryRow(ctx, `SELECT 1 FROM platform.repositories WHERE security_scope_id=$1::uuid AND source_kind=$2 AND source_fingerprint_scheme=$3 AND source_fingerprint=$4`, request.Scope().ScopeID(), request.Source().Kind(), request.Source().FingerprintScheme(), digestBytes(request.Source().Fingerprint())).Scan(&sameScope)
			if lookupErr == nil {
				return persistence.RepositoryRecord{}, persistence.NewError(persistence.ErrorIdempotencyConflict, operation, false, nil)
			}
			if errors.Is(lookupErr, pgx.ErrNoRows) {
				return persistence.RepositoryRecord{}, persistence.NewError(persistence.ErrorNotFound, operation, false, nil)
			}
			return persistence.RepositoryRecord{}, failure(operation, lookupErr)
		}
		return persistence.RepositoryRecord{}, failure(operation, err)
	}
	if storedRequest != string(request.RequestID()) || !equalBytes(storedDigest, digest) {
		return persistence.RepositoryRecord{}, persistence.NewError(persistence.ErrorIdempotencyConflict, operation, false, nil)
	}
	if tag.RowsAffected() == 1 {
		if err := audit(ctx, tx, now, operation, request.Actor(), request.RequestID(), request.Scope(), request.RepositoryID(), "", ""); err != nil {
			return persistence.RepositoryRecord{}, failure(operation, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return persistence.RepositoryRecord{}, failure(operation, err)
	}
	return record, nil
}

func (adapter *Adapter) GetRepository(ctx context.Context, query persistence.RepositoryQuery) (persistence.RepositoryRecord, error) {
	const operation = "get-repository"
	if err := validateIDs(query.Scope().ScopeID(), string(query.RepositoryID())); err != nil {
		return persistence.RepositoryRecord{}, invalid(operation)
	}
	record, _, _, err := repositoryByID(ctx, adapter.database, query.Scope().ScopeID(), query.RepositoryID())
	if err != nil {
		return persistence.RepositoryRecord{}, failure(operation, err)
	}
	return record, nil
}

func (adapter *Adapter) ListRepositories(ctx context.Context, request persistence.RepositoryListRequest) (persistence.RepositoryPage, error) {
	const operation = "list-repositories"
	if err := validateIDs(request.Scope().ScopeID()); err != nil {
		return persistence.RepositoryPage{}, invalid(operation)
	}
	offset, err := decodeCursor(request.Cursor())
	if err != nil {
		return persistence.RepositoryPage{}, invalid(operation)
	}
	rows, err := adapter.database.Query(ctx, repositorySelect+`
		WHERE security_scope_id=$1::uuid ORDER BY created_at DESC, repository_id LIMIT $2 OFFSET $3`,
		request.Scope().ScopeID(), request.PageSize()+1, offset)
	if err != nil {
		return persistence.RepositoryPage{}, failure(operation, err)
	}
	defer rows.Close()
	results := make([]persistence.RepositoryRecord, 0, request.PageSize()+1)
	for rows.Next() {
		record, _, _, scanErr := scanRepository(rows)
		if scanErr != nil {
			return persistence.RepositoryPage{}, failure(operation, scanErr)
		}
		results = append(results, record)
	}
	if err := rows.Err(); err != nil {
		return persistence.RepositoryPage{}, failure(operation, err)
	}
	next := persistence.Cursor("")
	if len(results) > request.PageSize() {
		results = results[:request.PageSize()]
		next = encodeCursor(offset + request.PageSize())
	}
	return persistence.NewRepositoryPage(results, next), nil
}

func (adapter *Adapter) ArchiveRepository(ctx context.Context, request persistence.ArchiveRepositoryRequest) (persistence.RepositoryRecord, error) {
	return adapter.changeRepositoryState(ctx, "archive-repository", request.Scope(), request.RequestID(), request.RepositoryID(), request.Actor(), persistence.RepositoryArchived)
}

func (adapter *Adapter) BeginScan(ctx context.Context, request persistence.BeginScanRequest) (persistence.ScanRecord, error) {
	const operation = "begin-scan"
	if err := validateIDs(request.Scope().ScopeID(), string(request.RepositoryID()), string(request.ScanID())); err != nil {
		return persistence.ScanRecord{}, invalid(operation)
	}
	now := adapter.config.Clock.Now().UTC()
	digest := digestParts(request.AnalysisProfileDigest().String(), request.SourceRevision())
	tx, err := adapter.database.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return persistence.ScanRecord{}, failure(operation, err)
	}
	defer rollback(ctx, tx)
	var state string
	if err := tx.QueryRow(ctx, `SELECT lifecycle_state FROM platform.repositories WHERE repository_id=$1::uuid AND security_scope_id=$2::uuid FOR UPDATE`, string(request.RepositoryID()), request.Scope().ScopeID()).Scan(&state); err != nil {
		return persistence.ScanRecord{}, failure(operation, err)
	}
	if state != string(persistence.RepositoryActive) {
		return persistence.ScanRecord{}, lifecycle(operation)
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO platform.repository_scans (
			scan_id,repository_id,idempotency_key,request_digest,analysis_profile_digest,
			source_revision,lifecycle_state,requested_at,started_at
		) VALUES ($1::uuid,$2::uuid,$3,$4,$5,NULLIF($6,''),'running',$7,$7)
		ON CONFLICT DO NOTHING`, string(request.ScanID()), string(request.RepositoryID()), string(request.RequestID()), digest,
		digestBytes(request.AnalysisProfileDigest()), request.SourceRevision(), now)
	if err != nil {
		return persistence.ScanRecord{}, failure(operation, err)
	}
	record, storedRequest, storedDigest, err := scanByID(ctx, tx, request.Scope().ScopeID(), request.RepositoryID(), request.ScanID())
	if err != nil {
		return persistence.ScanRecord{}, failure(operation, err)
	}
	if storedRequest != string(request.RequestID()) || !equalBytes(storedDigest, digest) {
		return persistence.ScanRecord{}, persistence.NewError(persistence.ErrorIdempotencyConflict, operation, false, nil)
	}
	if tag.RowsAffected() == 1 {
		if err := audit(ctx, tx, now, operation, request.Actor(), request.RequestID(), request.Scope(), request.RepositoryID(), request.ScanID(), ""); err != nil {
			return persistence.ScanRecord{}, failure(operation, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return persistence.ScanRecord{}, failure(operation, err)
	}
	return record, nil
}

func (adapter *Adapter) GetScan(ctx context.Context, query persistence.ScanQuery) (persistence.ScanRecord, error) {
	const operation = "get-scan"
	if err := validateIDs(query.Scope().ScopeID(), string(query.RepositoryID()), string(query.ScanID())); err != nil {
		return persistence.ScanRecord{}, invalid(operation)
	}
	record, _, _, err := scanByID(ctx, adapter.database, query.Scope().ScopeID(), query.RepositoryID(), query.ScanID())
	if err != nil {
		return persistence.ScanRecord{}, failure(operation, err)
	}
	return record, nil
}

func (adapter *Adapter) ListScans(ctx context.Context, request persistence.ScanListRequest) (persistence.ScanPage, error) {
	const operation = "list-scans"
	if err := validateIDs(request.Scope().ScopeID(), string(request.RepositoryID())); err != nil {
		return persistence.ScanPage{}, invalid(operation)
	}
	offset, err := decodeCursor(request.Cursor())
	if err != nil {
		return persistence.ScanPage{}, invalid(operation)
	}
	rows, err := adapter.database.Query(ctx, scanSelect+`
		WHERE r.security_scope_id=$1::uuid AND s.repository_id=$2::uuid
		ORDER BY s.requested_at DESC,s.scan_id LIMIT $3 OFFSET $4`, request.Scope().ScopeID(), string(request.RepositoryID()), request.PageSize()+1, offset)
	if err != nil {
		return persistence.ScanPage{}, failure(operation, err)
	}
	defer rows.Close()
	results := make([]persistence.ScanRecord, 0, request.PageSize()+1)
	for rows.Next() {
		record, _, _, scanErr := scanScan(rows)
		if scanErr != nil {
			return persistence.ScanPage{}, failure(operation, scanErr)
		}
		results = append(results, record)
	}
	if err := rows.Err(); err != nil {
		return persistence.ScanPage{}, failure(operation, err)
	}
	next := persistence.Cursor("")
	if len(results) > request.PageSize() {
		results = results[:request.PageSize()]
		next = encodeCursor(offset + request.PageSize())
	}
	return persistence.NewScanPage(results, next), nil
}

func (adapter *Adapter) FailScan(ctx context.Context, request persistence.FinishScanRequest) (persistence.ScanRecord, error) {
	return adapter.finishScan(ctx, "fail-scan", request, persistence.ScanFailed)
}

func (adapter *Adapter) CancelScan(ctx context.Context, request persistence.FinishScanRequest) (persistence.ScanRecord, error) {
	return adapter.finishScan(ctx, "cancel-scan", request, persistence.ScanCancelled)
}

func (adapter *Adapter) StagePayload(ctx context.Context, request persistence.StagePayloadRequest, reader io.Reader) (persistence.PayloadReceipt, error) {
	const operation = "stage-payload"
	if reader == nil || validateIDs(request.Scope().ScopeID(), string(request.RepositoryID()), string(request.ScanID())) != nil {
		return persistence.PayloadReceipt{}, invalid(operation)
	}
	tx, err := adapter.database.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return persistence.PayloadReceipt{}, failure(operation, err)
	}
	defer rollback(ctx, tx)
	var marker int
	if err := tx.QueryRow(ctx, `SELECT 1 FROM platform.repository_scans s JOIN platform.repositories r ON r.repository_id=s.repository_id WHERE r.security_scope_id=$1::uuid AND s.repository_id=$2::uuid AND s.scan_id=$3::uuid AND s.lifecycle_state='running' FOR UPDATE OF s`, request.Scope().ScopeID(), string(request.RepositoryID()), string(request.ScanID())).Scan(&marker); err != nil {
		return persistence.PayloadReceipt{}, failure(operation, err)
	}
	expectedChunks := chunkCount(request.ExpectedSize())
	created := false
	if err := tx.QueryRow(ctx, `INSERT INTO platform.artifact_payloads(payload_digest,payload_size,chunk_size,chunk_count,created_at) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING RETURNING true`, digestBytes(request.Digest()), int64(request.ExpectedSize()), ChunkSize, expectedChunks, adapter.config.Clock.Now().UTC()).Scan(&created); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return persistence.PayloadReceipt{}, failure(operation, err)
	}
	hash := sha256.New()
	buffer := make([]byte, ChunkSize)
	var total uint64
	ordinal := 0
	for {
		if err := ctx.Err(); err != nil {
			return persistence.PayloadReceipt{}, failure(operation, err)
		}
		count, readErr := io.ReadFull(reader, buffer)
		if count > 0 {
			_, _ = hash.Write(buffer[:count])
			total += uint64(count)
			if total > uint64(request.ExpectedSize()) {
				return persistence.PayloadReceipt{}, integrity(operation)
			}
			if created {
				if _, err := tx.Exec(ctx, `INSERT INTO platform.artifact_payload_chunks(payload_digest,chunk_ordinal,chunk_bytes) VALUES($1,$2,$3)`, digestBytes(request.Digest()), ordinal, buffer[:count]); err != nil {
					return persistence.PayloadReceipt{}, failure(operation, err)
				}
			}
			ordinal++
		}
		if errors.Is(readErr, io.EOF) || errors.Is(readErr, io.ErrUnexpectedEOF) {
			break
		}
		if readErr != nil {
			return persistence.PayloadReceipt{}, persistence.NewError(persistence.ErrorIntegrityFailure, operation, false, readErr)
		}
	}
	var actual persistence.Digest
	copy(actual[:], hash.Sum(nil))
	if total != uint64(request.ExpectedSize()) || actual != request.Digest() || ordinal != expectedChunks {
		return persistence.PayloadReceipt{}, integrity(operation)
	}
	if !created {
		var storedSize int64
		var storedChunks int
		if err := tx.QueryRow(ctx, `SELECT payload_size,chunk_count FROM platform.artifact_payloads WHERE payload_digest=$1`, digestBytes(request.Digest())).Scan(&storedSize, &storedChunks); err != nil {
			return persistence.PayloadReceipt{}, failure(operation, err)
		}
		if storedSize != int64(total) || storedChunks != ordinal {
			return persistence.PayloadReceipt{}, integrity(operation)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return persistence.PayloadReceipt{}, failure(operation, err)
	}
	disposition := persistence.DispositionAlreadyPresent
	if created {
		disposition = persistence.DispositionCreated
	}
	receipt, err := persistence.NewPayloadReceipt(request.Digest(), persistence.ByteCount(total), disposition)
	if err != nil {
		return persistence.PayloadReceipt{}, failure(operation, err)
	}
	return receipt, nil
}

func (adapter *Adapter) PublishScan(ctx context.Context, request persistence.PublishScanRequest) (persistence.PublicationReceipt, error) {
	const operation = "publish-scan"
	if validateIDs(request.Scope().ScopeID(), string(request.RepositoryID()), string(request.ScanID())) != nil {
		return persistence.PublicationReceipt{}, invalid(operation)
	}
	tx, err := adapter.database.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return persistence.PublicationReceipt{}, failure(operation, err)
	}
	defer rollback(ctx, tx)
	if _, err := tx.Exec(ctx, `SET CONSTRAINTS ALL DEFERRED`); err != nil {
		return persistence.PublicationReceipt{}, failure(operation, err)
	}
	var state string
	if err := tx.QueryRow(ctx, `SELECT s.lifecycle_state FROM platform.repository_scans s JOIN platform.repositories r ON r.repository_id=s.repository_id WHERE r.security_scope_id=$1::uuid AND s.repository_id=$2::uuid AND s.scan_id=$3::uuid FOR UPDATE OF s,r`, request.Scope().ScopeID(), string(request.RepositoryID()), string(request.ScanID())).Scan(&state); err != nil {
		return persistence.PublicationReceipt{}, failure(operation, err)
	}
	if state == string(persistence.ScanSucceeded) {
		var scheme string
		var digest []byte
		var count uint32
		if err := tx.QueryRow(ctx, `SELECT manifest_scheme,artifact_set_digest,artifact_count FROM platform.scan_publications WHERE scan_id=$1::uuid`, string(request.ScanID())).Scan(&scheme, &digest, &count); err != nil {
			return persistence.PublicationReceipt{}, failure(operation, err)
		}
		if scheme != request.ManifestScheme() || !equalBytes(digest, digestBytes(request.ManifestDigest())) || count != uint32(len(request.Artifacts())) {
			return persistence.PublicationReceipt{}, persistence.NewError(persistence.ErrorIdempotencyConflict, operation, false, nil)
		}
		return persistence.NewPublicationReceipt(request.ScanID(), scheme, request.ManifestDigest(), count, persistence.DispositionAlreadyPresent)
	}
	if state != string(persistence.ScanRunning) {
		return persistence.PublicationReceipt{}, lifecycle(operation)
	}
	now := adapter.config.Clock.Now().UTC()
	if _, err := tx.Exec(ctx, `UPDATE platform.repository_scans SET lifecycle_state='succeeded',finished_at=$1,published_at=$1 WHERE scan_id=$2::uuid`, now, string(request.ScanID())); err != nil {
		return persistence.PublicationReceipt{}, failure(operation, err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO platform.scan_publications(scan_id,repository_id,lifecycle_state,manifest_scheme,artifact_set_digest,artifact_count,published_at) VALUES($1::uuid,$2::uuid,'succeeded',$3,$4,$5,$6)`, string(request.ScanID()), string(request.RepositoryID()), request.ManifestScheme(), digestBytes(request.ManifestDigest()), len(request.Artifacts()), now); err != nil {
		return persistence.PublicationReceipt{}, failure(operation, err)
	}
	for _, artifact := range request.Artifacts() {
		if validateIDs(string(artifact.ArtifactID())) != nil {
			return persistence.PublicationReceipt{}, invalid(operation)
		}
		var size int64
		if err := tx.QueryRow(ctx, `SELECT payload_size FROM platform.artifact_payloads WHERE payload_digest=$1`, digestBytes(artifact.PayloadDigest())).Scan(&size); err != nil {
			return persistence.PublicationReceipt{}, failure(operation, err)
		}
		if size != int64(artifact.PayloadSize()) {
			return persistence.PublicationReceipt{}, integrity(operation)
		}
		stable := any(nil)
		if artifact.StableIDScheme() != "" {
			stable = artifact.StableIDScheme()
		}
		if _, err := tx.Exec(ctx, `INSERT INTO platform.artifact_envelopes(artifact_id,scan_id,artifact_name,artifact_version,stable_id_scheme,codec_name,codec_version,media_type,producer_name,producer_version,payload_digest,payload_size,created_at) VALUES($1::uuid,$2::uuid,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, string(artifact.ArtifactID()), string(request.ScanID()), artifact.Artifact().Name(), artifact.Artifact().Version(), stable, artifact.Codec().Name(), artifact.Codec().Version(), artifact.Codec().MediaType(), artifact.Producer().Name(), artifact.Producer().Version(), digestBytes(artifact.PayloadDigest()), int64(artifact.PayloadSize()), now); err != nil {
			return persistence.PublicationReceipt{}, failure(operation, err)
		}
	}
	for _, dependency := range request.Dependencies() {
		if _, err := tx.Exec(ctx, `INSERT INTO platform.artifact_dependencies(scan_id,consumer_artifact_id,dependency_ordinal,source_artifact_id,declared_name,declared_version) VALUES($1::uuid,$2::uuid,$3,$4::uuid,$5,$6)`, string(request.ScanID()), string(dependency.ConsumerArtifactID()), dependency.Ordinal(), string(dependency.SourceArtifactID()), dependency.DeclaredArtifact().Name(), dependency.DeclaredArtifact().Version()); err != nil {
			return persistence.PublicationReceipt{}, failure(operation, err)
		}
	}
	for _, projection := range request.Projections() {
		if validateIDs(string(projection.ProjectionID())) != nil {
			return persistence.PublicationReceipt{}, invalid(operation)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO platform.artifact_projections(projection_id,artifact_id,source_payload_digest,projector_name,projector_version,projection_schema_version,projection_digest_scheme,projection_digest,document,document_size,record_count,created_at) VALUES($1::uuid,$2::uuid,$3,$4,$5,$6,$7,$8,$9::jsonb,$10,$11,$12)`, string(projection.ProjectionID()), string(projection.ArtifactID()), digestBytes(projection.SourceDigest()), projection.Projector().Name(), projection.Projector().Version(), projection.SchemaVersion(), projection.DigestScheme(), digestBytes(projection.ProjectionDigest()), string(projection.CanonicalJSON()), len(projection.CanonicalJSON()), projection.RecordCount(), now); err != nil {
			return persistence.PublicationReceipt{}, failure(operation, err)
		}
	}
	for _, diagnostic := range request.Diagnostics() {
		var path, line, column any
		if diagnostic.RelativePath() != "" {
			path = diagnostic.RelativePath()
		}
		if diagnostic.Line() != 0 {
			line = diagnostic.Line()
		}
		if diagnostic.Column() != 0 {
			column = diagnostic.Column()
		}
		if _, err := tx.Exec(ctx, `INSERT INTO platform.projected_diagnostics(projection_id,diagnostic_ordinal,severity,code,engine_name,relative_path,line_number,column_number,message) VALUES($1::uuid,$2,$3,$4,$5,$6,$7,$8,$9)`, string(diagnostic.ProjectionID()), diagnostic.Ordinal(), diagnostic.Severity(), diagnostic.Code(), diagnostic.Engine(), path, line, column, diagnostic.Message()); err != nil {
			return persistence.PublicationReceipt{}, failure(operation, err)
		}
	}
	for _, statistic := range request.Statistics() {
		integer, decimal, boolean, text, unit := statisticColumns(statistic)
		if _, err := tx.Exec(ctx, `INSERT INTO platform.projected_statistics(projection_id,metric_key,value_kind,integer_value,decimal_value,boolean_value,text_value,unit) VALUES($1::uuid,$2,$3,$4,$5::numeric,$6,$7,$8)`, string(statistic.ProjectionID()), statistic.Key(), string(statistic.Value().Kind()), integer, decimal, boolean, text, unit); err != nil {
			return persistence.PublicationReceipt{}, failure(operation, err)
		}
	}
	if request.MakeCurrent() {
		if _, err := tx.Exec(ctx, `UPDATE platform.repositories SET current_scan_id=$1::uuid,updated_at=$2 WHERE repository_id=$3::uuid`, string(request.ScanID()), now, string(request.RepositoryID())); err != nil {
			return persistence.PublicationReceipt{}, failure(operation, err)
		}
	}
	if err := audit(ctx, tx, now, operation, request.Actor(), request.RequestID(), request.Scope(), request.RepositoryID(), request.ScanID(), ""); err != nil {
		return persistence.PublicationReceipt{}, failure(operation, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return persistence.PublicationReceipt{}, failure(operation, err)
	}
	return persistence.NewPublicationReceipt(request.ScanID(), request.ManifestScheme(), request.ManifestDigest(), uint32(len(request.Artifacts())), persistence.DispositionCreated)
}

func (adapter *Adapter) GetArtifact(ctx context.Context, query persistence.ArtifactQuery) (persistence.ArtifactRecord, error) {
	const operation = "get-artifact"
	if validateIDs(query.Scope().ScopeID(), string(query.RepositoryID()), string(query.ScanID()), string(query.ArtifactID())) != nil {
		return persistence.ArtifactRecord{}, invalid(operation)
	}
	record, err := artifactByID(ctx, adapter.database, query.Scope().ScopeID(), query.RepositoryID(), query.ScanID(), query.ArtifactID())
	if err != nil {
		return persistence.ArtifactRecord{}, failure(operation, err)
	}
	return record, nil
}

func (adapter *Adapter) ListArtifacts(ctx context.Context, request persistence.ArtifactListRequest) (persistence.ArtifactPage, error) {
	const operation = "list-artifacts"
	if validateIDs(request.Scope().ScopeID(), string(request.RepositoryID()), string(request.ScanID())) != nil {
		return persistence.ArtifactPage{}, invalid(operation)
	}
	offset, err := decodeCursor(request.Cursor())
	if err != nil {
		return persistence.ArtifactPage{}, invalid(operation)
	}
	rows, err := adapter.database.Query(ctx, artifactSelect+` WHERE r.security_scope_id=$1::uuid AND r.repository_id=$2::uuid AND e.scan_id=$3::uuid ORDER BY e.artifact_name,e.artifact_id LIMIT $4 OFFSET $5`, request.Scope().ScopeID(), string(request.RepositoryID()), string(request.ScanID()), request.PageSize()+1, offset)
	if err != nil {
		return persistence.ArtifactPage{}, failure(operation, err)
	}
	defer rows.Close()
	results := make([]persistence.ArtifactRecord, 0, request.PageSize()+1)
	for rows.Next() {
		record, scanErr := scanArtifact(rows)
		if scanErr != nil {
			return persistence.ArtifactPage{}, failure(operation, scanErr)
		}
		results = append(results, record)
	}
	if err := rows.Err(); err != nil {
		return persistence.ArtifactPage{}, failure(operation, err)
	}
	next := persistence.Cursor("")
	if len(results) > request.PageSize() {
		results = results[:request.PageSize()]
		next = encodeCursor(offset + request.PageSize())
	}
	return persistence.NewArtifactPage(results, next), nil
}

func (adapter *Adapter) ExportPayload(ctx context.Context, query persistence.PayloadQuery, writer io.Writer) (persistence.PayloadReceipt, error) {
	const operation = "export-payload"
	if writer == nil || validateIDs(query.Scope().ScopeID(), string(query.RepositoryID()), string(query.ScanID()), string(query.ArtifactID())) != nil {
		return persistence.PayloadReceipt{}, invalid(operation)
	}
	digest, size, err := adapter.streamPayload(ctx, query, writer)
	if err != nil {
		return persistence.PayloadReceipt{}, err
	}
	return persistence.NewPayloadReceipt(digest, size, persistence.DispositionAlreadyPresent)
}

func (adapter *Adapter) VerifyPayload(ctx context.Context, query persistence.PayloadQuery) (persistence.VerificationReceipt, error) {
	digest, size, err := adapter.streamPayload(ctx, query, io.Discard)
	if err != nil {
		return persistence.VerificationReceipt{}, err
	}
	return persistence.NewVerificationReceipt(digest, size)
}

func (adapter *Adapter) MarkRepositoryForPurge(ctx context.Context, request persistence.MarkForPurgeRequest) (persistence.RepositoryRecord, error) {
	return adapter.changeRepositoryState(ctx, "mark-for-purge", request.Scope(), request.RequestID(), request.RepositoryID(), request.Actor(), persistence.RepositoryPurgePending)
}

func (adapter *Adapter) PurgeRepositoryBatch(ctx context.Context, request persistence.PurgeBatchRequest) (persistence.PurgeReceipt, error) {
	const operation = "purge-repository-batch"
	if validateIDs(request.Scope().ScopeID(), string(request.RepositoryID())) != nil {
		return persistence.PurgeReceipt{}, invalid(operation)
	}
	tx, err := adapter.database.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return persistence.PurgeReceipt{}, failure(operation, err)
	}
	defer rollback(ctx, tx)
	var state string
	if err := tx.QueryRow(ctx, `SELECT lifecycle_state FROM platform.repositories WHERE security_scope_id=$1::uuid AND repository_id=$2::uuid FOR UPDATE`, request.Scope().ScopeID(), string(request.RepositoryID())).Scan(&state); err != nil {
		return persistence.PurgeReceipt{}, failure(operation, err)
	}
	if state != string(persistence.RepositoryPurgePending) {
		return persistence.PurgeReceipt{}, lifecycle(operation)
	}
	if _, err := tx.Exec(ctx, `UPDATE platform.repositories SET current_scan_id=NULL,updated_at=$1 WHERE repository_id=$2::uuid`, adapter.config.Clock.Now().UTC(), string(request.RepositoryID())); err != nil {
		return persistence.PurgeReceipt{}, failure(operation, err)
	}
	rows, err := tx.Query(ctx, `SELECT scan_id::text FROM platform.repository_scans WHERE repository_id=$1::uuid ORDER BY requested_at LIMIT $2 FOR UPDATE`, string(request.RepositoryID()), request.Limit())
	if err != nil {
		return persistence.PurgeReceipt{}, failure(operation, err)
	}
	var scans []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return persistence.PurgeReceipt{}, failure(operation, err)
		}
		scans = append(scans, id)
	}
	rows.Close()
	var artifacts uint64
	for _, scanID := range scans {
		var artifactCount int64
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM platform.artifact_envelopes WHERE scan_id=$1::uuid`, scanID).Scan(&artifactCount); err != nil {
			return persistence.PurgeReceipt{}, failure(operation, err)
		}
		artifacts += uint64(artifactCount)
		statements := []string{
			`DELETE FROM platform.artifact_dependencies WHERE scan_id=$1::uuid`,
			`DELETE FROM platform.artifact_projections WHERE artifact_id IN (SELECT artifact_id FROM platform.artifact_envelopes WHERE scan_id=$1::uuid)`,
			`DELETE FROM platform.artifact_envelopes WHERE scan_id=$1::uuid`,
			`DELETE FROM platform.scan_publications WHERE scan_id=$1::uuid`,
			`DELETE FROM platform.repository_scans WHERE scan_id=$1::uuid`,
		}
		for _, statement := range statements {
			if _, err := tx.Exec(ctx, statement, scanID); err != nil {
				return persistence.PurgeReceipt{}, failure(operation, err)
			}
		}
	}
	var remaining int64
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM platform.repository_scans WHERE repository_id=$1::uuid`, string(request.RepositoryID())).Scan(&remaining); err != nil {
		return persistence.PurgeReceipt{}, failure(operation, err)
	}
	complete := remaining == 0
	if complete {
		if _, err := tx.Exec(ctx, `DELETE FROM platform.repositories WHERE repository_id=$1::uuid`, string(request.RepositoryID())); err != nil {
			return persistence.PurgeReceipt{}, failure(operation, err)
		}
	}
	if err := audit(ctx, tx, adapter.config.Clock.Now().UTC(), operation, request.Actor(), request.RequestID(), request.Scope(), request.RepositoryID(), "", ""); err != nil {
		return persistence.PurgeReceipt{}, failure(operation, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return persistence.PurgeReceipt{}, failure(operation, err)
	}
	return persistence.NewPurgeReceipt(artifacts, uint64(len(scans)), complete), nil
}

func (adapter *Adapter) GarbageCollectPayloads(ctx context.Context, request persistence.GarbageCollectionRequest) (persistence.GarbageCollectionReceipt, error) {
	const operation = "garbage-collect-payloads"
	if validateIDs(request.Scope().ScopeID()) != nil {
		return persistence.GarbageCollectionReceipt{}, invalid(operation)
	}
	tx, err := adapter.database.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return persistence.GarbageCollectionReceipt{}, failure(operation, err)
	}
	defer rollback(ctx, tx)
	var count uint64
	var bytes uint64
	rows, err := tx.Query(ctx, `WITH candidates AS (SELECT p.payload_digest,p.payload_size FROM platform.artifact_payloads p WHERE p.created_at < now()-interval '24 hours' AND NOT EXISTS(SELECT 1 FROM platform.artifact_envelopes e WHERE e.payload_digest=p.payload_digest) ORDER BY p.created_at LIMIT $1 FOR UPDATE SKIP LOCKED), deleted AS (DELETE FROM platform.artifact_payloads p USING candidates c WHERE p.payload_digest=c.payload_digest RETURNING c.payload_size) SELECT count(*),COALESCE(sum(payload_size),0) FROM deleted`, request.Limit())
	if err != nil {
		return persistence.GarbageCollectionReceipt{}, failure(operation, err)
	}
	if rows.Next() {
		if err := rows.Scan(&count, &bytes); err != nil {
			rows.Close()
			return persistence.GarbageCollectionReceipt{}, failure(operation, err)
		}
	}
	rows.Close()
	if err := audit(ctx, tx, adapter.config.Clock.Now().UTC(), operation, request.Actor(), request.RequestID(), request.Scope(), "", "", ""); err != nil {
		return persistence.GarbageCollectionReceipt{}, failure(operation, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return persistence.GarbageCollectionReceipt{}, failure(operation, err)
	}
	return persistence.NewGarbageCollectionReceipt(count, persistence.ByteCount(bytes)), nil
}

// --- internal lifecycle and row mapping ---

type rowScanner interface{ Scan(...any) error }

const repositorySelect = `SELECT security_scope_id::text,repository_id::text,idempotency_key,registration_digest,display_name,source_kind,source_fingerprint_scheme,source_fingerprint,lifecycle_state,current_scan_id,created_at,updated_at FROM platform.repositories `
const scanSelect = `SELECT r.security_scope_id::text,s.repository_id::text,s.scan_id::text,s.idempotency_key,s.request_digest,s.analysis_profile_digest,s.source_revision,s.lifecycle_state,s.failure_code,s.failure_summary,s.requested_at,s.started_at,s.finished_at FROM platform.repository_scans s JOIN platform.repositories r ON r.repository_id=s.repository_id `
const artifactSelect = `SELECT r.security_scope_id::text,r.repository_id::text,e.scan_id::text,e.artifact_id::text,e.artifact_name,e.artifact_version,e.stable_id_scheme,e.codec_name,e.codec_version,e.media_type,e.producer_name,e.producer_version,e.payload_digest,e.payload_size,e.created_at FROM platform.artifact_envelopes e JOIN platform.repository_scans s ON s.scan_id=e.scan_id JOIN platform.repositories r ON r.repository_id=s.repository_id `

func repositoryByID(ctx context.Context, query interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, scope string, repositoryID persistence.RepositoryID) (persistence.RepositoryRecord, string, []byte, error) {
	return scanRepository(query.QueryRow(ctx, repositorySelect+`WHERE security_scope_id=$1::uuid AND repository_id=$2::uuid`, scope, string(repositoryID)))
}

func scanRepository(row rowScanner) (persistence.RepositoryRecord, string, []byte, error) {
	var scope, repository, idempotency, display, kind, scheme, state string
	var registration, source []byte
	var current pgtype.UUID
	var created, updated time.Time
	if err := row.Scan(&scope, &repository, &idempotency, &registration, &display, &kind, &scheme, &source, &state, &current, &created, &updated); err != nil {
		return persistence.RepositoryRecord{}, "", nil, err
	}
	digest, err := parseDigest(source)
	if err != nil {
		return persistence.RepositoryRecord{}, "", nil, err
	}
	identity, err := persistence.NewSourceIdentity(kind, scheme, digest)
	if err != nil {
		return persistence.RepositoryRecord{}, "", nil, err
	}
	currentID := ""
	if current.Valid {
		currentID = uuidText(current)
	}
	record, err := persistence.NewRepositoryRecord(scope, persistence.RepositoryID(repository), display, identity, persistence.RepositoryState(state), persistence.ScanID(currentID), created, updated)
	return record, idempotency, registration, err
}

func scanByID(ctx context.Context, query interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, scope string, repositoryID persistence.RepositoryID, scanID persistence.ScanID) (persistence.ScanRecord, string, []byte, error) {
	return scanScan(query.QueryRow(ctx, scanSelect+`WHERE r.security_scope_id=$1::uuid AND s.repository_id=$2::uuid AND s.scan_id=$3::uuid`, scope, string(repositoryID), string(scanID)))
}

func scanScan(row rowScanner) (persistence.ScanRecord, string, []byte, error) {
	var scope, repository, scan, idempotency, state string
	var requestDigest, profile []byte
	var revision, reason, message sql.NullString
	var requested time.Time
	var started, finished sql.NullTime
	if err := row.Scan(&scope, &repository, &scan, &idempotency, &requestDigest, &profile, &revision, &state, &reason, &message, &requested, &started, &finished); err != nil {
		return persistence.ScanRecord{}, "", nil, err
	}
	digest, err := parseDigest(profile)
	if err != nil {
		return persistence.ScanRecord{}, "", nil, err
	}
	record, err := persistence.NewScanRecord(scope, persistence.RepositoryID(repository), persistence.ScanID(scan), digest, revision.String, persistence.ScanState(state), reason.String, message.String, requested, started.Time, finished.Time)
	return record, idempotency, requestDigest, err
}

func artifactByID(ctx context.Context, query interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, scope string, repositoryID persistence.RepositoryID, scanID persistence.ScanID, artifactID persistence.ArtifactID) (persistence.ArtifactRecord, error) {
	return scanArtifact(query.QueryRow(ctx, artifactSelect+`WHERE r.security_scope_id=$1::uuid AND r.repository_id=$2::uuid AND e.scan_id=$3::uuid AND e.artifact_id=$4::uuid`, scope, string(repositoryID), string(scanID), string(artifactID)))
}

func scanArtifact(row rowScanner) (persistence.ArtifactRecord, error) {
	var scope, repo, scan, id, name, version, codecName, codecVersion, media, producerName, producerVersion string
	var stable sql.NullString
	var raw []byte
	var size int64
	var created time.Time
	if err := row.Scan(&scope, &repo, &scan, &id, &name, &version, &stable, &codecName, &codecVersion, &media, &producerName, &producerVersion, &raw, &size, &created); err != nil {
		return persistence.ArtifactRecord{}, err
	}
	artifact, err := persistence.NewVersionedName(name, version)
	if err != nil {
		return persistence.ArtifactRecord{}, err
	}
	codec, err := persistence.NewCodec(codecName, codecVersion, media)
	if err != nil {
		return persistence.ArtifactRecord{}, err
	}
	producer, err := persistence.NewVersionedName(producerName, producerVersion)
	if err != nil {
		return persistence.ArtifactRecord{}, err
	}
	digest, err := parseDigest(raw)
	if err != nil {
		return persistence.ArtifactRecord{}, err
	}
	return persistence.NewArtifactRecord(scope, persistence.RepositoryID(repo), persistence.ScanID(scan), persistence.ArtifactID(id), artifact, stable.String, codec, producer, digest, persistence.ByteCount(size), created)
}

func (adapter *Adapter) finishScan(ctx context.Context, operation string, request persistence.FinishScanRequest, target persistence.ScanState) (persistence.ScanRecord, error) {
	if validateIDs(request.Scope().ScopeID(), string(request.RepositoryID()), string(request.ScanID())) != nil {
		return persistence.ScanRecord{}, invalid(operation)
	}
	now := adapter.config.Clock.Now().UTC()
	tx, err := adapter.database.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return persistence.ScanRecord{}, failure(operation, err)
	}
	defer rollback(ctx, tx)
	tag, err := tx.Exec(ctx, `UPDATE platform.repository_scans s SET lifecycle_state=$1,finished_at=$2,failure_code=$3,failure_summary=NULLIF($4,'') FROM platform.repositories r WHERE r.repository_id=s.repository_id AND r.security_scope_id=$5::uuid AND s.repository_id=$6::uuid AND s.scan_id=$7::uuid AND s.lifecycle_state='running'`, string(target), now, request.ReasonCode(), request.SafeMessage(), request.Scope().ScopeID(), string(request.RepositoryID()), string(request.ScanID()))
	if err != nil {
		return persistence.ScanRecord{}, failure(operation, err)
	}
	if tag.RowsAffected() == 0 {
		record, _, _, findErr := scanByID(ctx, tx, request.Scope().ScopeID(), request.RepositoryID(), request.ScanID())
		if findErr != nil {
			return persistence.ScanRecord{}, failure(operation, findErr)
		}
		if record.State() == target && record.ReasonCode() == request.ReasonCode() && record.SafeMessage() == request.SafeMessage() {
			return record, nil
		}
		return persistence.ScanRecord{}, lifecycle(operation)
	}
	if err := audit(ctx, tx, now, operation, request.Actor(), request.RequestID(), request.Scope(), request.RepositoryID(), request.ScanID(), ""); err != nil {
		return persistence.ScanRecord{}, failure(operation, err)
	}
	record, _, _, err := scanByID(ctx, tx, request.Scope().ScopeID(), request.RepositoryID(), request.ScanID())
	if err != nil {
		return persistence.ScanRecord{}, failure(operation, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return persistence.ScanRecord{}, failure(operation, err)
	}
	return record, nil
}

func (adapter *Adapter) changeRepositoryState(ctx context.Context, operation string, scope persistence.Scope, requestID persistence.RequestID, repositoryID persistence.RepositoryID, actor persistence.AuditActor, target persistence.RepositoryState) (persistence.RepositoryRecord, error) {
	if validateIDs(scope.ScopeID(), string(repositoryID)) != nil {
		return persistence.RepositoryRecord{}, invalid(operation)
	}
	now := adapter.config.Clock.Now().UTC()
	tx, err := adapter.database.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return persistence.RepositoryRecord{}, failure(operation, err)
	}
	defer rollback(ctx, tx)
	allowed := persistence.RepositoryActive
	if target == persistence.RepositoryPurgePending {
		allowed = persistence.RepositoryArchived
	}
	tag, err := tx.Exec(ctx, `UPDATE platform.repositories SET lifecycle_state=$1,archived_at=COALESCE(archived_at,$2),updated_at=$2 WHERE security_scope_id=$3::uuid AND repository_id=$4::uuid AND lifecycle_state=$5`, string(target), now, scope.ScopeID(), string(repositoryID), string(allowed))
	if err != nil {
		return persistence.RepositoryRecord{}, failure(operation, err)
	}
	record, _, _, findErr := repositoryByID(ctx, tx, scope.ScopeID(), repositoryID)
	if findErr != nil {
		return persistence.RepositoryRecord{}, failure(operation, findErr)
	}
	if tag.RowsAffected() == 0 && record.State() != target {
		return persistence.RepositoryRecord{}, lifecycle(operation)
	}
	if tag.RowsAffected() == 1 {
		if err := audit(ctx, tx, now, operation, actor, requestID, scope, repositoryID, "", ""); err != nil {
			return persistence.RepositoryRecord{}, failure(operation, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return persistence.RepositoryRecord{}, failure(operation, err)
	}
	return record, nil
}

func (adapter *Adapter) streamPayload(ctx context.Context, query persistence.PayloadQuery, writer io.Writer) (persistence.Digest, persistence.ByteCount, error) {
	const operation = "read-payload"
	if query.Digest().IsZero() || validateIDs(query.Scope().ScopeID(), string(query.RepositoryID()), string(query.ScanID()), string(query.ArtifactID())) != nil {
		return persistence.Digest{}, 0, invalid(operation)
	}
	var size int64
	var stored []byte
	err := adapter.database.QueryRow(ctx, `SELECT p.payload_size,p.payload_digest FROM platform.artifact_envelopes e JOIN platform.repository_scans s ON s.scan_id=e.scan_id JOIN platform.repositories r ON r.repository_id=s.repository_id JOIN platform.artifact_payloads p ON p.payload_digest=e.payload_digest WHERE r.security_scope_id=$1::uuid AND r.repository_id=$2::uuid AND e.scan_id=$3::uuid AND e.artifact_id=$4::uuid AND e.payload_digest=$5`, query.Scope().ScopeID(), string(query.RepositoryID()), string(query.ScanID()), string(query.ArtifactID()), digestBytes(query.Digest())).Scan(&size, &stored)
	if err != nil {
		return persistence.Digest{}, 0, failure(operation, err)
	}
	rows, err := adapter.database.Query(ctx, `SELECT chunk_bytes FROM platform.artifact_payload_chunks WHERE payload_digest=$1 ORDER BY chunk_ordinal`, stored)
	if err != nil {
		return persistence.Digest{}, 0, failure(operation, err)
	}
	defer rows.Close()
	hash := sha256.New()
	var total int64
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return persistence.Digest{}, 0, failure(operation, err)
		}
		var chunk []byte
		if err := rows.Scan(&chunk); err != nil {
			return persistence.Digest{}, 0, failure(operation, err)
		}
		_, _ = hash.Write(chunk)
		count, writeErr := writer.Write(chunk)
		total += int64(count)
		if writeErr != nil || count != len(chunk) {
			return persistence.Digest{}, 0, persistence.NewError(persistence.ErrorInternal, operation, false, writeErr)
		}
	}
	if err := rows.Err(); err != nil {
		return persistence.Digest{}, 0, failure(operation, err)
	}
	actual := hash.Sum(nil)
	if total != size || !equalBytes(actual, stored) || !equalBytes(stored, digestBytes(query.Digest())) {
		return persistence.Digest{}, 0, integrity(operation)
	}
	return query.Digest(), persistence.ByteCount(size), nil
}

func audit(ctx context.Context, tx pgx.Tx, now time.Time, event string, actor persistence.AuditActor, requestID persistence.RequestID, scope persistence.Scope, repositoryID persistence.RepositoryID, scanID persistence.ScanID, artifactID persistence.ArtifactID) error {
	var repo, scan, artifact any
	if repositoryID != "" {
		repo = string(repositoryID)
	}
	if scanID != "" {
		scan = string(scanID)
	}
	if artifactID != "" {
		artifact = string(artifactID)
	}
	_, err := tx.Exec(ctx, `INSERT INTO platform.audit_events(occurred_at,event_type,outcome,actor_kind,actor_id,correlation_id,security_scope_id,repository_id,scan_id,artifact_id,safe_details) VALUES($1,$2,'succeeded',$3,$4,$5::uuid,$6::uuid,$7::uuid,$8::uuid,$9::uuid,'{}'::jsonb)`, now, event, actor.Kind(), actor.ID(), correlationUUID(requestID), scope.ScopeID(), repo, scan, artifact)
	return err
}

func statisticColumns(statistic persistence.StatisticSubmission) (any, any, any, any, any) {
	var integer, decimal, boolean, text, unit any
	value := statistic.Value()
	switch value.Kind() {
	case persistence.StatisticInteger:
		integer = value.Integer()
	case persistence.StatisticDecimal:
		decimal = value.Decimal()
	case persistence.StatisticBoolean:
		boolean = value.Boolean()
	case persistence.StatisticText:
		text = value.Text()
	}
	if statistic.Unit() != "" {
		unit = statistic.Unit()
	}
	return integer, decimal, boolean, text, unit
}
func chunkCount(size persistence.ByteCount) int {
	if size == 0 {
		return 0
	}
	return int((uint64(size) + ChunkSize - 1) / ChunkSize)
}
func validateIDs(values ...string) error {
	for _, value := range values {
		if validateUUID(value) != nil {
			return fmt.Errorf("identifier")
		}
	}
	return nil
}
func rollback(ctx context.Context, tx pgx.Tx) { _ = tx.Rollback(ctx) }
func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var different byte
	for i := range a {
		different |= a[i] ^ b[i]
	}
	return different == 0
}
func uuidText(value pgtype.UUID) string {
	driverValue, err := value.Value()
	if err != nil || driverValue == nil {
		return ""
	}
	return driverValue.(string)
}

var _ persistence.Port = (*Adapter)(nil)
