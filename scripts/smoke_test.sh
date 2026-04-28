#!/usr/bin/env bash
set -euo pipefail

BASE="${DIET_API_URL:-http://localhost:8080}/v1"
COOKIE_JAR=$(mktemp /tmp/diet-smoke-XXXXX.txt)
trap 'rm -f "$COOKIE_JAR"' EXIT

PASS=0
FAIL=0
TODAY=$(date +%Y-%m-%d)

green() { printf '\033[0;32m✓ %s\033[0m\n' "$1"; }
red()   { printf '\033[0;31m✗ %s\033[0m\n' "$1"; }

check() {
  local name="$1" expected="$2"
  shift 2
  local status
  status=$(curl -s -o /dev/null -w '%{http_code}' --cookie-jar "$COOKIE_JAR" --cookie "$COOKIE_JAR" "$@")
  if [[ "$status" == "$expected" ]]; then
    green "$name ($status)"
    ((PASS++))
  else
    red "$name (got $status, want $expected)"
    ((FAIL++))
  fi
}

echo "=== Diet Tracker v2 Smoke Test ==="
echo "Base URL: $BASE"
echo ""

# Health
check "GET /health" 200 "$BASE/health"

# Register (may return 409 if already exists — accept both)
REG_STATUS=$(curl -s -o /dev/null -w '%{http_code}' \
  --cookie-jar "$COOKIE_JAR" --cookie "$COOKIE_JAR" \
  -X POST "$BASE/auth/register" \
  -H 'Content-Type: application/json' \
  -d '{"username":"smoketest","email":"smoke@test.com","password":"Smoke1234!"}')
if [[ "$REG_STATUS" == "201" || "$REG_STATUS" == "409" ]]; then
  green "POST /auth/register ($REG_STATUS)"
  ((PASS++))
else
  red "POST /auth/register (got $REG_STATUS, want 201 or 409)"
  ((FAIL++))
fi

# Login
check "POST /auth/login" 200 \
  -X POST "$BASE/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"smoketest","password":"Smoke1234!"}'

check "GET /auth/me" 200 "$BASE/auth/me"
check "POST /auth/refresh" 200 -X POST "$BASE/auth/refresh"

# Profile
check "GET /profile" 200 "$BASE/profile"
check "PUT /profile" 200 \
  -X PUT "$BASE/profile" \
  -H 'Content-Type: application/json' \
  -d '{"height_in":70,"age":30,"sex":"male","activity_level":"moderately_active","goal_type":"maintain","units":"imperial"}'

# Nutrition
check "POST /nutrition/logs" 201 \
  -X POST "$BASE/nutrition/logs" \
  -H 'Content-Type: application/json' \
  -d "{\"date\":\"$TODAY\",\"calories\":2000,\"protein_g\":150,\"carbs_g\":200,\"fat_g\":70,\"fiber_g\":30,\"water_ml\":2500,\"notes\":\"smoke test\"}"

check "GET /nutrition/logs" 200 "$BASE/nutrition/logs"
check "GET /nutrition/logs/{date}" 200 "$BASE/nutrition/logs/$TODAY"
check "PUT /nutrition/logs/{date}" 200 \
  -X PUT "$BASE/nutrition/logs/$TODAY" \
  -H 'Content-Type: application/json' \
  -d "{\"date\":\"$TODAY\",\"calories\":2100,\"protein_g\":155,\"carbs_g\":210,\"fat_g\":72,\"fiber_g\":32,\"water_ml\":2600,\"notes\":\"updated\"}"

# Biometrics
check "POST /biometrics" 201 \
  -X POST "$BASE/biometrics" \
  -H 'Content-Type: application/json' \
  -d "{\"date\":\"$TODAY\",\"weight_lbs\":180,\"waist_cm\":85,\"grip_strength_kg\":50,\"bolt_score\":25,\"sleep_hours\":7.5,\"sleep_quality\":7,\"subjective_feel\":7,\"notes\":\"smoke\"}"

check "GET /biometrics" 200 "$BASE/biometrics"
check "GET /biometrics/{date}" 200 "$BASE/biometrics/$TODAY"
check "PUT /biometrics/{date}" 200 \
  -X PUT "$BASE/biometrics/$TODAY" \
  -H 'Content-Type: application/json' \
  -d "{\"date\":\"$TODAY\",\"weight_lbs\":179.5,\"waist_cm\":84.5,\"grip_strength_kg\":51,\"bolt_score\":26,\"sleep_hours\":8,\"sleep_quality\":8,\"subjective_feel\":8,\"notes\":\"updated\"}"

# Workouts
check "POST /workouts" 201 \
  -X POST "$BASE/workouts" \
  -H 'Content-Type: application/json' \
  -d "{\"date\":\"$TODAY\",\"slot\":1,\"title\":\"Smoke Test Workout\",\"duration_min\":45,\"raw_notes\":\"test\"}"

check "GET /workouts" 200 "$BASE/workouts"
check "GET /workouts/{date}/{slot}" 200 "$BASE/workouts/$TODAY/1"
check "PUT /workouts/{date}/{slot}" 200 \
  -X PUT "$BASE/workouts/$TODAY/1" \
  -H 'Content-Type: application/json' \
  -d "{\"date\":\"$TODAY\",\"slot\":1,\"title\":\"Updated Workout\",\"duration_min\":50,\"raw_notes\":\"updated\"}"

# Targets
check "GET /targets" 200 "$BASE/targets"
check "PUT /targets" 200 \
  -X PUT "$BASE/targets" \
  -H 'Content-Type: application/json' \
  -d '{"calories_kcal":2200,"protein_g":160,"carbs_g":220,"fat_g":75}'
check "GET /targets/history" 200 "$BASE/targets/history"

# Saved meals
check "POST /meals/saved" 201 \
  -X POST "$BASE/meals/saved" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Smoke Meal","calories":500,"protein_g":40,"carbs_g":50,"fat_g":15}'
check "GET /meals/saved" 200 "$BASE/meals/saved"
check "GET /meals/templates" 200 "$BASE/meals/templates"

# Measurements
check "POST /measurements" 201 \
  -X POST "$BASE/measurements" \
  -H 'Content-Type: application/json' \
  -d "{\"date\":\"$TODAY\",\"neck_cm\":38,\"chest_cm\":100,\"waist_cm\":85,\"hips_cm\":95,\"thigh_cm\":55,\"bicep_cm\":35,\"notes\":\"smoke\"}"
check "GET /measurements" 200 "$BASE/measurements"

# Calculations
check "GET /calc/tdee" 200 "$BASE/calc/tdee?days=30"
check "GET /calc/macros" 200 "$BASE/calc/macros"
check "GET /calc/readiness" 200 "$BASE/calc/readiness"
check "GET /calc/bodyfat" 200 "$BASE/calc/bodyfat?method=navy"
check "GET /dashboard" 200 "$BASE/dashboard"

# Export
check "GET /export/nutrition" 200 "$BASE/export/nutrition?from=2026-01-01&to=$TODAY&format=md" -H 'Accept: application/json'
check "GET /export/workouts" 200 "$BASE/export/workouts?from=2026-01-01&to=$TODAY&format=md" -H 'Accept: application/json'
check "GET /export/combined" 200 "$BASE/export/combined?from=2026-01-01&to=$TODAY" -H 'Accept: application/json'

# Cleanup — delete test workout and nutrition log
check "DELETE /workouts/{date}/{slot}" 204 \
  -X DELETE "$BASE/workouts/$TODAY/1"
check "DELETE /nutrition/logs/{date}" 204 \
  -X DELETE "$BASE/nutrition/logs/$TODAY"

# Logout
check "POST /auth/logout" 200 -X POST "$BASE/auth/logout"

echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="
if [[ $FAIL -gt 0 ]]; then
  exit 1
fi
