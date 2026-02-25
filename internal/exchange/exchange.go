package exchange

import (
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"
)

// PeerExpiredCallback è chiamata quando un peer scade
type PeerExpiredCallback func(peerIndex uint32, reason string)

const (
	endpointTTL  = 3 * time.Minute  // Endpoint timeout: 3 minuti (era 1)
	handshakeTTL = 45 * time.Second // Handshake timeout: 45 secondi (era 30)
	sessionTTL   = 12 * time.Minute // Sessione VPN: 12 minuti (era 5) - più stabile
)

type endpointInfo struct {
	refs      int
	addr      net.UDPAddr
	expiredAt time.Time
}

func (e *endpointInfo) isExpired() bool {
	if e.refs <= 0 && e.expiredAt.Before(time.Now()) {
		return true
	}
	return false
}

type peerInfo struct {
	index       uint32
	addr        *endpointInfo
	established bool
	counterpart uint32
	expiredAt   time.Time
}

func (p *peerInfo) isExpired() bool {
	return p.isExpiredAt(time.Now())
}

func (p *peerInfo) isExpiredAt(t time.Time) bool {
	if p.expiredAt.Before(t) {
		return true
	}
	return false
}

// ExchangeTable is a concurrency-safe table that maintains wireguard peer information.
type ExchangeTable struct {
	mu            sync.RWMutex
	endpoints     map[string]*endpointInfo
	peers         map[uint32]peerInfo
	peerPubkeys   map[uint32][32]byte // index → pubkey used in handshake initiation
	onPeerExpired PeerExpiredCallback
}

func (t *ExchangeTable) refEndpoint(addr net.UDPAddr) *endpointInfo {
	addrStr := addr.String()
	e, ok := t.endpoints[addrStr]
	if ok {
		e.refs += 1
		return e
	}
	e = &endpointInfo{
		addr: addr,
		refs: 1,
	}
	t.endpoints[addrStr] = e
	return e
}

func (t *ExchangeTable) derefEndpoint(endpoint *endpointInfo) {
	endpoint.refs -= 1
	if endpoint.refs <= 0 {
		endpoint.expiredAt = time.Now().Add(endpointTTL)
	}
}

func (t *ExchangeTable) cleanup() {
	now := time.Now()
	for index, peer := range t.peers {
		if peer.isExpiredAt(now) {
			slog.Debug("remove expired peer information", "index", index)

			// Notifica la scadenza del peer se c'è un callback
			if t.onPeerExpired != nil {
				reason := "timeout"
				if peer.established {
					reason = "session_timeout"
				} else {
					reason = "handshake_timeout"
				}
				t.onPeerExpired(index, reason)
			}

			t.derefEndpoint(peer.addr)
			delete(t.peers, index)
		}
	}
	for addr, endpoint := range t.endpoints {
		if endpoint.isExpired() {
			slog.Debug("remove expired endpoint information", "addr", addr)
			delete(t.endpoints, addr)
		}
	}
}

// AddPeerAddr adds a new peer's endpoint address to the exchange table.
func (t *ExchangeTable) AddPeerAddr(index uint32, addr net.UDPAddr) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.cleanup()
	if _, ok := t.peers[index]; ok {
		return fmt.Errorf("peer index collision detected on %d", index)
	}
	t.peers[index] = peerInfo{
		index:       index,
		addr:        t.refEndpoint(addr),
		established: false,
		counterpart: 0,
		expiredAt:   time.Now().Add(handshakeTTL),
	}
	return nil
}

// UpdatePeerAddr updates the endpoint address of a peer given its index.
func (t *ExchangeTable) UpdatePeerAddr(index uint32, addr net.UDPAddr) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.cleanup()
	peer, ok := t.peers[index]
	if !ok {
		return fmt.Errorf("failed to update: peer %d not found", index)
	}
	t.derefEndpoint(peer.addr)
	peer.addr = t.refEndpoint(addr)
	t.peers[index] = peer
	return nil
}

// GetPeerAddr retrieves the endpoint address of a peer using its index.
func (t *ExchangeTable) GetPeerAddr(index uint32) (net.UDPAddr, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	peer, ok := t.peers[index]
	if !ok || peer.isExpired() {
		return net.UDPAddr{}, fmt.Errorf("peer %d not found", index)
	}
	return peer.addr.addr, nil
}

// ListAddrs returns all known endpoint addresses from the exchange table.
func (t *ExchangeTable) ListAddrs(exclude net.UDPAddr) []net.UDPAddr {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var addrs []net.UDPAddr
	for _, endpoint := range t.endpoints {
		if !endpoint.isExpired() && endpoint.addr.String() != exclude.String() {
			addrs = append(addrs, endpoint.addr)
		}
	}
	return addrs
}

// AssociatePeers associates two peers in the same wireguard session.
// After linking, the counterpart peer can be retrieved using GetPeerCounterpart.
func (t *ExchangeTable) AssociatePeers(sender, receiver uint32) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.cleanup()
	s, ok := t.peers[sender]
	if !ok {
		return fmt.Errorf("failed to associate peers: sender %d not found", sender)
	}

	r, ok := t.peers[receiver]
	if !ok {
		return fmt.Errorf("failed to associate peers: receiver %d not found", receiver)
	}

	if s.established && r.established && s.counterpart == receiver && r.counterpart == sender {
		return nil
	}

	if s.established || r.established {
		return fmt.Errorf("sender or receiver has already been assoicated with another peer")
	}

	expiredAt := time.Now().Add(sessionTTL)
	s.established = true
	s.counterpart = receiver
	s.expiredAt = expiredAt
	t.peers[sender] = s

	r.established = true
	r.counterpart = sender
	r.expiredAt = expiredAt
	t.peers[receiver] = r

	return nil
}

// GetPeerCounterpart retrieves the counterpart of a given peer from the same session.
func (t *ExchangeTable) GetPeerCounterpart(index uint32) (uint32, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	peer, ok := t.peers[index]
	if !ok || !peer.established || peer.isExpired() {
		return 0, fmt.Errorf("peer %d doesn't exist or has no counterpart", index)
	}

	return peer.counterpart, nil
}

// Contains checks if an address exists in the exchange table.
func (t *ExchangeTable) Contains(addr net.UDPAddr) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	endpoint, ok := t.endpoints[addr.String()]
	if !ok || endpoint.isExpired() {
		return false
	}
	return true
}

func MakeExchangeTable() ExchangeTable {
	return ExchangeTable{
		endpoints:   make(map[string]*endpointInfo),
		peers:       make(map[uint32]peerInfo),
		peerPubkeys: make(map[uint32][32]byte),
	}
}

// SetPeerExpiredCallback imposta la callback per quando un peer scade
func (t *ExchangeTable) SetPeerExpiredCallback(callback PeerExpiredCallback) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onPeerExpired = callback
}

// SetPeerPubkey records the matched public key for a given peer index.
// Called during handshake initiation after the MAC1 check succeeds.
func (t *ExchangeTable) SetPeerPubkey(index uint32, pubkey []byte) {
	if len(pubkey) != 32 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	var k [32]byte
	copy(k[:], pubkey)
	t.peerPubkeys[index] = k
}

// EvictSessionsByRemovedKeys selectively evicts sessions whose associated pubkey
// is NOT in the provided allowedKeys list.
// Peers whose keys are still allowed are completely unaffected — zero downtime.
func (t *ExchangeTable) EvictSessionsByRemovedKeys(allowedKeys [][]byte) {
	// Build fast lookup set
	allowed := make(map[[32]byte]bool, len(allowedKeys))
	for _, k := range allowedKeys {
		if len(k) == 32 {
			var key [32]byte
			copy(key[:], k)
			allowed[key] = true
		}
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	evicted := 0
	for index, peer := range t.peers {
		pubkey, known := t.peerPubkeys[index]
		if !known {
			// Can't identify this peer's key — leave it alone
			continue
		}
		if allowed[pubkey] {
			// Key still valid — skip, zero downtime
			continue
		}
		// Key was removed — evict this peer and its linked counterpart
		if t.onPeerExpired != nil {
			t.onPeerExpired(index, "key_removed")
		}
		if peer.established {
			counterpart := peer.counterpart
			if t.onPeerExpired != nil {
				t.onPeerExpired(counterpart, "key_removed")
			}
			delete(t.peers, counterpart)
			delete(t.peerPubkeys, counterpart)
		}
		delete(t.peers, index)
		delete(t.peerPubkeys, index)
		evicted++
	}
	slog.Info("ExchangeTable: selective eviction done",
		"evicted_sessions", evicted,
		"remaining_sessions", len(t.peers))
}

// EvictAllSessions clears the entire peer and endpoint table.
// Used after a key-list update to force re-handshake for all peers.
func (t *ExchangeTable) EvictAllSessions() {
	t.mu.Lock()
	defer t.mu.Unlock()

	for index, peer := range t.peers {
		if t.onPeerExpired != nil {
			t.onPeerExpired(index, "key_reload")
		}
		delete(t.peers, index)
		_ = peer
	}
	for addr := range t.endpoints {
		delete(t.endpoints, addr)
	}
	for k := range t.peerPubkeys {
		delete(t.peerPubkeys, k)
	}
	slog.Info("ExchangeTable: all sessions evicted")
}
