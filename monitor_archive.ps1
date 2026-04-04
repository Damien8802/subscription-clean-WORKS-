$health = Invoke-RestMethod -Uri "http://localhost:8080/archive/api/health" -Method GET
if (-not $health.is_healthy) {
    $body = @{message = $health.warnings -join "; "} | ConvertTo-Json
    Invoke-RestMethod -Uri "http://localhost:8080/api/notify" -Method POST -Body $body -ContentType "application/json"
}
