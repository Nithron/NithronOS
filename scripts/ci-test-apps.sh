#!/bin/bash
# CI smoke tests for App Catalog functionality

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test configuration
BASE_URL="${BASE_URL:-http://localhost:8090}"
AUTH_TOKEN="${AUTH_TOKEN:-test-token}"
TEST_APP="whoami"
TEST_APP_PORT="8080"

# Helper functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

check_dependency() {
    if ! command -v $1 &> /dev/null; then
        log_error "$1 is not installed"
        exit 1
    fi
}

wait_for_service() {
    local url=$1
    local max_attempts=30
    local attempt=0
    
    log_info "Waiting for service at $url..."
    while [ $attempt -lt $max_attempts ]; do
        if curl -f -s "$url/api/v1/health" > /dev/null 2>&1; then
            log_info "Service is ready!"
            return 0
        fi
        attempt=$((attempt + 1))
        sleep 2
    done
    
    log_error "Service did not become ready in time"
    return 1
}

api_request() {
    local method=$1
    local endpoint=$2
    local data="${3:-}"
    
    local args=(
        -X "$method"
        -H "Authorization: Bearer $AUTH_TOKEN"
        -H "Content-Type: application/json"
        -s
        -w "\n%{http_code}"
    )
    
    if [ -n "$data" ]; then
        args+=(-d "$data")
    fi
    
    local response=$(curl "${args[@]}" "$BASE_URL$endpoint")
    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n-1)
    
    echo "$body"
    return $([ "$http_code" -ge 200 ] && [ "$http_code" -lt 300 ])
}

# Pre-flight checks
log_info "Starting App Catalog CI smoke tests"

check_dependency curl
check_dependency jq
check_dependency docker

# Start services if not running (for local testing)
if [ "${CI:-false}" != "true" ]; then
    log_info "Starting local services..."
    
    # Start nosd
    if ! pgrep -x "nosd" > /dev/null; then
        ./backend/nosd/nosd &
        NOSD_PID=$!
        trap "kill $NOSD_PID 2>/dev/null || true" EXIT
    fi
    
    wait_for_service "$BASE_URL"
fi

# Test 1: Get catalog
log_info "Test 1: Fetching app catalog"
CATALOG=$(api_request GET "/api/v1/apps/catalog")
if [ $? -ne 0 ]; then
    log_error "Failed to fetch catalog"
    exit 1
fi

# Verify whoami app exists in catalog
if ! echo "$CATALOG" | jq -e ".entries[] | select(.id == \"$TEST_APP\")" > /dev/null; then
    log_error "Test app '$TEST_APP' not found in catalog"
    exit 1
fi
log_info "✓ Catalog contains test app '$TEST_APP'"

# Test 2: Install whoami app
log_info "Test 2: Installing $TEST_APP app"
INSTALL_DATA=$(cat <<EOF
{
    "id": "$TEST_APP",
    "params": {
        "WHOAMI_PORT": "$TEST_APP_PORT",
        "WHOAMI_NAME": "CI Test Instance"
    }
}
EOF
)

INSTALL_RESPONSE=$(api_request POST "/api/v1/apps/install" "$INSTALL_DATA")
if [ $? -ne 0 ]; then
    log_error "Failed to install app"
    echo "$INSTALL_RESPONSE"
    exit 1
fi
log_info "✓ App installation initiated"

# Wait for app to become healthy
log_info "Waiting for app to become healthy..."
HEALTH_CHECK_ATTEMPTS=0
MAX_HEALTH_ATTEMPTS=30

while [ $HEALTH_CHECK_ATTEMPTS -lt $MAX_HEALTH_ATTEMPTS ]; do
    APP_STATUS=$(api_request GET "/api/v1/apps/$TEST_APP")
    if [ $? -eq 0 ]; then
        STATUS=$(echo "$APP_STATUS" | jq -r '.status')
        HEALTH=$(echo "$APP_STATUS" | jq -r '.health.status')
        
        if [ "$STATUS" = "running" ] && [ "$HEALTH" = "healthy" ]; then
            log_info "✓ App is running and healthy"
            break
        fi
    fi
    
    HEALTH_CHECK_ATTEMPTS=$((HEALTH_CHECK_ATTEMPTS + 1))
    sleep 2
done

if [ $HEALTH_CHECK_ATTEMPTS -eq $MAX_HEALTH_ATTEMPTS ]; then
    log_error "App did not become healthy in time"
    exit 1
fi

# Test 3: Access app through Caddy reverse proxy
log_info "Test 3: Testing app access through reverse proxy"
APP_RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/apps/$TEST_APP/")
if [ "$APP_RESPONSE" -eq 200 ]; then
    log_info "✓ App accessible through reverse proxy (HTTP $APP_RESPONSE)"
else
    log_error "App not accessible through reverse proxy (HTTP $APP_RESPONSE)"
    exit 1
fi

# Test 4: Get app logs
log_info "Test 4: Fetching app logs"
LOGS=$(api_request GET "/api/v1/apps/$TEST_APP/logs?limit=10")
if [ $? -ne 0 ]; then
    log_error "Failed to fetch app logs"
    exit 1
fi
log_info "✓ Successfully fetched app logs"

# Test 5: Stop app
log_info "Test 5: Stopping app"
STOP_RESPONSE=$(api_request POST "/api/v1/apps/$TEST_APP/stop" "")
if [ $? -ne 0 ]; then
    log_error "Failed to stop app"
    exit 1
fi

sleep 3

# Verify app is stopped
APP_STATUS=$(api_request GET "/api/v1/apps/$TEST_APP")
STATUS=$(echo "$APP_STATUS" | jq -r '.status')
if [ "$STATUS" = "stopped" ]; then
    log_info "✓ App successfully stopped"
else
    log_error "App status is not 'stopped': $STATUS"
    exit 1
fi

# Test 6: Start app
log_info "Test 6: Starting app"
START_RESPONSE=$(api_request POST "/api/v1/apps/$TEST_APP/start" "")
if [ $? -ne 0 ]; then
    log_error "Failed to start app"
    exit 1
fi

sleep 5

# Verify app is running again
APP_STATUS=$(api_request GET "/api/v1/apps/$TEST_APP")
STATUS=$(echo "$APP_STATUS" | jq -r '.status')
if [ "$STATUS" = "running" ]; then
    log_info "✓ App successfully started"
else
    log_error "App status is not 'running': $STATUS"
    exit 1
fi

# Test 7: Upgrade app (change a parameter)
log_info "Test 7: Upgrading app configuration"
UPGRADE_DATA=$(cat <<EOF
{
    "params": {
        "WHOAMI_PORT": "$TEST_APP_PORT",
        "WHOAMI_NAME": "CI Test Upgraded"
    }
}
EOF
)

UPGRADE_RESPONSE=$(api_request POST "/api/v1/apps/$TEST_APP/upgrade" "$UPGRADE_DATA")
if [ $? -ne 0 ]; then
    log_error "Failed to upgrade app"
    exit 1
fi
log_info "✓ App upgrade initiated"

sleep 5

# Test 8: Uninstall app
log_info "Test 8: Uninstalling app"
DELETE_DATA='{"keep_data": false}'
DELETE_RESPONSE=$(api_request DELETE "/api/v1/apps/$TEST_APP" "$DELETE_DATA")
if [ $? -ne 0 ]; then
    log_error "Failed to uninstall app"
    exit 1
fi
log_info "✓ App uninstalled"

# Verify app is gone
sleep 3
INSTALLED_APPS=$(api_request GET "/api/v1/apps/installed")
if echo "$INSTALLED_APPS" | jq -e ".items[] | select(.id == \"$TEST_APP\")" > /dev/null 2>&1; then
    log_error "App still appears in installed list after deletion"
    exit 1
fi
log_info "✓ App successfully removed from system"

# Test 9: Validate catalog templates
log_info "Test 9: Validating all catalog templates"
TEMPLATE_ERRORS=0

for APP_ID in $(echo "$CATALOG" | jq -r '.entries[].id'); do
    log_info "  Checking template for: $APP_ID"
    
    # Check compose file exists
    if [ ! -f "usr/share/nos/apps/templates/$APP_ID/compose.yaml" ]; then
        log_error "  Missing compose.yaml for $APP_ID"
        TEMPLATE_ERRORS=$((TEMPLATE_ERRORS + 1))
    fi
    
    # Check schema file exists
    if [ ! -f "usr/share/nos/apps/templates/$APP_ID/schema.json" ]; then
        log_error "  Missing schema.json for $APP_ID"
        TEMPLATE_ERRORS=$((TEMPLATE_ERRORS + 1))
    fi
    
    # Check README exists
    if [ ! -f "usr/share/nos/apps/templates/$APP_ID/README.md" ]; then
        log_warn "  Missing README.md for $APP_ID (optional)"
    fi
    
    # Validate schema.json is valid JSON
    if [ -f "usr/share/nos/apps/templates/$APP_ID/schema.json" ]; then
        if ! jq empty "usr/share/nos/apps/templates/$APP_ID/schema.json" 2>/dev/null; then
            log_error "  Invalid JSON in schema.json for $APP_ID"
            TEMPLATE_ERRORS=$((TEMPLATE_ERRORS + 1))
        fi
    fi
done

if [ $TEMPLATE_ERRORS -gt 0 ]; then
    log_error "Found $TEMPLATE_ERRORS template errors"
    exit 1
fi
log_info "✓ All templates validated successfully"

# Summary
log_info ""
log_info "========================================="
log_info "App Catalog CI Smoke Tests: ${GREEN}PASSED${NC}"
log_info "========================================="
log_info ""
log_info "Summary:"
log_info "  ✓ Catalog API functional"
log_info "  ✓ App installation working"
log_info "  ✓ App lifecycle management (start/stop/upgrade)"
log_info "  ✓ Reverse proxy integration"
log_info "  ✓ App uninstallation"
log_info "  ✓ All templates valid"

exit 0
