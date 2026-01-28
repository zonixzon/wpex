#!/bin/bash

# Script per buildare l'immagine Docker di WPEX con monitoring
set -e

# Colori per output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Funzioni helper
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Configurazione
IMAGE_NAME="wpex"
TAG=${1:-"latest"}
FULL_IMAGE_NAME="${IMAGE_NAME}:${TAG}"
WPEX_VERSION=${WPEX_VERSION:-$(git describe --tags --always 2>/dev/null || echo "dev-$(date +%Y%m%d)")}

log_info "🏗️  Building WPEX Docker image..."
log_info "Image: ${FULL_IMAGE_NAME}"
log_info "Version: ${WPEX_VERSION}"

# Verifica che Docker sia installato
if ! command -v docker &> /dev/null; then
    log_error "Docker non è installato o non è nel PATH"
    exit 1
fi

# Verifica che siamo nella directory corretta
if [ ! -f "Dockerfile" ]; then
    log_error "Dockerfile non trovato. Esegui questo script dalla root del progetto WPEX."
    exit 1
fi

# Build dell'immagine
log_info "🔨 Building Docker image..."
docker build \
    --build-arg WPEX_VERSION="${WPEX_VERSION}" \
    --tag "${FULL_IMAGE_NAME}" \
    --platform linux/amd64,linux/arm64 \
    .

if [ $? -eq 0 ]; then
    log_success "✅ Immagine Docker creata con successo!"
    log_info "📦 Immagine: ${FULL_IMAGE_NAME}"
    log_info "🏷️  Versione: ${WPEX_VERSION}"
    
    # Mostra informazioni sull'immagine
    log_info "📊 Informazioni immagine:"
    docker images "${IMAGE_NAME}" --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}\t{{.CreatedAt}}"
    
    echo ""
    log_info "🚀 Per avviare il container:"
    echo "   docker run -d -p 40000:40000/udp -p 8080:8080 ${FULL_IMAGE_NAME}"
    echo ""
    log_info "🐳 Oppure usa Docker Compose:"
    echo "   docker-compose up -d"
    echo ""
    log_info "🌐 Dashboard sarà disponibile su: http://localhost:8080"
    
else
    log_error "❌ Build fallito!"
    exit 1
fi