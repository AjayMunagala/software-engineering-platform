SET lock_timeout = '5s';
SET statement_timeout = '5min';
SET LOCAL ROLE platform_owner;

CREATE TABLE platform.artifact_projections (
    projection_id uuid PRIMARY KEY,
    artifact_id uuid NOT NULL,
    source_payload_digest bytea NOT NULL,
    projector_name text COLLATE "C" NOT NULL,
    projector_version text COLLATE "C" NOT NULL,
    projection_schema_version text COLLATE "C" NOT NULL,
    projection_digest_scheme text COLLATE "C" NOT NULL,
    projection_digest bytea NOT NULL,
    document jsonb NOT NULL,
    document_size integer NOT NULL,
    record_count integer NOT NULL,
    created_at timestamptz NOT NULL,
    CONSTRAINT artifact_projections_version_unique
        UNIQUE (artifact_id, projector_name, projector_version, projection_schema_version),
    CONSTRAINT artifact_projections_payload_fk FOREIGN KEY (source_payload_digest)
        REFERENCES platform.artifact_payloads (payload_digest) ON DELETE RESTRICT,
    CONSTRAINT artifact_projections_envelope_fk
        FOREIGN KEY (artifact_id, source_payload_digest)
        REFERENCES platform.artifact_envelopes (artifact_id, payload_digest) ON DELETE CASCADE,
    CONSTRAINT artifact_projections_source_digest_length
        CHECK (octet_length(source_payload_digest) = 32),
    CONSTRAINT artifact_projections_projection_digest_length
        CHECK (octet_length(projection_digest) = 32),
    CONSTRAINT artifact_projections_projector_name_length
        CHECK (octet_length(projector_name) BETWEEN 1 AND 128),
    CONSTRAINT artifact_projections_projector_version_length
        CHECK (octet_length(projector_version) BETWEEN 1 AND 64),
    CONSTRAINT artifact_projections_schema_version_length
        CHECK (octet_length(projection_schema_version) BETWEEN 1 AND 64),
    CONSTRAINT artifact_projections_digest_scheme_length
        CHECK (octet_length(projection_digest_scheme) BETWEEN 1 AND 128),
    CONSTRAINT artifact_projections_document_object CHECK (jsonb_typeof(document) = 'object'),
    CONSTRAINT artifact_projections_document_size CHECK (document_size BETWEEN 0 AND 8388608),
    CONSTRAINT artifact_projections_record_count CHECK (record_count >= 0)
);

CREATE TABLE platform.projected_diagnostics (
    projection_id uuid NOT NULL,
    diagnostic_ordinal integer NOT NULL,
    severity text COLLATE "C" NOT NULL,
    code text COLLATE "C" NOT NULL,
    engine_name text COLLATE "C" NOT NULL,
    relative_path text,
    line_number integer,
    column_number integer,
    message text NOT NULL,
    PRIMARY KEY (projection_id, diagnostic_ordinal),
    CONSTRAINT projected_diagnostics_projection_fk FOREIGN KEY (projection_id)
        REFERENCES platform.artifact_projections (projection_id) ON DELETE CASCADE,
    CONSTRAINT projected_diagnostics_ordinal CHECK (diagnostic_ordinal >= 0),
    CONSTRAINT projected_diagnostics_severity CHECK (severity IN ('info', 'warning', 'error')),
    CONSTRAINT projected_diagnostics_code_length CHECK (octet_length(code) BETWEEN 1 AND 128),
    CONSTRAINT projected_diagnostics_engine_length
        CHECK (octet_length(engine_name) BETWEEN 1 AND 128),
    CONSTRAINT projected_diagnostics_path_length
        CHECK (relative_path IS NULL OR octet_length(relative_path) <= 4096),
    CONSTRAINT projected_diagnostics_line CHECK (line_number IS NULL OR line_number > 0),
    CONSTRAINT projected_diagnostics_column CHECK (column_number IS NULL OR column_number > 0),
    CONSTRAINT projected_diagnostics_message_length CHECK (octet_length(message) <= 4096)
);

CREATE TABLE platform.projected_statistics (
    projection_id uuid NOT NULL,
    metric_key text COLLATE "C" NOT NULL,
    value_kind text COLLATE "C" NOT NULL,
    integer_value bigint,
    decimal_value numeric,
    boolean_value boolean,
    text_value text,
    unit text COLLATE "C",
    PRIMARY KEY (projection_id, metric_key),
    CONSTRAINT projected_statistics_projection_fk FOREIGN KEY (projection_id)
        REFERENCES platform.artifact_projections (projection_id) ON DELETE CASCADE,
    CONSTRAINT projected_statistics_metric_key_length
        CHECK (octet_length(metric_key) BETWEEN 1 AND 128),
    CONSTRAINT projected_statistics_kind
        CHECK (value_kind IN ('integer', 'decimal', 'boolean', 'text')),
    CONSTRAINT projected_statistics_typed_value CHECK (
        (value_kind = 'integer' AND integer_value IS NOT NULL
            AND decimal_value IS NULL AND boolean_value IS NULL AND text_value IS NULL)
        OR (value_kind = 'decimal' AND integer_value IS NULL
            AND decimal_value IS NOT NULL AND boolean_value IS NULL AND text_value IS NULL)
        OR (value_kind = 'boolean' AND integer_value IS NULL
            AND decimal_value IS NULL AND boolean_value IS NOT NULL AND text_value IS NULL)
        OR (value_kind = 'text' AND integer_value IS NULL
            AND decimal_value IS NULL AND boolean_value IS NULL AND text_value IS NOT NULL)
    ),
    CONSTRAINT projected_statistics_text_length
        CHECK (text_value IS NULL OR octet_length(text_value) <= 4096),
    CONSTRAINT projected_statistics_unit_length
        CHECK (unit IS NULL OR octet_length(unit) BETWEEN 1 AND 64)
);

RESET ROLE;
