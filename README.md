# wpex — WireGuard Packet Relay

`wpex` is a transparent relay server for WireGuard, enabling NAT traversal without compromising end-to-end encryption.

## Features

- **Transparent relay**: Cannot tamper with or decrypt WireGuard traffic by design.
- **Zero MTU overhead**: No tunneling, no extra encapsulation.
- **Works with any WireGuard client**: No software changes required on peers (Mikrotik, Linux, Windows, mobile…).
- **Allowlist-based anti-amplification**: Validates `mac1` against a whitelist of authorized WireGuard public keys — no private key needed.
- **Real-time statistics HTTP server**: Exposes peer status, handshake metrics, bytes transferred and health scores via JSON API.
- **Remote diagnostics**: Run `ping` and `traceroute` from inside the relay container via HTTP API.
- **Hot-reload of allowed keys**: Update the authorized public key list at runtime without restarting the process.
- **Multi-CPU UDP listener**: Spawns one goroutine per CPU core for high-throughput packet forwarding.
- **Peer cleanup**: Automatically expires stale peer sessions (default: 45s timeout, 5s check interval).

---

## Why `wpex`

Common WireGuard NAT traversal approaches and their tradeoffs:

| Approach | Downside |
|---|---|
| TURN / DERP | Requires installing a client on every WireGuard peer |
| Hub-and-spoke IP forwarding | Packets are decrypted on the cloud server |
| Tunneling | MTU overhead + isolation complexity |

`wpex` solves all three: no agent required, no decryption, no MTU overhead.

---

## Installation

### Docker (recommended for production via WPEX Orchestrator)

```bash
docker run -d \
  -p 40000:40000/udp \
  -p 8080:8080 \
  nikoceps/wpex-monitoring:latest \
  --port 40000 \
  --allow AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA= \
  --allow BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB= \
  --stats :8080
```

### Build from Source

Requires Go 1.21+:

```bash
git clone https://github.com/weiiwang01/wpex.git
cd wpex
go build -o wpex .
```

---

## Usage

### Command-Line Flags

| Flag | Default | Description |
|---|---|---|
| `--port` | `40000` | UDP port to listen on |
| `--bind` | `` (all interfaces) | Address to bind to |
| `--allow` | _(none)_ | Authorize a WireGuard public key (base64). Repeat for multiple keys. |
| `--broadcast-rate` | `0` (unlimited) | Max broadcast packets/sec (anti-amplification fallback) |
| `--stats` | `` (disabled) | Enable HTTP stats server on this address (e.g. `:8080`) |
| `--debug` | `false` | Enable verbose debug logging |
| `--version` | `false` | Print version and exit |

### Basic Example

```bash
# Relay on port 40000 with two authorized peers and stats server
wpex \
  --port 40000 \
  --allow AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA= \
  --allow BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB= \
  --stats :8080
```

### WireGuard Client Configuration

Both peers must point their `Endpoint` to the relay and enable `PersistentKeepalive`:

**Peer A** (`wg0.conf`):
```ini
[Interface]
PrivateKey = <private_key_A>

[Peer]
PublicKey  = <public_key_B>
Endpoint   = relay.example.com:40000
PersistentKeepalive = 25
AllowedIPs = <peer_B_subnet>
```

**Peer B** (`wg0.conf`):
```ini
[Interface]
PrivateKey = <private_key_B>

[Peer]
PublicKey  = <public_key_A>
Endpoint   = relay.example.com:40000
PersistentKeepalive = 25
AllowedIPs = <peer_A_subnet>
```

> Both public keys must be passed to `wpex` via `--allow` or the handshake will be rejected (`invalid mac1`).

---

## Protections Against Amplification Attacks

By default, `wpex` broadcasts handshake initiations to all known endpoints. This can be exploited for amplification attacks.

### Option 1 — Public key allowlist (recommended)

Pass all authorized WireGuard public keys via `--allow`. `wpex` will verify the `mac1` field of every handshake initiation and reject unknown peers immediately. This is the approach used by WPEX Orchestrator.

```bash
wpex --allow <pubkey1> --allow <pubkey2>
```

### Option 2 — Broadcast rate limit

Limit how many broadcast packets per second `wpex` will send:

```bash
wpex --broadcast-rate 3
```

Formula for an ideal rate: given `N` peers and `K` peer-to-peer pairs, the theoretical max broadcast rate is `(N - 1) × K × 2 / 5`.

---

## Statistics & Monitoring

Enable the HTTP server with `--stats :8080`:

### Endpoints

| Path | Method | Description |
|---|---|---|
| `/` | GET | HTML dashboard with live stats |
| `/health` | GET | Simple health check |
| `/stats` | GET | Raw statistics JSON |
| `/api/v1/stats` | GET | Enhanced stats: success rate, sorted peer list, uptime |
| `/api/v1/health` | GET | Weighted health score with component breakdown |
| `/api/v1/config` | GET | Runtime configuration (CLI args, Go version, goroutines) |
| `/api/v1/config/reload` | POST | Hot-reload authorized public keys without restart |
| `/api/v1/diagnostics/ping` | POST | Run `ping` from inside the container |
| `/api/v1/diagnostics/traceroute` | POST | Run `traceroute` from inside the container |

### Example: `/api/v1/stats` response

```json
{
  "uptime_human": "2h5m30s",
  "total_handshakes": 42,
  "successful_handshakes": 41,
  "success_rate": 97.6,
  "active_sessions": 2,
  "connected_peers": 2,
  "total_bytes": 104857600,
  "total_transfer_mb": 100.0,
  "peers": [
    {
      "index": 12345,
      "address": "93.43.72.138:13231",
      "status": "connected",
      "bytes_sent": 52428800,
      "bytes_received": 52428800,
      "uptime_seconds": 7530
    }
  ]
}
```

### Example: hot-reload allowed keys

```bash
curl -X POST http://localhost:8080/api/v1/config/reload \
  -H 'Content-Type: application/json' \
  -d '{"public_keys": ["AAAA...=", "BBBB...="]}'
```

---

## How `wpex` Works

### Packet Routing

Each WireGuard session uses a random 32-bit **peer index** to identify the sender. `wpex` builds a routing table:

```
peer_index → UDP endpoint (IP:port)
```

Packets are forwarded to the peer matching the `receiver_index` in the WireGuard message header.

### Handshake Initiation (broadcast phase)

The initial handshake has no receiver index yet. `wpex` broadcasts it to all known endpoints. Only the correct peer responds; others discard it.

### Anti-Replay via Cookie

When a new endpoint initiates a handshake, `wpex` sends a **cookie reply** (constructable using only the public key). The peer must re-send the handshake with a valid `mac2` derived from that cookie — this proves the source IP is legitimate and prevents replay attacks. The `mac2` is stripped before forwarding.

### Peer Cleanup

Stale peers (no activity for 45 seconds) are evicted every 5 seconds to prevent memory leaks. Duplicates (same address, multiple peer indexes) are resolved keeping the newest active session.

---

## Logging

Key log messages:

```
INFO  server listening                addr=:40000
INFO  HTTP statistics server started  addr=:8080
INFO  Handshake initiated             peer_index=12345 address=1.2.3.4:51820 total_handshakes=1
INFO  Handshake completed             sender_index=12345 receiver_index=67890 active_sessions=1
WARN  invalid mac1 in handshake initiation  addr=1.2.3.4:51821   ← unknown pubkey → rejected
INFO  Removed duplicate peer          removed_index=999 kept_index=12345 address=1.2.3.4:51820
INFO  Peer disconnected               peer_index=12345 reason=session_timeout session_duration=10m
```

---

## Integration with WPEX Orchestrator

When deployed via [WPEX Orchestrator](https://github.com/nikSlavv/wpex-orchestrator), `wpex` runs as a Kubernetes Deployment in the `wpex` namespace. The Orchestrator:

1. Creates the Deployment with `--port`, `--allow` (one per authorized key), and `--stats :8080`
2. Creates a NodePort Service exposing the UDP port externally and TCP 8080 internally
3. Polls `/api/v1/stats` and `/api/v1/health` periodically for the dashboard
4. Executes diagnostics via K8s pod exec (ping/traceroute)
