#!/bin/bash
set -euo pipefail

# nOS App API integration test
# Tests the M3 App Catalog API endpoints

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

API_BASE="http://localhost:9000/api/v1"
AUTH_TOKEN=""

log() {
    echo -e "${GREEN}[TEST]${NC} $*"
}

error() {
    echo -e "${RED}[ERROR]${NC} $*" >&2
    exit 1
}

warn() {
    echo -e "${YELLOW}[WARNING]${NC} $*"
}

# Login first to get auth token
login() {
    log "Logging in..."
    response=$(curl -s -c cookies.txt -X POST "${API_BASE%/v1}/auth/login" \
        -H "Content-Type: application/json" \
        -d '{"username":"admin@example.com","password":"admin123"}')
    
    if echo "$response" | grep -q "ok"; then
        log "Login successful"
    else
        error "Login failed: $response"
    fi
}

# Test catalog endpoint
test_catalog() {
    log "Testing GET /api/v1/apps/catalog..."
    
    response=$(curl -s -b cookies.txt "${API_BASE}/apps/catalog")
    
    if echo "$response" | jq -e '.entries' >/dev/null 2>&1; then
        count=$(echo "$response" | jq '.entries | length')
        log "✓ Catalog returned $count apps"
        
        # Check for our sample apps
        if echo "$response" | jq -e '.entries[] | select(.id == "whoami")' >/dev/null 2>&1; then
            log "✓ Found whoami app in catalog"
        else
            warn "whoami app not found in catalog"
        fi
        
        if echo "$response" | jq -e '.entries[] | select(.id == "nextcloud")' >/dev/null 2>&1; then
            log "✓ Found nextcloud app in catalog"
        else
            warn "nextcloud app not found in catalog"
        fi
    else
        error "Invalid catalog response: $response"
    fi
}

# Test installed apps
test_installed() {
    log "Testing GET /api/v1/apps/installed..."
    
    response=$(curl -s -b cookies.txt "${API_BASE}/apps/installed")
    
    if echo "$response" | jq -e '.items' >/dev/null 2>&1; then
        count=$(echo "$response" | jq '.items | length')
        log "✓ Found $count installed apps"
    else
        error "Invalid installed apps response: $response"
    fi
}

# Test install app
test_install() {
    log "Testing POST /api/v1/apps/install (whoami)..."
    
    response=$(curl -s -b cookies.txt -X POST "${API_BASE}/apps/install" \
        -H "Content-Type: application/json" \
        -H "X-CSRF-Token: test" \
        -d '{
            "id": "whoami",
            "params": {
                "WHOAMI_PORT": "8090",
                "WHOAMI_NAME": "Test App"
            }
        }')
    
    if echo "$response" | jq -e '.message' | grep -q "installed successfully"; then
        log "✓ App installed successfully"
    elif echo "$response" | jq -e '.error' | grep -q "already installed"; then
        log "✓ App already installed (expected)"
    else
        warn "Install response: $response"
    fi
}

# Test get app details
test_get_app() {
    log "Testing GET /api/v1/apps/{id}..."
    
    response=$(curl -s -b cookies.txt "${API_BASE}/apps/whoami")
    
    if echo "$response" | jq -e '.id' >/dev/null 2>&1; then
        status=$(echo "$response" | jq -r '.status')
        health=$(echo "$response" | jq -r '.health.status')
        log "✓ App details retrieved - Status: $status, Health: $health"
    else
        warn "App not found or invalid response: $response"
    fi
}

# Test app operations
test_operations() {
    # Only test if app is installed
    if curl -s -b cookies.txt "${API_BASE}/apps/whoami" | jq -e '.id' >/dev/null 2>&1; then
        log "Testing app operations..."
        
        # Test stop
        log "Testing POST /api/v1/apps/{id}/stop..."
        response=$(curl -s -b cookies.txt -X POST "${API_BASE}/apps/whoami/stop" \
            -H "X-CSRF-Token: test")
        if echo "$response" | jq -e '.message' | grep -q "stopped"; then
            log "✓ App stopped"
        else
            warn "Stop response: $response"
        fi
        
        sleep 2
        
        # Test start
        log "Testing POST /api/v1/apps/{id}/start..."
        response=$(curl -s -b cookies.txt -X POST "${API_BASE}/apps/whoami/start" \
            -H "X-CSRF-Token: test")
        if echo "$response" | jq -e '.message' | grep -q "started"; then
            log "✓ App started"
        else
            warn "Start response: $response"
        fi
        
        # Test health check
        log "Testing POST /api/v1/apps/{id}/health..."
        response=$(curl -s -b cookies.txt -X POST "${API_BASE}/apps/whoami/health" \
            -H "X-CSRF-Token: test")
        if echo "$response" | jq -e '.health' >/dev/null 2>&1; then
            log "✓ Health check completed"
        else
            warn "Health check response: $response"
        fi
        
        # Test events
        log "Testing GET /api/v1/apps/{id}/events..."
        response=$(curl -s -b cookies.txt "${API_BASE}/apps/whoami/events?limit=10")
        if echo "$response" | jq -e '.events' >/dev/null 2>&1; then
            count=$(echo "$response" | jq '.events | length')
            log "✓ Retrieved $count events"
        else
            warn "Events response: $response"
        fi
    else
        log "Skipping operations test - app not installed"
    fi
}

# Main test execution
main() {
    log "Starting nOS App API tests..."
    
    # Check if nosd is running
    if ! curl -sf "http://localhost:9000/api/health" >/dev/null 2>&1; then
        error "nosd is not running on port 9000"
    fi
    
    # Check for required tools
    if ! command -v jq >/dev/null 2>&1; then
        error "jq is required for JSON parsing"
    fi
    
    # Run tests
    login
    test_catalog
    test_installed
    test_install
    test_get_app
    test_operations
    
    # Cleanup
    rm -f cookies.txt
    
    echo
    log "${GREEN}All tests completed!${NC}"
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --api-base)
            API_BASE="$2"
            shift 2
            ;;
        --help)
            echo "Usage: $0 [--api-base URL]"
            echo "  --api-base    Base URL for API (default: http://localhost:9000/api/v1)"
            exit 0
            ;;
        *)
            error "Unknown option: $1"
            ;;
    esac
done

main
