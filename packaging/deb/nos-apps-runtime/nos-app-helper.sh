#!/bin/bash
set -euo pipefail

# nOS App Helper - CLI utilities for Docker/Compose operations
# Called by nosd and systemd units for app lifecycle management

APPS_ROOT="/srv/apps"
STATE_DIR="/var/lib/nos/apps/state"
SNAPSHOT_DIR="/srv/apps/.snapshots"

# Logging
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] nos-app-helper: $*" >&2
}

error() {
    log "ERROR: $*"
    exit 1
}

# Check if Docker is operational
docker_ok() {
    if ! docker info &>/dev/null; then
        return 1
    fi
    return 0
}

# Compose operations
compose_up() {
    local project_dir="$1"
    local app_id="$(basename "$project_dir")"
    
    if [[ ! -d "$project_dir" ]]; then
        error "Project directory not found: $project_dir"
    fi
    
    log "Starting app: $app_id"
    docker compose \
        --project-directory "$project_dir" \
        --project-name "nos-app-${app_id}" \
        up -d --remove-orphans
}

compose_down() {
    local project_dir="$1"
    local app_id="$(basename "$project_dir")"
    
    if [[ ! -d "$project_dir" ]]; then
        error "Project directory not found: $project_dir"
    fi
    
    log "Stopping app: $app_id"
    docker compose \
        --project-directory "$project_dir" \
        --project-name "nos-app-${app_id}" \
        down
}

compose_ps() {
    local project_dir="$1"
    local app_id="$(basename "$project_dir")"
    
    if [[ ! -d "$project_dir" ]]; then
        error "Project directory not found: $project_dir"
    fi
    
    docker compose \
        --project-directory "$project_dir" \
        --project-name "nos-app-${app_id}" \
        ps --format json
}

# Health check for a container
health_read() {
    local container="$1"
    local health_check_url="${2:-}"
    
    # Get container health from Docker
    local docker_health=$(docker inspect "$container" 2>/dev/null | \
        jq -r '.[0].State.Health.Status // "unknown"')
    
    # Optional HTTP health check
    local http_health="not_configured"
    if [[ -n "$health_check_url" ]]; then
        if curl -sf -m 5 "$health_check_url" &>/dev/null; then
            http_health="healthy"
        else
            http_health="unhealthy"
        fi
    fi
    
    # Return JSON result
    jq -n \
        --arg docker "$docker_health" \
        --arg http "$http_health" \
        --arg container "$container" \
        '{
            container: $container,
            docker_health: $docker,
            http_health: $http,
            timestamp: now | strftime("%Y-%m-%dT%H:%M:%SZ")
        }'
}

# Pre-start checks and setup
pre_start() {
    local app_id="$1"
    local app_dir="${APPS_ROOT}/${app_id}"
    
    # Ensure Docker is running
    if ! docker_ok; then
        error "Docker is not running"
    fi
    
    # Check if app directory exists
    if [[ ! -d "$app_dir" ]]; then
        error "App directory not found: $app_dir"
    fi
    
    # Ensure config directory exists
    if [[ ! -d "${app_dir}/config" ]]; then
        error "App config directory not found: ${app_dir}/config"
    fi
    
    # Check for docker-compose.yml
    if [[ ! -f "${app_dir}/config/docker-compose.yml" ]] && \
       [[ ! -f "${app_dir}/config/docker-compose.yaml" ]] && \
       [[ ! -f "${app_dir}/config/compose.yml" ]] && \
       [[ ! -f "${app_dir}/config/compose.yaml" ]]; then
        error "No compose file found in ${app_dir}/config"
    fi
    
    # Create data directory if missing
    if [[ ! -d "${app_dir}/data" ]]; then
        log "Creating data directory: ${app_dir}/data"
        mkdir -p "${app_dir}/data"
        chown nos:nos "${app_dir}/data"
    fi
    
    log "Pre-start checks passed for app: $app_id"
}

# List all installed apps
list_apps() {
    local apps=()
    
    if [[ -d "$APPS_ROOT" ]]; then
        for app_dir in "$APPS_ROOT"/*; do
            if [[ -d "$app_dir" ]] && [[ -d "${app_dir}/config" ]]; then
                apps+=("$(basename "$app_dir")")
            fi
        done
    fi
    
    printf '%s\n' "${apps[@]}" | jq -R . | jq -s '{apps: .}'
}

# Get app status
app_status() {
    local app_id="$1"
    local app_dir="${APPS_ROOT}/${app_id}/config"
    
    if [[ ! -d "$app_dir" ]]; then
        echo '{"status": "not_found"}'
        return
    fi
    
    local containers_json=$(compose_ps "$app_dir" 2>/dev/null || echo '[]')
    local running_count=$(echo "$containers_json" | jq '[.[] | select(.State == "running")] | length')
    local total_count=$(echo "$containers_json" | jq 'length')
    
    jq -n \
        --arg app_id "$app_id" \
        --argjson running "$running_count" \
        --argjson total "$total_count" \
        --argjson containers "$containers_json" \
        '{
            app_id: $app_id,
            running_containers: $running,
            total_containers: $total,
            status: (if $total == 0 then "stopped" elif $running == $total then "running" else "partial" end),
            containers: $containers
        }'
}

# Main command dispatcher
main() {
    local cmd="${1:-}"
    shift || true
    
    case "$cmd" in
        docker-ok)
            docker_ok && echo "Docker is operational" || error "Docker is not operational"
            ;;
        compose-up)
            compose_up "$@"
            ;;
        compose-down)
            compose_down "$@"
            ;;
        compose-ps)
            compose_ps "$@"
            ;;
        health-read)
            health_read "$@"
            ;;
        pre-start)
            pre_start "$@"
            ;;
        list-apps)
            list_apps
            ;;
        app-status)
            app_status "$@"
            ;;
        *)
            cat <<EOF
Usage: $(basename "$0") COMMAND [ARGS...]

Commands:
    docker-ok              Check if Docker is operational
    compose-up DIR         Start app with docker-compose up
    compose-down DIR       Stop app with docker-compose down
    compose-ps DIR         List containers for app
    health-read CONTAINER  Get health status of container
    pre-start APP_ID       Pre-start checks for app
    list-apps              List all installed apps
    app-status APP_ID      Get status of an app
EOF
            exit 1
            ;;
    esac
}

main "$@"
