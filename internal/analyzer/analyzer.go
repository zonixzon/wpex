package analyzer

import (
	"encoding/binary"
	"fmt"
	"log"
	"log/slog"
	"net"
	"time"

	"github.com/weiiwang01/wpex/internal/exchange"
	"github.com/weiiwang01/wpex/internal/stats"
)

const (
	macSize                 = 16
	handshakeInitiationSize = 148
	handshakeResponseSize   = 92
	cookieReplySize         = 64
)

type WireguardAnalyzer struct {
	table   exchange.ExchangeTable
	checker macChecker
	stats   *stats.VPNStats
}

func (t *WireguardAnalyzer) decodeIndex(index []byte) uint32 {
	return binary.BigEndian.Uint32(index)
}

func (t *WireguardAnalyzer) analyseHandshakeInitiation(packet []byte, peer net.UDPAddr) ([]net.UDPAddr, []byte) {
	logger := slog.With("addr", peer.String())
	if len(packet) != handshakeInitiationSize {
		logger.Warn(fmt.Sprintf("invalid handshake initiation: expected length %d, got %d", handshakeInitiationSize, len(packet)))
		return nil, nil
	}

	var matchedPubkey []byte // will be set if key check is enabled

	if t.checker.RequireCheck() {
		pubkey := t.checker.MatchPubkey(packet)
		if pubkey == nil {
			logger.Warn("invalid mac1 in handshake initiation")
			return nil, nil
		}
		matchedPubkey = pubkey
		mac2Ok := t.checker.VerifyMac2(peer, packet)
		known := t.table.Contains(peer)
		if !mac2Ok && !known {
			logger.Debug("send cookie reply to handshake initiation from unknown endpoint")
			reply, err := t.checker.CreateReply(pubkey, peer, packet)
			if err != nil {
				logger.Error(fmt.Sprintf("failed to create cookie reply: %s", err))
				return nil, nil
			}
			return []net.UDPAddr{peer}, reply
		}
		if mac2Ok {
			logger.Debug("strip mac2 from handshake initiation")
			newPacket := make([]byte, handshakeInitiationSize)
			copy(newPacket, packet[:handshakeInitiationSize-macSize])
			packet = newPacket
		}
	}
	sender := t.decodeIndex(packet[4:8])
	if err := t.table.AddPeerAddr(sender, peer); err != nil {
		logger.Error(fmt.Sprintf("failed to add address: %s", err))
		return nil, nil
	}

	// Record the pubkey → sender index mapping for selective key revocation
	if matchedPubkey != nil {
		t.table.SetPeerPubkey(sender, matchedPubkey)
	}

	// Log handshake initiation to stats
	if t.stats != nil {
		t.stats.LogHandshakeInitiated(sender, peer)
	}

	addresses := t.table.ListAddrs(peer)
	slog.Debug("handshake initiation message received", "addr", peer.String(), "sender", sender, "broadcast", len(addresses))
	return addresses, packet
}

func (t *WireguardAnalyzer) analyseHandshakeResponse(packet []byte, peer net.UDPAddr) ([]net.UDPAddr, []byte) {
	logger := slog.With("addr", peer.String())
	if len(packet) != handshakeResponseSize {
		logger.Warn(fmt.Sprintf("invalid handshake response: expected length %d, got %d", handshakeResponseSize, len(packet)))
		return nil, nil
	}
	if t.checker.RequireCheck() {
		if t.checker.MatchPubkey(packet) == nil {
			logger.Warn("invalid mac1 in handshake response")
			return nil, nil
		}
		if t.checker.VerifyMac2(peer, packet) {
			logger.Debug("strip mac2 from handshake response")
		}
	}
	sender := t.decodeIndex(packet[4:8])
	if err := t.table.AddPeerAddr(sender, peer); err != nil {
		logger.Error(fmt.Sprintf("failed to add address: %s", err))
		return nil, nil
	}
	receiverIdx := t.decodeIndex(packet[8:12])
	receiver, err := t.table.GetPeerAddr(receiverIdx)
	slog.Debug("handshake response message received", "addr", peer.String(), "sender", sender, "receiver", receiverIdx, "forward", receiver.String())
	if err != nil {
		logger.Warn(fmt.Sprintf("unknown receiver in handshake response: %s", err))
		return nil, nil
	}
	err = t.table.AssociatePeers(sender, receiverIdx)
	if err != nil {
		logger.Error(fmt.Sprintf("failed to link peers: %s", err))
		return nil, nil
	}

	// Log handshake completion to stats
	if t.stats != nil {
		t.stats.LogHandshakeCompleted(sender, receiverIdx)
	}

	slog.Info("🤝 Handshake completed - peers associated",
		"sender_index", sender,
		"receiver_index", receiverIdx,
		"sender_addr", peer.String(),
		"receiver_addr", receiver.String())

	return []net.UDPAddr{receiver}, packet
}

func (t *WireguardAnalyzer) analyseCookieReply(packet []byte, peer net.UDPAddr) ([]net.UDPAddr, []byte) {
	logger := slog.With("addr", peer.String())
	if len(packet) != cookieReplySize {
		logger.Warn(fmt.Sprintf("invalid wireguard cookie reply message: expected length %d, got %d", cookieReplySize, len(packet)))
		return nil, nil
	}
	receiverIdx := t.decodeIndex(packet[4:8])
	receiver, err := t.table.GetPeerAddr(receiverIdx)
	slog.Debug("cookie reply message received", "addr", peer.String(), "receiver", receiver, "forward", receiver.String())
	if err != nil {
		logger.Warn(fmt.Sprintf("unknown receiver in cookie reply: %s", err))
		return nil, nil
	}
	return []net.UDPAddr{receiver}, packet
}

func (t *WireguardAnalyzer) analyseTransportData(packet []byte, peer net.UDPAddr) ([]net.UDPAddr, []byte) {
	receiverIdx := t.decodeIndex(packet[4:8])
	receiver, err := t.table.GetPeerAddr(receiverIdx)
	if err != nil {
		slog.Warn("Transport data: unknown receiver",
			"receiver_index", receiverIdx,
			"source_addr", peer.String(),
			"error", err.Error())
		return nil, nil
	}

	sender, err := t.table.GetPeerCounterpart(receiverIdx)
	if err != nil {
		slog.Warn("Transport data: unknown sender (no counterpart)",
			"receiver_index", receiverIdx,
			"receiver_addr", receiver.String(),
			"source_addr", peer.String(),
			"error", err.Error())
		return nil, nil
	}

	addr, err := t.table.GetPeerAddr(sender)
	if err != nil {
		slog.Warn("Transport data: no sender address record",
			"sender_index", sender,
			"receiver_index", receiverIdx,
			"source_addr", peer.String(),
			"error", err.Error())
		return nil, nil
	}

	if !addrEqual(addr, peer) {
		slog.Info("🔄 Roaming detected in transport data",
			"sender_index", sender,
			"old_addr", addr.String(),
			"new_addr", peer.String())
		err := t.table.UpdatePeerAddr(sender, peer)
		if err != nil {
			slog.Error("Failed to update sender address during roaming",
				"sender_index", sender,
				"new_addr", peer.String(),
				"error", err.Error())
			return nil, nil
		}
	}

	// Log data transfer to stats
	if t.stats != nil {
		t.stats.LogDataTransfer(sender, receiverIdx, len(packet), peer)
	}

	slog.Debug("✅ Transport data forwarded",
		"from_index", sender,
		"to_index", receiverIdx,
		"from_addr", peer.String(),
		"to_addr", receiver.String(),
		"packet_size", len(packet))

	return []net.UDPAddr{receiver}, packet
}

// Analyse updates the exchange table with the source address and returns the forwarding address for this packet.
func (t *WireguardAnalyzer) Analyse(packet []byte, peer net.UDPAddr) ([]net.UDPAddr, []byte) {
	const (
		handshakeInitiationType = iota + 1
		handshakeResponseType
		cookieReplyType
		transportDataType
	)
	if len(packet) < 16 {
		slog.Error("invalid wireguard message: too short", "addr", peer.String())
		return nil, nil
	}
	msgType := int(binary.LittleEndian.Uint32(packet[:4]))
	switch msgType {
	case handshakeInitiationType:
		return t.analyseHandshakeInitiation(packet, peer)
	case handshakeResponseType:
		return t.analyseHandshakeResponse(packet, peer)
	case cookieReplyType:
		return t.analyseCookieReply(packet, peer)
	case transportDataType:
		return t.analyseTransportData(packet, peer)
	default:
		slog.Error("unknown message type", "addr", peer.String())
		return nil, nil
	}
}

func MakeWireguardAnalyzer(pubkeys [][]byte) WireguardAnalyzer {
	secret, err := token(32)
	if err != nil {
		log.Fatal(fmt.Errorf("failed to generate cookie secret: %w", err))
	}

	vpnStats := stats.NewVPNStats()
	table := exchange.MakeExchangeTable()

	// Imposta il callback per le disconnessioni
	table.SetPeerExpiredCallback(func(peerIndex uint32, reason string) {
		vpnStats.LogPeerDisconnected(peerIndex, reason)
	})

	return WireguardAnalyzer{
		table: table,
		checker: macChecker{
			pubkeys: pubkeys,
			secret:  [32]byte(secret),
			start:   time.Now(),
		},
		stats: vpnStats,
	}
}

// GetStats restituisce le statistiche VPN
func (t *WireguardAnalyzer) GetStats() *stats.VPNStats {
	return t.stats
}

// UpdateAllowedKeys hot-swaps the allowed WireGuard public keys.
// Only sessions belonging to removed keys are evicted.
// Sessions for still-valid keys are completely unaffected (zero downtime).
func (t *WireguardAnalyzer) UpdateAllowedKeys(newKeys [][]byte) {
	t.checker.UpdatePubkeys(newKeys)
	t.table.EvictSessionsByRemovedKeys(newKeys)
	slog.Info("🔑 Allowed keys updated — removed-key sessions evicted, valid peers unaffected",
		"allowed_key_count", len(newKeys))
}
