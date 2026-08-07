#!/bin/bash
# HYDRA Fullstack Verification Script
# Usage: bash scripts/verify.sh

APP_URL="${APP_URL:-https://hydra-fullstack.fly.dev}"

echo "═══════════════════════════════════════════════════════════════"
echo "  HYDRA Fullstack - System Verification"
echo "═══════════════════════════════════════════════════════════════"
echo ""
echo "Target: $APP_URL"
echo ""

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

check_endpoint() {
    local endpoint=$1
    local name=$2
    local method=${3:-GET}

    echo -n "  $name ... "

    if [ "$method" = "POST" ]; then
        response=$(curl -sf -X POST "${APP_URL}${endpoint}" -H "Content-Type: application/json" -d '{}' 2>/dev/null)
    else
        response=$(curl -sf "${APP_URL}${endpoint}" 2>/dev/null)
    fi

    if [ $? -eq 0 ] && [ -n "$response" ]; then
        echo -e "${GREEN}✓ PASS${NC}"
        return 0
    else
        echo -e "${RED}✗ FAIL${NC}"
        return 1
    fi
}

echo "HTTP Endpoints:"
check_endpoint "/health" "Health Check"
check_endpoint "/alerts/status" "Alert Status"
check_endpoint "/alerts/history?limit=5" "Alert History"
check_endpoint "/alerts/active" "Active Alerts"
check_endpoint "/api/metrics" "Prometheus Metrics"
check_endpoint "/memorial/season" "Memorial Season"
check_endpoint "/memorial/canvas/pixels" "Canvas Pixels"
check_endpoint "/dashboard/view" "Dashboard HTML"

echo ""
echo "gRPC Endpoints (requires grpcurl):"
if command -v grpcurl >/dev/null 2>&1; then
    echo -n "  gRPC HealthCheck ... "
    if grpcurl -plaintext localhost:50051 hydra.RenderService/HealthCheck >/dev/null 2>&1; then
        echo -e "${GREEN}✓ PASS${NC}"
    else
        echo -e "${YELLOW}⚠ SKIP${NC} (gRPC server not reachable)"
    fi
else
    echo -e "  ${YELLOW}⚠ grpcurl not installed${NC}"
fi

echo ""
echo "Manual Test Commands:"
echo ""
echo "  # Fire a test alert"
echo "  curl -X POST ${APP_URL}/api/v1/alerts/fire \"
echo "    -H 'Content-Type: application/json' \"
echo "    -d '{"alert_type":"Test","severity":"WARNING","message":"Hello"}'"
echo ""
echo "  # Submit a render job"
echo "  curl -X POST ${APP_URL}/api/render/stress \"
echo "    -H 'Content-Type: application/json' \"
echo "    -d '{"width":3840,"height":2160,"iterations":1000}'"
echo ""
echo "  # Acknowledge an alert"
echo "  curl -X POST ${APP_URL}/alerts/acknowledge \"
echo "    -H 'Content-Type: application/json' \"
echo "    -d '{"alert_id":"alt-xxx","acked_by":"admin"}'"
echo ""
