\set ON_ERROR_STOP on

BEGIN;

INSERT INTO persistence_spike.repositories(repository_id, lifecycle_state)
VALUES ('00000000-0000-4000-8000-000000000001', 'active');

INSERT INTO persistence_spike.scans(scan_id, repository_id, lifecycle_state)
VALUES (
  '00000000-0000-4000-8000-000000000002',
  '00000000-0000-4000-8000-000000000001',
  'succeeded'
);

INSERT INTO persistence_spike.publications(
  scan_id,
  repository_id,
  lifecycle_state,
  artifact_set_digest,
  artifact_count,
  published_at
) VALUES (
  '00000000-0000-4000-8000-000000000002',
  '00000000-0000-4000-8000-000000000001',
  'succeeded',
  decode(repeat('11', 32), 'hex'),
  2,
  clock_timestamp()
);

WITH payload AS (
  SELECT representation, payload_digest, payload_size
  FROM persistence_spike.payloads
  WHERE representation = 'chunk-1m-core'
  ORDER BY payload_size
  LIMIT 1
)
INSERT INTO persistence_spike.artifact_envelopes(
  artifact_id,
  scan_id,
  artifact_name,
  artifact_version,
  representation,
  payload_digest,
  payload_size
)
SELECT artifact_id,
       '00000000-0000-4000-8000-000000000002'::uuid,
       artifact_name,
       '1.0.0',
       payload.representation,
       payload.payload_digest,
       payload.payload_size
FROM payload
CROSS JOIN (VALUES
  ('00000000-0000-4000-8000-000000000003'::uuid, 'scale-consumer'),
  ('00000000-0000-4000-8000-000000000004'::uuid, 'scale-source')
) AS artifact(artifact_id, artifact_name);

INSERT INTO persistence_spike.artifact_dependencies(
  scan_id,
  consumer_artifact_id,
  dependency_ordinal,
  source_artifact_id
)
SELECT '00000000-0000-4000-8000-000000000002'::uuid,
       '00000000-0000-4000-8000-000000000003'::uuid,
       ordinal,
       '00000000-0000-4000-8000-000000000004'::uuid
FROM generate_series(0, 999999) AS ordinal;

UPDATE persistence_spike.repositories
SET current_scan_id = '00000000-0000-4000-8000-000000000002'
WHERE repository_id = '00000000-0000-4000-8000-000000000001';

COMMIT;

ANALYZE persistence_spike.artifact_dependencies;
ANALYZE persistence_spike.artifact_envelopes;
ANALYZE persistence_spike.publications;
