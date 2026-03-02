while($true) {
    Write-Host "🚀 Запуск туннеля для saas-pro.ru на порту 8080..." -ForegroundColor Green
    
    # Используем порт 8080 вместо 80
    $process = Start-Process -NoNewWindow -PassThru -FilePath "ssh" -ArgumentList "-o ServerAliveInterval=30 -R saas-pro.ru:8080:localhost:8080 serveo.net"
    
    # Ждем завершения процесса
    $process.WaitForExit()
    
    Write-Host "⚠️ Туннель упал, перезапуск через 5 секунд..." -ForegroundColor Yellow
    Start-Sleep -Seconds 5
}