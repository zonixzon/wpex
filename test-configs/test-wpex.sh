#!/bin/bash
# Script di test per WPEX - Simula traffico tra due reti

echo "🚀 WPEX Test Script - Simulazione di due reti"
echo "============================================="

# Controlla se WireGuard è installato
if ! command -v wg &> /dev/null; then
    echo "❌ WireGuard non è installato. Installalo prima:"
    echo "   sudo apt update && sudo apt install wireguard"
    exit 1
fi

# Funzione per pulire le interfacce
cleanup() {
    echo "🧹 Pulizia interfacce..."
    sudo wg-quick down peer1 2>/dev/null || true
    sudo wg-quick down peer2 2>/dev/null || true
    echo "✅ Pulizia completata"
}

# Setup interfacce WireGuard
setup_peers() {
    echo "📡 Setup Peer 1 (Rete A: 10.0.1.0/24)..."
    sudo wg-quick up ./peer1.conf
    
    echo "📡 Setup Peer 2 (Rete B: 10.0.2.0/24)..."
    sudo wg-quick up ./peer2.conf
    
    echo "✅ Peer configurati"
}

# Test di connettività
test_connectivity() {
    echo "🔍 Test di connettività..."
    
    # Test ping da Peer 1 a Peer 2
    echo "📤 Ping da 10.0.1.1 a 10.0.2.1..."
    ping -c 3 10.0.2.1 -I 10.0.1.1 || echo "❌ Ping fallito"
    
    # Test ping da Peer 2 a Peer 1
    echo "📤 Ping da 10.0.2.1 a 10.0.1.1..."
    ping -c 3 10.0.1.1 -I 10.0.2.1 || echo "❌ Ping fallito"
}

# Monitor statistiche WPEX
monitor_stats() {
    echo "📊 Statistiche WPEX:"
    curl -s http://localhost:8080/stats | jq . || echo "❌ Errore nel recupero statistiche"
}

# Menu principale
case "$1" in
    "setup")
        cleanup
        setup_peers
        ;;
    "test")
        test_connectivity
        ;;
    "stats")
        monitor_stats
        ;;
    "cleanup")
        cleanup
        ;;
    *)
        echo "Uso: $0 {setup|test|stats|cleanup}"
        echo ""
        echo "Comandi:"
        echo "  setup   - Configura i peer WireGuard"
        echo "  test    - Testa la connettività tra le reti"
        echo "  stats   - Mostra le statistiche WPEX"
        echo "  cleanup - Rimuove le configurazioni"
        exit 1
        ;;
esac