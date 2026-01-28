# Script PowerShell per buildare l'immagine Docker di WPEX con monitoring

param(
    [string]$Tag = "latest",
    [switch]$Push = $false,
    [switch]$Help = $false
)

if ($Help) {
    Write-Host @"
🐳 WPEX Docker Build Script

USAGE:
    .\build-docker.ps1 [-Tag <tag>] [-Push] [-Help]

PARAMETERS:
    -Tag     Tag per l'immagine Docker (default: latest)
    -Push    Push dell'immagine su registry dopo il build
    -Help    Mostra questo help

EXAMPLES:
    .\build-docker.ps1
    .\build-docker.ps1 -Tag v1.0.0
    .\build-docker.ps1 -Tag latest -Push

"@
    exit 0
}

# Configurazione
$ImageName = "wpex"
$FullImageName = "${ImageName}:${Tag}"

# Ottieni versione da Git o usa timestamp
try {
    $WpexVersion = (git describe --tags --always 2>$null).Trim()
    if ([string]::IsNullOrEmpty($WpexVersion)) {
        $WpexVersion = "dev-$(Get-Date -Format 'yyyyMMdd-HHmmss')"
    }
} catch {
    $WpexVersion = "dev-$(Get-Date -Format 'yyyyMMdd-HHmmss')"
}

Write-Host "🏗️  Building WPEX Docker image..." -ForegroundColor Blue
Write-Host "📦 Image: $FullImageName" -ForegroundColor Cyan
Write-Host "🏷️  Version: $WpexVersion" -ForegroundColor Cyan

# Verifica Docker
if (!(Get-Command docker -ErrorAction SilentlyContinue)) {
    Write-Host "❌ Docker non è installato o non è nel PATH" -ForegroundColor Red
    exit 1
}

# Verifica Dockerfile
if (!(Test-Path "Dockerfile")) {
    Write-Host "❌ Dockerfile non trovato. Esegui questo script dalla root del progetto WPEX." -ForegroundColor Red
    exit 1
}

# Build dell'immagine
Write-Host "🔨 Building Docker image..." -ForegroundColor Yellow

$buildArgs = @(
    "build"
    "--build-arg", "WPEX_VERSION=$WpexVersion"
    "--tag", $FullImageName
    "."
)

try {
    & docker $buildArgs
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✅ Immagine Docker creata con successo!" -ForegroundColor Green
        Write-Host "📦 Immagine: $FullImageName" -ForegroundColor Cyan
        Write-Host "🏷️  Versione: $WpexVersion" -ForegroundColor Cyan
        
        # Mostra informazioni sull'immagine
        Write-Host "`n📊 Informazioni immagine:" -ForegroundColor Blue
        docker images $ImageName --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}\t{{.CreatedAt}}"
        
        Write-Host "`n🚀 Per avviare il container:" -ForegroundColor Green
        Write-Host "   docker run -d -p 40000:40000/udp -p 8080:8080 $FullImageName" -ForegroundColor White
        
        Write-Host "`n🐳 Oppure usa Docker Compose:" -ForegroundColor Green
        Write-Host "   docker-compose up -d" -ForegroundColor White
        
        Write-Host "`n🌐 Dashboard sarà disponibile su: http://localhost:8080" -ForegroundColor Green
        
        # Push opzionale
        if ($Push) {
            Write-Host "`n📤 Pushing image to registry..." -ForegroundColor Yellow
            docker push $FullImageName
            if ($LASTEXITCODE -eq 0) {
                Write-Host "✅ Push completato!" -ForegroundColor Green
            } else {
                Write-Host "❌ Push fallito!" -ForegroundColor Red
            }
        }
        
    } else {
        Write-Host "❌ Build fallito!" -ForegroundColor Red
        exit 1
    }
    
} catch {
    Write-Host "❌ Errore durante il build: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}