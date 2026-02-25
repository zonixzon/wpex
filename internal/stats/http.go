package stats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Reloader is satisfied by *analyzer.WireguardAnalyzer — kept as interface
// to avoid import cycles (stats → analyzer would be circular).
type Reloader interface {
	UpdateAllowedKeys(newKeys [][]byte)
}

// HTTPServer gestisce l'endpoint HTTP per le statistiche
type HTTPServer struct {
	server   *http.Server
	stats    *VPNStats
	reloader Reloader // optional: hot-reload handler
}

// NewHTTPServer crea un nuovo server HTTP per le statistiche
func NewHTTPServer(addr string, stats *VPNStats, reloader Reloader) *HTTPServer {
	mux := http.NewServeMux()

	httpServer := &HTTPServer{
		stats:    stats,
		reloader: reloader,
		server: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
	}

	// Endpoint per le statistiche in formato JSON
	mux.HandleFunc("/stats", httpServer.handleStats)

	// Endpoint per le statistiche in formato HTML
	mux.HandleFunc("/", httpServer.handleStatsHTML)

	// Endpoint per la salute del server
	mux.HandleFunc("/health", httpServer.handleHealth)

	// API v1 endpoints (SaaS orchestrator integration)
	mux.HandleFunc("/api/v1/stats", httpServer.handleAPIStats)
	mux.HandleFunc("/api/v1/health", httpServer.handleAPIHealth)
	mux.HandleFunc("/api/v1/config", httpServer.handleAPIConfig)
	mux.HandleFunc("/api/v1/config/reload", httpServer.handleAPIConfigReload)
	mux.HandleFunc("/api/v1/diagnostics/ping", httpServer.handleAPIPing)
	mux.HandleFunc("/api/v1/diagnostics/traceroute", httpServer.handleAPITraceroute)

	return httpServer
}

// Start avvia il server HTTP in background
func (h *HTTPServer) Start() error {
	go func() {
		slog.Info("Starting HTTP statistics server", "addr", h.server.Addr)
		if err := h.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
		}
	}()
	return nil
}

// Stop ferma il server HTTP
func (h *HTTPServer) Stop(ctx context.Context) error {
	slog.Info("Stopping HTTP statistics server")
	return h.server.Shutdown(ctx)
}

// handleStats gestisce le richieste per /stats (JSON)
func (h *HTTPServer) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.stats.GetStats()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // Per CORS se necessario

	if err := json.NewEncoder(w).Encode(stats); err != nil {
		slog.Error("Error encoding stats to JSON", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handleStatsHTML gestisce le richieste per / (HTML)
func (h *HTTPServer) handleStatsHTML(w http.ResponseWriter, r *http.Request) {
	stats := h.stats.GetStats()

	w.Header().Set("Content-Type", "text/html")

	html := generateStatsHTMLDebug(stats) // Usa versione debug temporaneamente
	fmt.Fprint(w, html)
}

// handleHealth gestisce le richieste per /health
func (h *HTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(h.stats.StartTime)

	health := map[string]interface{}{
		"status":    "ok",
		"uptime":    uptime.String(),
		"timestamp": time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// generateStatsHTML genera una pagina HTML con le statistiche
func generateStatsHTML(stats VPNStats) string {
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

	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <title>WPEX VPN Server Statistics</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap" rel="stylesheet">
    <style>
        *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
            margin: 0;
            padding: 32px;
            background-color: #0e0e14;
            color: #e8e8f0;
            -webkit-font-smoothing: antialiased;
        }
        @keyframes fadeInUp {
            from { opacity: 0; transform: translateY(24px); }
            to   { opacity: 1; transform: translateY(0); }
        }
        @keyframes gradientShift {
            0%%   { background-position: 0%% 50%%; }
            50%%  { background-position: 100%% 50%%; }
            100%% { background-position: 0%% 50%%; }
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
            animation: fadeInUp 0.5s ease-out;
        }
        .header {
            border-bottom: 1px solid rgba(255, 255, 255, 0.07);
            padding-bottom: 20px;
            margin-bottom: 28px;
        }
        .header h1 {
            font-size: 1.6rem;
            font-weight: 700;
        }
        .gradient-text {
            background: linear-gradient(135deg, #9b8afb, #60a5fa, #34d399);
            background-size: 200%% auto;
            -webkit-background-clip: text;
            background-clip: text;
            -webkit-text-fill-color: transparent;
            animation: gradientShift 4s ease infinite;
        }
        .header p {
            color: #9090a8;
            font-size: 0.9rem;
            margin-top: 6px;
        }
        .badge {
            display: inline-flex;
            padding: 4px 10px;
            border-radius: 20px;
            font-size: 0.78rem;
            font-weight: 500;
            background: rgba(124, 106, 239, 0.12);
            color: #9b8afb;
            border: 1px solid rgba(124, 106, 239, 0.2);
            margin-bottom: 12px;
        }
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 16px;
            margin-bottom: 32px;
        }
        .stat-card {
            background: rgba(255, 255, 255, 0.03);
            border: 1px solid rgba(255, 255, 255, 0.07);
            padding: 20px;
            border-radius: 14px;
            transition: all 0.3s cubic-bezier(.4, 0, .2, 1);
            animation: fadeInUp 0.4s ease-out both;
        }
        .stat-card:hover {
            border-color: rgba(124, 106, 239, 0.25);
            box-shadow: 0 16px 40px rgba(0, 0, 0, 0.35);
            transform: translateY(-4px);
        }
        .stat-value {
            font-size: 1.5rem;
            font-weight: 700;
            color: #9b8afb;
        }
        .stat-label {
            font-size: 0.82rem;
            font-weight: 500;
            color: #9090a8;
            margin-top: 6px;
        }
        .section-title {
            font-size: 1.08rem;
            font-weight: 600;
            margin-bottom: 16px;
            color: #e8e8f0;
        }
        .peers-table {
            width: 100%%;
            border-collapse: collapse;
            margin-top: 8px;
            background: rgba(255, 255, 255, 0.03);
            border: 1px solid rgba(255, 255, 255, 0.07);
            border-radius: 14px;
            overflow: hidden;
        }
        .peers-table th,
        .peers-table td {
            padding: 12px 16px;
            text-align: left;
            border-bottom: 1px solid rgba(255, 255, 255, 0.05);
        }
        .peers-table th {
            background: linear-gradient(135deg, #7c6aef, #6354c4);
            color: #fff;
            font-size: 0.82rem;
            font-weight: 600;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }
        .peers-table td {
            font-size: 0.88rem;
            color: #9090a8;
        }
        .peers-table tbody tr {
            transition: background 0.2s ease;
        }
        .peers-table tbody tr:hover {
            background: rgba(255, 255, 255, 0.04);
        }
        .peers-table tbody tr:last-child td {
            border-bottom: none;
        }
        .status-connected {
            color: #34d399;
            font-weight: 600;
        }
        .status-handshaking {
            color: #fbbf24;
            font-weight: 600;
        }
        .status-disconnected {
            color: #f87171;
            font-weight: 600;
        }
        .refresh-btn {
            display: inline-flex;
            align-items: center;
            gap: 8px;
            background: linear-gradient(135deg, #7c6aef, #6354c4);
            color: white;
            border: none;
            padding: 10px 22px;
            border-radius: 8px;
            cursor: pointer;
            margin-bottom: 24px;
            font-family: inherit;
            font-size: 0.88rem;
            font-weight: 500;
            transition: all 0.3s cubic-bezier(.4, 0, .2, 1);
        }
        .refresh-btn:hover {
            background: linear-gradient(135deg, #8b7af0, #7c6aef);
            box-shadow: 0 4px 20px rgba(124, 106, 239, 0.35);
            transform: translateY(-1px);
        }
        .refresh-btn:active { transform: translateY(0); }
        .footer {
            margin-top: 40px;
            padding-top: 20px;
            border-top: 1px solid rgba(255, 255, 255, 0.07);
            text-align: center;
            color: #606078;
            font-size: 0.8rem;
        }
        @media (max-width: 640px) {
            body { padding: 16px; }
            .stats-grid { grid-template-columns: repeat(2, 1fr); gap: 10px; }
            .stat-card { padding: 14px; }
            .stat-value { font-size: 1.2rem; }
            .peers-table { font-size: 0.8rem; }
            .peers-table th, .peers-table td { padding: 8px 10px; }
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
            <span class="badge">WPEX VPN Server</span>
            <h1><span class="gradient-text">Server Statistics</span></h1>
            <p>Monitoring dashboard — Last updated: %s</p>
        </div>
        
        <button class="refresh-btn" onclick="refreshPage()">&#x21bb; Refresh</button>
        
        <div class="stats-grid">
            <div class="stat-card">
                <div class="stat-value">%d</div>
                <div class="stat-label">Connected Peers</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">%d</div>
                <div class="stat-label">Handshaking Peers</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">%d</div>
                <div class="stat-label">Active Sessions</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">%s</div>
                <div class="stat-label">Server Uptime</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">%d</div>
                <div class="stat-label">Total Handshakes</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">%.1f%%</div>
                <div class="stat-label">Success Rate</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">%d</div>
                <div class="stat-label">Data Packets</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">%.2f MB</div>
                <div class="stat-label">Total Transfer</div>
            </div>
        </div>
        
        <div class="section-title">Peer Details</div>
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
            <tbody>`,
		time.Now().Format("2006-01-02 15:04:05"),
		connectedPeers,
		handshakingPeers,
		int(stats.ActiveSessions), // Convertito a int
		uptime.String(),
		int(stats.TotalHandshakes), // Convertito a int
		successRate,
		int(stats.TotalDataPackets), // Convertito a int
		float64(stats.TotalBytesTransferred)/(1024*1024))

	// Aggiungi righe per ogni peer
	for _, peer := range stats.Peers {
		statusClass := "status-disconnected"
		switch peer.Status {
		case PeerStatusConnected:
			statusClass = "status-connected"
		case PeerStatusHandshaking:
			statusClass = "status-handshaking"
		}

		html += fmt.Sprintf(`
                <tr>
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
	}

	if len(stats.Peers) == 0 {
		html += `
                <tr>
                    <td colspan="7" style="text-align: center; color: #606078; font-style: italic; padding: 32px;">
                        No peers currently tracked
                    </td>
                </tr>`
	}

	html += `
            </tbody>
        </table>
        
        <div class="footer">
            WPEX VPN Server — Page auto-refreshes every 30 seconds
        </div>
    </div>
</body>
</html>`

	return html
}

// formatBytes formatta i bytes in formato human-readable
func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
