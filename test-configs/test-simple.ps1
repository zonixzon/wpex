# Test semplice per WPEX
param([string]$Action = "help")

function Test-WPEX {
    Write-Host "Test connettivita WPEX..." -ForegroundColor Yellow
    
    try {
        $health = Invoke-RestMethod -Uri "http://localhost:8080/health"
        Write-Host "WPEX Health: $($health.status)" -ForegroundColor Green
        Write-Host "Uptime: $($health.uptime)" -ForegroundColor Cyan
    }
    catch {
        Write-Host "WPEX non risponde!" -ForegroundColor Red
        return
    }
    
    Write-Host "Simulazione traffico..." -ForegroundColor Yellow
    
    for ($i = 1; $i -le 5; $i++) {
        try {
            $udpClient = New-Object System.Net.Sockets.UdpClient
            $bytes = [System.Text.Encoding]::UTF8.GetBytes("TEST_PACKET_$i")
            $endpoint = New-Object System.Net.IPEndPoint([System.Net.IPAddress]::Parse("127.0.0.1"), 40000)
            $bytesSent = $udpClient.Send($bytes, $bytes.Length, $endpoint)
            Write-Host "Inviati $bytesSent bytes (packet $i)" -ForegroundColor Green
            $udpClient.Close()
            Start-Sleep -Milliseconds 500
        }
        catch {
            Write-Host "Errore packet $i" -ForegroundColor Red
        }
    }
}

function Get-Stats {
    try {
        $stats = Invoke-RestMethod -Uri "http://localhost:8080/stats"
        Write-Host "Statistiche WPEX:" -ForegroundColor Green
        $stats | ConvertTo-Json -Depth 3
    }
    catch {
        Write-Host "Errore statistiche!" -ForegroundColor Red
    }
}

switch ($Action) {
    "test" { Test-WPEX; Start-Sleep 2; Get-Stats }
    "stats" { Get-Stats }
    default { 
        Write-Host "Uso: .\test-simple.ps1 test|stats"
        Write-Host "  test  - Test completo"
        Write-Host "  stats - Solo statistiche"
    }
}