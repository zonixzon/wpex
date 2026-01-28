# WPEX VPN Server - Configurazione di esempio con monitoring

## Avvio base senza monitoring
wpex --port 40000 --broadcast-rate 3

## Avvio con server di statistiche HTTP
wpex --port 40000 --broadcast-rate 3 --stats :8080

## Avvio con filtering dei peer e monitoring
wpex --port 40000 \
     --allow AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA= \
     --allow BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB= \
     --broadcast-rate 3 \
     --stats :8080

## Avvio con debug abilitato e monitoring
wpex --port 40000 --debug --stats :8080

## Avvio con indirizzo specifico per le statistiche  
wpex --port 40000 --stats 0.0.0.0:8080

## Docker con monitoring
docker run -d -p 40000:40000/udp -p 8080:8080 \
  ghcr.io/weiiwang01/wpex:latest \
  --broadcast-rate 3 --stats :8080

## Docker con filtering e monitoring
docker run -d -p 40000:40000/udp -p 8080:8080 \
  ghcr.io/weiiwang01/wpex:latest \
  --allow AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA= \
  --broadcast-rate 3 \
  --stats :8080

# Endpoints disponibili una volta avviato con --stats:

# Dashboard Web (HTML)
# http://localhost:8080/

# API Statistiche (JSON)  
# http://localhost:8080/stats

# Health Check
# http://localhost:8080/health

# Il dashboard mostra:
# - Numero di peer connessi in tempo reale
# - Peer in fase di handshake
# - Sessioni VPN attive
# - Statistiche di trasferimento dati
# - Durata delle connessioni
# - Tasso di successo degli handshake
# - Aggiornamento automatico ogni 30 secondi

# I log includono:
# - Inizio e completamento degli handshake
# - Connessione e disconnessione dei peer
# - Trasferimento dati tra peer
# - Statistiche periodiche del server
# - Rilevamento del roaming dei peer