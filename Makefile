# Makefile per WPEX Docker

# Variabili
IMAGE_NAME := wpex
TAG := latest
FULL_IMAGE_NAME := $(IMAGE_NAME):$(TAG)
WPEX_VERSION := $(shell git describe --tags --always 2>/dev/null || echo "dev-$(shell date +%Y%m%d)")

.PHONY: help build run stop clean logs shell test health

# Help
help: ## Mostra questo help
	@echo "🐳 WPEX Docker Makefile"
	@echo ""
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Build
build: ## Build dell'immagine Docker
	@echo "🏗️  Building WPEX Docker image..."
	@echo "📦 Image: $(FULL_IMAGE_NAME)"
	@echo "🏷️  Version: $(WPEX_VERSION)"
	docker build \
		--build-arg WPEX_VERSION="$(WPEX_VERSION)" \
		--tag "$(FULL_IMAGE_NAME)" \
		.
	@echo "✅ Build completato!"

# Build multi-platform
build-multi: ## Build multi-platform (amd64, arm64)
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		--build-arg WPEX_VERSION="$(WPEX_VERSION)" \
		--tag "$(FULL_IMAGE_NAME)" \
		--push \
		.

# Run container
run: ## Avvia il container WPEX
	docker run -d \
		--name wpex-server \
		-p 40000:40000/udp \
		-p 8080:8080/tcp \
		--restart unless-stopped \
		$(FULL_IMAGE_NAME) \
		--port 40000 \
		--stats :8080 \
		--broadcast-rate 3 \
		--debug
	@echo "🚀 Container avviato!"
	@echo "🌐 Dashboard: http://localhost:8080"

# Run con Docker Compose
up: ## Avvia con Docker Compose
	docker-compose up -d
	@echo "🚀 Stack avviato con Docker Compose!"
	@echo "🌐 Dashboard: http://localhost:8080"

# Stop container
stop: ## Ferma il container
	docker stop wpex-server || true
	docker rm wpex-server || true

# Stop Docker Compose
down: ## Ferma Docker Compose stack
	docker-compose down -v

# Clean
clean: ## Rimuovi container e immagini
	docker stop wpex-server || true
	docker rm wpex-server || true
	docker rmi $(FULL_IMAGE_NAME) || true
	docker system prune -f

# Logs
logs: ## Mostra i log del container
	docker logs -f wpex-server

# Logs con Docker Compose
logs-compose: ## Mostra i log con Docker Compose
	docker-compose logs -f wpex

# Shell nel container
shell: ## Accesso shell al container
	docker exec -it wpex-server /bin/sh

# Test dell'immagine
test: ## Test dell'immagine Docker
	@echo "🧪 Testing Docker image..."
	docker run --rm $(FULL_IMAGE_NAME) --version
	@echo "✅ Test completato!"

# Health check
health: ## Verifica health del container
	@echo "🏥 Checking container health..."
	@docker ps --filter name=wpex-server --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
	@echo ""
	@echo "🔍 Testing endpoints:"
	@curl -s http://localhost:8080/health || echo "❌ Health endpoint non raggiungibile"
	@curl -s http://localhost:8080/stats > /dev/null && echo "✅ Stats endpoint OK" || echo "❌ Stats endpoint non raggiungibile"

# Push to registry
push: build ## Build e push to registry
	docker push $(FULL_IMAGE_NAME)
	@echo "📤 Push completato!"

# Rebuild from scratch
rebuild: clean build ## Clean completo e rebuild

# Dev mode - build and run with debug
dev: build ## Build e run in modalità development
	$(MAKE) stop
	docker run -d \
		--name wpex-server \
		-p 40000:40000/udp \
		-p 8080:8080/tcp \
		-v $(PWD)/logs:/var/log/wpex \
		$(FULL_IMAGE_NAME) \
		--port 40000 \
		--stats :8080 \
		--debug
	@echo "🔧 Dev container avviato con volume logs montato"
	@echo "🌐 Dashboard: http://localhost:8080"

# Show Docker info
info: ## Mostra informazioni Docker
	@echo "📊 Docker Images:"
	@docker images $(IMAGE_NAME) --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}\t{{.CreatedAt}}"
	@echo ""
	@echo "🏃 Running Containers:"
	@docker ps --filter ancestor=$(FULL_IMAGE_NAME) --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"