// Versione di debug della funzione generateStatsHTML per identificare il problema di formattazione

package stats

import (
	"fmt"
	"strings"
	"time"
)

// generateStatsHTMLDebug genera HTML con debug per identificare problemi di formattazione
func generateStatsHTMLDebug(stats VPNStats) string {
	uptime := time.Since(stats.StartTime)
	
	connectedPeers := 0
	handshakingPeers := 0
	for _, peer := range stats.Peers {
		switch peer.Status {
		case PeerStatusConnected:
			connectedPeers++
		case PeerStatusHandshaking:
			handshakingPeers++
		}
	}
	
	successRate := 0.0
	if stats.TotalHandshakes > 0 {
		successRate = float64(stats.SuccessfulHandshakes) / float64(stats.TotalHandshakes) * 100
	}
	
	// Debug: costruiamo passo passo per essere sicuri
	var htmlParts []string
	
	htmlParts = append(htmlParts, `<!DOCTYPE html>
<html>
<head>
    <title>WPEX VPN Server Statistics</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            margin: 0;
            padding: 20px;
            background-color: #f5f5f5;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
            background: white;
            padding: 20px;
            border-radius: 8px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }
        .header {
            border-bottom: 2px solid #007acc;
            padding-bottom: 10px;
            margin-bottom: 20px;
        }
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 15px;
            margin-bottom: 30px;
        }
        .stat-card {
            background: #f8f9fa;
            padding: 15px;
            border-radius: 5px;
            border-left: 4px solid #007acc;
        }
        .stat-value {
            font-size: 24px;
            font-weight: bold;
            color: #007acc;
        }
        .stat-label {
            font-size: 14px;
            color: #666;
            margin-top: 5px;
        }
        .peers-table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 20px;
        }
        .peers-table th,
        .peers-table td {
            padding: 10px;
            text-align: left;
            border-bottom: 1px solid #ddd;
        }
        .peers-table th {
            background-color: #007acc;
            color: white;
        }
        .status-connected {
            color: #28a745;
            font-weight: bold;
        }
        .status-handshaking {
            color: #ffc107;
            font-weight: bold;
        }
        .status-disconnected {
            color: #dc3545;
            font-weight: bold;
        }
        .refresh-btn {
            background: #007acc;
            color: white;
            border: none;
            padding: 10px 20px;
            border-radius: 5px;
            cursor: pointer;
            margin-bottom: 20px;
        }
        .refresh-btn:hover {
            background: #005999;
        }
    </style>
    <script>
        function refreshPage() {
            location.reload();
        }
        
        // Auto-refresh ogni 30 secondi
        setInterval(refreshPage, 30000);
    </script>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🛡️ WPEX VPN Server Statistics</h1>`)
	
	// Aggiungi timestamp in modo sicuro
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	htmlParts = append(htmlParts, fmt.Sprintf("<p>Server monitoring dashboard - Last updated: %s</p>", timestamp))
	htmlParts = append(htmlParts, `        </div>
        
        <button class="refresh-btn" onclick="refreshPage()">🔄 Refresh</button>
        
        <div class="stats-grid">`)
	
	// Aggiungi ogni stat-card una per volta per evitare errori
	htmlParts = append(htmlParts, fmt.Sprintf(`            <div class="stat-card">
                <div class="stat-value">%d</div>
                <div class="stat-label">Connected Peers</div>
            </div>`, connectedPeers))
	
	htmlParts = append(htmlParts, fmt.Sprintf(`            <div class="stat-card">
                <div class="stat-value">%d</div>
                <div class="stat-label">Handshaking Peers</div>
            </div>`, handshakingPeers))
	
	htmlParts = append(htmlParts, fmt.Sprintf(`            <div class="stat-card">
                <div class="stat-value">%d</div>
                <div class="stat-label">Active Sessions</div>
            </div>`, int(stats.ActiveSessions)))
	
	htmlParts = append(htmlParts, fmt.Sprintf(`            <div class="stat-card">
                <div class="stat-value">%s</div>
                <div class="stat-label">Server Uptime</div>
            </div>`, uptime.String()))
	
	htmlParts = append(htmlParts, fmt.Sprintf(`            <div class="stat-card">
                <div class="stat-value">%d</div>
                <div class="stat-label">Total Handshakes</div>
            </div>`, int(stats.TotalHandshakes)))
	
	htmlParts = append(htmlParts, fmt.Sprintf(`            <div class="stat-card">
                <div class="stat-value">%.1f%%</div>
                <div class="stat-label">Success Rate</div>
            </div>`, successRate))
	
	htmlParts = append(htmlParts, fmt.Sprintf(`            <div class="stat-card">
                <div class="stat-value">%d</div>
                <div class="stat-label">Data Packets</div>
            </div>`, int(stats.TotalDataPackets)))
	
	htmlParts = append(htmlParts, fmt.Sprintf(`            <div class="stat-card">
                <div class="stat-value">%.2f MB</div>
                <div class="stat-label">Total Transfer</div>
            </div>`, float64(stats.TotalBytesTransferred)/(1024*1024)))
	
	htmlParts = append(htmlParts, `        </div>
        
        <h2>📋 Peer Details</h2>
        <table class="peers-table">
            <thead>
                <tr>
                    <th>Index</th>
                    <th>Address</th>
                    <th>Status</th>
                    <th>Connected At</th>
                    <th>Last Seen</th>
                    <th>Bytes Sent</th>
                    <th>Bytes Received</th>
                </tr>
            </thead>
            <tbody>`)
	
	// Aggiungi righe per ogni peer
	for _, peer := range stats.Peers {
		statusClass := "status-disconnected"
		switch peer.Status {
		case PeerStatusConnected:
			statusClass = "status-connected"
		case PeerStatusHandshaking:
			statusClass = "status-handshaking"
		}
		
		peerRow := fmt.Sprintf(`                <tr>
                    <td>%d</td>
                    <td>%s</td>
                    <td><span class="%s">%s</span></td>
                    <td>%s</td>
                    <td>%s</td>
                    <td>%s</td>
                    <td>%s</td>
                </tr>`,
			peer.Index,
			peer.Address,
			statusClass,
			peer.Status.String(),
			peer.ConnectedAt.Format("15:04:05"),
			peer.LastSeen.Format("15:04:05"),
			formatBytes(peer.BytesSent),
			formatBytes(peer.BytesRecv))
		
		htmlParts = append(htmlParts, peerRow)
	}
	
	if len(stats.Peers) == 0 {
		htmlParts = append(htmlParts, `                <tr>
                    <td colspan="7" style="text-align: center; color: #666; font-style: italic;">
                        No peers currently tracked
                    </td>
                </tr>`)
	}
	
	htmlParts = append(htmlParts, `            </tbody>
        </table>
        
        <div style="margin-top: 30px; padding-top: 20px; border-top: 1px solid #ddd; color: #666; text-align: center;">
            <small>WPEX VPN Server - Page auto-refreshes every 30 seconds</small>
        </div>
    </div>
</body>
</html>`)
	
	return strings.Join(htmlParts, "\n")
}