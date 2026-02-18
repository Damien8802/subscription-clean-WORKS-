Write-Host "=== БЕЗОПАСНОЕ УДАЛЕНИЕ ДУБЛИКАТОВ ШАБЛОНОВ ===" -ForegroundColor Cyan

# Создаем backup папку
$backupDir = "backup_templates_$(Get-Date -Format 'yyyyMMdd_HHmmss')"
mkdir $backupDir

Write-Host "Backup папка: $backupDir" -ForegroundColor Gray

# Находим явные дубликаты (одна версия используется, другая - нет)
$duplicatePairs = @(
    @{Used = "my-subscriptions.html"; Unused = "my_subscriptions.html"},
    @{Used = "payment-success.html"; Unused = "payment_success.html"},
    @{Used = "usdt-payment.html"; Unused = "usdt_payment.html"},
    @{Used = "realtime-dashboard.html"; Unused = "realtime_dashboard.html"},
    @{Used = "security-monitor.html"; Unused = "security_monitor.html"},
    @{Used = "rub-payment.html"; Unused = "rub_payment.html"}
)

Write-Host "`n🔍 Поиск дубликатов для удаления:" -ForegroundColor Yellow

foreach ($pair in $duplicatePairs) {
    $usedPath = "templates\$($pair.Used)"
    $unusedPath = "templates\$($pair.Unused)"
    
    if (Test-Path $usedPath -and Test-Path $unusedPath) {
        # Проверяем, что действительно неиспользуемый файл
        $usedInCode = Select-String -Path "handlers\*.go", "main.go" -Pattern "`"$($pair.Unused)`"" -List
        
        if (-not $usedInCode) {
            Write-Host "  ✅ $($pair.Unused) → $($pair.Used)" -ForegroundColor Green
            Write-Host "     Копируем в backup и удаляем..." -ForegroundColor Gray
            
            # Копируем в backup
            Copy-Item $unusedPath "$backupDir\$($pair.Unused)"
            # Удаляем дубликат
            Remove-Item $unusedPath
            
            Write-Host "     Удален: $($pair.Unused)" -ForegroundColor Green
        } else {
            Write-Host "  ⚠️  $($pair.Unused) используется в коде, пропускаем" -ForegroundColor Yellow
        }
    } else {
        if (-not (Test-Path $usedPath)) {
            Write-Host "  ❌ Основной файл отсутствует: $($pair.Used)" -ForegroundColor Red
        }
        if (-not (Test-Path $unusedPath)) {
            Write-Host "  ℹ️  Дубликат уже удален: $($pair.Unused)" -ForegroundColor Gray
        }
    }
}

# Проверяем пустые или битые файлы
Write-Host "`n🔍 Проверка пустых/битых файлов:" -ForegroundColor Yellow

Get-ChildItem "templates" -File -Filter "*.html" | ForEach-Object {
    $content = Get-Content $_.FullName -Raw
    if ([string]::IsNullOrWhiteSpace($content) -or $_.Length -lt 10) {
        Write-Host "  ⚠️  Подозрительный файл: $($_.Name) (размер: $($_.Length) байт)" -ForegroundColor Yellow
        # Копируем в backup на всякий случай
        Copy-Item $_.FullName "$backupDir\$($_.Name)"
    }
}

Write-Host "`n📊 ИТОГ:" -ForegroundColor Cyan
$totalBefore = 67  # Из вывода сервера
$totalAfter = (Get-ChildItem "templates" -File -Filter "*.html").Count
Write-Host "  Было: $totalBefore шаблонов" -ForegroundColor Gray
Write-Host "  Стало: $totalAfter шаблонов" -ForegroundColor Green
Write-Host "  Удалено: $($totalBefore - $totalAfter) дубликатов" -ForegroundColor Green
Write-Host "  Backup сохранен в: $backupDir" -ForegroundColor Gray

Write-Host "`n🎯 Дубликаты удалены безопасно!" -ForegroundColor Cyan
