#!/bin/bash
set -euo pipefail

# nOS App Snapshot Helper - Btrfs-aware snapshot management
# Provides snapshot and rollback functionality for app data directories

APPS_ROOT="/srv/apps"
SNAPSHOT_ROOT="/srv/apps/.snapshots"

# Logging
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] nos-app-snapshot: $*" >&2
}

error() {
    log "ERROR: $*"
    exit 1
}

warn() {
    log "WARN: $*"
}

# Check if a path is on a Btrfs filesystem
is_btrfs() {
    local path="$1"
    local fs_type
    
    # Get filesystem type using stat
    fs_type=$(stat -f -c %T "$path" 2>/dev/null || echo "unknown")
    
    if [[ "$fs_type" == "btrfs" ]]; then
        return 0
    else
        return 1
    fi
}

# Check if a path is a Btrfs subvolume
is_subvolume() {
    local path="$1"
    
    if ! is_btrfs "$path"; then
        return 1
    fi
    
    # Check using btrfs subvolume show
    if btrfs subvolume show "$path" &>/dev/null; then
        return 0
    else
        return 1
    fi
}

# Create a Btrfs subvolume for app data if it doesn't exist
ensure_subvolume() {
    local app_id="$1"
    local data_dir="${APPS_ROOT}/${app_id}/data"
    
    # Create parent directory if needed
    if [[ ! -d "$(dirname "$data_dir")" ]]; then
        mkdir -p "$(dirname "$data_dir")"
    fi
    
    # If data dir doesn't exist and parent is Btrfs, create as subvolume
    if [[ ! -e "$data_dir" ]] && is_btrfs "$(dirname "$data_dir")"; then
        log "Creating Btrfs subvolume: $data_dir"
        btrfs subvolume create "$data_dir"
        chown nos:nos "$data_dir"
        chmod 755 "$data_dir"
        return 0
    fi
    
    # If data dir exists but isn't a subvolume on Btrfs, warn
    if [[ -d "$data_dir" ]] && is_btrfs "$data_dir" && ! is_subvolume "$data_dir"; then
        warn "Data directory exists but is not a subvolume: $data_dir"
        return 1
    fi
    
    # If not on Btrfs, just ensure directory exists
    if [[ ! -d "$data_dir" ]]; then
        mkdir -p "$data_dir"
        chown nos:nos "$data_dir"
        chmod 755 "$data_dir"
    fi
    
    return 0
}

# Create a snapshot (Btrfs or rsync fallback)
snapshot_pre() {
    local app_id="$1"
    local snapshot_name="${2:-pre-change}"
    local data_dir="${APPS_ROOT}/${app_id}/data"
    local snapshot_dir="${SNAPSHOT_ROOT}/${app_id}"
    local timestamp=$(date +%Y%m%d-%H%M%S)
    local snapshot_path="${snapshot_dir}/${timestamp}-${snapshot_name}"
    
    # Check if data directory exists
    if [[ ! -d "$data_dir" ]]; then
        warn "Data directory not found, skipping snapshot: $data_dir"
        return 0
    fi
    
    # Create snapshot directory
    mkdir -p "$snapshot_dir"
    
    # Try Btrfs snapshot first
    if is_subvolume "$data_dir"; then
        log "Creating Btrfs snapshot: $snapshot_path"
        btrfs subvolume snapshot -r "$data_dir" "$snapshot_path"
        
        # Store metadata
        cat > "${snapshot_path}.meta" <<EOF
{
    "app_id": "$app_id",
    "type": "btrfs",
    "timestamp": "$timestamp",
    "name": "$snapshot_name",
    "source": "$data_dir",
    "created": "$(date -Iseconds)"
}
EOF
    else
        # Fallback to rsync copy
        warn "Not a Btrfs subvolume, using rsync for snapshot (this may be slow)"
        log "Creating rsync snapshot: $snapshot_path"
        
        # Create snapshot with rsync
        rsync -aHAXS --delete "$data_dir/" "$snapshot_path/"
        
        # Make it read-only
        chmod -R a-w "$snapshot_path"
        
        # Store metadata
        cat > "${snapshot_path}.meta" <<EOF
{
    "app_id": "$app_id",
    "type": "rsync",
    "timestamp": "$timestamp",
    "name": "$snapshot_name",
    "source": "$data_dir",
    "created": "$(date -Iseconds)"
}
EOF
    fi
    
    log "Snapshot created: $snapshot_path"
    echo "$snapshot_path"
}

# Rollback to a snapshot
rollback() {
    local app_id="$1"
    local snapshot_timestamp="$2"
    local data_dir="${APPS_ROOT}/${app_id}/data"
    local snapshot_dir="${SNAPSHOT_ROOT}/${app_id}"
    
    # Find snapshot
    local snapshot_path=""
    for snap in "${snapshot_dir}/${snapshot_timestamp}"*; do
        if [[ -e "$snap" ]] && [[ ! "$snap" == *.meta ]]; then
            snapshot_path="$snap"
            break
        fi
    done
    
    if [[ -z "$snapshot_path" ]] || [[ ! -e "$snapshot_path" ]]; then
        error "Snapshot not found: ${snapshot_timestamp}"
    fi
    
    # Read metadata
    local meta_file="${snapshot_path}.meta"
    if [[ ! -f "$meta_file" ]]; then
        error "Snapshot metadata not found: $meta_file"
    fi
    
    local snap_type=$(jq -r '.type' "$meta_file")
    
    log "Rolling back to snapshot: $snapshot_path (type: $snap_type)"
    
    # Create a backup of current state before rollback
    snapshot_pre "$app_id" "pre-rollback"
    
    if [[ "$snap_type" == "btrfs" ]]; then
        # Btrfs rollback: delete current and snapshot from backup
        if is_subvolume "$data_dir"; then
            log "Removing current subvolume: $data_dir"
            btrfs subvolume delete "$data_dir"
        else
            log "Removing current directory: $data_dir"
            rm -rf "$data_dir"
        fi
        
        log "Creating new snapshot from: $snapshot_path"
        btrfs subvolume snapshot "$snapshot_path" "$data_dir"
        
        # Remove read-only flag from restored snapshot
        btrfs property set "$data_dir" ro false
        
    else
        # Rsync rollback
        log "Restoring from rsync snapshot: $snapshot_path"
        
        # Remove current data
        rm -rf "$data_dir"
        mkdir -p "$data_dir"
        
        # Restore from snapshot
        rsync -aHAXS "$snapshot_path/" "$data_dir/"
        
        # Restore write permissions
        chmod -R u+w "$data_dir"
    fi
    
    # Fix ownership
    chown -R nos:nos "$data_dir"
    
    log "Rollback completed successfully"
}

# List snapshots for an app
list_snapshots() {
    local app_id="$1"
    local snapshot_dir="${SNAPSHOT_ROOT}/${app_id}"
    
    if [[ ! -d "$snapshot_dir" ]]; then
        echo '{"snapshots": []}'
        return
    fi
    
    local snapshots=()
    
    for meta_file in "${snapshot_dir}"/*.meta; do
        if [[ -f "$meta_file" ]]; then
            local meta=$(cat "$meta_file")
            snapshots+=("$meta")
        fi
    done
    
    # Convert to JSON array
    if [[ ${#snapshots[@]} -eq 0 ]]; then
        echo '{"snapshots": []}'
    else
        printf '%s\n' "${snapshots[@]}" | jq -s '{snapshots: .}'
    fi
}

# Clean old snapshots (keep N most recent)
prune_snapshots() {
    local app_id="$1"
    local keep_count="${2:-5}"
    local snapshot_dir="${SNAPSHOT_ROOT}/${app_id}"
    
    if [[ ! -d "$snapshot_dir" ]]; then
        return 0
    fi
    
    log "Pruning snapshots for app: $app_id (keeping $keep_count most recent)"
    
    # Get all snapshots sorted by timestamp (newest first)
    local snapshots=()
    while IFS= read -r -d '' snapshot; do
        snapshots+=("$snapshot")
    done < <(find "$snapshot_dir" -maxdepth 1 -type d -name "[0-9]*" -print0 | sort -zr)
    
    # Also handle Btrfs subvolumes
    if is_btrfs "$snapshot_dir"; then
        while IFS= read -r snapshot; do
            if [[ ! " ${snapshots[@]} " =~ " ${snapshot} " ]]; then
                snapshots+=("$snapshot")
            fi
        done < <(btrfs subvolume list -o "$snapshot_dir" 2>/dev/null | awk '{print $NF}' | sort -r)
    fi
    
    # Remove old snapshots
    local count=0
    for snapshot in "${snapshots[@]}"; do
        count=$((count + 1))
        if [[ $count -gt $keep_count ]]; then
            log "Removing old snapshot: $snapshot"
            
            # Check if it's a Btrfs subvolume
            if is_subvolume "$snapshot"; then
                btrfs subvolume delete "$snapshot"
            else
                rm -rf "$snapshot"
            fi
            
            # Remove metadata file
            rm -f "${snapshot}.meta"
        fi
    done
    
    log "Snapshot pruning completed"
}

# Main command dispatcher
main() {
    local cmd="${1:-}"
    shift || true
    
    case "$cmd" in
        ensure-subvolume)
            ensure_subvolume "$@"
            ;;
        snapshot-pre)
            snapshot_pre "$@"
            ;;
        rollback)
            rollback "$@"
            ;;
        list-snapshots)
            list_snapshots "$@"
            ;;
        prune-snapshots)
            prune_snapshots "$@"
            ;;
        is-btrfs)
            is_btrfs "$@" && echo "yes" || echo "no"
            ;;
        *)
            cat <<EOF
Usage: $(basename "$0") COMMAND [ARGS...]

Commands:
    ensure-subvolume APP_ID           Ensure app data dir is a Btrfs subvolume
    snapshot-pre APP_ID [NAME]        Create pre-change snapshot
    rollback APP_ID TIMESTAMP         Rollback to a snapshot
    list-snapshots APP_ID             List all snapshots for an app
    prune-snapshots APP_ID [KEEP]     Remove old snapshots (default: keep 5)
    is-btrfs PATH                     Check if path is on Btrfs
EOF
            exit 1
            ;;
    esac
}

# Check for required tools
if ! command -v btrfs &>/dev/null; then
    warn "btrfs command not found, Btrfs operations will fail"
fi

if ! command -v rsync &>/dev/null; then
    error "rsync is required but not installed"
fi

main "$@"
