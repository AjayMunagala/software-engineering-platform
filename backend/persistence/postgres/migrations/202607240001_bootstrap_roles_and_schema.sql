SET lock_timeout = '5s';
SET statement_timeout = '5min';

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'platform_owner') THEN
        CREATE ROLE platform_owner
            NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'platform_migrator') THEN
        CREATE ROLE platform_migrator
            NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'platform_ingestor') THEN
        CREATE ROLE platform_ingestor
            NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'platform_query_reader') THEN
        CREATE ROLE platform_query_reader
            NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'platform_artifact_reader') THEN
        CREATE ROLE platform_artifact_reader
            NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'platform_retention_worker') THEN
        CREATE ROLE platform_retention_worker
            NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'platform_audit_writer') THEN
        CREATE ROLE platform_audit_writer
            NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'platform_auditor') THEN
        CREATE ROLE platform_auditor
            NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'platform_backup') THEN
        CREATE ROLE platform_backup
            NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION;
    END IF;
END
$$;

GRANT platform_owner TO platform_migrator;

CREATE SCHEMA platform AUTHORIZATION platform_owner;
REVOKE ALL ON SCHEMA platform FROM PUBLIC;

-- Atlas writes statement/revision progress inside the same transaction in
-- which owner-scoped DDL runs. Only the non-login owner role receives this
-- access; runtime roles remain excluded from migration history.
GRANT USAGE ON SCHEMA atlas_schema_revisions TO platform_owner, platform_migrator;
GRANT SELECT, INSERT, UPDATE, DELETE
    ON ALL TABLES IN SCHEMA atlas_schema_revisions TO platform_owner, platform_migrator;
GRANT USAGE, SELECT, UPDATE
    ON ALL SEQUENCES IN SCHEMA atlas_schema_revisions TO platform_owner, platform_migrator;
