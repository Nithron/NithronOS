# Storage health

## SMART
The system collects basic drive health via `smartctl` when available. The UI can display pass/fail status, temperature (°C), and select counters.

### Thresholds & alerts
nOS evaluates SMART summaries against tunable thresholds and surfaces alerts in the UI (topbar bell):

- Temperature: warn at 60°C, critical at 70°C (defaults)
- Reallocated sectors: warn when >= 1
- Media errors (NVMe): warn when >= 1
- SMART overall health: if failed → critical

Configuration file: `/etc/nos/health.yaml` (JSON also supported at `/etc/nos/health.json`). Example JSON:

```
{
  "smart": { "tempWarn": 60, "tempCrit": 70, "reallocatedWarn": 1, "mediaErrWarn": 1 }
}
```

Alerts are persisted to `/var/lib/nos/alerts.json` atomically. You can manually trigger a scan via `POST /api/v1/health/scan` (the UI will periodically refresh alerts). Email/webhook notifications will arrive in a later milestone.

### TRIM (SSD longevity)
nOS enables a weekly `fstrim -av` timer out of the box to issue TRIM to filesystems and devices that support it. On SSDs, periodic TRIM helps the controller recycle blocks and maintain write performance. If you use `discard=async` in your mount options, the kernel will perform TRIM asynchronously during normal operation; periodic TRIM remains safe and typically quick on modern systems, and acts as a backstop.

The last TRIM time is shown in Settings → Schedules. The service units are `nos-fstrim.service` and `nos-fstrim.timer`.

## Scrub
For Btrfs pools, a monthly scrub is recommended to detect and correct silent errors. By default, nOS schedules scrub on the first Sunday each month.

## Editing maintenance schedules
nOS runs periodic maintenance tasks to keep storage healthy. Two timers are configurable in the UI under Settings → Schedules:

- Smart Scan (SMART): weekly by default — "Sun 03:00"
- Btrfs Scrub: monthly on the first Sunday — "Sun *-*-01..07 03:00"

The schedule format is systemd OnCalendar. Inputs are validated server-side; invalid values return a typed error with hints so you can correct the expression.


