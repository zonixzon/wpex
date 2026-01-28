# Script PowerShell per testare WPEX su Windows
# Simula traffico UDP per testare il relay

Write-Host "WPEX Test Script per Windows" -ForegroundColor Green
Write-Host "===============================" -ForegroundColor Green

# Funzione per inviare traffico UDP di test
function Send-UDPTraffic {
    param(
        [string]$SourceIP,
        [string]$DestIP,
        [int]$Port = 40000,
        [string]$Message = "Test WPEX relay"
    )
    
    Write-Host "Invio traffico UDP da $SourceIP a ${DestIP}:$Port" -ForegroundColor Yellow
    
    try {
        $udpClient = New-Object System.Net.Sockets.UdpClient
        $udpClient.Client.Bind([System.Net.IPEndPoint]::new([System.Net.IPAddress]::Parse($SourceIP), 0))
        
        $bytes = [System.Text.Encoding]::UTF8.GetBytes($Message)
        $endpoint = New-Object System.Net.IPEndPoint([System.Net.IPAddress]::Parse($DestIP), $Port)
        
        $bytesSent = $udpClient.Send($bytes, $bytes.Length, $endpoint)
        Write-Host "Inviati $bytesSent bytes" -ForegroundColor Green
        
        $udpClient.Close()
    }
    catch {
        Write-Host "Errore invio UDP: $($_.Exception.Message)" -ForegroundColor Red
    }
}

# Funzione per monitorare le statistiche WPEX
function Get-WPEXStats {
    Write-Host "📊 Recupero statistiche WPEX..." -ForegroundColor Yellow
    
    try {
        $response = Invoke-RestMethod -Uri "http://localhost:8080/stats" -Method GET
        Write-Host "✅ Statistiche WPEX:" -ForegroundColor Green
        $response | ConvertTo-Json -Depth 3 | Write-Host
    }
    catch {
        Write-Host "❌ Errore nel recupero statistiche: $($_.Exception.Message)" -ForegroundColor Red
    }
}

# Funzione per testare la connettività WPEX
function Test-WPEXConnectivity {
    Write-Host "🔍 Test connettività WPEX..." -ForegroundColor Yellow
    
    # Test health check
    try {
        $health = Invoke-RestMethod -Uri "http://localhost:8080/health" -Method GET
        Write-Host "✅ WPEX Health Check: $($health.status)" -ForegroundColor Green
        Write-Host "⏱️  Uptime: $($health.uptime)" -ForegroundColor Cyan
    }
    catch {
        Write-Host "❌ WPEX non risponde: $($_.Exception.Message)" -ForegroundColor Red
        return
    }
    
    # Simula traffico WireGuard (pacchetti UDP verso porta 40000)
    Write-Host "🔄 Simulazione traffico WireGuard..." -ForegroundColor Yellow
    
    # Simula handshake iniziale
    Send-UDPTraffic -SourceIP "127.0.0.1" -DestIP "127.0.0.1" -Port 40000 -Message "WG_HANDSHAKE_INIT"
    Start-Sleep -Seconds 1
    
    # Simula risposta handshake
    Send-UDPTraffic -SourceIP "127.0.0.1" -DestIP "127.0.0.1" -Port 40000 -Message "WG_HANDSHAKE_RESP"
    Start-Sleep -Seconds 1
    
    # Simula trasferimento dati
    for ($i = 1; $i -le 5; $i++) {
        Send-UDPTraffic -SourceIP "127.0.0.1" -DestIP "127.0.0.1" -Port 40000 -Message "DATA_PACKET_$i"
        Start-Sleep -Milliseconds 500
    }
    
    Write-Host "✅ Simulazione traffico completata" -ForegroundColor Green
}

# Menu principale
param(
    [string]$Action = "help"
)

switch ($Action.ToLower()) {
    "test" {
        Test-WPEXConnectivity
        Start-Sleep -Seconds 2
        Get-WPEXStats
    }
    "stats" {
        Get-WPEXStats
    }
    "traffic" {
        Write-Host "🚀 Invio traffico continuo per 30 secondi..." -ForegroundColor Yellow
        $endTime = (Get-Date).AddSeconds(30)
        $counter = 1
        
        while ((Get-Date) -lt $endTime) {
            Send-UDPTraffic -SourceIP "127.0.0.1" -DestIP "127.0.0.1" -Port 40000 -Message "CONTINUOUS_PACKET_$counter"
            $counter++
            Start-Sleep -Milliseconds 1000
        }
        
        Write-Host "✅ Traffico continuo completato" -ForegroundColor Green
        Get-WPEXStats
    }
    "help" {
        Write-Host "Uso: .\test-wpex.ps1 -Action <comando>" -ForegroundColor Cyan
        Write-Host ""
        Write-Host "Comandi disponibili:" -ForegroundColor White
        Write-Host "  test     - Esegue test completo di connettività" -ForegroundColor Yellow
        Write-Host "  stats    - Mostra solo le statistiche WPEX" -ForegroundColor Yellow
        Write-Host "  traffic  - Invia traffico continuo per 30 secondi" -ForegroundColor Yellow
        Write-Host "  help     - Mostra questo aiuto" -ForegroundColor Yellow
        Write-Host ""
        Write-Host "Esempi:" -ForegroundColor White
        Write-Host "  .\test-wpex.ps1 -Action test" -ForegroundColor Gray
        Write-Host "  .\test-wpex.ps1 -Action stats" -ForegroundColor Gray
    }
    default {
        Write-Host "❌ Comando non riconosciuto: $Action" -ForegroundColor Red
        Write-Host "Usa -Action help per vedere i comandi disponibili" -ForegroundColor Yellow
    }
}