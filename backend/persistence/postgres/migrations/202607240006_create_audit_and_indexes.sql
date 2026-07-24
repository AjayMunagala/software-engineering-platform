SET lock_timeout = '5s';
SET statement_timeout = '5min';
SET LOCAL ROLE platform_owner;

CREATE TABLE platform.audit_events (
    audit_event_id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    occurred_at timestamptz NOT NULL,
    event_type text COLLATE "C" NOT NULL,
    outcome text COLLATE "C" NOT NULL,
    actor_kind text COLLATE "C" NOT NULL,
    actor_id text NOT NULL,
    correlation_id uuid NOT NULL,
    security_scope_id uuid NOT NULL,
    repository_id uuid,
    scan_id uuid,
    artifact_id uuid,
    safe_details jsonb NOT NULL,
    CONSTRAINT audit_events_event_type_length CHECK (octet_length(event_type) BETWEEN 1 AND 128),
    CONSTRAINT audit_events_outcome CHECK (outcome IN ('succeeded', 'failed', 'denied')),
    CONSTRAINT audit_events_actor_kind_length CHECK (octet_length(actor_kind) BETWEEN 1 AND 128),
    CONSTRAINT audit_events_actor_id_length CHECK (octet_length(actor_id) BETWEEN 1 AND 256),
    CONSTRAINT audit_events_safe_details_object CHECK (jsonb_typeof(safe_details) = 'object'),
    CONSTRAINT audit_events_safe_details_length
        CHECK (octet_length(safe_details::text) <= 65536)
);

CREATE INDEX repositories_scope_state_created_idx
    ON platform.repositories (security_scope_id, lifecycle_state, created_at DESC, repository_id);
CREATE INDEX repository_scans_repository_requested_idx
    ON platform.repository_scans (repository_id, requested_at DESC, scan_id);
CREATE INDEX repository_scans_repository_state_requested_idx
    ON platform.repository_scans (repository_id, lifecycle_state, requested_at DESC, scan_id);
CREATE INDEX repository_scans_running_recovery_idx
    ON platform.repository_scans (started_at, scan_id) WHERE lifecycle_state = 'running';
CREATE INDEX artifact_envelopes_name_version_created_idx
    ON platform.artifact_envelopes (artifact_name, artifact_version, created_at DESC);
CREATE INDEX artifact_envelopes_payload_reference_idx
    ON platform.artifact_envelopes (payload_digest, artifact_id);
CREATE INDEX artifact_dependencies_source_consumer_idx
    ON platform.artifact_dependencies (source_artifact_id, consumer_artifact_id);
CREATE INDEX projected_diagnostics_filter_idx
    ON platform.projected_diagnostics (severity, code, projection_id, diagnostic_ordinal);
CREATE INDEX audit_events_repository_time_idx
    ON platform.audit_events (repository_id, occurred_at DESC) WHERE repository_id IS NOT NULL;
CREATE INDEX audit_events_scan_time_idx
    ON platform.audit_events (scan_id, occurred_at DESC) WHERE scan_id IS NOT NULL;
CREATE INDEX audit_events_correlation_time_idx
    ON platform.audit_events (correlation_id, occurred_at DESC);

RESET ROLE;
