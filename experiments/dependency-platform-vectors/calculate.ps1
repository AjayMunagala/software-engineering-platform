# Independent specification calculator. No backend imports, production encoder,
# database, network, or file writes. Emits JSON to stdout for review/checking.
$ErrorActionPreference = 'Stop'
$utf8 = [System.Text.UTF8Encoding]::new($false, $true)
function Hex([byte[]] $bytes) { [Convert]::ToHexString($bytes).ToLowerInvariant() }
function Hash([byte[]] $bytes) { [System.Security.Cryptography.SHA256]::HashData($bytes) }
function U([uint64] $value) {
    $bytes = [BitConverter]::GetBytes($value)
    if ([BitConverter]::IsLittleEndian) { [Array]::Reverse($bytes) }
    return ,$bytes
}
function S([string] $value) {
    $bytes = $utf8.GetBytes($value)
    return ,([byte[]]((U $bytes.Length) + $bytes))
}
function Frame($parts) {
    $stream = [System.IO.MemoryStream]::new()
    try { foreach ($part in $parts) { $stream.Write($part, 0, $part.Length) }; return ,$stream.ToArray() }
    finally { $stream.Dispose() }
}
$empty = '{"artifact":{"name":"dependency-inventory","version":"0.1.0","engine_name":"dependency-intelligence-core","engine_version":"0.1.0","node_id_scheme_version":"dependency-node-id/v1","edge_id_scheme_version":"dependency-edge-id/v1","containment_id_scheme_version":"dependency-containment-id/v1"},"source_artifacts":[],"nodes":[],"containment":[],"dependencies":[],"strong_components":[],"cycles":[],"diagnostics":[],"statistics":{"nodes_by_kind":{},"edges_by_graph":{},"edges_by_resolution":{},"diagnostics":0,"omitted_diagnostics":0,"omitted_nodes":0,"omitted_containment":0,"omitted_edges":0,"omitted_evidence":0}}'
if ($utf8.GetByteCount($empty) -ne 613) { throw 'Frozen empty fixture length mismatch' }
$unicode = $empty.Replace('"source_artifacts":[]','"source_artifacts":[{"name":"fixture/π\u003c\u0026","version":"1.0.0"}]').Replace('"omitted_edges":0','"omitted_edges":1')
# Reuse the already frozen base-input digest; do not regenerate accepted vectors.
$analysis = ',"analysis":{"engine_name":"dependency-graph-analysis","engine_version":"0.1.0","input_digest":"dependency-analysis-input/v1:sha256:27f88fcd44316dd1e7b7b93288fc9a98cb212d1f8cb82c315c537732c66060b2","digest_scheme":"dependency-analysis-input/v1","scc_id_scheme":"dependency-scc-id/v1","cycle_id_scheme":"dependency-cycle-id/v1","projection_policy":"dependency-local-projection/v1","graphs":['
$graphs = foreach ($kind in @('file','module','package')) {
    '{"graph":"' + $kind + '","eligible_nodes":0,"eligible_edges":0,"components":0,"cyclic_components":0,"boundary_edges":0,"topology_limited":false,"explanation_limited":false,"reason_codes":[]}'
}
$analyzed = $empty.Substring(0,$empty.Length-1) + $analysis + ($graphs -join ',') + ']}}'
$escaping = $empty.Replace('"diagnostics":[]','"diagnostics":[{"code":"fixture","message":"π\u003c\u003e\u0026\u2028\u2029\"\\\b\f\n\r\t\u0000"}]').Replace('"diagnostics":0','"diagnostics":1')
$counts = $empty.Replace('"nodes_by_kind":{}','"nodes_by_kind":{"external_package":0,"file":9007199254740993,"module":18446744073709551615}').Replace('"omitted_evidence":0','"omitted_evidence":18446744073709551615')
$evidence = '{"source":{"artifact_name":"fixture","artifact_version":"1.0.0","source_id":"x"},"rule":"a"},{"source":{"artifact_name":"fixture","artifact_version":"1.0.0","source_id":"y"},"file":"路径/π.go","start_line":1,"start_column":2,"rule":"b","value":"\u003c\u0026"}'
$node = '{"id":"dependency-node-id/v1:sha256:' + ('a'*64) + '","kind":"file","language":"go","name":"π","qualified_name":"π","repository_path":"路径/π.go","resolution":"resolved_local","source_identity":{"artifact_name":"fixture","artifact_version":"1.0.0","source_id":"x"},"evidence":[' + $evidence + ']}'
$multi = $empty.Replace('"nodes":[]','"nodes":['+$node+']').Replace('"nodes_by_kind":{}','"nodes_by_kind":{"file":1}')
$profileParts = @(
    (S 'dependency-platform-profile/v1'), (S 'dependency-publication'), (S '0.1.0'),
    (S 'dependency-inventory'), (S '0.1.0'), (S 'dependency-canonical-json'), (S '0.1.0'),
    (S 'application/json'), (S 'dependency-platform-artifact-id/v1'),
    (S 'dependency-platform-integration'), (S '0.1.0'), (S 'dependency-platform-manifest/v1'),
    (S 'logical-source-references-only'), (U 1), (U 0), (U 0)
)
$profilePreimage = Frame $profileParts
$profileHash = Hash $profilePreimage
$fixtures = [ordered]@{ empty=$empty; unicode_omission=$unicode; analyzed_empty=$analyzed; escaping=$escaping; uint64_wire_only=$counts; multi_evidence_wire_only=$multi }
$vectors = @()
foreach ($name in $fixtures.Keys) {
    $payload = $utf8.GetBytes($fixtures[$name] + "`n")
    $payloadHash = Hash $payload
    $scope = '11111111-1111-4111-8111-111111111111'
    $repo = '22222222-2222-4222-8222-222222222222'
    $scan = '33333333-3333-4333-8333-333333333333'
    $parent = '44444444-4444-4444-8444-444444444444'
    $revision = 'fixture-π'
    $idPreimage = Frame @((S 'dependency-platform-artifact-id/v1'),(S $scope),(S $repo),(S $scan),(S 'dependency-inventory'),(S '0.1.0'))
    $idHash = Hash $idPreimage
    $idBytes = [byte[]]$idHash[0..15]
    $idBytes[6] = ($idBytes[6] -band 15) -bor 128
    $idBytes[8] = ($idBytes[8] -band 63) -bor 128
    $h = Hex $idBytes
    $uuid = $h.Substring(0,8)+'-'+$h.Substring(8,4)+'-'+$h.Substring(12,4)+'-'+$h.Substring(16,4)+'-'+$h.Substring(20,12)
    $manifestParts = @((S 'dependency-platform-manifest/v1'),(S $scope),(S $repo),(S $scan),$profileHash,(S $revision),(U 1),(S $uuid),(S 'dependency-inventory'),(S '0.1.0'),(S 'dependency-platform-artifact-id/v1'),(S 'dependency-canonical-json'),(S '0.1.0'),(S 'application/json'),(S 'dependency-platform-integration'),(S '0.1.0'),$payloadHash,(U $payload.Length),(U 0),(U 0),(U 0))
    $manifestPreimage = Frame $manifestParts
    $manifestHash = Hash $manifestPreimage
    $children = [ordered]@{}
    foreach ($op in @('begin','stage','publish','fail','cancel')) {
        $preimage = Frame @((S 'dependency-platform-request/v1'),(S $parent),(S $op),(S $scope),(S $repo),(S $scan),$manifestHash)
        $children[$op] = [ordered]@{ preimage_hex=(Hex $preimage); request_id=(Hex (Hash $preimage)) }
    }
    $vectors += [ordered]@{ name=$name; scope_id=$scope; repository_id=$repo; scan_id=$scan; parent_request_id=$parent; source_revision=$revision; payload_utf8=($fixtures[$name]+"`n"); payload_base64=[Convert]::ToBase64String($payload); payload_size=$payload.Length; payload_sha256=(Hex $payloadHash); artifact_preimage_hex=(Hex $idPreimage); artifact_sha256=(Hex $idHash); artifact_uuid=$uuid; manifest_preimage_hex=(Hex $manifestPreimage); manifest_sha256=(Hex $manifestHash); child_operations=$children }
}
# Identity mutations are framing vectors only, not additional payloads.
$mutations = @()
foreach ($field in @('scope','repository','scan','artifact_name','artifact_version')) {
    $values = @('11111111-1111-4111-8111-111111111111','22222222-2222-4222-8222-222222222222','33333333-3333-4333-8333-333333333333','dependency-inventory','0.1.0')
    $position = @('scope','repository','scan','artifact_name','artifact_version').IndexOf($field)
    if ($position -lt 3) { $values[$position] = '55555555-5555-4555-8555-555555555555' } else { $values[$position] += '-changed' }
    $parts = @((S 'dependency-platform-artifact-id/v1')); foreach ($value in $values) { $parts += ,(S $value) }
    $preimage = Frame $parts; $hashValue = Hash $preimage; $b = [byte[]]$hashValue[0..15]
    $b[6]=($b[6] -band 15) -bor 128; $b[8]=($b[8] -band 63) -bor 128; $h=Hex $b
    $mutations += [ordered]@{ changed_field=$field; inputs=$values; preimage_hex=(Hex $preimage); sha256=(Hex $hashValue); uuid=($h.Substring(0,8)+'-'+$h.Substring(8,4)+'-'+$h.Substring(12,4)+'-'+$h.Substring(16,4)+'-'+$h.Substring(20,12)) }
}
[ordered]@{ specification='1b9f4d63fbce5489f11d7dcdbd2d63d77e9ceca8'; calculator='independent PowerShell/.NET; no production imports'; profile_preimage_hex=(Hex $profilePreimage); profile_sha256=(Hex $profileHash); vectors=$vectors; artifact_identity_mutations=$mutations } | ConvertTo-Json -Depth 30
