#!/usr/bin/env bash
# Restore the Litestream replica and prove it contains real data.
#
# This is an operator-run disaster-recovery drill: it needs real Tigris/S3
# credentials in .env and network access to the bucket. It never touches the
# production database — the replica is restored to a scratch directory.
#
# Usage: make restore-drill   (or: bash scripts/restore_drill.sh)
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="$ROOT_DIR/.env"

OUT_DIR="${RESTORE_DIR:-/tmp/opencode/restore_drill}"
OUT="$OUT_DIR/diet.db"

RED=$'\033[0;31m'
GREEN=$'\033[0;32m'
YELLOW=$'\033[0;33m'
RESET=$'\033[0m'

info() { printf '%s\n' "$*"; }
warn() { printf '%sWARN:%s %s\n' "$YELLOW" "$RESET" "$*"; }
fail() {
	printf '%sFAIL:%s %s\n' "$RED" "$RESET" "$*" >&2
	exit 1
}
pass() { printf '%sPASS:%s %s\n' "$GREEN" "$RESET" "$*"; }

# --- Environment -----------------------------------------------------------
# Read only the keys this drill needs. Sourcing .env wholesale would execute
# arbitrary values (a JWT secret containing spaces is not valid shell), and
# the Makefile already exports .env for `make restore-drill` anyway.
env_get() {
	local key="$1" line value
	[[ -f "$ENV_FILE" ]] || return 0
	line="$(grep -E "^[[:space:]]*${key}=" "$ENV_FILE" | tail -n 1 || true)"
	[[ -n "$line" ]] || return 0
	value="${line#*=}"
	value="${value%$'\r'}"
	if [[ ${#value} -ge 2 ]]; then
		case "$value" in
		\"*\") value="${value:1:${#value}-2}" ;;
		\'*\') value="${value:1:${#value}-2}" ;;
		esac
	fi
	printf '%s' "$value"
}

if [[ ! -f "$ENV_FILE" ]]; then
	warn "no .env found at $ENV_FILE"
fi

missing=""
for var in BUCKET_NAME AWS_REGION AWS_ENDPOINT_URL_S3 AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY; do
	if [[ -z "${!var:-}" ]]; then
		value="$(env_get "$var")"
		if [[ -n "$value" ]]; then
			export "$var=$value"
		fi
	fi
	if [[ -z "${!var:-}" ]]; then
		missing="${missing:+$missing }$var"
	fi
done
if [[ -n "$missing" ]]; then
	fail "missing credentials: $missing
Create .env from .env.example and fill in the Litestream/Tigris values:
  BUCKET_NAME, AWS_REGION, AWS_ENDPOINT_URL_S3, AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY
See DEPLOY.md (First-Time Setup -> Tigris bucket)."
fi

# --- Tooling ---------------------------------------------------------------
if ! command -v litestream >/dev/null 2>&1; then
	fail "litestream is not on PATH.
Install it with one of:
  macOS:  brew install litestream
  Linux:  download a release from https://github.com/benbjohnson/litestream/releases
          (or: go install github.com/benbjohnson/litestream/cmd/litestream@v0.3.13)
Then re-run: make restore-drill"
fi

info "== Restore drill =="
info "Bucket:   $BUCKET_NAME"
info "Endpoint: $AWS_ENDPOINT_URL_S3"
info "Source:   s3://$BUCKET_NAME/diet.db"
info "Target:   $OUT"
info ""

mkdir -p "$OUT_DIR"
# Always restore into a clean file so a previous drill cannot mask a failed run.
rm -f "$OUT" "$OUT-wal" "$OUT-shm"

# Litestream v0.3.13 recognizes -force-path-style for S3 endpoints such as
# Tigris; older/newer builds may not, so only pass it when advertised.
LITESTREAM_HELP="$(litestream restore -h 2>&1 || true)"

# Inline flags via positional parameters keeps this portable to bash 3.2,
# where an empty array under `set -u` is an unbound-variable error.
set -- restore \
	-o "$OUT" \
	-if-replica-exists \
	-access-key-id "$AWS_ACCESS_KEY_ID" \
	-secret-access-key "$AWS_SECRET_ACCESS_KEY" \
	-endpoint "$AWS_ENDPOINT_URL_S3" \
	-region "$AWS_REGION"
if grep -q -- '-force-path-style' <<<"$LITESTREAM_HELP"; then
	set -- "$@" -force-path-style
fi
set -- "$@" "s3://$BUCKET_NAME/diet.db"

info "Restoring replica..."
if ! litestream "$@"; then
	fail "litestream restore failed — check the bucket, endpoint, and credentials in .env"
fi

if [[ ! -s "$OUT" ]]; then
	fail "restore produced no database (no replica at s3://$BUCKET_NAME/diet.db yet)"
fi

# --- Verify ----------------------------------------------------------------
info ""
info "Checking the restored database (read-only, no migrations)..."
set +e
CHECK_OUTPUT="$(cd "$ROOT_DIR" && go run ./cmd/dbcheck "$OUT" 2>&1)"
CHECK_STATUS=$?
set -e
printf '%s\n' "$CHECK_OUTPUT"

NUTRITION_ROWS="$(awk '$1 == "nutrition_logs" { print $2 }' <<<"$CHECK_OUTPUT" | tail -n 1)"

info ""
info "------------------------------------------------------------"
if [[ $CHECK_STATUS -ne 0 ]]; then
	fail "restored database is unusable — schema check failed (see output above)"
fi
if [[ -z "$NUTRITION_ROWS" || "$NUTRITION_ROWS" -le 0 ]]; then
	fail "restored database has the schema but no nutrition_logs rows — replica is empty or stale"
fi

pass "replica restores cleanly with real data: all 12 expected tables present, $NUTRITION_ROWS nutrition_logs rows."
info "Restored file kept for inspection: $OUT"
info "Clean up with: rm -rf \"$OUT_DIR\""
