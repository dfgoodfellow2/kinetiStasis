
.PHONY: dev dev-web build pwa-build run tidy tui clean-all kill-server check-env dev-full smoke-test deploy gen-icons

DB_PATH ?= ./data/diet.db

# Auto-load .env if it exists
-include .env
export

# Kill any existing server on port 8080
kill-server:
	@echo "Checking for existing server on port 8080..."
	@-sudo fuser -k 8080/tcp 2>/dev/null || true
	@echo "Port 8080 freed (if it was occupied)"

# Check .env configuration
check-env:
	@echo "Checking .env configuration..."
	@if [ -f .env ]; then \
		if grep -q "JWT_SECRET=REPLACE_WITH_OPENSSL_RAND_HEX_32" .env; then \
			echo "⚠️  WARNING: JWT_SECRET is still a placeholder!"; \
			echo "   Run: openssl rand -hex 32"; \
			echo "   Then update .env with the output"; \
		elif grep -q "JWT_SECRET=" .env; then \
			echo "✅ JWT_SECRET is set"; \
		fi; \
	else \
		echo "❌ .env file not found!"; \
	fi

dev:
	mkdir -p data
	DB_PATH=$(DB_PATH) ENV=development go run ./cmd/server

dev-web:
	cd ui/web && npm run dev

pwa-build:
	cd ui/web && npm install && npm run build
	mkdir -p internal/web/dist
	cp -r ui/web/dist/. internal/web/dist/

build: pwa-build
	mkdir -p data
	CGO_ENABLED=0 go build -tags pwa -ldflags="-s -w" \
		-o bin/diet-tracker-server ./cmd/server

run: kill-server check-env pwa-build
	mkdir -p data
	CGO_ENABLED=0 go build -a -tags pwa -ldflags="-s -w" \
		-o bin/diet-tracker-server ./cmd/server
	@echo ""
	@echo "🎯 Server will start with NEW code (SameSite=Lax for dev)"
	@echo "📝 Don't forget to:"
	@echo "   1. Clear browser cookies for localhost:8080"
	@echo "   2. Hard refresh (Ctrl+Shift+R)"
	@echo "   3. Access via http://localhost:8080 (not :5173)"
	@echo ""
	DB_PATH=$(DB_PATH) ./bin/diet-tracker-server

# Run both Go server and Vite dev server (with proxy)
# NOTE: Vite proxies to :8080. Start the Go server first, then Vite.
# This target documents the TWO-terminal startup sequence.
dev-full: kill-server check-env
	@echo "Terminal 1: Starting Go server..."
	DB_PATH=$(DB_PATH) ENV=development ./bin/diet-tracker-server &
	@echo "Waiting for server to be ready..."
	sleep 2
	@echo "Terminal 2: Starting Vite dev server..."
	cd ui/web && npm run dev

tidy:
	go mod tidy

tui:
	mkdir -p bin
	go build -o bin/diet-tracker-ui ./cmd/tui/

clean-all: kill-server
	@echo "Cleaning build artifacts..."
	rm -rf bin/diet-tracker-server
	rm -rf ui/web/dist ui/web/node_modules
	rm -rf internal/web/dist
	@echo "✅ Clean complete"

gen-icons:
	mkdir -p ui/web/public
	convert -background '#10b981' -fill white \
		-font DejaVu-Sans-Bold -pointsize 96 \
		-gravity center -size 192x192 \
		label:'🥗' ui/web/public/icon-192.png 2>/dev/null || \
	convert -size 192x192 xc:'#10b981' ui/web/public/icon-192.png
	convert -size 512x512 xc:'#10b981' ui/web/public/icon-512.png
	@echo "Icons generated in ui/web/public/"

smoke-test:
	@bash scripts/smoke_test.sh

deploy:
	fly deploy
