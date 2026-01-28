package main

import (
	"fmt"
	"time"
	"github.com/weiiwang01/wpex/internal/stats"
)

func debugStatsFormatting() {
	// Crea statistiche di esempio
	vpnStats := stats.NewVPNStats()
	statsData := vpnStats.GetStats()
	
	// Simula gli stessi calcoli della funzione generateStatsHTML
	uptime := time.Since(statsData.StartTime)
	
	connectedPeers := 0
	handshakingPeers := 0
	for _, peer := range statsData.Peers {
		switch peer.Status {
		case stats.PeerStatusConnected:
			connectedPeers++
		case stats.PeerStatusHandshaking:
			handshakingPeers++
		}
	}
	
	successRate := 0.0
	if statsData.TotalHandshakes > 0 {
		successRate = float64(statsData.SuccessfulHandshakes) / float64(statsData.TotalHandshakes) * 100
	}
	
	fmt.Println("🔍 Debug dei tipi di dati per fmt.Sprintf:")
	fmt.Printf("1. time.Now().Format(...): %T = %v\n", time.Now().Format("2006-01-02 15:04:05"), time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("2. connectedPeers: %T = %v\n", connectedPeers, connectedPeers)
	fmt.Printf("3. handshakingPeers: %T = %v\n", handshakingPeers, handshakingPeers)
	fmt.Printf("4. int(stats.ActiveSessions): %T = %v\n", int(statsData.ActiveSessions), int(statsData.ActiveSessions))
	fmt.Printf("5. uptime.String(): %T = %v\n", uptime.String(), uptime.String())
	fmt.Printf("6. int(stats.TotalHandshakes): %T = %v\n", int(statsData.TotalHandshakes), int(statsData.TotalHandshakes))
	fmt.Printf("7. successRate: %T = %v\n", successRate, successRate)
	fmt.Printf("8. int(stats.TotalDataPackets): %T = %v\n", int(statsData.TotalDataPackets), int(statsData.TotalDataPackets))
	fmt.Printf("9. bytes/(1024*1024): %T = %v\n", float64(statsData.TotalBytesTransferred)/(1024*1024), float64(statsData.TotalBytesTransferred)/(1024*1024))
	
	fmt.Println("\n✅ Prova del format string completo:")
	testFormat := `
	Timestamp: %s
	Connected: %d 
	Handshaking: %d
	ActiveSessions: %d
	Uptime: %s
	TotalHandshakes: %d
	SuccessRate: %.1f%%
	DataPackets: %d
	TotalTransfer: %.2f MB`
	
	result := fmt.Sprintf(testFormat,
		time.Now().Format("2006-01-02 15:04:05"),
		connectedPeers,
		handshakingPeers,
		int(statsData.ActiveSessions),
		uptime.String(),
		int(statsData.TotalHandshakes),
		successRate,
		int(statsData.TotalDataPackets),
		float64(statsData.TotalBytesTransferred)/(1024*1024))
		
	fmt.Println(result)
}

func main() {
	debugStatsFormatting()
}