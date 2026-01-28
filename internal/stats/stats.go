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
	return &VPNStats{
		StartTime:   time.Now(),
		Peers:       make(map[uint32]*PeerInfo),
		EndpointMap: make(map[string]uint32),
	}
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

	// Aggiorna le statistiche del peer mittente
	if peer, exists := s.Peers[senderIndex]; exists {
		peer.BytesSent += uint64(bytes)
		peer.LastSeen = time.Now()
		peer.Address = senderAddr.String()
		s.EndpointMap[senderAddr.String()] = senderIndex
	}

	// Aggiorna le statistiche del peer destinatario
	if peer, exists := s.Peers[receiverIndex]; exists {
		peer.BytesRecv += uint64(bytes)
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
	for _, peer := range s.Peers {
		if peer.Status == PeerStatusConnected {
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
	for index, peer := range s.Peers {
		if peer.Status != PeerStatusConnected && now.Sub(peer.LastSeen) > timeout {
			slog.Debug("Removing expired peer from stats", "peer_index", index, "last_seen", peer.LastSeen)
			
			// Rimuovi dalla mappa degli endpoint
			if addr := peer.Address; addr != "" {
				delete(s.EndpointMap, addr)
			}
			
			delete(s.Peers, index)
		}
	}
}

// ToJSON converte le statistiche in formato JSON
func (s *VPNStats) ToJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	return json.MarshalIndent(s, "", "  ")
}