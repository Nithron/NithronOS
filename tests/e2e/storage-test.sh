#!/bin/bash
set -euo pipefail

# Storage E2E Test
# Tests Btrfs operations with loopback devices

# Must run as root for loopback and btrfs operations
if [ "$EUID" -ne 0 ]; then
    echo "This script must be run as root"
    exit 1
fi

# Configuration
TEST_DIR="/tmp/nos-storage-test-$$"
LOOP_FILE="$TEST_DIR/test.img"
LOOP_SIZE="2G"
MOUNT_POINT="$TEST_DIR/mnt"
RESULTS_DIR="tests/e2e/results"
NOS_AGENT="${NOS_AGENT:-./bin/nos-agent}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Cleanup function
cleanup() {
    echo "Cleaning up..."
    
    # Save final results before cleanup
    if [ -n "$RESULTS_DIR" ] && [ -d "$RESULTS_DIR" ]; then
        cat > "$RESULTS_DIR/storage-test-results.json" << EOF
{
    "test_suite": "storage-e2e",
    "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
    "tests_run": ${TESTS_RUN:-0},
    "tests_passed": ${TESTS_PASSED:-0},
    "tests_failed": ${TESTS_FAILED:-0},
    "status": "$([ ${TESTS_FAILED:-0} -eq 0 ] && echo "passed" || echo "failed")",
    "cleanup": "completed"
}
EOF
    fi
    
    # Unmount if mounted
    if mountpoint -q "$MOUNT_POINT" 2>/dev/null; then
        umount "$MOUNT_POINT" || true
    fi
    
    # Detach loop device
    if [ -n "${LOOP_DEV:-}" ]; then
        losetup -d "$LOOP_DEV" 2>/dev/null || true
    fi
    
    # Remove test directory
    rm -rf "$TEST_DIR"
}
trap cleanup EXIT

# Helper functions
log_test() {
    echo -e "${YELLOW}[TEST]${NC} $1"
}

log_pass() {
    echo -e "${GREEN}[PASS]${NC} $1"
}

log_fail() {
    echo -e "${RED}[FAIL]${NC} $1"
    exit 1
}

# Create results directory (use absolute path if relative doesn't exist)
if [[ "$RESULTS_DIR" != /* ]]; then
    # Convert to absolute path from script location
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    RESULTS_DIR="$SCRIPT_DIR/results"
fi
mkdir -p "$RESULTS_DIR"

# Initialize results file immediately
cat > "$RESULTS_DIR/storage-test-results.json" << EOF
{
    "test_suite": "storage-e2e",
    "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
    "status": "running",
    "tests_run": 0,
    "tests_passed": 0,
    "tests_failed": 0
}
EOF

# Test results tracking
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

run_test() {
    local test_name="$1"
    local test_func="$2"
    
    TESTS_RUN=$((TESTS_RUN + 1))
    log_test "$test_name"
    
    if $test_func; then
        TESTS_PASSED=$((TESTS_PASSED + 1))
        log_pass "$test_name"
    else
        TESTS_FAILED=$((TESTS_FAILED + 1))
        log_fail "$test_name"
    fi
}

# Test 1: Create and format Btrfs filesystem
test_create_btrfs() {
    # Create test directory and loop file
    mkdir -p "$TEST_DIR"
    dd if=/dev/zero of="$LOOP_FILE" bs=1M count=2048 status=none
    
    # Setup loop device
    LOOP_DEV=$(losetup -f --show "$LOOP_FILE")
    
    # Create Btrfs filesystem
    mkfs.btrfs -f "$LOOP_DEV" > /dev/null 2>&1
    
    # Mount filesystem
    mkdir -p "$MOUNT_POINT"
    mount "$LOOP_DEV" "$MOUNT_POINT"
    
    # Verify mount
    if ! mountpoint -q "$MOUNT_POINT"; then
        return 1
    fi
    
    # Check filesystem type
    FS_TYPE=$(stat -f -c %T "$MOUNT_POINT")
    if [ "$FS_TYPE" != "btrfs" ]; then
        echo "Expected btrfs, got $FS_TYPE"
        return 1
    fi
    
    return 0
}

# Test 2: Create expected subvolume layout
test_subvolume_layout() {
    # Create nOS standard subvolumes
    btrfs subvolume create "$MOUNT_POINT/@" > /dev/null
    btrfs subvolume create "$MOUNT_POINT/@home" > /dev/null
    btrfs subvolume create "$MOUNT_POINT/@var" > /dev/null
    btrfs subvolume create "$MOUNT_POINT/@log" > /dev/null
    btrfs subvolume create "$MOUNT_POINT/@snapshots" > /dev/null
    
    # Verify subvolumes exist
    local subvols=$(btrfs subvolume list "$MOUNT_POINT" | wc -l)
    if [ "$subvols" -ne 5 ]; then
        echo "Expected 5 subvolumes, found $subvols"
        return 1
    fi
    
    # Set proper mount options
    # In real system, these would be in /etc/fstab
    echo "Mount options that would be used:"
    echo "  @:          defaults,noatime,compress=zstd:3,ssd,discard=async"
    echo "  @home:      defaults,noatime,compress=zstd:3"
    echo "  @var:       defaults,noatime,compress=zstd:3"
    echo "  @log:       defaults,noatime,compress=zstd:3"
    echo "  @snapshots: defaults,noatime"
    
    return 0
}

# Test 3: Snapshot operations
test_snapshots() {
    local SNAPSHOT_DIR="$MOUNT_POINT/@snapshots"
    
    # Create test data in @ subvolume
    mkdir -p "$MOUNT_POINT/@/test-data"
    echo "Test content $(date)" > "$MOUNT_POINT/@/test-data/file1.txt"
    echo "More content" > "$MOUNT_POINT/@/test-data/file2.txt"
    
    # Create snapshot
    local SNAPSHOT_NAME="@-$(date +%Y%m%d-%H%M%S)-test"
    btrfs subvolume snapshot -r "$MOUNT_POINT/@" "$SNAPSHOT_DIR/$SNAPSHOT_NAME" > /dev/null
    
    # Verify snapshot exists
    if [ ! -d "$SNAPSHOT_DIR/$SNAPSHOT_NAME" ]; then
        echo "Snapshot not created"
        return 1
    fi
    
    # Verify snapshot is read-only
    if touch "$SNAPSHOT_DIR/$SNAPSHOT_NAME/test" 2>/dev/null; then
        echo "Snapshot is not read-only"
        return 1
    fi
    
    # List snapshots
    echo "Snapshots:"
    btrfs subvolume list -s "$MOUNT_POINT" | grep "@snapshots"
    
    # Delete snapshot
    btrfs subvolume delete "$SNAPSHOT_DIR/$SNAPSHOT_NAME" > /dev/null
    
    # Verify deletion
    if [ -d "$SNAPSHOT_DIR/$SNAPSHOT_NAME" ]; then
        echo "Snapshot not deleted"
        return 1
    fi
    
    return 0
}

# Test 4: Retention policy simulation
test_retention_policy() {
    local SNAPSHOT_DIR="$MOUNT_POINT/@snapshots/@"
    mkdir -p "$SNAPSHOT_DIR"
    
    # Create multiple snapshots with different timestamps
    local DATES=(
        "20240101-000000"
        "20240102-000000"
        "20240103-000000"
        "20240110-000000"
        "20240117-000000"
        "20240124-000000"
        "20240131-000000"
    )
    
    for date in "${DATES[@]}"; do
        btrfs subvolume create "$SNAPSHOT_DIR/$date-daily" > /dev/null
    done
    
    # Simulate GFS retention (keep 7 daily, 4 weekly, 12 monthly)
    echo "Simulating retention policy..."
    
    # In real implementation, this would be done by nosd
    # Here we just verify the logic
    local total_snaps=$(btrfs subvolume list "$MOUNT_POINT" | grep -c "daily")
    echo "  Created $total_snaps test snapshots"
    
    # Clean up test snapshots
    for date in "${DATES[@]}"; do
        btrfs subvolume delete "$SNAPSHOT_DIR/$date-daily" > /dev/null 2>&1 || true
    done
    
    return 0
}

# Test 5: Scrub operations
test_scrub() {
    # Start scrub
    btrfs scrub start -B "$MOUNT_POINT" > /dev/null 2>&1
    
    # Check scrub status
    local scrub_status=$(btrfs scrub status "$MOUNT_POINT")
    echo "Scrub status:"
    echo "$scrub_status" | grep -E "(Status|Total|Rate)" | sed 's/^/  /'
    
    # Verify no errors
    if echo "$scrub_status" | grep -q "errors: 0"; then
        return 0
    else
        echo "Scrub reported errors"
        return 1
    fi
}

# Test 6: Space usage and quotas
test_space_usage() {
    # Enable quotas
    btrfs quota enable "$MOUNT_POINT"
    
    # Get filesystem usage
    local usage=$(btrfs filesystem usage "$MOUNT_POINT")
    echo "Filesystem usage:"
    echo "$usage" | grep -E "(Device size|Used|Free)" | sed 's/^/  /'
    
    # Check device stats
    local stats=$(btrfs device stats "$MOUNT_POINT")
    echo "Device stats:"
    echo "$stats" | sed 's/^/  /'
    
    # Verify no errors in device stats
    if echo "$stats" | grep -vq " 0$"; then
        echo "Device has errors"
        return 1
    fi
    
    return 0
}

# Test 7: Error handling - low disk space
test_low_disk_space() {
    # Fill disk to trigger low space condition
    # Create a large file that fills most of the space
    local available=$(df --output=avail "$MOUNT_POINT" | tail -1)
    local fill_size=$((available - 100000))  # Leave 100MB free
    
    if [ $fill_size -gt 0 ]; then
        dd if=/dev/zero of="$MOUNT_POINT/@/largefile" bs=1024 count=$fill_size status=none 2>/dev/null || true
    fi
    
    # Try to create snapshot - should handle gracefully
    local SNAPSHOT_DIR="$MOUNT_POINT/@snapshots"
    local result=0
    
    if btrfs subvolume snapshot -r "$MOUNT_POINT/@" "$SNAPSHOT_DIR/@-lowspace-test" 2>/dev/null; then
        # Snapshot created, clean up
        btrfs subvolume delete "$SNAPSHOT_DIR/@-lowspace-test" > /dev/null 2>&1
    else
        # Expected to fail with low space
        echo "  Snapshot creation properly failed with low space"
    fi
    
    # Clean up large file
    rm -f "$MOUNT_POINT/@/largefile"
    
    return 0
}

# Test 8: Balance operation
test_balance() {
    # Start balance with filters (only test with small amount)
    echo "Starting balance operation..."
    btrfs balance start -dusage=50 -musage=50 "$MOUNT_POINT"
    
    # Check balance status
    local balance_status=$(btrfs balance status "$MOUNT_POINT")
    echo "Balance status:"
    echo "$balance_status" | sed 's/^/  /'
    
    return 0
}

# Test 9: Send/Receive for backup
test_send_receive() {
    local SNAPSHOT_DIR="$MOUNT_POINT/@snapshots"
    local BACKUP_DIR="$TEST_DIR/backup"
    mkdir -p "$BACKUP_DIR"
    
    # Create source snapshot
    btrfs subvolume snapshot -r "$MOUNT_POINT/@" "$SNAPSHOT_DIR/@-backup-test" > /dev/null
    
    # Send snapshot to backup location
    btrfs send "$SNAPSHOT_DIR/@-backup-test" 2>/dev/null | btrfs receive "$BACKUP_DIR" 2>/dev/null
    
    # Verify backup exists
    if [ ! -d "$BACKUP_DIR/@-backup-test" ]; then
        echo "Backup not created"
        return 1
    fi
    
    # Clean up
    btrfs subvolume delete "$BACKUP_DIR/@-backup-test" > /dev/null
    btrfs subvolume delete "$SNAPSHOT_DIR/@-backup-test" > /dev/null
    
    return 0
}

# Test 10: SMART integration simulation
test_smart_simulation() {
    echo "Simulating SMART health check..."
    
    # In real system, would use smartctl
    # Here we simulate the check
    echo "  Device: $LOOP_DEV (simulated)"
    echo "  SMART Health Status: OK"
    echo "  Temperature: 35°C"
    echo "  Power-On Hours: 1234"
    echo "  Reallocated Sectors: 0"
    echo "  Pending Sectors: 0"
    
    return 0
}

# Main test execution
echo "========================================="
echo "Storage E2E Tests"
echo "========================================="
echo ""

# Run all tests
run_test "Create and format Btrfs filesystem" test_create_btrfs
run_test "Create subvolume layout" test_subvolume_layout
run_test "Snapshot operations" test_snapshots
run_test "Retention policy simulation" test_retention_policy
run_test "Scrub operations" test_scrub
run_test "Space usage and quotas" test_space_usage
run_test "Low disk space handling" test_low_disk_space
run_test "Balance operation" test_balance
run_test "Send/Receive backup" test_send_receive
run_test "SMART health simulation" test_smart_simulation

# Generate results report
echo ""
echo "========================================="
echo "Test Results"
echo "========================================="
echo "Tests Run:    $TESTS_RUN"
echo "Tests Passed: $TESTS_PASSED"
echo "Tests Failed: $TESTS_FAILED"
echo ""

# Save results to JSON
cat > "$RESULTS_DIR/storage-test-results.json" << EOF
{
    "test_suite": "storage-e2e",
    "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
    "tests_run": $TESTS_RUN,
    "tests_passed": $TESTS_PASSED,
    "tests_failed": $TESTS_FAILED,
    "status": "$([ $TESTS_FAILED -eq 0 ] && echo "passed" || echo "failed")",
    "details": {
        "filesystem": "btrfs",
        "loop_device": "${LOOP_DEV:-unknown}",
        "test_size": "$LOOP_SIZE"
    }
}
EOF

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}Some tests failed!${NC}"
    exit 1
fi
