# backup.ps1 - Бэкап с паролем
Write-Host "📀 ЗАПУСК БЭКАПА" -ForegroundColor Cyan
Write-Host "=========================" -ForegroundColor Cyan

$date = Get-Date -Format "yyyy-MM-dd-HHmmss"
$backupDir = "C:\Projects\backups\subscription-clean"
$backupFile = "$backupDir\backup_$date.sql"

# Создаем папку если нет
New-Item -ItemType Directory -Force -Path $backupDir | Out-Null

Write-Host "📀 Создание бэкапа базы данных GO..." -ForegroundColor Yellow

# Путь к pg_dump для PostgreSQL 12
$pgDump = "C:\Program Files\PostgreSQL\12\bin\pg_dump.exe"

if (-not (Test-Path $pgDump)) {
    Write-Host "❌ pg_dump не найден по пути: $pgDump" -ForegroundColor Red
    pause
    exit
}

Write-Host "✅ Найден pg_dump: $pgDump" -ForegroundColor Green

# Пароль из .env
$env:PGPASSWORD = "6213110"

# Создаем бэкап
Write-Host "🔧 Выполняется бэкап базы данных..." -ForegroundColor Yellow
& $pgDump -U postgres -h localhost -d GO -f $backupFile 2>&1

if (Test-Path $backupFile) {
    $size = [math]::Round((Get-Item $backupFile).Length / 1KB, 2)
    Write-Host "✅ Бэкап создан: $backupFile" -ForegroundColor Green
    Write-Host "   Размер: $size KB" -ForegroundColor Green
    
    # Сжимаем
    Write-Host "📦 Сжатие бэкапа..." -ForegroundColor Yellow
    Compress-Archive -Path $backupFile -DestinationPath "$backupFile.zip" -Force
    Remove-Item $backupFile
    
    $zipSize = [math]::Round((Get-Item "$backupFile.zip").Length / 1KB, 2)
    Write-Host "✅ Бэкап сжат: $backupFile.zip ($zipSize KB)" -ForegroundColor Green
    
    # Удаляем старые бэкапы (старше 7 дней)
    Write-Host "🗑️ Очистка старых бэкапов..." -ForegroundColor Yellow
    $oldBackups = Get-ChildItem $backupDir -Filter "*.zip" | Where-Object { $_.CreationTime -lt (Get-Date).AddDays(-7) }
    if ($oldBackups) {
        $oldBackups | Remove-Item -Force
        Write-Host "✅ Удалено $($oldBackups.Count) старых бэкапов" -ForegroundColor Green
    } else {
        Write-Host "✅ Старых бэкапов не найдено" -ForegroundColor Green
    }
    
    Write-Host ""
    Write-Host "📊 СТАТИСТИКА:" -ForegroundColor Cyan
    Write-Host "   Папка: $backupDir" -ForegroundColor Gray
    Write-Host "   Файлов: $( (Get-ChildItem $backupDir -Filter "*.zip").Count )" -ForegroundColor Gray
    $totalSize = [math]::Round((Get-ChildItem $backupDir -Filter "*.zip" | Measure-Object -Property Length -Sum).Sum / 1MB, 2)
    Write-Host "   Общий размер: $totalSize MB" -ForegroundColor Gray
    Write-Host ""
    Write-Host "✅ БЭКАП ЗАВЕРШЕН УСПЕШНО!" -ForegroundColor Green
} else {
    Write-Host "❌ ОШИБКА: Бэкап не создан!" -ForegroundColor Red
    Write-Host "Проверьте:" -ForegroundColor Yellow
    Write-Host "  1. Запущен ли PostgreSQL?" -ForegroundColor Yellow
    Write-Host "  2. Правильный ли пароль (6213110)?" -ForegroundColor Yellow
    Write-Host "  3. Существует ли база GO?" -ForegroundColor Yellow
}

# Очищаем пароль из переменной окружения
$env:PGPASSWORD = ""
pause
