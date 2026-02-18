Write-Host "🔍 Проверка проекта:" -ForegroundColor Cyan
$files = @("main.go", "go.mod", "go.sum", ".env", "templates/")
foreach ($file in $files) {
    Write-Host "$(if (Test-Path $file) {'✅'} else {'❌'}) $file" -ForegroundColor $(if (Test-Path $file) {'Green'} else {'Red'})
}
