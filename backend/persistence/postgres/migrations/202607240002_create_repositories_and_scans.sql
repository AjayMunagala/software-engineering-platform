SET lock_timeout = '5s';
SET statement_timeout = '5min';
SET LOCAL ROLE platform_owner;

CREATE TABLE platform.repositories (
    repository_id uuid PRIMARY KEY,
    security_scope_id uuid NOT NULL,
    idempotency_key text NOT NULL,
    registration_digest bytea NOT NULL,
    display_name text NOT NULL,
    source_kind text COLLATE "C" NOT NULL,
    source_fingerprint_scheme text COLLATE "C" NOT NULL,
    source_fingerprint bytea NOT NULL,
    lifecycle_state text COLLATE "C" NOT NULL,
    current_scan_id uuid,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    archived_at timestamptz,
    CONSTRAINT repositories_scope_idempotency_unique
        UNIQUE (security_scope_id, idempotency_key),
    CONSTRAINT repositories_source_unique
        UNIQUE (security_scope_id, source_kind, source_fingerprint_scheme, source_fingerprint),
    CONSTRAINT repositories_idempotency_key_length
        CHECK (octet_length(idempotency_key) BETWEEN 1 AND 256),
    CONSTRAINT repositories_registration_digest_length
        CHECK (octet_length(registration_digest) = 32),
    CONSTRAINT repositories_display_name_length
        CHECK (octet_length(display_name) BETWEEN 1 AND 512),
    CONSTRAINT repositories_source_kind_length
        CHECK (octet_length(source_kind) BETWEEN 1 AND 64),
    CONSTRAINT repositories_source_scheme_length
        CHECK (octet_length(source_fingerprint_scheme) BETWEEN 1 AND 128),
    CONSTRAINT repositories_source_digest_length
        CHECK (octet_length(source_fingerprint) = 32),
    CONSTRAINT repositories_lifecycle_state
        CHECK (lifecycle_state IN ('active', 'archived', 'purge_pending')),
    CONSTRAINT repositories_archive_state
        CHECK (
            (lifecycle_state = 'active' AND archived_at IS NULL)
            OR (lifecycle_state IN ('archived', 'purge_pending') AND archived_at IS NOT NULL)
        ),
    CONSTRAINT repositories_timestamp_order CHECK (updated_at >= created_at)
);

CREATE TABLE platform.repository_scans (
    scan_id uuid PRIMARY KEY,
    repository_id uuid NOT NULL,
    idempotency_key text NOT NULL,
    request_digest bytea NOT NULL,
    analysis_profile_digest bytea NOT NULL,
    source_revision text,
    lifecycle_state text COLLATE "C" NOT NULL,
    requested_at timestamptz NOT NULL,
    started_at timestamptz,
    finished_at timestamptz,
    published_at timestamptz,
    failure_code text COLLATE "C",
    failure_summary text,
    CONSTRAINT repository_scans_repository_scan_unique UNIQUE (repository_id, scan_id),
    CONSTRAINT repository_scans_publication_proof_unique
        UNIQUE (repository_id, scan_id, lifecycle_state),
    CONSTRAINT repository_scans_idempotency_unique UNIQUE (repository_id, idempotency_key),
    CONSTRAINT repository_scans_repository_fk FOREIGN KEY (repository_id)
        REFERENCES platform.repositories (repository_id) ON DELETE RESTRICT,
    CONSTRAINT repository_scans_idempotency_key_length
        CHECK (octet_length(idempotency_key) BETWEEN 1 AND 256),
    CONSTRAINT repository_scans_request_digest_length CHECK (octet_length(request_digest) = 32),
    CONSTRAINT repository_scans_profile_digest_length
        CHECK (octet_length(analysis_profile_digest) = 32),
    CONSTRAINT repository_scans_source_revision_length
        CHECK (source_revision IS NULL OR octet_length(source_revision) <= 512),
    CONSTRAINT repository_scans_lifecycle_state
        CHECK (lifecycle_state IN ('requested', 'running', 'succeeded', 'failed', 'cancelled')),
    CONSTRAINT repository_scans_timestamp_order CHECK (
        (started_at IS NULL OR started_at >= requested_at)
        AND (finished_at IS NULL OR finished_at >= COALESCE(started_at, requested_at))
        AND (published_at IS NULL OR published_at >= COALESCE(started_at, requested_at))
    ),
    CONSTRAINT repository_scans_state_fields CHECK (
        (lifecycle_state = 'requested'
            AND started_at IS NULL AND finished_at IS NULL AND published_at IS NULL
            AND failure_code IS NULL AND failure_summary IS NULL)
        OR (lifecycle_state = 'running'
            AND started_at IS NOT NULL AND finished_at IS NULL AND published_at IS NULL
            AND failure_code IS NULL AND failure_summary IS NULL)
        OR (lifecycle_state = 'succeeded'
            AND started_at IS NOT NULL AND finished_at IS NOT NULL AND published_at IS NOT NULL
            AND failure_code IS NULL AND failure_summary IS NULL)
        OR (lifecycle_state IN ('failed', 'cancelled')
            AND finished_at IS NOT NULL AND published_at IS NULL AND failure_code IS NOT NULL)
    ),
    CONSTRAINT repository_scans_failure_code_length
        CHECK (failure_code IS NULL OR octet_length(failure_code) BETWEEN 1 AND 128),
    CONSTRAINT repository_scans_failure_summary_length
        CHECK (failure_summary IS NULL OR octet_length(failure_summary) <= 4096)
);

CREATE TABLE platform.scan_publications (
    scan_id uuid PRIMARY KEY,
    repository_id uuid NOT NULL,
    lifecycle_state text COLLATE "C" NOT NULL,
    manifest_scheme text COLLATE "C" NOT NULL,
    artifact_set_digest bytea NOT NULL,
    artifact_count integer NOT NULL,
    published_at timestamptz NOT NULL,
    CONSTRAINT scan_publications_repository_scan_unique UNIQUE (repository_id, scan_id),
    CONSTRAINT scan_publications_scan_fk
        FOREIGN KEY (repository_id, scan_id, lifecycle_state)
        REFERENCES platform.repository_scans (repository_id, scan_id, lifecycle_state)
        ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED,
    CONSTRAINT scan_publications_succeeded CHECK (lifecycle_state = 'succeeded'),
    CONSTRAINT scan_publications_manifest_scheme_length
        CHECK (octet_length(manifest_scheme) BETWEEN 1 AND 128),
    CONSTRAINT scan_publications_digest_length CHECK (octet_length(artifact_set_digest) = 32),
    CONSTRAINT scan_publications_artifact_count CHECK (artifact_count BETWEEN 1 AND 256)
);

ALTER TABLE platform.repositories
    ADD CONSTRAINT repositories_current_scan_fk
    FOREIGN KEY (repository_id, current_scan_id)
    REFERENCES platform.scan_publications (repository_id, scan_id)
    ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED;

RESET ROLE;
