SET lock_timeout = '5s';
SET statement_timeout = '5min';
SET LOCAL ROLE platform_owner;

REVOKE ALL ON SCHEMA platform FROM PUBLIC;
REVOKE ALL ON ALL TABLES IN SCHEMA platform FROM PUBLIC;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA platform FROM PUBLIC;

GRANT USAGE ON SCHEMA platform TO
    platform_migrator,
    platform_ingestor,
    platform_query_reader,
    platform_artifact_reader,
    platform_retention_worker,
    platform_audit_writer,
    platform_auditor,
    platform_backup;

GRANT SELECT, INSERT ON
    platform.repositories,
    platform.repository_scans,
    platform.scan_publications,
    platform.artifact_payloads,
    platform.artifact_payload_chunks,
    platform.artifact_envelopes,
    platform.artifact_dependencies,
    platform.artifact_projections,
    platform.projected_diagnostics,
    platform.projected_statistics
TO platform_ingestor;
GRANT UPDATE (lifecycle_state, current_scan_id, updated_at, archived_at)
    ON platform.repositories TO platform_ingestor;
GRANT UPDATE (
    lifecycle_state,
    started_at,
    finished_at,
    published_at,
    failure_code,
    failure_summary
) ON platform.repository_scans TO platform_ingestor;

GRANT SELECT ON
    platform.repositories,
    platform.repository_scans,
    platform.scan_publications,
    platform.artifact_payloads,
    platform.artifact_envelopes,
    platform.artifact_dependencies,
    platform.artifact_projections,
    platform.projected_diagnostics,
    platform.projected_statistics
TO platform_query_reader;

GRANT SELECT ON
    platform.repositories,
    platform.repository_scans,
    platform.scan_publications,
    platform.artifact_payloads,
    platform.artifact_payload_chunks,
    platform.artifact_envelopes,
    platform.artifact_dependencies,
    platform.artifact_projections,
    platform.projected_diagnostics,
    platform.projected_statistics
TO platform_artifact_reader;

GRANT SELECT, UPDATE, DELETE ON
    platform.repositories,
    platform.repository_scans,
    platform.scan_publications,
    platform.artifact_payloads,
    platform.artifact_payload_chunks,
    platform.artifact_envelopes,
    platform.artifact_dependencies,
    platform.artifact_projections,
    platform.projected_diagnostics,
    platform.projected_statistics
TO platform_retention_worker;

GRANT INSERT ON platform.audit_events TO
    platform_ingestor,
    platform_retention_worker,
    platform_audit_writer;
GRANT USAGE, SELECT ON SEQUENCE platform.audit_events_audit_event_id_seq TO
    platform_ingestor,
    platform_retention_worker,
    platform_audit_writer;

GRANT SELECT ON
    platform.repositories,
    platform.repository_scans,
    platform.scan_publications,
    platform.artifact_envelopes,
    platform.artifact_dependencies,
    platform.artifact_projections,
    platform.projected_diagnostics,
    platform.projected_statistics,
    platform.audit_events
TO platform_auditor;

GRANT SELECT ON ALL TABLES IN SCHEMA platform TO platform_backup;
GRANT SELECT ON ALL SEQUENCES IN SCHEMA platform TO platform_backup;

ALTER DEFAULT PRIVILEGES FOR ROLE platform_owner IN SCHEMA platform
    REVOKE ALL ON TABLES FROM PUBLIC;
ALTER DEFAULT PRIVILEGES FOR ROLE platform_owner IN SCHEMA platform
    REVOKE ALL ON SEQUENCES FROM PUBLIC;

RESET ROLE;
