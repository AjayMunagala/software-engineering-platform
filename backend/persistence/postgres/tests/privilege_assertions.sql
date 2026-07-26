\set ON_ERROR_STOP on

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_namespace n
        CROSS JOIN LATERAL aclexplode(
            COALESCE(n.nspacl, acldefault('n', n.nspowner))
        ) privilege
        WHERE n.nspname = 'platform'
          AND privilege.grantee = 0
          AND privilege.privilege_type IN ('USAGE', 'CREATE')
    ) THEN
        RAISE EXCEPTION 'PUBLIC unexpectedly has platform schema privileges';
    END IF;
    IF has_schema_privilege('platform_ingestor', 'platform', 'CREATE') THEN
        RAISE EXCEPTION 'platform_ingestor unexpectedly has DDL rights';
    END IF;
    IF NOT has_schema_privilege('platform_migrator',
        'atlas_schema_revisions', 'USAGE')
       OR NOT has_table_privilege('platform_migrator',
            'atlas_schema_revisions.atlas_schema_revisions', 'SELECT,INSERT,UPDATE,DELETE') THEN
        RAISE EXCEPTION 'migrator cannot maintain schema revision history';
    END IF;
    IF has_table_privilege('platform_migrator', 'platform.repositories', 'SELECT') THEN
        RAISE EXCEPTION 'migrator unexpectedly inherits application-table access';
    END IF;
    IF has_table_privilege('platform_query_reader',
        'platform.artifact_payload_chunks', 'SELECT') THEN
        RAISE EXCEPTION 'query reader unexpectedly sees exact payload chunks';
    END IF;
    IF has_table_privilege('platform_query_reader', 'platform.audit_events', 'SELECT') THEN
        RAISE EXCEPTION 'query reader unexpectedly sees audit events';
    END IF;
    IF NOT has_table_privilege('platform_artifact_reader',
        'platform.artifact_payload_chunks', 'SELECT') THEN
        RAISE EXCEPTION 'artifact reader cannot read exact chunks';
    END IF;
    IF NOT has_table_privilege('platform_ingestor', 'platform.repositories', 'INSERT') THEN
        RAISE EXCEPTION 'ingestor cannot insert repositories';
    END IF;
    IF has_table_privilege('platform_ingestor', 'platform.repositories', 'DELETE') THEN
        RAISE EXCEPTION 'ingestor unexpectedly has delete rights';
    END IF;
    IF NOT has_table_privilege('platform_retention_worker',
        'platform.artifact_envelopes', 'DELETE') THEN
        RAISE EXCEPTION 'retention worker cannot delete selected operational rows';
    END IF;
    IF NOT has_table_privilege('platform_ingestor',
        'platform.runtime_compatibility', 'SELECT')
       OR NOT has_table_privilege('platform_artifact_reader',
            'platform.runtime_compatibility', 'SELECT')
       OR NOT has_table_privilege('platform_retention_worker',
            'platform.runtime_compatibility', 'SELECT') THEN
        RAISE EXCEPTION 'runtime roles cannot read the compatibility proof';
    END IF;
    IF has_table_privilege('platform_ingestor',
        'platform.runtime_compatibility', 'INSERT,UPDATE,DELETE')
       OR has_table_privilege('platform_artifact_reader',
            'platform.runtime_compatibility', 'INSERT,UPDATE,DELETE')
       OR has_table_privilege('platform_retention_worker',
            'platform.runtime_compatibility', 'INSERT,UPDATE,DELETE') THEN
        RAISE EXCEPTION 'runtime role can mutate the compatibility proof';
    END IF;
    IF NOT has_table_privilege('platform_audit_writer', 'platform.audit_events', 'INSERT')
       OR has_table_privilege('platform_audit_writer', 'platform.audit_events', 'SELECT') THEN
        RAISE EXCEPTION 'audit-writer privilege boundary is invalid';
    END IF;
    IF NOT has_table_privilege('platform_auditor', 'platform.audit_events', 'SELECT')
       OR has_table_privilege('platform_auditor',
            'platform.artifact_payload_chunks', 'SELECT') THEN
        RAISE EXCEPTION 'auditor privilege boundary is invalid';
    END IF;
    IF EXISTS (
        SELECT 1
        FROM pg_roles
        WHERE rolname LIKE 'platform_%'
          AND (rolcanlogin OR rolsuper OR rolcreatedb OR rolcreaterole)
    ) THEN
        RAISE EXCEPTION 'a platform capability role has elevated/login attributes';
    END IF;
    IF EXISTS (
        SELECT 1
        FROM pg_auth_members m
        JOIN pg_roles member_role ON member_role.oid = m.member
        JOIN pg_roles granted_role ON granted_role.oid = m.roleid
        WHERE granted_role.rolname = 'platform_owner'
          AND member_role.rolname <> 'platform_migrator'
    ) THEN
        RAISE EXCEPTION 'a non-migrator role can assume platform_owner';
    END IF;
END
$$;

SELECT 'privilege assertions passed' AS result;
