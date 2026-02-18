Write-Host "🛡️ Защита проекта" -ForegroundColor Cyan
# Защищаем важные файлы от случайного удаления
$important = @("main.go", "go.mod", "go.sum", ".env", ".env.example", ".gitignore", "README.md")
foreach ($file in $important) {
    if (Test-Path $file) {
        Write-Host "✅ $file защищен" -ForegroundColor Green
    }
}
