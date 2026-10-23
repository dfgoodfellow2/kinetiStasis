#!/usr/bin/env bash
# Run this once to set all required secrets on fly.io.
# Replace the placeholder values with real secrets.
#
# Usage: bash scripts/set_fly_secrets.sh

# fly storage create sets these automatically — only run this script when setting manually or rotating keys.
# Best-effort cleanup of the old, dead LITESTREAM_* names that nothing reads.
fly secrets unset LITESTREAM_BUCKET LITESTREAM_REGION LITESTREAM_ACCESS_KEY_ID LITESTREAM_SECRET_ACCESS_KEY LITESTREAM_ENDPOINT 2>/dev/null || true

# S3 endpoint: both https://fly.storage.tigris.dev and https://t3.storage.dev work;
# t3.storage.dev is the current global endpoint (production's live value).
fly secrets set \
  JWT_SECRET="$(openssl rand -hex 32)" \
  GEMINI_API_KEY="your-gemini-api-key" \
  BUCKET_NAME="your-tigris-bucket" \
  AWS_REGION="auto" \
  AWS_ENDPOINT_URL_S3="https://t3.storage.dev" \
  AWS_ACCESS_KEY_ID="your-access-key-id" \
  AWS_SECRET_ACCESS_KEY="your-secret-access-key"

echo "Secrets set. Verify with: fly secrets list"
