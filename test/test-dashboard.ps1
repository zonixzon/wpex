# 🧪 WPEX Dashboard Test Script
# 
# Questo script avvia WPEX con il server di statistiche e testa il dashboard

param(
    [string]$Action = "help"
)

function Show-Help {
    Write-Host "🧪 WPEX Dashboard Test Utility" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "Usage:" -ForegroundColor Yellow
    Write-Host "  .\test-dashboard.ps1 start    - Avvia WPEX con stats server"
    Write-Host "  .\test-dashboard.ps1 test     - Testa il dashboard"
    Write-Host "  .\test-dashboard.ps1 demo     - Esegue demo delle statistiche" 
    Write-Host "  .\test-dashboard.ps1 stop     - Ferma WPEX"
    Write-Host "  .\test-dashboard.ps1 help     - Mostra questo aiuto"
    Write-Host ""
}

function Start-WPEX {
    Write-Host "🚀 Starting WPEX with stats server..." -ForegroundColor Green
    Write-Host "📊 Stats will be available at: http://localhost:8080/" -ForegroundColor Cyan
    Write-Host "⚠️ Press Ctrl+C to stop the server" -ForegroundColor Yellow
    Write-Host ""
    
    # Avvia WPEX in modalità test locale
    & "..\wpex.exe" --stats ":8080" ":9999"
}

function Test-Dashboard {
    Write-Host "🧪 Testing WPEX Dashboard..." -ForegroundColor Green
    go run dashboard_demo.go
}

function Run-Demo {
    Write-Host "📊 Running statistics demo..." -ForegroundColor Green
    Set-Location "..\example"
    go run stats_demo.go
    Set-Location "..\test"
}

function Stop-WPEX {
    Write-Host "🛑 Stopping WPEX processes..." -ForegroundColor Red
    Get-Process -Name "wpex" -ErrorAction SilentlyContinue | Stop-Process -Force
    Write-Host "✅ WPEX stopped" -ForegroundColor Green
}

switch ($Action.ToLower()) {
    "start" { Start-WPEX }
    "test" { Test-Dashboard }  
    "demo" { Run-Demo }
    "stop" { Stop-WPEX }
    "help" { Show-Help }
    default { Show-Help }
}