SET lock_timeout = '5s';
SET statement_timeout = '5min';
SET LOCAL ROLE platform_owner;

CREATE TABLE platform.artifact_envelopes (
    artifact_id uuid PRIMARY KEY,
    scan_id uuid NOT NULL,
    artifact_name text COLLATE "C" NOT NULL,
    artifact_version text COLLATE "C" NOT NULL,
    stable_id_scheme text COLLATE "C",
    codec_name text COLLATE "C" NOT NULL,
    codec_version text COLLATE "C" NOT NULL,
    media_type text COLLATE "C" NOT NULL,
    producer_name text COLLATE "C" NOT NULL,
    producer_version text COLLATE "C" NOT NULL,
    payload_digest bytea NOT NULL,
    payload_size bigint NOT NULL,
    created_at timestamptz NOT NULL,
    CONSTRAINT artifact_envelopes_scan_name_unique UNIQUE (scan_id, artifact_name),
    CONSTRAINT artifact_envelopes_scan_artifact_unique UNIQUE (scan_id, artifact_id),
    CONSTRAINT artifact_envelopes_artifact_digest_unique UNIQUE (artifact_id, payload_digest),
    CONSTRAINT artifact_envelopes_scan_fk FOREIGN KEY (scan_id)
        REFERENCES platform.repository_scans (scan_id) ON DELETE RESTRICT,
    CONSTRAINT artifact_envelopes_publication_fk FOREIGN KEY (scan_id)
        REFERENCES platform.scan_publications (scan_id)
        ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED,
    CONSTRAINT artifact_envelopes_payload_fk FOREIGN KEY (payload_digest, payload_size)
        REFERENCES platform.artifact_payloads (payload_digest, payload_size) ON DELETE RESTRICT,
    CONSTRAINT artifact_envelopes_artifact_name_length
        CHECK (octet_length(artifact_name) BETWEEN 1 AND 128),
    CONSTRAINT artifact_envelopes_artifact_version_length
        CHECK (octet_length(artifact_version) BETWEEN 1 AND 64),
    CONSTRAINT artifact_envelopes_stable_id_length
        CHECK (stable_id_scheme IS NULL OR octet_length(stable_id_scheme) BETWEEN 1 AND 128),
    CONSTRAINT artifact_envelopes_codec_name_length
        CHECK (octet_length(codec_name) BETWEEN 1 AND 64),
    CONSTRAINT artifact_envelopes_codec_version_length
        CHECK (octet_length(codec_version) BETWEEN 1 AND 64),
    CONSTRAINT artifact_envelopes_media_type_length
        CHECK (octet_length(media_type) BETWEEN 1 AND 128),
    CONSTRAINT artifact_envelopes_producer_name_length
        CHECK (octet_length(producer_name) BETWEEN 1 AND 128),
    CONSTRAINT artifact_envelopes_producer_version_length
        CHECK (octet_length(producer_version) BETWEEN 1 AND 64)
);

CREATE TABLE platform.artifact_dependencies (
    scan_id uuid NOT NULL,
    consumer_artifact_id uuid NOT NULL,
    dependency_ordinal integer NOT NULL,
    source_artifact_id uuid NOT NULL,
    declared_name text COLLATE "C" NOT NULL,
    declared_version text COLLATE "C" NOT NULL,
    PRIMARY KEY (consumer_artifact_id, dependency_ordinal),
    CONSTRAINT artifact_dependencies_consumer_source_unique
        UNIQUE (consumer_artifact_id, source_artifact_id),
    CONSTRAINT artifact_dependencies_consumer_fk
        FOREIGN KEY (scan_id, consumer_artifact_id)
        REFERENCES platform.artifact_envelopes (scan_id, artifact_id) ON DELETE RESTRICT,
    CONSTRAINT artifact_dependencies_source_fk
        FOREIGN KEY (scan_id, source_artifact_id)
        REFERENCES platform.artifact_envelopes (scan_id, artifact_id) ON DELETE RESTRICT,
    CONSTRAINT artifact_dependencies_distinct CHECK (consumer_artifact_id <> source_artifact_id),
    CONSTRAINT artifact_dependencies_ordinal CHECK (dependency_ordinal BETWEEN 0 AND 4095),
    CONSTRAINT artifact_dependencies_name_length
        CHECK (octet_length(declared_name) BETWEEN 1 AND 128),
    CONSTRAINT artifact_dependencies_version_length
        CHECK (octet_length(declared_version) BETWEEN 1 AND 64)
);

RESET ROLE;
