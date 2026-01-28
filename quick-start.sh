#!/bin/bash

# Script di avvio rapido per WPEX con Docker
# Questo script automatizza il setup completo di WPEX con monitoring

set -e

# Colori
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m'

# Banner
echo -e "${BLUE}"
cat << "EOF"
 ██╗    ██╗██████╗ ███████╗██╗  ██╗
 ██║    ██║██╔══██╗██╔════╝╚██╗██╔╝
 ██║ █╗ ██║██████╔╝█████╗   ╚███╔╝ 
 ██║███╗██║██╔═══╝ ██╔══╝   ██╔██╗ 
 ╚███╔███╔╝██║     ███████╗██╔╝ ██╗
  ╚══╝╚══╝ ╚═╝     ╚══════╝╚═╝  ╚═╝
                                   
 WireGuard Packet Exchange Server
 🛡️  con Monitoring Dashboard
EOF
echo -e "${NC}"

log() {
    echo -e "${GREEN}[$(date +'%H:%M:%S')]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[$(date +'%H:%M:%S')] WARNING:${NC} $1"
}

error() {
    echo -e "${RED}[$(date +'%H:%M:%S')] ERROR:${NC} $1"
}

info() {
    echo -e "${BLUE}[$(date +'%H:%M:%S')] INFO:${NC} $1"
}

# Verifica prerequisiti
check_prerequisites() {
    log "🔍 Checking prerequisites..."
    
    if ! command -v docker &> /dev/null; then
        error "Docker non è installato!"
        echo "Installalo da: https://docs.docker.com/get-docker/"
        exit 1
    fi
    
    if ! docker info &> /dev/null; then
        error "Docker non è in esecuzione!"
        echo "Avvia Docker Desktop e riprova."
        exit 1
    fi
    
    log "✅ Prerequisites OK"
}

# Build dell'immagine
build_image() {
    log "🏗️  Building WPEX Docker image..."
    
    WPEX_VERSION=${WPEX_VERSION:-"$(git describe --tags --always 2>/dev/null || echo 'dev-latest')"}
    
    docker build \
        --build-arg WPEX_VERSION="$WPEX_VERSION" \
        --tag wpex:latest \
        . || {
        error "Build fallito!"
        exit 1
    }
    
    log "✅ Image built successfully!"
}

# Setup configurazione
setup_config() {
    log "⚙️  Setting up configuration..."
    
    # Crea directory se non esistono
    mkdir -p data logs
    
    # Copia file env di esempio se non esiste
    if [ ! -f ".env" ]; then
        if [ -f ".env.example" ]; then
            cp .env.example .env
            info "Created .env from template"
        fi
    fi
    
    log "✅ Configuration ready"
}

# Avvio servizi
start_services() {
    log "🚀 Starting WPEX services..."
    
    # Ferma container esistenti
    docker stop wpex-server 2>/dev/null || true
    docker rm wpex-server 2>/dev/null || true
    
    # Avvia nuovo container
    docker run -d \
        --name wpex-server \
        -p 40000:40000/udp \
        -p 8080:8080/tcp \
        -v "$(pwd)/data:/data" \
        -v "$(pwd)/logs:/var/log/wpex" \
        --restart unless-stopped \
        wpex:latest \
        --port 40000 \
        --stats :8080 \
        --broadcast-rate 3 \
        --debug || {
        error "Failed to start container!"
        exit 1
    }
    
    log "✅ WPEX server started!"
}

# Test dei servizi
test_services() {
    log "🧪 Testing services..."
    
    sleep 3
    
    # Test health endpoint
    if curl -sf http://localhost:8080/health >/dev/null; then
        log "✅ Health endpoint OK"
    else
        warn "❌ Health endpoint not reachable yet"
    fi
    
    # Test stats endpoint
    if curl -sf http://localhost:8080/stats >/dev/null; then
        log "✅ Stats endpoint OK"
    else
        warn "❌ Stats endpoint not reachable yet"
    fi
}

# Mostra informazioni finali
show_info() {
    echo
    echo -e "${GREEN}🎉 WPEX è ora in esecuzione!${NC}"
    echo
    echo -e "${PURPLE}📊 Dashboard:${NC}     http://localhost:8080"
    echo -e "${PURPLE}📈 Stats API:${NC}     http://localhost:8080/stats"
    echo -e "${PURPLE}🏥 Health:${NC}        http://localhost:8080/health"
    echo -e "${PURPLE}🔌 VPN Port:${NC}      UDP 40000"
    echo
    echo -e "${BLUE}📋 Useful commands:${NC}"
    echo "  docker logs -f wpex-server    # View logs"
    echo "  docker stop wpex-server       # Stop server"
    echo "  docker start wpex-server      # Start server"
    echo
    echo -e "${YELLOW}⚠️  Configure your WireGuard clients to use:${NC}"
    echo "  Endpoint: $(curl -s ifconfig.me || echo 'YOUR_SERVER_IP'):40000"
    echo
}

# Menu principale
main_menu() {
    echo
    echo -e "${BLUE}What would you like to do?${NC}"
    echo "1) 🏗️  Full setup (build + configure + start)"
    echo "2) 🚀 Quick start (using existing image)"
    echo "3) 🛠️  Build image only"
    echo "4) ⚙️  Configure only"
    echo "5) 📊 Show status"
    echo "6) 🛑 Stop services"
    echo "7) 🧹 Clean up"
    echo "8) ❌ Exit"
    echo
    read -p "Choose option [1-8]: " choice
    
    case $choice in
        1)
            check_prerequisites
            build_image
            setup_config
            start_services
            test_services
            show_info
            ;;
        2)
            check_prerequisites
            setup_config
            start_services
            test_services
            show_info
            ;;
        3)
            check_prerequisites
            build_image
            ;;
        4)
            setup_config
            ;;
        5)
            docker ps --filter name=wpex-server --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
            ;;
        6)
            docker stop wpex-server 2>/dev/null || true
            log "🛑 Services stopped"
            ;;
        7)
            docker stop wpex-server 2>/dev/null || true
            docker rm wpex-server 2>/dev/null || true
            docker rmi wpex:latest 2>/dev/null || true
            log "🧹 Cleanup completed"
            ;;
        8)
            log "👋 Goodbye!"
            exit 0
            ;;
        *)
            error "Invalid choice!"
            main_menu
            ;;
    esac
}

# Esecuzione
if [ $# -eq 0 ]; then
    main_menu
else
    # Execution with parameters
    case "$1" in
        "build")
            check_prerequisites
            build_image
            ;;
        "start")
            check_prerequisites
            setup_config
            start_services
            test_services
            show_info
            ;;
        "stop")
            docker stop wpex-server 2>/dev/null || true
            log "🛑 Services stopped"
            ;;
        "logs")
            docker logs -f wpex-server
            ;;
        "status")
            docker ps --filter name=wpex-server --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
            ;;
        *)
            echo "Usage: $0 [build|start|stop|logs|status]"
            exit 1
            ;;
    esac
fi