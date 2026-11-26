#!/bin/bash
set -euo pipefail

# nOS Apps Runtime smoke test
# Tests Docker installation, app deployment, and snapshot functionality

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

TEST_APP_ID="smoke-test-$$"
TEST_APP_DIR="/srv/apps/${TEST_APP_ID}"
CLEANUP_ON_EXIT=true

log() {
    echo -e "${GREEN}[SMOKE TEST]${NC} $*"
}

error() {
    echo -e "${RED}[ERROR]${NC} $*" >&2
    exit 1
}

warn() {
    echo -e "${YELLOW}[WARNING]${NC} $*"
}

success() {
    echo -e "${GREEN}✓${NC} $*"
}

fail() {
    echo -e "${RED}✗${NC} $*"
    return 1
}

# Cleanup function
cleanup() {
    if [[ "$CLEANUP_ON_EXIT" == "true" ]]; then
        log "Cleaning up test artifacts..."
        
        # Stop test app if running
        if systemctl is-active --quiet "nos-app@${TEST_APP_ID}.service" 2>/dev/null; then
            systemctl stop "nos-app@${TEST_APP_ID}.service" || true
        fi
        
        # Remove test app directory
        if [[ -d "$TEST_APP_DIR" ]]; then
            # Handle Btrfs subvolumes
            if command -v btrfs &>/dev/null; then
                for subvol in $(btrfs subvolume list "$TEST_APP_DIR" 2>/dev/null | awk '{print $NF}'); do
                    btrfs subvolume delete "$subvol" 2>/dev/null || true
                done
            fi
            rm -rf "$TEST_APP_DIR"
        fi
        
        # Clean up snapshots
        rm -rf "/srv/apps/.snapshots/${TEST_APP_ID}"
        
        # Remove any leftover containers
        docker rm -f "nos-app-${TEST_APP_ID}-hello-1" 2>/dev/null || true
    fi
}

trap cleanup EXIT

# Check prerequisites
check_prerequisites() {
    log "Checking prerequisites..."
    
    # Check if running as root
    if [[ $EUID -ne 0 ]]; then
        error "This script must be run as root"
    fi
    
    # Check if package is installed
    if ! dpkg -l nos-apps-runtime &>/dev/null 2>&1; then
        warn "nos-apps-runtime package not installed"
        warn "Install it first with: dpkg -i dist/deb/nos-apps-runtime_*.deb"
        # Don't fail, as we might be testing individual components
    fi
    
    success "Prerequisites check passed"
}

# Test Docker installation
test_docker_installation() {
    log "Testing Docker installation..."
    
    # Check if Docker is installed
    if ! command -v docker &>/dev/null; then
        fail "Docker not installed"
        return 1
    fi
    success "Docker binary found"
    
    # Check Docker service
    if ! systemctl is-active --quiet docker; then
        fail "Docker service not running"
        return 1
    fi
    success "Docker service is active"
    
    # Check Docker functionality
    if ! docker info &>/dev/null; then
        fail "Docker info failed"
        return 1
    fi
    success "Docker is operational"
    
    # Check docker-compose
    if ! docker compose version &>/dev/null; then
        fail "docker-compose plugin not found"
        return 1
    fi
    success "docker-compose plugin available"
    
    # Check service user is in docker group
    local service_user="nos"
    if id "nosd" &>/dev/null; then
        service_user="nosd"
    fi
    
    if ! groups "$service_user" | grep -q docker; then
        fail "$service_user not in docker group"
        return 1
    fi
    success "$service_user is in docker group"
    
    return 0
}

# Test directory structure
test_directory_structure() {
    log "Testing directory structure..."
    
    local dirs=(
        "/srv/apps"
        "/srv/apps/.snapshots"
        "/var/lib/nos/apps/state"
        "/etc/nos/apps"
        "/usr/share/nos/apps"
        "/usr/lib/nos/apps"
    )
    
    for dir in "${dirs[@]}"; do
        if [[ ! -d "$dir" ]]; then
            fail "Directory missing: $dir"
            return 1
        fi
        success "Directory exists: $dir"
    done
    
    # Check permissions
    if [[ ! -w "/srv/apps" ]]; then
        fail "/srv/apps not writable"
        return 1
    fi
    success "Directory permissions correct"
    
    return 0
}

# Test CLI helpers
test_cli_helpers() {
    log "Testing CLI helpers..."
    
    # Test docker_ok command
    if ! /usr/lib/nos/apps/nos-app-helper.sh docker-ok &>/dev/null; then
        fail "docker-ok command failed"
        return 1
    fi
    success "docker-ok command works"
    
    # Test list-apps command
    local apps_json=$(/usr/lib/nos/apps/nos-app-helper.sh list-apps)
    if ! echo "$apps_json" | jq -e '.apps' &>/dev/null; then
        fail "list-apps command failed"
        return 1
    fi
    success "list-apps command works"
    
    return 0
}

# Test app deployment
test_app_deployment() {
    log "Testing app deployment with hello-world..."
    
    # Create test app directory
    mkdir -p "${TEST_APP_DIR}/config"
    mkdir -p "${TEST_APP_DIR}/data"
    
    # Create docker-compose.yml
    cat > "${TEST_APP_DIR}/config/docker-compose.yml" <<EOF
version: '3.8'
services:
  hello:
    image: hello-world:latest
    container_name: nos-app-${TEST_APP_ID}-hello
    restart: "no"
    read_only: true
    security_opt:
      - no-new-privileges:true
    labels:
      - "nos.app.id=${TEST_APP_ID}"
      - "nos.app.name=Smoke Test"
EOF
    
    # Set ownership
    chown -R nos:nos "${TEST_APP_DIR}/config"
    
    # Pre-start checks
    if ! /usr/lib/nos/apps/nos-app-helper.sh pre-start "$TEST_APP_ID"; then
        fail "pre-start checks failed"
        return 1
    fi
    success "Pre-start checks passed"
    
    # Start app using systemd
    log "Starting app via systemd..."
    systemctl start "nos-app@${TEST_APP_ID}.service"
    
    # Wait a moment for container to run
    sleep 3
    
    # Check if service started
    if ! systemctl is-active --quiet "nos-app@${TEST_APP_ID}.service"; then
        fail "App service failed to start"
        systemctl status "nos-app@${TEST_APP_ID}.service" --no-pager || true
        return 1
    fi
    success "App service started"
    
    # Check app status
    local status_json=$(/usr/lib/nos/apps/nos-app-helper.sh app-status "$TEST_APP_ID")
    local app_status=$(echo "$status_json" | jq -r '.status')
    
    # hello-world exits immediately, so it might show as stopped - that's ok
    if [[ "$app_status" != "running" ]] && [[ "$app_status" != "stopped" ]]; then
        fail "Unexpected app status: $app_status"
        echo "$status_json" | jq .
        return 1
    fi
    success "App deployment successful (status: $app_status)"
    
    # Stop the app
    systemctl stop "nos-app@${TEST_APP_ID}.service"
    success "App stopped successfully"
    
    return 0
}

# Test snapshot functionality
test_snapshot_functionality() {
    log "Testing snapshot functionality..."
    
    # Create some test data
    echo "test data v1" > "${TEST_APP_DIR}/data/test.txt"
    
    # Check if Btrfs is available
    local snapshot_type="rsync"
    if /usr/lib/nos/apps/nos-app-snapshot.sh is-btrfs "${TEST_APP_DIR}/data" | grep -q "yes"; then
        snapshot_type="btrfs"
        
        # Ensure subvolume
        if ! /usr/lib/nos/apps/nos-app-snapshot.sh ensure-subvolume "$TEST_APP_ID"; then
            warn "Failed to create Btrfs subvolume, falling back to rsync"
        fi
    fi
    log "Using snapshot type: $snapshot_type"
    
    # Create a snapshot
    local snapshot_path=$(/usr/lib/nos/apps/nos-app-snapshot.sh snapshot-pre "$TEST_APP_ID" "test")
    if [[ -z "$snapshot_path" ]]; then
        fail "Failed to create snapshot"
        return 1
    fi
    success "Snapshot created: $snapshot_path"
    
    # Modify data
    echo "test data v2" > "${TEST_APP_DIR}/data/test.txt"
    
    # List snapshots
    local snapshots_json=$(/usr/lib/nos/apps/nos-app-snapshot.sh list-snapshots "$TEST_APP_ID")
    local snapshot_count=$(echo "$snapshots_json" | jq '.snapshots | length')
    if [[ "$snapshot_count" -lt 1 ]]; then
        fail "No snapshots found"
        return 1
    fi
    success "Found $snapshot_count snapshot(s)"
    
    # Get snapshot timestamp for rollback
    local timestamp=$(basename "$snapshot_path" | cut -d- -f1-2)
    
    # Rollback
    log "Testing rollback to snapshot..."
    if ! /usr/lib/nos/apps/nos-app-snapshot.sh rollback "$TEST_APP_ID" "$timestamp"; then
        fail "Rollback failed"
        return 1
    fi
    
    # Verify rollback
    if [[ "$(cat "${TEST_APP_DIR}/data/test.txt")" != "test data v1" ]]; then
        fail "Rollback verification failed"
        return 1
    fi
    success "Rollback successful - data restored"
    
    # Test snapshot pruning
    log "Testing snapshot pruning..."
    
    # Create multiple snapshots
    for i in {1..7}; do
        echo "test data v$i" > "${TEST_APP_DIR}/data/test.txt"
        /usr/lib/nos/apps/nos-app-snapshot.sh snapshot-pre "$TEST_APP_ID" "test-$i" >/dev/null
        sleep 1  # Ensure different timestamps
    done
    
    # Prune snapshots (keep 3)
    /usr/lib/nos/apps/nos-app-snapshot.sh prune-snapshots "$TEST_APP_ID" 3
    
    # Check remaining snapshots
    snapshots_json=$(/usr/lib/nos/apps/nos-app-snapshot.sh list-snapshots "$TEST_APP_ID")
    snapshot_count=$(echo "$snapshots_json" | jq '.snapshots | length')
    
    if [[ "$snapshot_count" -gt 3 ]]; then
        fail "Pruning failed - expected max 3 snapshots, found $snapshot_count"
        return 1
    fi
    success "Snapshot pruning successful - $snapshot_count snapshots remaining"
    
    return 0
}

# Test Docker daemon configuration
test_docker_config() {
    log "Testing Docker daemon configuration..."
    
    if [[ ! -f /etc/docker/daemon.json ]]; then
        fail "Docker daemon.json not found"
        return 1
    fi
    success "Docker daemon.json exists"
    
    # Check key configurations
    local log_driver=$(jq -r '.["log-driver"]' /etc/docker/daemon.json)
    if [[ "$log_driver" != "local" ]]; then
        fail "Log driver not set to 'local'"
        return 1
    fi
    success "Log driver configured correctly"
    
    local storage_driver=$(jq -r '.["storage-driver"]' /etc/docker/daemon.json)
    if [[ "$storage_driver" != "overlay2" ]]; then
        fail "Storage driver not set to 'overlay2'"
        return 1
    fi
    success "Storage driver configured correctly"
    
    return 0
}

# Main test execution
main() {
    log "Starting nOS Apps Runtime smoke test..."
    
    local failed=0
    
    # Run tests
    check_prerequisites || ((failed++))
    test_docker_installation || ((failed++))
    test_directory_structure || ((failed++))
    test_docker_config || ((failed++))
    test_cli_helpers || ((failed++))
    test_app_deployment || ((failed++))
    test_snapshot_functionality || ((failed++))
    
    # Summary
    echo
    if [[ $failed -eq 0 ]]; then
        echo -e "${GREEN}================================${NC}"
        echo -e "${GREEN}  All smoke tests passed! ✓${NC}"
        echo -e "${GREEN}================================${NC}"
        exit 0
    else
        echo -e "${RED}================================${NC}"
        echo -e "${RED}  $failed test(s) failed ✗${NC}"
        echo -e "${RED}================================${NC}"
        exit 1
    fi
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --no-cleanup)
            CLEANUP_ON_EXIT=false
            shift
            ;;
        --help)
            echo "Usage: $0 [--no-cleanup]"
            echo "  --no-cleanup    Don't clean up test artifacts on exit"
            exit 0
            ;;
        *)
            error "Unknown option: $1"
            ;;
    esac
done

main
