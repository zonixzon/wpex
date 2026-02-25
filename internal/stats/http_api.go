package stats

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

// ──────────────────────────────────────────────────────────────────────
// API v1 endpoints — consumed by the WPEX Orchestrator SaaS dashboard.
// These are additive; existing /stats, / and /health remain untouched.
// ──────────────────────────────────────────────────────────────────────

// safeTarget validates that a diagnostic target is a hostname or IP (no injection).
var safeTarget = regexp.MustCompile(`^[a-zA-Z0-9.\-:]+$`)

// setCORS sets common CORS + JSON headers.
func setCORS(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

// ── /api/v1/stats ────────────────────────────────────────────────────
// Enhanced stats with computed fields (success rate, uptime, sorted peer list).
func (h *HTTPServer) handleAPIStats(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCORS(w)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	raw := h.stats.GetStats()
	uptime := time.Since(raw.StartTime)

	successRate := 0.0
	if raw.TotalHandshakes > 0 {
		successRate = float64(raw.SuccessfulHandshakes) / float64(raw.TotalHandshakes) * 100
	}

	// Build a sorted peer list (array, not map).
	type peerJSON struct {
		Index       uint32  `json:"index"`
		Address     string  `json:"address"`
		Status      string  `json:"status"`
		ConnectedAt string  `json:"connected_at"`
		LastSeen    string  `json:"last_seen"`
		BytesSent   uint64  `json:"bytes_sent"`
		BytesRecv   uint64  `json:"bytes_received"`
		UptimeSec   float64 `json:"uptime_seconds"`
	}

	peers := make([]peerJSON, 0, len(raw.Peers))
	for _, p := range raw.Peers {
		peerUptime := 0.0
		if p.Status == PeerStatusConnected {
			peerUptime = time.Since(p.ConnectedAt).Seconds()
		}
		peers = append(peers, peerJSON{
			Index:       p.Index,
			Address:     p.Address,
			Status:      p.Status.String(),
			ConnectedAt: p.ConnectedAt.UTC().Format(time.RFC3339),
			LastSeen:    p.LastSeen.UTC().Format(time.RFC3339),
			BytesSent:   p.BytesSent,
			BytesRecv:   p.BytesRecv,
			UptimeSec:   peerUptime,
		})
	}
	sort.Slice(peers, func(i, j int) bool { return peers[i].Index < peers[j].Index })

	connectedCount := 0
	handshakingCount := 0
	for _, p := range raw.Peers {
		switch p.Status {
		case PeerStatusConnected:
			connectedCount++
		case PeerStatusHandshaking:
			handshakingCount++
		}
	}

	resp := map[string]interface{}{
		"start_time":            raw.StartTime.UTC().Format(time.RFC3339),
		"uptime_seconds":        uptime.Seconds(),
		"uptime_human":          uptime.String(),
		"total_handshakes":      raw.TotalHandshakes,
		"successful_handshakes": raw.SuccessfulHandshakes,
		"success_rate":          successRate,
		"active_sessions":       raw.ActiveSessions,
		"total_data_packets":    raw.TotalDataPackets,
		"total_bytes":           raw.TotalBytesTransferred,
		"total_transfer_mb":     float64(raw.TotalBytesTransferred) / (1024 * 1024),
		"connected_peers":       connectedCount,
		"handshaking_peers":     handshakingCount,
		"peers":                 peers,
	}

	setCORS(w)
	json.NewEncoder(w).Encode(resp)
}

// ── /api/v1/health ───────────────────────────────────────────────────
// Structured health check with component-level breakdown.
func (h *HTTPServer) handleAPIHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCORS(w)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	raw := h.stats.GetStats()
	uptime := time.Since(raw.StartTime)

	// Compute component scores (0-100).
	successRate := 100.0
	if raw.TotalHandshakes > 0 {
		successRate = float64(raw.SuccessfulHandshakes) / float64(raw.TotalHandshakes) * 100
	}

	connectedPeers := 0
	totalPeers := len(raw.Peers)
	for _, p := range raw.Peers {
		if p.Status == PeerStatusConnected {
			connectedPeers++
		}
	}

	peerScore := 100.0
	if totalPeers > 0 {
		peerScore = float64(connectedPeers) / float64(totalPeers) * 100
	}

	uptimeScore := 100.0
	if uptime < 5*time.Minute {
		uptimeScore = 50.0 // Just started — penalise slightly
	}

	// Global health = weighted average
	globalHealth := successRate*0.4 + peerScore*0.35 + uptimeScore*0.25

	status := "healthy"
	if globalHealth < 70 {
		status = "degraded"
	}
	if globalHealth < 40 {
		status = "critical"
	}

	resp := map[string]interface{}{
		"status":         status,
		"health_score":   globalHealth,
		"uptime":         uptime.String(),
		"uptime_seconds": uptime.Seconds(),
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
		"components": map[string]interface{}{
			"handshake_success": map[string]interface{}{
				"score":  successRate,
				"detail": fmt.Sprintf("%d/%d handshakes succeeded", raw.SuccessfulHandshakes, raw.TotalHandshakes),
			},
			"peer_connectivity": map[string]interface{}{
				"score":  peerScore,
				"detail": fmt.Sprintf("%d/%d peers connected", connectedPeers, totalPeers),
			},
			"uptime": map[string]interface{}{
				"score":  uptimeScore,
				"detail": uptime.String(),
			},
		},
		"stats_summary": map[string]interface{}{
			"active_sessions":    raw.ActiveSessions,
			"total_data_packets": raw.TotalDataPackets,
			"total_bytes":        raw.TotalBytesTransferred,
		},
	}

	setCORS(w)
	json.NewEncoder(w).Encode(resp)
}

// ── /api/v1/config ───────────────────────────────────────────────────
// Returns current runtime configuration.
// WPEX does not use a config file — everything is CLI arguments.
func (h *HTTPServer) handleAPIConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCORS(w)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	resp := map[string]interface{}{
		"config_source": "cli_args",
		"stats_server":  h.server.Addr,
		"go_version":    runtime.Version(),
		"num_cpu":       runtime.NumCPU(),
		"goroutines":    runtime.NumGoroutine(),
		"note":          "WPEX relay is configured via command-line arguments. Runtime config changes are not supported.",
	}

	setCORS(w)
	json.NewEncoder(w).Encode(resp)
}

// ── /api/v1/config/reload ────────────────────────────────────────────
// Hot-reloads the allowed WireGuard public keys without restarting the process.
// Body: {"public_keys": ["<base64-encoded-32-byte-key>", ...]}
func (h *HTTPServer) handleAPIConfigReload(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCORS(w)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	if h.reloader == nil {
		setCORS(w)
		w.WriteHeader(http.StatusNotImplemented)
		json.NewEncoder(w).Encode(map[string]string{"error": "hot-reload not available"})
		return
	}

	var body struct {
		PublicKeys []string `json:"public_keys"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		setCORS(w)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON body"})
		return
	}

	var newKeys [][]byte
	for _, b64 := range body.PublicKeys {
		key, err := base64.StdEncoding.DecodeString(b64)
		if err != nil || len(key) != 32 {
			setCORS(w)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("invalid key: %s", b64)})
			return
		}
		newKeys = append(newKeys, key)
	}

	h.reloader.UpdateAllowedKeys(newKeys)

	slog.Info("Hot-reload: allowed keys updated via HTTP", "count", len(newKeys))
	setCORS(w)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "ok",
		"key_count":  len(newKeys),
		"message":    "Allowed keys updated. All sessions evicted — valid peers will re-handshake automatically.",
		"timestamp":  time.Now().UTC(),
	})
}


// ── /api/v1/diagnostics/ping ─────────────────────────────────────────
// Runs ping from inside the relay container.
func (h *HTTPServer) handleAPIPing(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCORS(w)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Target string `json:"target"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Target == "" {
		setCORS(w)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "missing or invalid 'target' field"})
		return
	}

	if !safeTarget.MatchString(body.Target) || len(body.Target) > 253 {
		setCORS(w)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid target format"})
		return
	}

	slog.Info("API diagnostics: ping", "target", body.Target)

	// Use -c 4 on Linux (Docker), -n 4 on Windows (dev)
	args := []string{"-c", "4", body.Target}
	if runtime.GOOS == "windows" {
		args = []string{"-n", "4", body.Target}
	}

	out, err := exec.Command("ping", args...).CombinedOutput()

	resp := map[string]interface{}{
		"target":    body.Target,
		"output":    string(out),
		"success":   err == nil,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	if err != nil {
		resp["error"] = err.Error()
	}

	setCORS(w)
	json.NewEncoder(w).Encode(resp)
}

// ── /api/v1/diagnostics/traceroute ───────────────────────────────────
// Runs traceroute from inside the relay container.
func (h *HTTPServer) handleAPITraceroute(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setCORS(w)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Target string `json:"target"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Target == "" {
		setCORS(w)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "missing or invalid 'target' field"})
		return
	}

	if !safeTarget.MatchString(body.Target) || len(body.Target) > 253 {
		setCORS(w)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid target format"})
		return
	}

	slog.Info("API diagnostics: traceroute", "target", body.Target)

	// traceroute on Linux, tracert on Windows
	cmd := "traceroute"
	args := []string{"-m", "15", body.Target}
	if runtime.GOOS == "windows" {
		cmd = "tracert"
		args = []string{"-h", "15", body.Target}
	}

	out, err := exec.Command(cmd, args...).CombinedOutput()

	// Parse hops from output (best-effort)
	hops := []string{}
	for _, line := range strings.Split(string(out), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			hops = append(hops, trimmed)
		}
	}

	resp := map[string]interface{}{
		"target":    body.Target,
		"output":    string(out),
		"hops":      hops,
		"hop_count": len(hops),
		"success":   err == nil,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	if err != nil {
		resp["error"] = err.Error()
	}

	setCORS(w)
	json.NewEncoder(w).Encode(resp)
}
