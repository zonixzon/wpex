# 🐳 WPEX Docker Setup

Questo documento spiega come costruire e utilizzare l'immagine Docker di WPEX con il sistema di monitoring integrato.

## 🚀 Quick Start

### 1. Build dell'immagine

**Windows:**
```powershell
.\build-docker.ps1
```

**Linux/macOS:**
```bash
chmod +x build-docker.sh
./build-docker.sh
```

### 2. Avvio con Docker Compose (Raccomandato)

```bash
docker-compose up -d
```

### 3. Avvio manuale con Docker

```bash
docker run -d \
  --name wpex-server \
  -p 40000:40000/udp \
  -p 8080:8080/tcp \
  wpex:latest \
  --port 40000 \
  --stats :8080 \
  --broadcast-rate 3
```

## 📊 Accesso al Monitoring

- **Dashboard Web**: http://localhost:8080
- **API JSON**: http://localhost:8080/stats  
- **Health Check**: http://localhost:8080/health

## ⚙️ Configurazione

### Variabili d'ambiente

| Variabile | Descrizione | Default |
|-----------|-------------|---------|
| `TZ` | Fuso orario | `UTC` |

### Porte

| Porta | Protocollo | Descrizione |
|-------|------------|-------------|
| 40000 | UDP | Server VPN WireGuard |
| 8080 | TCP | Dashboard monitoring |

### Volumi

| Volume | Descrizione |
|--------|-------------|
| `/data` | Directory dati applicazione |
| `/var/log/wpex` | Log dell'applicazione |

## 🔧 Opzioni Avanzate

### Build con tag personalizzato

```bash
# Windows
.\build-docker.ps1 -Tag "v1.2.0"

# Linux/macOS  
./build-docker.sh v1.2.0
```

### Build multi-architettura

```bash
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --tag wpex:latest \
  --build-arg WPEX_VERSION="$(git describe --tags --always)" \
  .
```

### Con filtraggio peer (whitelist)

```bash
docker run -d \
  --name wpex-server \
  -p 40000:40000/udp \
  -p 8080:8080/tcp \
  wpex:latest \
  --port 40000 \
  --stats :8080 \
  --allow "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=" \
  --allow "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB=" \
  --broadcast-rate 3
```

## 🏥 Health Check

Il container include un health check automatico che verifica:
- ✅ Raggiungibilità del server HTTP
- ✅ Risposta dell'endpoint `/health`
- ✅ Tempo di risposta sotto 5 secondi

Verifica stato:
```bash
docker ps
# Guarda la colonna STATUS per "(healthy)"
```

## 🔍 Debugging

### Visualizza log

```bash
# Log del container
docker logs wpex-server

# Log in tempo reale
docker logs -f wpex-server

# Con Docker Compose
docker-compose logs -f wpex
```

### Accesso al container

```bash
# Shell interattiva
docker exec -it wpex-server /bin/sh
```

### Test di connettività

```bash
# Test porta VPN
nc -u localhost 40000

# Test dashboard
curl http://localhost:8080/health
```

## 📝 Docker Compose Avanzato

### Con reverse proxy NGINX

Scommenta la sezione `nginx` in `docker-compose.yml` e crea `nginx.conf`:

```nginx
events {
    worker_connections 1024;
}

http {
    upstream wpex {
        server wpex:8080;
    }
    
    server {
        listen 80;
        
        location / {
            proxy_pass http://wpex;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
        }
    }
}
```

### Con persistenza dati

```yaml
services:
  wpex:
    # ... altre configurazioni
    volumes:
      - ./data:/data
      - ./logs:/var/log/wpex
```

## 🔐 Sicurezza

### Considerazioni di sicurezza

1. **Utente non-root**: Il container usa un utente dedicato `wpex:wpex` (UID 1000)
2. **Porte esposte**: Esponi solo le porte necessarie
3. **Rete isolata**: Usa reti Docker custom per isolamento
4. **Secrets**: Non includere chiavi private nell'immagine

### Esempio con secrets

```bash
docker run -d \
  --name wpex-server \
  -p 40000:40000/udp \
  -p 8080:8080/tcp \
  -v /secure/path/keys.txt:/data/keys.txt:ro \
  wpex:latest \
  --port 40000 \
  --stats :8080 \
  --config-file /data/keys.txt
```

## 🚨 Troubleshooting

### Problemi comuni

| Problema | Soluzione |
|----------|-----------|
| Porta 40000 in uso | Cambia mapping: `-p 40001:40000/udp` |
| Dashboard non raggiungibile | Verifica firewall e porta 8080 |
| Build fallisce | Verifica connessione internet e Docker |
| Health check fallisce | Controlla log: `docker logs wpex-server` |

### Reset completo

```bash
# Ferma e rimuovi container
docker-compose down -v

# Rimuovi immagini
docker image rm wpex:latest

# Rebuild completo
docker-compose up --build -d
```

## 📊 Monitoraggio Esterno

### Prometheus metrics (futuro)

Il dashboard espone endpoint compatibili per integrazione futura con Prometheus:

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'wpex'
    static_configs:
      - targets: ['wpex:8080']
```

### Log aggregation

Per aggregare log con ELK o simili:

```yaml
services:
  wpex:
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"
```