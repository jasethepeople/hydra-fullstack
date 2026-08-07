#!/bin/bash
set -euo pipefail

# HYDRA Fullstack Deployment Script
# Usage: DISCORD_WEBHOOK="..." bash scripts/deploy.sh

APP_NAME="hydra-fullstack"
FLY_REGION="iad"
HEALTH_CHECK_TIMEOUT=30

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[PASS]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[FAIL]${NC} $1"; }
log_step() { echo -e "${CYAN}[STEP $1/7]${NC} $2"; }

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "  HYDRA FULLSTACK v1.0 - Production Deployment"
echo "═══════════════════════════════════════════════════════════════"
echo ""

# ─── STEP 1: Validate Environment ───
log_step 1 "Validating environment..."

command -v flyctl >/dev/null 2>&1 || { log_error "flyctl not installed. Install: https://fly.io/docs/hands-on/install-flyctl/"; exit 1; }
command -v curl >/dev/null 2>&1 || { log_error "curl not installed"; exit 1; }

if ! flyctl auth whoami >/dev/null 2>&1; then
    log_error "Not logged into Fly.io. Run: flyctl auth login"
    exit 1
fi

log_success "Environment validated"

# ─── STEP 2: Push Secrets ───
log_step 2 "Configuring secrets..."

if [ -n "${DISCORD_WEBHOOK:-}" ]; then
    log_info "Setting Discord webhook..."
    flyctl secrets set DISCORD_WEBHOOK="$DISCORD_WEBHOOK" --app "$APP_NAME" >/dev/null 2>&1
    log_success "Discord webhook configured"
else
    log_warn "DISCORD_WEBHOOK not set — alerts will be in-app only"
fi

if [ -n "${REDIS_ADDR:-}" ]; then
    flyctl secrets set REDIS_ADDR="$REDIS_ADDR" --app "$APP_NAME" >/dev/null 2>&1
    log_success "Redis configured"
fi

if [ -n "${NATS_URL:-}" ]; then
    flyctl secrets set NATS_URL="$NATS_URL" NATS_ENABLED="true" --app "$APP_NAME" >/dev/null 2>&1
    log_success "NATS configured"
fi

# ─── STEP 3: Build & Deploy ───
log_step 3 "Building and deploying container..."
log_info "This takes 2-3 minutes..."

flyctl deploy --app "$APP_NAME" --region "$FLY_REGION" --wait-timeout=300

log_success "Container deployed"

# ─── STEP 4: Health Check ───
log_step 4 "Running health checks..."

APP_URL="https://${APP_NAME}.fly.dev"
HEALTHY=false
RETRIES=0
MAX_RETRIES=10

while [ $RETRIES -lt $MAX_RETRIES ]; do
    if curl -sf "${APP_URL}/health" >/dev/null 2>&1; then
        HEALTHY=true
        break
    fi
    RETRIES=$((RETRIES + 1))
    log_info "Health check $RETRIES/$MAX_RETRIES..."
    sleep 3
done

if [ "$HEALTHY" = true ]; then
    log_success "All health checks passed"
else
    log_error "Health check failed"
    exit 1
fi

# ─── STEP 5: Verify Endpoints ───
log_step 5 "Verifying endpoints..."

for endpoint in /health /alerts/status /memorial/season /api/metrics; do
    if curl -sf "${APP_URL}${endpoint}" >/dev/null 2>&1; then
        log_success "$endpoint"
    else
        log_warn "$endpoint not responding"
    fi
done

# ─── STEP 6: Stress Test ───
log_step 6 "Running stress test..."

STRESS_RESPONSE=$(curl -sf -X POST "${APP_URL}/api/render/stress" \
    -H "Content-Type: application/json" \
    -d '{"width":3840,"height":2160,"iterations":500}' 2>/dev/null || echo "{}")

if echo "$STRESS_RESPONSE" | grep -q "completed"; then
    log_success "Stress test passed"
else
    log_warn "Stress test inconclusive"
fi

# ─── STEP 7: Verify Alerts ───
log_step 7 "Verifying alert system..."

ALERT_STATUS=$(curl -sf "${APP_URL}/alerts/status" 2>/dev/null || echo "{}")

if echo "$ALERT_STATUS" | grep -q "discord_configured"; then
    DISCORD_CFG=$(echo "$ALERT_STATUS" | grep -o '"discord_configured":true' || echo "")
    if [ -n "$DISCORD_CFG" ]; then
        log_success "Discord alerts configured"
    else
        log_warn "Discord not configured"
    fi
fi

# ─── SUMMARY ───
echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "  DEPLOYMENT COMPLETE"
echo "═══════════════════════════════════════════════════════════════"
echo ""
echo "  Application URL:  $APP_URL"
echo "  Memorial Site:    $APP_URL/"
echo "  Alert Dashboard:  $APP_URL/dashboard/view"
echo "  Alert Status:     $APP_URL/alerts/status"
echo "  Metrics:          $APP_URL/metrics"
echo "  Health Check:     $APP_URL/health"
echo ""
echo "  Verification Commands:"
echo "    curl $APP_URL/health"
echo "    curl $APP_URL/alerts/status | jq"
echo "    curl $APP_URL/alerts/history?limit=5 | jq"
echo ""
echo "  Discord notifications should arrive within 2 minutes"
echo "  if DISCORD_WEBHOOK was configured."
echo ""
