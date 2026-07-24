SET lock_timeout = '5s';
SET statement_timeout = '5min';
SET LOCAL ROLE platform_owner;

CREATE TABLE platform.artifact_payloads (
    payload_digest bytea PRIMARY KEY,
    payload_size bigint NOT NULL,
    chunk_size integer NOT NULL,
    chunk_count integer NOT NULL,
    created_at timestamptz NOT NULL,
    CONSTRAINT artifact_payloads_digest_size_unique UNIQUE (payload_digest, payload_size),
    CONSTRAINT artifact_payloads_digest_length CHECK (octet_length(payload_digest) = 32),
    CONSTRAINT artifact_payloads_size CHECK (payload_size BETWEEN 0 AND 8589934592),
    CONSTRAINT artifact_payloads_chunk_size CHECK (chunk_size = 4194304),
    CONSTRAINT artifact_payloads_chunk_count CHECK (
        (payload_size = 0 AND chunk_count = 0)
        OR (
            payload_size > 0
            AND chunk_count BETWEEN 1 AND 2048
            AND chunk_count = ((payload_size + chunk_size - 1) / chunk_size)
        )
    )
);

CREATE TABLE platform.artifact_payload_chunks (
    payload_digest bytea NOT NULL,
    chunk_ordinal integer NOT NULL,
    chunk_bytes bytea NOT NULL,
    PRIMARY KEY (payload_digest, chunk_ordinal),
    CONSTRAINT artifact_payload_chunks_payload_fk FOREIGN KEY (payload_digest)
        REFERENCES platform.artifact_payloads (payload_digest) ON DELETE CASCADE,
    CONSTRAINT artifact_payload_chunks_ordinal CHECK (chunk_ordinal BETWEEN 0 AND 2047),
    CONSTRAINT artifact_payload_chunks_size
        CHECK (octet_length(chunk_bytes) BETWEEN 1 AND 4194304)
);

RESET ROLE;
