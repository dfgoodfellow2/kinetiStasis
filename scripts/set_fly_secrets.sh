#!/usr/bin/env bash
# Run this once to set all required secrets on fly.io.
# Replace the placeholder values with real secrets.
#
# Usage: bash scripts/set_fly_secrets.sh

fly secrets set \
  JWT_SECRET="$(openssl rand -hex 32)" \
  GEMINI_API_KEY="your-gemini-api-key" \
  LITESTREAM_BUCKET="your-tigris-bucket" \
  LITESTREAM_REGION="auto" \
  LITESTREAM_ACCESS_KEY_ID="your-access-key-id" \
  LITESTREAM_SECRET_ACCESS_KEY="your-secret-access-key" \
  LITESTREAM_ENDPOINT="https://fly.storage.tigris.dev"

echo "Secrets set. Verify with: fly secrets list"
