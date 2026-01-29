package stats

import (
	"encoding/json"
	"log/slog"
	"net"
	"sync"
	"time"
)

// PeerStatus rappresenta lo stato di un peer
type PeerStatus int

const (
	PeerStatusHandshaking PeerStatus = iota // Durante handshake
	PeerStatusConnected                     // Connesso e stabilito
	PeerStatusDisconnected                  // Disconnesso/scaduto
)

func (ps PeerStatus) String() string {
	switch ps {
	case PeerStatusHandshaking:
		return "handshaking"
	case PeerStatusConnected:
		return "connected"
	case PeerStatusDisconnected:
		return "disconnected"
	default:
		return "unknown"
	}
}

// PeerInfo contiene informazioni su un peer specifico
type PeerInfo struct {
	Index       uint32     `json:"index"`
	Address     string     `json:"address"`
	Status      PeerStatus `json:"status"`
	ConnectedAt time.Time  `json:"connected_at"`
	LastSeen    time.Time  `json:"last_seen"`
	BytesSent   uint64     `json:"bytes_sent"`
	BytesRecv   uint64     `json:"bytes_received"`
}

// VPNStats contiene le statistiche globali del server VPN
type VPNStats struct {
	mu                    sync.RWMutex
	StartTime             time.Time            `json:"start_time"`
	TotalHandshakes       uint64               `json:"total_handshakes"`
	SuccessfulHandshakes  uint64               `json:"successful_handshakes"`
	ActiveSessions        uint64               `json:"active_sessions"`
	TotalDataPackets      uint64               `json:"total_data_packets"`
	TotalBytesTransferred uint64               `json:"total_bytes_transferred"`
	Peers                 map[uint32]*PeerInfo `json:"peers"`
	EndpointMap           map[string]uint32    `json:"-"` // Mappa indirizzo -> peer index
}

// NewVPNStats crea una nuova istanza di statistiche VPN
func NewVPNStats() *VPNStats {
	stats := &VPNStats{
		StartTime:   time.Now(),
		Peers:       make(map[uint32]*PeerInfo),
		EndpointMap: make(map[string]uint32),
	}
	
	// Avvia il cleanup timer automatico (2 minuti di timeout per peer inattivi - bilanciato)
	stats.StartCleanupTimer(2 * time.Minute)
	
	return stats
}

// LogHandshakeInitiated registra l'inizio di un handshake
func (s *VPNStats) LogHandshakeInitiated(peerIndex uint32, addr net.UDPAddr) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.TotalHandshakes++
	
	// Crea o aggiorna le informazioni del peer
	peer, exists := s.Peers[peerIndex]
	if !exists {
		peer = &PeerInfo{
			Index:       peerIndex,
			Address:     addr.String(),
			Status:      PeerStatusHandshaking,
			ConnectedAt: time.Now(),
		}
		s.Peers[peerIndex] = peer
		s.EndpointMap[addr.String()] = peerIndex
	} else {
		peer.Status = PeerStatusHandshaking
		peer.Address = addr.String()
		s.EndpointMap[addr.String()] = peerIndex
	}
	peer.LastSeen = time.Now()

	slog.Info("Handshake initiated", 
		"peer_index", peerIndex, 
		"address", addr.String(),
		"total_handshakes", s.TotalHandshakes)
}

// LogHandshakeCompleted registra il completamento di un handshake
func (s *VPNStats) LogHandshakeCompleted(senderIndex, receiverIndex uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.SuccessfulHandshakes++
	s.ActiveSessions++

	// Aggiorna lo stato di entrambi i peer
	for _, index := range []uint32{senderIndex, receiverIndex} {
		if peer, exists := s.Peers[index]; exists {
			peer.Status = PeerStatusConnected
			peer.LastSeen = time.Now()
			if peer.ConnectedAt.IsZero() {
				peer.ConnectedAt = time.Now()
			}
		}
	}

	slog.Info("Handshake completed - VPN session established", 
		"sender_index", senderIndex,
		"receiver_index", receiverIndex,
		"active_sessions", s.ActiveSessions,
		"success_rate", float64(s.SuccessfulHandshakes)/float64(s.TotalHandshakes)*100)
}

// LogDataTransfer registra il trasferimento di dati
func (s *VPNStats) LogDataTransfer(senderIndex, receiverIndex uint32, bytes int, senderAddr net.UDPAddr) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.TotalDataPackets++
	s.TotalBytesTransferred += uint64(bytes)

	// Aggiorna sempre il LastSeen per il peer che invia dati
	if peer, exists := s.Peers[senderIndex]; exists {
		peer.BytesSent += uint64(bytes)
		peer.LastSeen = time.Now() // ⚡ CRITICO: Aggiorna LastSeen ad ogni pacchetto
		peer.Address = senderAddr.String()
		s.EndpointMap[senderAddr.String()] = senderIndex
		
		// Se il peer è in handshaking e invia dati, marcalo come connected
		if peer.Status == PeerStatusHandshaking {
			peer.Status = PeerStatusConnected
			if peer.ConnectedAt.IsZero() {
				peer.ConnectedAt = time.Now()
			}
			s.ActiveSessions++
			slog.Info("Peer marked as connected due to data transfer", 
				"peer_index", senderIndex,
				"address", peer.Address,
				"bytes_sent", peer.BytesSent)
		}
	} else {
		// Prima controlla se esiste già un peer con lo stesso indirizzo
		existingPeerIndex := uint32(0)
		for idx, existingPeer := range s.Peers {
			if existingPeer.Address == senderAddr.String() {
				existingPeerIndex = idx
				break
			}
		}
		
		if existingPeerIndex != 0 {
			// Esiste già un peer con questo indirizzo - aggiorna quello esistente
			existingPeer := s.Peers[existingPeerIndex]
			existingPeer.BytesSent += uint64(bytes)
			existingPeer.LastSeen = time.Now()
			if existingPeer.Status == PeerStatusHandshaking {
				existingPeer.Status = PeerStatusConnected
				if existingPeer.ConnectedAt.IsZero() {
					existingPeer.ConnectedAt = time.Now()
				}
				s.ActiveSessions++
			}
			slog.Info("Updated existing peer instead of creating duplicate", 
				"existing_peer_index", existingPeerIndex,
				"new_peer_index", senderIndex,
				"address", senderAddr.String())
		} else {
			// Solo se NON esiste un peer con questo indirizzo, creane uno nuovo
			s.Peers[senderIndex] = &PeerInfo{
				Index:       senderIndex,
				Address:     senderAddr.String(),
				Status:      PeerStatusConnected,
				ConnectedAt: time.Now(),
				LastSeen:    time.Now(),
				BytesSent:   uint64(bytes),
				BytesRecv:   0,
			}
			s.EndpointMap[senderAddr.String()] = senderIndex
			s.ActiveSessions++
			slog.Info("New peer auto-created as connected from data transfer", 
				"peer_index", senderIndex,
				"address", senderAddr.String())
		}
	}

	// Aggiorna sempre il LastSeen anche per il peer destinatario
	if peer, exists := s.Peers[receiverIndex]; exists {
		peer.BytesRecv += uint64(bytes)
		peer.LastSeen = time.Now() // ⚡ CRITICO: Aggiorna LastSeen anche per chi riceve
		
		// Se il peer è in handshaking e riceve dati, marcalo come connected
		if peer.Status == PeerStatusHandshaking {
			peer.Status = PeerStatusConnected
			if peer.ConnectedAt.IsZero() {
				peer.ConnectedAt = time.Now()
			}
			s.ActiveSessions++
			slog.Info("Peer marked as connected due to data reception", 
				"peer_index", receiverIndex,
				"address", peer.Address,
				"bytes_received", peer.BytesRecv)
		}
	}
}

// LogPeerDisconnected registra la disconnessione di un peer
func (s *VPNStats) LogPeerDisconnected(peerIndex uint32, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if peer, exists := s.Peers[peerIndex]; exists {
		if peer.Status == PeerStatusConnected {
			s.ActiveSessions--
		}
		peer.Status = PeerStatusDisconnected
		
		slog.Info("Peer disconnected", 
			"peer_index", peerIndex,
			"address", peer.Address,
			"reason", reason,
			"session_duration", time.Since(peer.ConnectedAt).String(),
			"bytes_sent", peer.BytesSent,
			"bytes_received", peer.BytesRecv,
			"active_sessions", s.ActiveSessions)
		
		// Rimuovi dalla mappa degli endpoint
		if addr := peer.Address; addr != "" {
			delete(s.EndpointMap, addr)
		}
	}
}

// GetConnectedPeersCount restituisce il numero di peer attualmente connessi
func (s *VPNStats) GetConnectedPeersCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	now := time.Now()
	
	for _, peer := range s.Peers {
		// Considera connesso solo se lo stato è Connected E non è scaduto (last_seen < 90s fa)
		if peer.Status == PeerStatusConnected && now.Sub(peer.LastSeen) < (90*time.Second) {
			count++
		}
	}
	return count
}

// GetHandshakingPeersCount restituisce il numero di peer in fase di handshake
func (s *VPNStats) GetHandshakingPeersCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, peer := range s.Peers {
		if peer.Status == PeerStatusHandshaking {
			count++
		}
	}
	return count
}

// GetStats restituisce una copia delle statistiche correnti
func (s *VPNStats) GetStats() VPNStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Crea una copia delle statistiche
	stats := VPNStats{
		StartTime:             s.StartTime,
		TotalHandshakes:       s.TotalHandshakes,
		SuccessfulHandshakes:  s.SuccessfulHandshakes,
		ActiveSessions:        s.ActiveSessions,
		TotalDataPackets:      s.TotalDataPackets,
		TotalBytesTransferred: s.TotalBytesTransferred,
		Peers:                 make(map[uint32]*PeerInfo),
	}

	// Copia i peer
	for k, v := range s.Peers {
		peerCopy := *v
		stats.Peers[k] = &peerCopy
	}

	return stats
}

// LogPeriodicStats registra periodicamente le statistiche del server
func (s *VPNStats) LogPeriodicStats() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	uptime := time.Since(s.StartTime)
	connectedPeers := 0
	handshakingPeers := 0
	
	for _, peer := range s.Peers {
		switch peer.Status {
		case PeerStatusConnected:
			connectedPeers++
		case PeerStatusHandshaking:
			handshakingPeers++
		}
	}

	slog.Info("VPN Server Statistics",
		"uptime", uptime.String(),
		"connected_peers", connectedPeers,
		"handshaking_peers", handshakingPeers,
		"active_sessions", s.ActiveSessions,
		"total_handshakes", s.TotalHandshakes,
		"successful_handshakes", s.SuccessfulHandshakes,
		"success_rate_percent", func() float64 {
			if s.TotalHandshakes > 0 {
				return float64(s.SuccessfulHandshakes) / float64(s.TotalHandshakes) * 100
			}
			return 0
		}(),
		"total_data_packets", s.TotalDataPackets,
		"total_bytes_mb", float64(s.TotalBytesTransferred)/(1024*1024))
}

// CleanupExpiredPeers rimuove i peer scaduti dalle statistiche
func (s *VPNStats) CleanupExpiredPeers(timeout time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	cleanedCount := 0
	
	for index, peer := range s.Peers {
		// Considera scaduti i peer che non inviano dati da più di timeout secondi
		timeSinceLastSeen := now.Sub(peer.LastSeen)
		
		// Se il peer è connesso ma non invia dati da troppo tempo, marcalo come disconnesso
		if peer.Status == PeerStatusConnected && timeSinceLastSeen > timeout {
			peer.Status = PeerStatusDisconnected
			s.ActiveSessions--
			slog.Info("Peer marked as disconnected due to timeout", 
				"peer_index", index, 
				"address", peer.Address,
				"last_seen", peer.LastSeen,
				"timeout_duration", timeSinceLastSeen.String())
		}
		
		// Rimuovi peer disconnessi da più di 2x timeout
		if peer.Status != PeerStatusConnected && timeSinceLastSeen > (timeout*2) {
			slog.Info("Removing expired peer from stats", 
				"peer_index", index, 
				"address", peer.Address,
				"last_seen", peer.LastSeen)
			
			// Rimuovi dalla mappa degli endpoint
			if addr := peer.Address; addr != "" {
				delete(s.EndpointMap, addr)
			}
			
			delete(s.Peers, index)
			cleanedCount++
		}
	}
	
	if cleanedCount > 0 {
		slog.Info("Cleanup completed", "removed_peers", cleanedCount, "active_sessions", s.ActiveSessions)
	}
}

// ToJSON converte le statistiche in formato JSON
func (s *VPNStats) ToJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	return json.MarshalIndent(s, "", "  ")
}

// StartCleanupTimer avvia un timer periodico per pulire i peer scaduti
func (s *VPNStats) StartCleanupTimer(timeout time.Duration) {
	ticker := time.NewTicker(30 * time.Second) // Cleanup ogni 30 secondi
	
	go func() {
		defer ticker.Stop()
		
		slog.Info("Started peer cleanup timer", "timeout", timeout, "interval", "30s")
		
		for range ticker.C {
			s.SyncPeerStatus()       // Prima sincronizza gli status
			s.CleanupExpiredPeers(timeout) // Poi pulisce i peer scaduti
		}
	}()
}

// SyncPeerStatus forza la sincronizzazione dello status dei peer
func (s *VPNStats) SyncPeerStatus() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	now := time.Now()
	syncedCount := 0
	
	// 1. Corregge status inconsistenti
	for index, peer := range s.Peers {
		// Se un peer ha traffico recente (< 90s) ma è ancora handshaking, marcalo connected
		if peer.Status == PeerStatusHandshaking && (peer.BytesSent > 0 || peer.BytesRecv > 0) {
			timeSinceLastSeen := now.Sub(peer.LastSeen)
			if timeSinceLastSeen < (90 * time.Second) {
				peer.Status = PeerStatusConnected
				if peer.ConnectedAt.IsZero() {
					peer.ConnectedAt = now
				}
				s.ActiveSessions++
				syncedCount++
				slog.Info("Sync: Peer status corrected to connected", 
					"peer_index", index,
					"address", peer.Address,
					"bytes_sent", peer.BytesSent,
					"bytes_received", peer.BytesRecv)
			}
		}
	}
	
	// 2. Rimuove peer duplicati (stesso indirizzo)
	addressMap := make(map[string]uint32)
	duplicatesRemoved := 0
	
	for index, peer := range s.Peers {
		if existingIndex, exists := addressMap[peer.Address]; exists {
			// Trovato duplicato - mantieni quello con più traffico
			existingPeer := s.Peers[existingIndex]
			totalBytesExisting := existingPeer.BytesSent + existingPeer.BytesRecv
			totalBytesCurrent := peer.BytesSent + peer.BytesRecv
			
			if totalBytesCurrent > totalBytesExisting {
				// Il peer corrente ha più traffico - rimuovi quello esistente
				if existingPeer.Status == PeerStatusConnected {
					s.ActiveSessions--
				}
				delete(s.Peers, existingIndex)
				addressMap[peer.Address] = index
				slog.Info("Removed duplicate peer (kept newer one)", 
					"removed_index", existingIndex,
					"kept_index", index,
					"address", peer.Address)
			} else {
				// Il peer esistente ha più traffico - rimuovi quello corrente
				if peer.Status == PeerStatusConnected {
					s.ActiveSessions--
				}
				delete(s.Peers, index)
				slog.Info("Removed duplicate peer (kept existing one)", 
					"removed_index", index,
					"kept_index", existingIndex,
					"address", peer.Address)
			}
			duplicatesRemoved++
		} else {
			addressMap[peer.Address] = index
		}
	}
	
	if syncedCount > 0 || duplicatesRemoved > 0 {
		slog.Info("Peer status synchronization completed", 
			"corrected_peers", syncedCount,
			"removed_duplicates", duplicatesRemoved)
	}
}