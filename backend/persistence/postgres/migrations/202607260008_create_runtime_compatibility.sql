SET lock_timeout = '5s';
SET statement_timeout = '5min';
SET LOCAL ROLE platform_owner;

CREATE TABLE platform.runtime_compatibility (
    singleton_key smallint PRIMARY KEY,
    contract_key text COLLATE "C" NOT NULL,
    schema_contract_version text COLLATE "C" NOT NULL,
    minimum_adapter_major integer NOT NULL,
    maximum_adapter_major integer NOT NULL,
    migration_revision text COLLATE "C" NOT NULL,
    published_at timestamptz NOT NULL,
    CONSTRAINT runtime_compatibility_singleton CHECK (singleton_key = 1),
    CONSTRAINT runtime_compatibility_contract_key_length
        CHECK (octet_length(contract_key) BETWEEN 1 AND 128),
    CONSTRAINT runtime_compatibility_schema_version_length
        CHECK (octet_length(schema_contract_version) BETWEEN 1 AND 64),
    CONSTRAINT runtime_compatibility_adapter_range CHECK (
        minimum_adapter_major BETWEEN 1 AND 2147483647
        AND maximum_adapter_major >= minimum_adapter_major
    ),
    CONSTRAINT runtime_compatibility_migration_revision CHECK (
        migration_revision ~ '^[0-9]{12}$'
    )
);

INSERT INTO platform.runtime_compatibility (
    singleton_key,
    contract_key,
    schema_contract_version,
    minimum_adapter_major,
    maximum_adapter_major,
    migration_revision,
    published_at
) VALUES (
    1,
    'aegis-postgresql-persistence',
    '1.0.0',
    1,
    1,
    '202607260008',
    transaction_timestamp()
);

REVOKE ALL ON platform.runtime_compatibility FROM PUBLIC;
GRANT SELECT ON platform.runtime_compatibility TO
    platform_ingestor,
    platform_artifact_reader,
    platform_retention_worker;

RESET ROLE;
