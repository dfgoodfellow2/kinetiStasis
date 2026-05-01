.PHONY: dev dev-web build pwa-build run tidy tui gen-icons smoke-test deploy

DB_PATH ?= ./data/diet.db

# Auto-load .env if it exists
-include .env
export

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

run: pwa-build
	mkdir -p data
	CGO_ENABLED=0 go build -a -tags pwa -ldflags="-s -w" \
		-o bin/diet-tracker-server ./cmd/server
	DB_PATH=$(DB_PATH) ./bin/diet-tracker-server

tidy:
	go mod tidy

tui:
	mkdir -p bin
	go build -o bin/diet-tui ./cmd/tui/

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
