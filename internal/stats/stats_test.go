package stats

import (
	"net"
	"testing"
	"time"
)

func TestVPNStats_BasicOperations(t *testing.T) {
	stats := NewVPNStats()
	
	// Test initial state
	if stats.GetConnectedPeersCount() != 0 {
		t.Errorf("Expected 0 connected peers, got %d", stats.GetConnectedPeersCount())
	}
	
	if stats.GetHandshakingPeersCount() != 0 {
		t.Errorf("Expected 0 handshaking peers, got %d", stats.GetHandshakingPeersCount())
	}
}

func TestVPNStats_HandshakeFlow(t *testing.T) {
	stats := NewVPNStats()
	addr, _ := net.ResolveUDPAddr("udp", "192.168.1.100:51820")
	
	// Test handshake initiation
	stats.LogHandshakeInitiated(12345, *addr)
	
	if stats.GetHandshakingPeersCount() != 1 {
		t.Errorf("Expected 1 handshaking peer, got %d", stats.GetHandshakingPeersCount())
	}
	
	if stats.TotalHandshakes != 1 {
		t.Errorf("Expected 1 total handshake, got %d", stats.TotalHandshakes)
	}
	
	// Test handshake completion
	addr2, _ := net.ResolveUDPAddr("udp", "192.168.1.101:51821")
	stats.LogHandshakeInitiated(67890, *addr2)
	stats.LogHandshakeCompleted(12345, 67890)
	
	if stats.GetConnectedPeersCount() != 2 {
		t.Errorf("Expected 2 connected peers, got %d", stats.GetConnectedPeersCount())
	}
	
	if stats.GetHandshakingPeersCount() != 0 {
		t.Errorf("Expected 0 handshaking peers, got %d", stats.GetHandshakingPeersCount())
	}
	
	if stats.SuccessfulHandshakes != 1 {
		t.Errorf("Expected 1 successful handshake, got %d", stats.SuccessfulHandshakes)
	}
}

func TestVPNStats_DataTransfer(t *testing.T) {
	stats := NewVPNStats()
	addr, _ := net.ResolveUDPAddr("udp", "192.168.1.100:51820")
	
	stats.LogHandshakeInitiated(12345, *addr)
	stats.LogHandshakeInitiated(67890, *addr)
	stats.LogHandshakeCompleted(12345, 67890)
	
	// Test data transfer logging
	stats.LogDataTransfer(12345, 67890, 1024, *addr)
	
	if stats.TotalDataPackets != 1 {
		t.Errorf("Expected 1 data packet, got %d", stats.TotalDataPackets)
	}
	
	if stats.TotalBytesTransferred != 1024 {
		t.Errorf("Expected 1024 bytes transferred, got %d", stats.TotalBytesTransferred)
	}
	
	peer := stats.Peers[12345]
	if peer.BytesSent != 1024 {
		t.Errorf("Expected 1024 bytes sent for peer 12345, got %d", peer.BytesSent)
	}
	
	peer = stats.Peers[67890]
	if peer.BytesRecv != 1024 {
		t.Errorf("Expected 1024 bytes received for peer 67890, got %d", peer.BytesRecv)
	}
}

func TestVPNStats_PeerDisconnection(t *testing.T) {
	stats := NewVPNStats()
	addr, _ := net.ResolveUDPAddr("udp", "192.168.1.100:51820")
	
	stats.LogHandshakeInitiated(12345, *addr)
	stats.LogHandshakeInitiated(67890, *addr)
	stats.LogHandshakeCompleted(12345, 67890)
	
	// Test peer disconnection
	stats.LogPeerDisconnected(12345, "timeout")
	
	peer := stats.Peers[12345]
	if peer.Status != PeerStatusDisconnected {
		t.Errorf("Expected peer to be disconnected, got status %v", peer.Status)
	}
	
	if stats.ActiveSessions != 0 {
		t.Errorf("Expected 0 active sessions after disconnection, got %d", stats.ActiveSessions)
	}
}

func TestVPNStats_JSONSerialization(t *testing.T) {
	stats := NewVPNStats()
	addr, _ := net.ResolveUDPAddr("udp", "192.168.1.100:51820")
	
	stats.LogHandshakeInitiated(12345, *addr)
	
	// Test JSON serialization
	jsonData, err := stats.ToJSON()
	if err != nil {
		t.Errorf("Failed to serialize to JSON: %v", err)
	}
	
	if len(jsonData) == 0 {
		t.Error("Expected non-empty JSON data")
	}
}

func TestVPNStats_CleanupExpiredPeers(t *testing.T) {
	stats := NewVPNStats()
	addr, _ := net.ResolveUDPAddr("udp", "192.168.1.100:51820")
	
	stats.LogHandshakeInitiated(12345, *addr)
	stats.LogPeerDisconnected(12345, "timeout")
	
	// Manually set last seen to old time to test cleanup
	peer := stats.Peers[12345]
	peer.LastSeen = time.Now().Add(-time.Hour)
	stats.Peers[12345] = peer
	
	// Test cleanup
	stats.CleanupExpiredPeers(30 * time.Minute)
	
	if _, exists := stats.Peers[12345]; exists {
		t.Error("Expected expired peer to be removed")
	}
}

func TestPeerStatus_String(t *testing.T) {
	tests := []struct {
		status   PeerStatus
		expected string
	}{
		{PeerStatusHandshaking, "handshaking"},
		{PeerStatusConnected, "connected"},
		{PeerStatusDisconnected, "disconnected"},
	}
	
	for _, test := range tests {
		if test.status.String() != test.expected {
			t.Errorf("Expected %s, got %s", test.expected, test.status.String())
		}
	}
}