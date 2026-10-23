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

`fly storage create` also injects the Litestream credentials as app secrets automatically
under the names `litestream.yml` reads: `BUCKET_NAME`, `AWS_REGION`, `AWS_ENDPOINT_URL_S3`,
`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`. Only set them manually if you are using your
own bucket or rotating keys.

### 4. Set secrets

```bash
bash scripts/set_fly_secrets.sh
```

Edit the script first to set your real values. Or set individually:

```bash
fly secrets set JWT_SECRET="$(openssl rand -hex 32)"
fly secrets set GEMINI_API_KEY="your-key"
fly secrets set BUCKET_NAME="your-bucket"
fly secrets set AWS_REGION="auto"
fly secrets set AWS_ENDPOINT_URL_S3="https://t3.storage.dev"
fly secrets set AWS_ACCESS_KEY_ID="your-key-id"
fly secrets set AWS_SECRET_ACCESS_KEY="your-secret"
```

Both Tigris endpoints work: `https://t3.storage.dev` is the current global endpoint (used in
production), and `https://fly.storage.tigris.dev` also works for fly-provisioned buckets.

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

If the volume is wiped, Litestream restores automatically on next startup. The container CMD
runs an explicit restore step before replication starts:

```
litestream restore -config /app/litestream.yml -if-db-not-exists -if-replica-exists /data/diet.db && litestream replicate -config /app/litestream.yml -exec /app/diet-tracker-server
```

The restore flag `-if-db-not-exists` skips work when a local DB is already present, and
`-if-replica-exists` skips download when no replica exists yet. On a fresh volume with no
replica, restore exits 0 and the goose migrations in the server create the schema from scratch.

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
