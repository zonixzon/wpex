package main

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/weiiwang01/wpex/internal/stats"
)

// Esempio di utilizzo delle nuove funzionalità di monitoring
func main() {
	// Crea un'istanza delle statistiche VPN
	vpnStats := stats.NewVPNStats()

	// Simula alcuni eventi VPN
	fmt.Println("🔧 Testing WPEX VPN Statistics System...")

	// Simula handshake iniziato
	addr1, _ := net.ResolveUDPAddr("udp", "192.168.1.100:51820")
	addr2, _ := net.ResolveUDPAddr("udp", "192.168.1.101:51821")

	vpnStats.LogHandshakeInitiated(12345, *addr1)
	vpnStats.LogHandshakeInitiated(67890, *addr2)

	// Simula handshake completato
	vpnStats.LogHandshakeCompleted(12345, 67890)

	// Simula trasferimento dati
	for i := 0; i < 10; i++ {
		vpnStats.LogDataTransfer(12345, 67890, 1024, *addr1)
		time.Sleep(100 * time.Millisecond)
	}

	// Mostra statistiche
	fmt.Println("\n📊 Current VPN Statistics:")
	fmt.Printf("Connected Peers: %d\n", vpnStats.GetConnectedPeersCount())
	fmt.Printf("Handshaking Peers: %d\n", vpnStats.GetHandshakingPeersCount())

	// Mostra statistiche periodiche
	vpnStats.LogPeriodicStats()

	// Genera JSON delle statistiche
	jsonStats, err := vpnStats.ToJSON()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\n📋 Statistics JSON:")
	fmt.Println(string(jsonStats))

	// Simula disconnessione
	time.Sleep(2 * time.Second)
	vpnStats.LogPeerDisconnected(12345, "user_requested")

	fmt.Println("\n✅ Test completed successfully!")
	fmt.Println("\n🌐 To enable HTTP statistics server, use:")
	fmt.Println("   wpex --stats :8080")
	fmt.Println("\n📈 Then visit:")
	fmt.Println("   http://localhost:8080/        - Dashboard")
	fmt.Println("   http://localhost:8080/stats   - JSON API")
	fmt.Println("   http://localhost:8080/health  - Health Check")
}
