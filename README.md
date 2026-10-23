# Diet Tracker v2

A personal diet and fitness tracking application for family use (2-10 users). Fully self-hosted, deployed on fly.io. Built as an API-first Go monolith with three client surfaces.

## Stack

- **Go 1.22+** — API server
- **SQLite (modernc, pure Go)** — database
- **Litestream** — continuous backup to S3/Tigris
- **Chi** — HTTP router
- **Bubble Tea** — TUI client
- **Svelte 5** — PWA client

## Three Clients

| Client | Path | Tech |
|--------|------|------|
| REST API | `cmd/server/` | Go + chi |
| Terminal UI (TUI) | `cmd/tui/` | Bubble Tea |
| Progressive Web App (PWA) | `ui/web/` | Svelte 5 + Tailwind |

## Architecture

- **Language**: Go 1.22+
- **Router**: `github.com/go-chi/chi/v5`
- **Database**: SQLite via `modernc.org/sqlite` (pure Go, no CGO)
- **Auth**: JWT HS256 access tokens (15 min) + opaque refresh tokens (30 days) — both httpOnly cookies; dev uses `SameSite=Lax`, production uses `SameSite=None; Secure`
- **Passwords**: bcrypt cost 12
- **Backup**: Litestream → Tigris/S3 continuous streaming
- **AI Parsing**: Google Gemini API (`gemini-2.5-flash-lite`), server-side only
- **Deployment**: fly.io, region `iad`, 256MB shared CPU
- **Multi-user**: Fully isolated data, first registered = admin

## Quick Start (Local Development)

### Prerequisites

- Go 1.22+
- Node.js 20+
- `make`
- (Optional) ImageMagick — for `make gen-icons`
- (Optional) `flyctl` — for deployment

### 1. Clone / Navigate to Project

```bash
cd /home/dfgoodfellow2/Projects/Personal/Diet_Tracker
```

### 2. Create Your `.env` File

```bash
cp .env.example .env
```

Edit `.env` — the minimum required values for local dev:

```env
PORT=8080
ENV=development
DB_PATH=./data/diet.db
JWT_SECRET=any-string-at-least-32-chars-long-here
GEMINI_API_KEY=           # leave blank to disable AI parsing
APP_DOMAIN=               # not needed for local dev
```

> `JWT_SECRET` is the only hard requirement. Generate a good one:
> ```bash
> openssl rand -hex 32
> ```

### 3. Running the API Server (Dev Mode)

```bash
make dev
```

- Starts Go server on `http://localhost:8080`
- Uses `ENV=development` (text logs, no PWA embedding)
- SQLite DB created at `./data/diet.db` on first run
- Goose migrations run automatically on startup

### 4. Running the PWA Dev Server

In a **second terminal**:

```bash
make dev-web
# or:
cd ui/web && npm install && npm run dev
```

- Starts Vite on `http://localhost:5173`
- All `/v1/` requests are proxied to `http://localhost:8080`
- Hot reload on Svelte file changes

### 5. Running the TUI Client

In a **separate terminal** (while server is running):

```bash
make tui                  # Build the binary first
./bin/diet-tracker-ui     # Run it
```

Or point at a different server:

```bash
DIET_API_URL=http://localhost:8080 ./bin/diet-tracker-ui
```

> **Frozen (2026-10-23):** the TUI is a secondary client, kept working at its
> current feature parity. New features are PWA-only; no new TUI screens or
> workflows are planned. See [docs/TUI_STATUS.md](docs/TUI_STATUS.md) for scope,
> policy, and known gaps.

### 6. Full Embedded Build (API + PWA in one binary)

```bash
make build
```

This:
1. Runs `cd ui/web && npm install && npm run build`
2. Copies `ui/web/dist/` → `internal/web/dist/`
3. Compiles Go with `-tags pwa` — dist is embedded in binary

Then run:

```bash
./bin/diet-tracker-server
# Open: http://localhost:8080
# API: http://localhost:8080/v1/health
# PWA: http://localhost:8080/
```

### 7. First User Registration

The first user to register automatically becomes **admin**.

```bash
curl -X POST http://localhost:8080/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","email":"admin@local.dev","password":"YourPassword123!"}'
```

## Makefile Targets

| Target | Description |
|--------|-------------|
| `make dev` | Run Go server in dev mode (no PWA) |
| `make dev-web` | Run Svelte dev server on :5173 |
| `make build` | Build PWA + embed + compile Go binary |
| `make tui` | Build TUI binary → `bin/diet-tracker-ui` (frozen; see `docs/TUI_STATUS.md`) |
| `make run` | Build then run the server |
| `make tidy` | `go mod tidy` |
| `make gen-icons` | Generate PNG PWA icons from SVG (needs ImageMagick) |
| `make smoke-test` | Run 35-endpoint smoke test against localhost |
| `make deploy` | `fly deploy` |

## Deployment (fly.io)

### Platform

fly.io — app name `diet-tracker-v2`, region `iad` (US East), 256MB shared CPU VM.

### First-Time Deploy

```bash
# 1. Authenticate
fly auth login

# 2. Create app (if not already created)
fly launch --no-deploy --name diet-tracker-v2 --region iad

# 3. Create persistent volume
fly volumes create diet_data --size 1 --region iad

# 4. Create Tigris bucket for Litestream
fly storage create

# 5. Set secrets
fly secrets set JWT_SECRET="$(openssl rand -hex 32)"
fly secrets set GEMINI_API_KEY="your-key"
# Litestream/Tigris backup secrets (BUCKET_NAME, AWS_REGION,
# AWS_ENDPOINT_URL_S3, AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY) are also
# required for deploy — see DEPLOY.md §3.

# 6. Deploy
make deploy
```

### Subsequent Deploys

```bash
make deploy
# or: fly deploy
```

fly.io performs rolling deploys — zero downtime.

### fly.toml Key Settings

```toml
app = "diet-tracker-v2"
primary_region = "iad"

[env]
PORT = "8080"
DB_PATH = "/data/diet.db"
ENV = "production"
APP_DOMAIN = "https://kinetistasis.com"

[[mounts]]
source = "diet_data"
destination = "/data"
initial_size = "1gb"
```

## Smoke Testing

Ensure the server is running first, then:

```bash
make smoke-test
```

To run against production:

```bash
DIET_API_URL=https://diet-tracker-v2.fly.dev make smoke-test
```

## API Endpoints

| Endpoint | Description |
|----------|-------------|
| `POST /v1/auth/register` | Register new user |
| `POST /v1/auth/login` | Login |
| `POST /v1/auth/refresh` | Refresh token |
| `POST /v1/auth/logout` | Logout |
| `GET /v1/auth/me` | Get current user |
| `GET /v1/admin/users` | List users (admin) |
| `DELETE /v1/admin/users/{userID}` | Delete user (admin) |
| `POST /v1/admin/users/{userID}/promote` | Promote user (admin) |
| `POST /v1/admin/users/{userID}/demote` | Demote user (admin) |
| `GET /v1/profile` | Get profile |
| `PUT /v1/profile` | Update profile |
| `GET /v1/nutrition/logs` | List nutrition logs |
| `GET /v1/nutrition/logs/{date}` | Get nutrition log by date |
| `POST /v1/nutrition/logs` | Log nutrition |
| `PUT /v1/nutrition/logs/{date}` | Update nutrition log |
| `DELETE /v1/nutrition/logs/{date}` | Delete nutrition log |
| `GET /v1/biometrics` | List biometric logs |
| `GET /v1/biometrics/{date}` | Get biometric log by date |
| `POST /v1/biometrics` | Log biometrics |
| `PUT /v1/biometrics/{date}` | Update biometric log |
| `DELETE /v1/biometrics/{date}` | Delete biometric log |
| `GET /v1/workouts` | Get workouts |
| `GET /v1/workouts/{date}/{slot}` | Get workout by date and slot |
| `POST /v1/workouts` | Log workout |
| `PUT /v1/workouts/{date}/{slot}` | Update workout |
| `DELETE /v1/workouts/{date}/{slot}` | Delete workout |
| `GET /v1/targets` | Get macro targets |
| `PUT /v1/targets` | Update macro targets |
| `GET /v1/targets/history` | Get macro target history |
| `GET /v1/meals/saved` | List saved meals |
| `POST /v1/meals/saved` | Save a meal |
| `DELETE /v1/meals/saved/{id}` | Delete saved meal |
| `GET /v1/meals/templates` | List meal templates |
| `POST /v1/meals/templates` | Create meal template |
| `DELETE /v1/meals/templates/{id}` | Delete meal template |
| `GET /v1/measurements` | Get measurements |
| `POST /v1/measurements` | Add measurement |
| `PUT /v1/measurements/{date}` | Update measurement |
| `DELETE /v1/measurements/{date}` | Delete measurement |
| `GET /v1/calc/tdee` | Calculate TDEE |
| `GET /v1/calc/macros` | Calculate macros |
| `GET /v1/calc/readiness` | Calculate readiness score |
| `GET /v1/calc/bodyfat` | Calculate body fat (Navy) |
| `POST /v1/calc/macros/distribute` | Auto-distribute macros for a calorie target |
| `GET /v1/dashboard` | Get dashboard data |
| `GET /v1/checkin` | Preview weekly check-in |
| `POST /v1/checkin` | Create weekly check-in |
| `POST /v1/parse/meal` | AI parse meal (Gemini) |
| `POST /v1/parse/workout` | AI parse workout (Gemini) |
| `GET /v1/export/nutrition` | Export nutrition (CSV/MD) |
| `GET /v1/export/workouts` | Export workouts (CSV/MD) |
| `GET /v1/export/combined` | Export combined nutrition + workouts (CSV/MD) |
| `GET /v1/health` | Health check |

## Repository

- **GitHub**: https://github.com/dfgoodfellow2/kinetiStasis
- **Local**: `/home/dfgoodfellow2/Projects/Personal/Diet_Tracker`

## License

MIT
