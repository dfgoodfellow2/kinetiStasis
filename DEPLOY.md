# Deployment Guide — Diet Tracker v2

## Prerequisites

- [flyctl](https://fly.io/docs/hands-on/install-flyctl/) installed and authenticated
- [Node.js 20+](https://nodejs.org/) for PWA builds
- Go 1.22+
- A [Tigris](https://console.tigris.dev/) (or S3-compatible) bucket for Litestream backups

## First-Time Setup

### 1. Create the fly.io app

```bash
fly launch --no-deploy --name diet-tracker-v2 --region iad
```

### 2. Create a persistent volume

```bash
fly volumes create diet_data --size 1 --region iad
```

### 3. Create a Tigris bucket (for Litestream)

```bash
fly storage create
```

Note the bucket name, region, access key, and secret key.

### 4. Set secrets

```bash
bash scripts/set_fly_secrets.sh
```

Edit the script first to set your real values. Or set individually:

```bash
fly secrets set JWT_SECRET="$(openssl rand -hex 32)"
fly secrets set GEMINI_API_KEY="your-key"
fly secrets set LITESTREAM_BUCKET="your-bucket"
fly secrets set LITESTREAM_REGION="auto"
fly secrets set LITESTREAM_ACCESS_KEY_ID="your-key-id"
fly secrets set LITESTREAM_SECRET_ACCESS_KEY="your-secret"
fly secrets set LITESTREAM_ENDPOINT="https://fly.storage.tigris.dev"
```

### 5. Deploy

```bash
make deploy
# or: fly deploy
```

## Local Development

```bash
# Copy and edit env
cp .env.example .env

# Start API server (no PWA)
make dev

# Start Svelte dev server (separate terminal)
make dev-web
# Open http://localhost:5173
```

## Full Local Build (with embedded PWA)

```bash
make build
./bin/diet-tracker-server
# Open http://localhost:8080
```

## Litestream Restore (disaster recovery)

If the volume is wiped, Litestream restores automatically on next startup via:
```
litestream replicate -exec /app/diet-tracker-server
```

To manually restore to a local path:
```bash
litestream restore -config litestream.yml -o ./recovered.db /data/diet.db
```

## Updating

```bash
git pull
make deploy
```

fly.io performs a rolling deploy — zero downtime.

## Custom Domain

```bash
fly certs create your-domain.com
# Follow DNS instructions from fly.io
```

Then update `APP_DOMAIN` in `fly.toml`:
```toml
APP_DOMAIN = "https://your-domain.com"
```

And redeploy:
```bash
make deploy
```

## Smoke Test

Run against local server:
```bash
make smoke-test
```

Run against production:
```bash
DIET_API_URL=https://diet-tracker-v2.fly.dev make smoke-test
```
