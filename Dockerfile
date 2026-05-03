# ── Stage 1: Build Svelte PWA ──────────────────────────────────────────────
FROM node:20-alpine AS pwa-builder

WORKDIR /app/ui/web

# Install dependencies first (layer cache)
COPY ui/web/package.json ui/web/package-lock.json* ./
RUN npm install --frozen-lockfile || npm install

# Copy source and build
COPY ui/web/ ./
RUN npm run build

# ── Stage 2: Build Go binary ───────────────────────────────────────────────
FROM golang:1.22-alpine AS go-builder

WORKDIR /app

RUN apk add --no-cache git

# Download deps first (layer cache)
COPY go.mod go.sum ./
RUN go mod download

# Copy all Go source
COPY . .

# Inject the built PWA into the embed target directory
COPY --from=pwa-builder /app/ui/web/dist ./internal/web/dist

# Build with pwa tag so embed.go is compiled in
RUN CGO_ENABLED=0 GOOS=linux go build -tags pwa -ldflags="-s -w" \
    -o /diet-tracker-server ./cmd/server

# ── Stage 3: Minimal runtime ───────────────────────────────────────────────
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

# Install Litestream
ADD https://github.com/benbjohnson/litestream/releases/download/v0.3.13/litestream-v0.3.13-linux-amd64.tar.gz /tmp/litestream.tar.gz
RUN tar -C /usr/local/bin -xzf /tmp/litestream.tar.gz && rm /tmp/litestream.tar.gz

WORKDIR /app

COPY --from=go-builder /diet-tracker-server .
COPY litestream.yml .

RUN mkdir -p /data

EXPOSE 8080
 
# This attempts a restore first, then starts the app + replication
CMD ["sh", "-c", "litestream restore -config /app/litestream.yml -if-db-not-exists -if-replica-exists /data/diet.db && litestream replicate -config /app/litestream.yml -exec /app/diet-tracker-server"]
