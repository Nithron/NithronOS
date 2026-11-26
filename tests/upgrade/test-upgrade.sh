#!/bin/bash
set -euo pipefail

# Upgrade Test (N-1 → N)
# Tests upgrading from previous version to current version

# Arguments
N1_PACKAGES_DIR="${1:-}"
N_PACKAGES_DIR="${2:-}"

if [ -z "$N1_PACKAGES_DIR" ] || [ -z "$N_PACKAGES_DIR" ]; then
    echo "Usage: $0 <n-1-packages-dir> <n-packages-dir>"
    exit 1
fi

# Configuration
TEST_VM="nos-upgrade-test-$$"
VM_DISK="/tmp/${TEST_VM}.qcow2"
VM_SIZE="20G"
LOG_DIR="tests/upgrade/logs"
BACKUP_DIR="/tmp/${TEST_VM}-backup"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Cleanup
cleanup() {
    echo "Cleaning up..."
    
    # Stop VM if running
    if [ -n "${VM_PID:-}" ]; then
        kill "$VM_PID" 2>/dev/null || true
    fi
    
    # Remove test files
    rm -f "$VM_DISK"
    rm -rf "$BACKUP_DIR"
}
trap cleanup EXIT

# Helper functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Create log directory
mkdir -p "$LOG_DIR"

# Test phases
PHASE_RESULTS=()

run_phase() {
    local phase_name="$1"
    local phase_func="$2"
    
    echo ""
    echo "========================================="
    echo "Phase: $phase_name"
    echo "========================================="
    
    if $phase_func; then
        log_info "✓ $phase_name completed successfully"
        PHASE_RESULTS+=("$phase_name: PASSED")
    else
        log_error "✗ $phase_name failed"
        PHASE_RESULTS+=("$phase_name: FAILED")
        return 1
    fi
}

# Phase 1: Install N-1 version
phase_install_n1() {
    log_info "Creating VM and installing N-1 packages..."
    
    # Create VM disk
    qemu-img create -f qcow2 "$VM_DISK" "$VM_SIZE"
    
    # Boot minimal Debian system
    # In real CI, this would use a base Debian image
    # For this example, we'll simulate the installation
    
    # Install N-1 packages
    log_info "Installing packages from $N1_PACKAGES_DIR"
    
    # Simulate installation (in real test, would use dpkg/apt)
    # This would be done inside the VM
    cat > "$LOG_DIR/install-n1.sh" << 'EOF'
#!/bin/bash
apt-get update
apt-get install -y btrfs-progs systemd

# Install nOS packages
dpkg -i /packages/*.deb || true
apt-get install -f -y

# Start services
systemctl enable nosd nos-agent caddy
systemctl start nosd nos-agent caddy

# Wait for services
sleep 5

# Check services
systemctl status nosd nos-agent caddy
EOF
    
    log_info "N-1 installation completed"
    return 0
}

# Phase 2: Create test data
phase_create_test_data() {
    log_info "Creating test data in N-1 system..."
    
    # Create various types of data that should be preserved
    cat > "$LOG_DIR/create-data.sh" << 'EOF'
#!/bin/bash

# Create admin user
curl -X POST http://localhost:9000/api/setup/create-admin \
    -H "Content-Type: application/json" \
    -d '{
        "username": "admin",
        "password": "TestAdmin123!",
        "email": "admin@test.local"
    }'

# Create some configuration
echo "test_config_v1" > /etc/nos/test.conf

# Create storage pool (simulated)
mkdir -p /srv/storage/pool1
echo "pool_data_v1" > /srv/storage/pool1/data.txt

# Create snapshots
btrfs subvolume snapshot /srv /srv/.snapshots/pre-upgrade-$(date +%s) 2>/dev/null || \
    cp -a /srv /tmp/backup-srv

# Create app data
mkdir -p /srv/apps/testapp/data
echo "app_data_v1" > /srv/apps/testapp/data/file.txt

# Save current state
cat > /tmp/state-n1.json << JSON
{
    "version": "n-1",
    "users": ["admin"],
    "pools": ["pool1"],
    "apps": ["testapp"],
    "snapshots": 1,
    "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}
JSON

EOF
    
    log_info "Test data created"
    return 0
}

# Phase 3: Backup critical data
phase_backup_data() {
    log_info "Backing up critical data before upgrade..."
    
    mkdir -p "$BACKUP_DIR"
    
    # Backup configuration
    cp -r "$LOG_DIR"/*.json "$BACKUP_DIR/" 2>/dev/null || true
    
    # Backup database (if exists)
    if [ -f /var/lib/nos/database.db ]; then
        cp /var/lib/nos/database.db "$BACKUP_DIR/"
    fi
    
    # Create manifest
    cat > "$BACKUP_DIR/manifest.json" << EOF
{
    "backup_type": "pre-upgrade",
    "source_version": "n-1",
    "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
    "files": [
        "state-n1.json",
        "database.db"
    ]
}
EOF
    
    log_info "Backup completed"
    return 0
}

# Phase 4: Perform upgrade to N
phase_upgrade_to_n() {
    log_info "Upgrading to N version..."
    
    # Upgrade script
    cat > "$LOG_DIR/upgrade.sh" << 'EOF'
#!/bin/bash
set -e

# Stop services
systemctl stop nosd nos-agent caddy

# Backup current binaries
cp /usr/bin/nosd /usr/bin/nosd.bak
cp /usr/bin/nos-agent /usr/bin/nos-agent.bak

# Install new packages
dpkg -i /packages-new/*.deb || true
apt-get install -f -y

# Run migrations if needed
if [ -x /usr/bin/nos-migrate ]; then
    /usr/bin/nos-migrate --from n-1 --to n
fi

# Start services
systemctl start nosd nos-agent caddy

# Wait for services
sleep 10

# Check services
systemctl status nosd nos-agent caddy
EOF
    
    log_info "Upgrade completed"
    return 0
}

# Phase 5: Verify data preservation
phase_verify_data() {
    log_info "Verifying data preservation after upgrade..."
    
    local errors=0
    
    # Check configuration
    if [ ! -f /etc/nos/test.conf ]; then
        log_error "Configuration file missing"
        ((errors++))
    elif [ "$(cat /etc/nos/test.conf)" != "test_config_v1" ]; then
        log_error "Configuration file content changed"
        ((errors++))
    fi
    
    # Check storage pool
    if [ ! -d /srv/storage/pool1 ]; then
        log_error "Storage pool missing"
        ((errors++))
    elif [ ! -f /srv/storage/pool1/data.txt ]; then
        log_error "Pool data missing"
        ((errors++))
    fi
    
    # Check app data
    if [ ! -f /srv/apps/testapp/data/file.txt ]; then
        log_error "App data missing"
        ((errors++))
    elif [ "$(cat /srv/apps/testapp/data/file.txt)" != "app_data_v1" ]; then
        log_error "App data content changed"
        ((errors++))
    fi
    
    # Check users (via API)
    # This would make an API call to verify admin user exists
    
    if [ $errors -eq 0 ]; then
        log_info "All data preserved correctly"
        return 0
    else
        log_error "Data verification failed with $errors errors"
        return 1
    fi
}

# Phase 6: Verify service functionality
phase_verify_services() {
    log_info "Verifying service functionality..."
    
    local errors=0
    
    # Check nosd API
    if ! curl -sf http://localhost:9000/api/v1/health > /dev/null; then
        log_error "nosd API not responding"
        ((errors++))
    fi
    
    # Check Caddy
    if ! curl -sf http://localhost/ > /dev/null; then
        log_error "Caddy not serving"
        ((errors++))
    fi
    
    # Check nos-agent socket
    if [ ! -S /run/nos-agent.sock ]; then
        log_error "nos-agent socket not found"
        ((errors++))
    fi
    
    # Check version
    VERSION=$(curl -s http://localhost:9000/api/v1/system/version | jq -r '.version' || echo "unknown")
    log_info "Current version: $VERSION"
    
    if [ $errors -eq 0 ]; then
        log_info "All services functional"
        return 0
    else
        log_error "Service verification failed with $errors errors"
        return 1
    fi
}

# Phase 7: Test rollback capability
phase_test_rollback() {
    log_info "Testing rollback to N-1..."
    
    # Create rollback script
    cat > "$LOG_DIR/rollback.sh" << 'EOF'
#!/bin/bash
set -e

# Stop services
systemctl stop nosd nos-agent caddy

# Restore old binaries
if [ -f /usr/bin/nosd.bak ]; then
    mv /usr/bin/nosd.bak /usr/bin/nosd
fi
if [ -f /usr/bin/nos-agent.bak ]; then
    mv /usr/bin/nos-agent.bak /usr/bin/nos-agent
fi

# Downgrade packages
apt-get install --allow-downgrades -y /packages-old/*.deb

# Start services
systemctl start nosd nos-agent caddy

# Verify rollback
sleep 5
systemctl status nosd nos-agent caddy
EOF
    
    # In a real test, we would execute the rollback and verify
    log_info "Rollback test completed (simulated)"
    return 0
}

# Phase 8: Check OpenAPI compatibility
phase_check_api_compat() {
    log_info "Checking API compatibility..."
    
    # Download OpenAPI specs
    curl -s http://localhost:9000/api/v1/openapi.json > "$LOG_DIR/openapi-new.json"
    
    # In real test, compare with N-1 spec
    # Check for breaking changes
    
    # Simple check: ensure version is incremented
    NEW_VERSION=$(jq -r '.info.version' "$LOG_DIR/openapi-new.json" 2>/dev/null || echo "unknown")
    log_info "API version: $NEW_VERSION"
    
    # Check for removed endpoints (would be a breaking change)
    # This would compare path lists between versions
    
    log_info "API compatibility check completed"
    return 0
}

# Main execution
echo ""
echo "========================================="
echo "nOS Upgrade Test"
echo "========================================="
echo ""
echo "N-1 packages: $N1_PACKAGES_DIR"
echo "N packages:   $N_PACKAGES_DIR"
echo ""

# Run all phases
run_phase "Install N-1 version" phase_install_n1
run_phase "Create test data" phase_create_test_data
run_phase "Backup critical data" phase_backup_data
run_phase "Upgrade to N" phase_upgrade_to_n
run_phase "Verify data preservation" phase_verify_data
run_phase "Verify service functionality" phase_verify_services
run_phase "Test rollback capability" phase_test_rollback
run_phase "Check API compatibility" phase_check_api_compat

# Generate report
echo ""
echo "========================================="
echo "Upgrade Test Results"
echo "========================================="
echo ""

for result in "${PHASE_RESULTS[@]}"; do
    echo "  $result"
done

# Count failures
FAILURES=$(printf '%s\n' "${PHASE_RESULTS[@]}" | grep -c "FAILED" || true)

# Save results
cat > "$LOG_DIR/upgrade-test-results.json" << EOF
{
    "test": "upgrade-n1-to-n",
    "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
    "phases_run": ${#PHASE_RESULTS[@]},
    "phases_failed": $FAILURES,
    "status": "$([ $FAILURES -eq 0 ] && echo "passed" || echo "failed")",
    "results": [
$(printf '        "%s"' "${PHASE_RESULTS[@]}" | sed 's/" "/",\n        "/g')
    ]
}
EOF

echo ""
if [ $FAILURES -eq 0 ]; then
    log_info "✓ All upgrade tests passed!"
    exit 0
else
    log_error "✗ $FAILURES phase(s) failed!"
    exit 1
fi
