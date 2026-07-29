param(
    [Parameter(Mandatory = $true)]
    [ValidateCount(2, 32)]
    [string[]]$Paths
)

$ErrorActionPreference = 'Stop'
$results = foreach ($path in $Paths) {
    $resolved = (Resolve-Path -LiteralPath $path).Path
    $value = Get-Content -LiteralPath $resolved -Raw | ConvertFrom-Json
    [pscustomobject]@{
        Path       = $resolved
        Case       = $value.case
        Pass       = $value.pass
        Source     = $value.source_manifest_sha256
        Normalized = $value.normalized_sha256
        Artifacts  = $value.artifact_count
    }
}

$caseNames = @($results | Select-Object -ExpandProperty Case -Unique)
$sources = @($results | Select-Object -ExpandProperty Source -Unique)
$digests = @($results | Select-Object -ExpandProperty Normalized -Unique)
$counts = @($results | Select-Object -ExpandProperty Artifacts -Unique)

$results | Format-Table Case, Pass, Artifacts, Source, Normalized -AutoSize

if ($caseNames.Count -ne 1) { throw 'results belong to different validation cases' }
if ($sources.Count -ne 1) { throw 'canonical source manifests differ' }
if ($counts.Count -ne 1) { throw 'artifact counts differ' }
if ($digests.Count -ne 1) { throw 'normalized durable outcomes differ' }

Write-Host "Phase 4.0.7 deterministic comparison passed: $($digests[0])"
