FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:1.25 as builder

ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH
ARG WPEX_VERSION

WORKDIR /build

# Copia i file di dipendenza
COPY go.mod go.sum ./

# Scarica le dipendenze
RUN go mod download

# Copia tutto il codice sorgente
COPY . .

# Costruisci l'eseguibile con ottimizzazioni
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-w -s -X main.version=${WPEX_VERSION}" \
    -o wpex .

# Stage finale - immagine leggera
FROM --platform=${TARGETPLATFORM:-linux/amd64} alpine:latest

# Installa ca-certificates per HTTPS e tzdata per timezone
RUN apk --no-cache add ca-certificates tzdata

# Crea utente non-root per sicurezza
RUN addgroup -g 1000 wpex && \
    adduser -D -u 1000 -G wpex wpex

# Crea directory per dati e log
RUN mkdir -p /data /var/log/wpex && \
    chown -R wpex:wpex /data /var/log/wpex

# Copia l'eseguibile
COPY --from=builder /build/wpex /usr/local/bin/wpex
RUN chmod +x /usr/local/bin/wpex

# Espone le porte
EXPOSE 40000/udp 8080/tcp

# Switch all'utente non-root
USER wpex

# Directory di lavoro
WORKDIR /data

# Health check per Docker
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Entry point con configurazione predefinita sensata
ENTRYPOINT ["/usr/local/bin/wpex"]
CMD ["--stats", ":8080"]
