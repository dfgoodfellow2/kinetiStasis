```markdown
# Diet Tracker v2

API-first diet and fitness tracking server. Designed for deployment on fly.io with SQLite + Litestream backups.

## Stack
- Go 1.22+ — API server
- SQLite (modernc, pure Go) — database
- Litestream — continuous backup to S3/Tigris
- Chi — HTTP router
- Bubble Tea — TUI client
- Svelte 5 — PWA client

## Quick Start (Development)

```bash
cp .env.example .env
# edit .env with your values
make dev
```

## Deploy

```bash
fly launch
fly secrets set JWT_SECRET=... GEMINI_API_KEY=...
fly deploy
```
```
