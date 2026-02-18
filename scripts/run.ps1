# run.ps1 - Скрипт запуска SaaSPro 3.0 Enhanced

param(
    [string]$Mode = "dev",
    [switch]$Docker,
    [switch]$Build,
    [switch]$Help
)

if ($Help) {
    Write-Host "Использование:" -ForegroundColor Green
    Write-Host "  .\run.ps1 [-Mode dev|prod] [-Docker] [-Build] [-Help]" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "Опции:" -ForegroundColor Green
    Write-Host "  -Mode     : dev (по умолчанию) или prod" -ForegroundColor Yellow
    Write-Host "  -Docker   : запуск через Docker" -ForegroundColor Yellow
    Write-Host "  -Build    : сборка проекта" -ForegroundColor Yellow
    Write-Host "  -Help     : показать эту справку" -ForegroundColor Yellow
    exit
}

Write-Host "🚀 SaaSPro 3.0 Enhanced" -ForegroundColor Cyan
Write-Host "========================" -ForegroundColor Cyan

# Режим запуска
if ($Mode -eq "prod") {
    $env:GIN_MODE = "release"
    Write-Host "⚡ Режим: ПРОДАКШЕН" -ForegroundColor Red
} else {
    $env:GIN_MODE = "debug"
    Write-Host "🔧 Режим: РАЗРАБОТКА" -ForegroundColor Green
}

# Проверка зависимостей
if ($Build) {
    Write-Host "📦 Сборка проекта..." -ForegroundColor Yellow
    go mod tidy
    go build -o saaspro.exe
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✅ Сборка успешна" -ForegroundColor Green
    } else {
        Write-Host "❌ Ошибка сборки" -ForegroundColor Red
        exit 1
    }
}

# Запуск через Docker
if ($Docker) {
    Write-Host "🐳 Запуск через Docker Compose..." -ForegroundColor Cyan
    if (Test-Path "docker-compose.yml") {
        docker-compose down
        docker-compose up -d --build
        Write-Host "✅ Docker контейнеры запущены" -ForegroundColor Green
        Write-Host "📊 Приложение доступно по: http://localhost:8080" -ForegroundColor Yellow
        Write-Host "📈 Метерики: http://localhost:8080/api/v1/metrics" -ForegroundColor Yellow
    } else {
        Write-Host "❌ Файл docker-compose.yml не найден" -ForegroundColor Red
    }
    exit
}

# Проверка папки логов
if (!(Test-Path "logs")) {
    New-Item -ItemType Directory -Path "logs" | Out-Null
    Write-Host "📁 Создана папка logs" -ForegroundColor Green
}

# Запуск приложения
Write-Host "🚀 Запуск приложения..." -ForegroundColor Cyan
go run main.go health.go rate_limiter.go logger.go
