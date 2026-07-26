\set ON_ERROR_STOP on

DO $$
DECLARE
    table_count integer;
    wrong_owner_count integer;
    revision_count integer;
    failed_revision_count integer;
BEGIN
    IF current_setting('server_version_num')::integer < 180000
       OR current_setting('server_version_num')::integer >= 190000 THEN
        RAISE EXCEPTION 'expected PostgreSQL 18, got %', current_setting('server_version');
    END IF;

    SELECT count(*) INTO table_count
    FROM information_schema.tables
    WHERE table_schema = 'platform' AND table_type = 'BASE TABLE';
    IF table_count <> 12 THEN
        RAISE EXCEPTION 'expected 12 platform tables, got %', table_count;
    END IF;

    SELECT count(*) INTO wrong_owner_count
    FROM pg_class c
    JOIN pg_namespace n ON n.oid = c.relnamespace
    JOIN pg_roles r ON r.oid = c.relowner
    WHERE n.nspname = 'platform'
      AND c.relkind IN ('r', 'p', 'S', 'i')
      AND r.rolname <> 'platform_owner';
    IF wrong_owner_count <> 0 THEN
        RAISE EXCEPTION '% platform objects have an unexpected owner', wrong_owner_count;
    END IF;

    IF (SELECT r.rolname FROM pg_namespace n JOIN pg_roles r ON r.oid = n.nspowner
        WHERE n.nspname = 'platform') <> 'platform_owner' THEN
        RAISE EXCEPTION 'platform schema owner is not platform_owner';
    END IF;

    SELECT count(*), count(*) FILTER (WHERE applied <> total OR error <> '')
        INTO revision_count, failed_revision_count
    FROM atlas_schema_revisions.atlas_schema_revisions;
    IF revision_count <> 8 OR failed_revision_count <> 0 THEN
        RAISE EXCEPTION 'expected eight successful revisions, got % total and % incomplete',
            revision_count, failed_revision_count;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM platform.runtime_compatibility
        WHERE singleton_key = 1
          AND contract_key = 'aegis-postgresql-persistence'
          AND schema_contract_version = '1.0.0'
          AND minimum_adapter_major = 1
          AND maximum_adapter_major = 1
          AND migration_revision = '202607260008'
          AND published_at IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'runtime compatibility record is missing or malformed';
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'repositories_current_scan_fk' AND condeferrable
    ) THEN
        RAISE EXCEPTION 'missing deferrable repository publication constraint';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'artifact_envelopes_publication_fk' AND condeferrable
    ) THEN
        RAISE EXCEPTION 'missing deferrable envelope publication constraint';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'artifact_payloads_chunk_size'
    ) THEN
        RAISE EXCEPTION 'missing accepted four-MiB chunk constraint';
    END IF;
END
$$;

SELECT 'schema assertions passed' AS result;
