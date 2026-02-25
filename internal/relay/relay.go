package relay

import (
	"context"
	"fmt"
	"github.com/weiiwang01/wpex/internal/analyzer"
	"github.com/weiiwang01/wpex/internal/stats"
	"golang.org/x/time/rate"
	"log"
	"log/slog"
	"net"
	"runtime"
	"syscall"
	"time"
)

type udpPacket struct {
	addr        net.Addr
	data        []byte
	source      net.Addr
	isBroadcast bool
}

type Relay struct {
	send     chan udpPacket
	analyzer analyzer.WireguardAnalyzer
	limit    *rate.Limiter
}

func (r *Relay) relay(conn *net.UDPConn) {
	buf := make([]byte, 65536)
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			slog.Error("error while receiving UDP packet", "error", err.Error(), "addr", remoteAddr)
			continue
		}
		packet := buf[:n]
		peers, send := r.analyzer.Analyse(packet, *remoteAddr)
		for _, peer := range peers {
			if len(peers) > 1 {
				if !r.limit.Allow() {
					slog.Warn("broadcast rate limit exceeded", "src", remoteAddr.String(), "dst", peer.String())
					continue
				}
			}
			_, err := conn.WriteToUDP(send, &peer)
			if err != nil {
				slog.Error("error while sending UDP packet", "error", err.Error(), "addr", peer.String())
			}
		}
	}
}

// Start starts the wireguard packet relay server.
func Start(address string, publicKeys [][]byte, broadcastLimit *rate.Limiter) {
	StartWithStatsServer(address, publicKeys, broadcastLimit, "")
}

// StartWithStatsServer starts the wireguard packet relay server with optional HTTP stats server.
func StartWithStatsServer(address string, publicKeys [][]byte, broadcastLimit *rate.Limiter, statsAddr string) {
	slog.Info("server listening", "addr", address)
	var lc = net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			var opErr error
			if err := c.Control(func(fd uintptr) { opErr = control(fd) }); err != nil {
				return err
			}
			return opErr
		},
	}
	relay := Relay{
		send:     make(chan udpPacket),
		analyzer: analyzer.MakeWireguardAnalyzer(publicKeys),
		limit:    broadcastLimit,
	}
	
	// Avvia il server HTTP per le statistiche se richiesto
	var httpServer *stats.HTTPServer
	if statsAddr != "" {
		httpServer = stats.NewHTTPServer(statsAddr, relay.analyzer.GetStats(), &relay.analyzer)
		if err := httpServer.Start(); err != nil {
			slog.Error("Failed to start HTTP stats server", "error", err)
		} else {
			slog.Info("HTTP statistics server started", "addr", statsAddr)
		}
	}
	
	// Avvia il logging periodico delle statistiche
	go func() {
		ticker := time.NewTicker(5 * time.Minute) // Log ogni 5 minuti
		cleanupTicker := time.NewTicker(30 * time.Second) // Cleanup bilanciato ogni 30 secondi
		defer ticker.Stop()
		defer cleanupTicker.Stop()
		
		for {
			select {
			case <-ticker.C:
				relay.analyzer.GetStats().LogPeriodicStats()
			case <-cleanupTicker.C:
				// Cleanup dei peer scaduti dalle statistiche (3 minuti per bilanciare reattività e stabilità)
				relay.analyzer.GetStats().CleanupExpiredPeers(3 * time.Minute)
			}
		}
	}()
	
	for i := 0; i < runtime.NumCPU(); i++ {
		l, err := lc.ListenPacket(context.Background(), "udp", address)
		if err != nil {
			log.Fatal(fmt.Sprintf("failed to listen on %s: %s", address, err))
		}
		conn := l.(*net.UDPConn)
		go relay.relay(conn)
	}
	
	// Graceful shutdown handler (opzionale)
	if httpServer != nil {
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			httpServer.Stop(ctx)
		}()
	}
	
	select {}
}
