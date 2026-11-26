# Monitoring & Alerts Guide

## Overview

nOS provides comprehensive real-time and historical monitoring with intelligent alerting. The system collects metrics for CPU, memory, disk, network, services, and more, storing them efficiently with automatic downsampling for long-term retention.

## Key Features

- **Real-time Metrics**: 5-second refresh for critical system metrics
- **Time-series Storage**: Efficient SQLite-based storage with automatic downsampling
- **Smart Alerts**: Threshold-based rules with hysteresis and cooldown
- **Multiple Channels**: Email, webhook, and ntfy notifications
- **Service Monitoring**: Track systemd service health and restarts
- **SMART Monitoring**: Disk health and temperature tracking
- **Btrfs Metrics**: Scrub status and error tracking

## System Metrics

### CPU Monitoring
- Overall usage percentage
- Per-core utilization
- Temperature monitoring (when available)
- Load averages (1/5/15 minute)

### Memory Monitoring
- RAM usage and availability
- Swap usage and activity
- Buffer and cache statistics
- Memory pressure indicators

### Disk Monitoring
- Space usage per filesystem
- I/O statistics (read/write bytes and operations)
- SMART health status
- Disk temperatures
- Btrfs-specific metrics (errors, scrub status)

### Network Monitoring
- Interface statistics (RX/TX bytes and packets)
- Error and drop counters
- Link state and speed
- Per-interface bandwidth usage

### Service Monitoring
Tracked services:
- `nosd` - Main API server
- `nos-agent` - Privileged operations
- `caddy` - Web server and reverse proxy
- `wireguard` - VPN service
- `docker` - Container runtime

## Alert System

### Alert Rules

Rules define conditions that trigger notifications:

```yaml
Rule Structure:
- Metric: What to monitor (cpu, memory, disk_space, etc.)
- Operator: Comparison (>, <, ==, !=)
- Threshold: Trigger value
- Duration: How long condition must persist
- Severity: info, warning, or critical
- Cooldown: Minimum time between alerts
```

### Predefined Rules

| Rule | Condition | Severity | Cooldown |
|------|-----------|----------|----------|
| High CPU | >90% for 5 min | Warning | 15 min |
| High Memory | >90% for 2 min | Warning | 15 min |
| Disk Space | >85% | Critical | 30 min |
| Service Down | Not running for 1 min | Critical | 5 min |
| SMART Failure | Health check failed | Critical | 60 min |
| High Disk Temp | >60°C for 5 min | Warning | 30 min |
| Backup Failed | Job failed | Warning | 60 min |
| Btrfs Errors | Any errors detected | Critical | 60 min |

### Hysteresis

Prevents alert flapping when values hover around threshold:

```
Firing threshold: 90%
Clearing threshold (10% hysteresis): 81%

This prevents rapid firing/clearing when value fluctuates around 90%
```

## Notification Channels

### Email

Configure SMTP settings for email alerts:

```json
{
  "type": "email",
  "config": {
    "smtp_host": "smtp.gmail.com",
    "smtp_port": 587,
    "smtp_user": "alerts@example.com",
    "smtp_password": "app-password",
    "use_starttls": true,
    "from": "nOS <alerts@example.com>",
    "to": ["admin@example.com"]
  }
}
```

### Webhook

Send JSON payloads to external services:

```json
{
  "type": "webhook",
  "config": {
    "url": "https://hooks.slack.com/services/xxx",
    "method": "POST",
    "headers": {
      "Content-Type": "application/json"
    },
    "secret": "optional-hmac-secret"
  }
}
```

### Ntfy

Push notifications to mobile devices:

```json
{
  "type": "ntfy",
  "config": {
    "server_url": "https://ntfy.sh",
    "topic": "nos-alerts",
    "priority": 4,
    "tags": ["warning", "computer"]
  }
}
```

## Data Retention

Time-series data is automatically downsampled:

| Resolution | Retention | Use Case |
|------------|-----------|----------|
| Raw (1s-1m) | 24 hours | Real-time monitoring |
| 1-minute | 7 days | Recent history |
| 1-hour | 30 days | Long-term trends |

### Storage Requirements

Approximate disk usage:
- Raw data: ~50MB/day
- 1-minute rollups: ~10MB/week
- 1-hour rollups: ~5MB/month

Total: ~200MB for 30 days (default retention)

## Dashboard

The monitoring dashboard provides:

### Overview Cards
- CPU usage with sparkline
- Memory usage with trend
- Disk usage per filesystem
- System load and uptime

### Service Health
- Status indicators (green/red)
- Restart counters
- Resource usage per service

### Storage Details
- Per-device usage and temperature
- SMART health status
- I/O statistics

### Network Status
- Interface link state
- Traffic counters
- Error indicators

## API Endpoints

### Metrics

```bash
# Get system overview
curl https://localhost/api/v1/monitor/overview

# Query time series
curl -X POST https://localhost/api/v1/monitor/timeseries \
  -H "Content-Type: application/json" \
  -d '{
    "metric": "cpu",
    "start_time": "2024-01-01T00:00:00Z",
    "end_time": "2024-01-01T01:00:00Z",
    "step": 60
  }'

# Get device metrics
curl https://localhost/api/v1/monitor/devices

# Get service status
curl https://localhost/api/v1/monitor/services
```

### Alerts

```bash
# List alert rules
curl https://localhost/api/v1/monitor/alerts/rules

# Create alert rule
curl -X POST https://localhost/api/v1/monitor/alerts/rules \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Custom Alert",
    "metric": "cpu",
    "operator": ">",
    "threshold": 80,
    "duration": 300,
    "severity": "warning"
  }'

# List notification channels
curl https://localhost/api/v1/monitor/alerts/channels

# Test notification channel
curl -X POST https://localhost/api/v1/monitor/alerts/channels/{id}/test
```

## Best Practices

### Alert Configuration

1. **Start Conservative**: Begin with higher thresholds and adjust based on experience
2. **Use Cooldowns**: Prevent alert spam with appropriate cooldown periods
3. **Apply Hysteresis**: Add 5-10% hysteresis for metrics that fluctuate
4. **Test Channels**: Always test notification channels after configuration

### Performance Tuning

1. **Metric Collection**: Default 60s interval balances freshness vs overhead
2. **SMART Polling**: Limited to every 5 minutes to avoid disk stress
3. **Service Checks**: Cached for 30s to reduce systemd queries
4. **Database Maintenance**: Automatic VACUUM at 3 AM daily

### Quiet Hours

Configure quiet hours to suppress non-critical alerts:

```json
{
  "quiet_hours": {
    "enabled": true,
    "start_time": "22:00",
    "end_time": "07:00",
    "weekends": false
  }
}
```

## Troubleshooting

### High CPU Usage from Monitoring

**Symptom**: Collector using >2% CPU continuously

**Solutions**:
- Increase collection interval in config
- Disable per-core CPU metrics
- Reduce SMART polling frequency

### Missing Metrics

**Symptom**: Gaps in time-series data

**Causes**:
- Service restart (check logs)
- Database lock contention
- Disk space issues

**Solutions**:
- Check `journalctl -u nosd`
- Verify database integrity
- Ensure adequate free space

### Alert Not Firing

**Symptom**: Condition met but no notification

**Checklist**:
1. Rule enabled? Check `enabled: true`
2. Duration requirement met?
3. Still in cooldown period?
4. Channel enabled and configured?
5. Quiet hours active?
6. Rate limit exceeded?

### Database Growth

**Symptom**: metrics.db growing too large

**Solutions**:
- Adjust retention periods
- Verify cleanup job running
- Manual VACUUM if needed:
  ```bash
  sqlite3 /var/lib/nos/metrics.db "VACUUM;"
  ```

## Advanced Configuration

### Custom Metrics

Add custom metrics via API:

```python
#!/usr/bin/env python3
import requests
import psutil

# Collect custom metric
process = psutil.Process()
custom_metric = process.num_threads()

# Store via API
requests.post('https://localhost/api/v1/monitor/custom', json={
    'metric': 'app_threads',
    'value': custom_metric,
    'labels': {'app': 'myapp'}
})
```

### Webhook Templates

Customize webhook payloads:

```json
{
  "template": "{\"text\": \"Alert: {{.RuleName}}\\nValue: {{.Value}}\\nHost: {{.Hostname}}\"}"
}
```

### Grafana Integration

Export metrics for external visualization:

```yaml
# Prometheus endpoint (future)
scrape_configs:
  - job_name: 'nos'
    static_configs:
      - targets: ['localhost:9090']
```

## Security Considerations

### Sensitive Data

- SMTP passwords stored encrypted
- Webhook secrets never displayed in UI
- API tokens masked in responses

### Rate Limiting

- Max 100 alerts/hour per channel
- Configurable per-channel limits
- Automatic backoff on failures

### Access Control

- Only admins can configure alerts
- Read-only users see metrics only
- Audit log for all changes

## Maintenance

### Regular Tasks

**Daily**:
- Review active alerts
- Check service health
- Monitor disk space trends

**Weekly**:
- Test critical alert channels
- Review and tune thresholds
- Check for unusual patterns

**Monthly**:
- Audit alert rules
- Clean up resolved events
- Update notification lists
- Review retention policies

## Integration Examples

### Slack Webhook

```javascript
// Slack webhook handler
{
  "url": "https://hooks.slack.com/services/T00/B00/XXX",
  "template": {
    "attachments": [{
      "color": "{{if eq .Severity \"critical\"}}danger{{else}}warning{{end}}",
      "title": "{{.Title}}",
      "text": "{{.Body}}",
      "fields": [
        {"title": "Metric", "value": "{{.Metric}}", "short": true},
        {"title": "Value", "value": "{{.Value}}", "short": true}
      ],
      "footer": "nOS",
      "ts": "{{.Timestamp}}"
    }]
  }
}
```

### PagerDuty Integration

```javascript
// PagerDuty webhook
{
  "url": "https://events.pagerduty.com/v2/enqueue",
  "headers": {
    "Authorization": "Token token=YOUR_TOKEN"
  },
  "template": {
    "routing_key": "YOUR_ROUTING_KEY",
    "event_action": "{{if eq .State \"firing\"}}trigger{{else}}resolve{{end}}",
    "dedup_key": "{{.RuleID}}",
    "payload": {
      "summary": "{{.Title}}",
      "severity": "{{.Severity}}",
      "source": "{{.Hostname}}",
      "custom_details": {
        "metric": "{{.Metric}}",
        "value": "{{.Value}}",
        "threshold": "{{.Threshold}}"
      }
    }
  }
}
```

## See Also

- [System Health](admin/health.md)
- [Service Management](admin/services.md)
- [Storage Monitoring](storage/health.md)
- [API Documentation](api/monitoring.yaml)
