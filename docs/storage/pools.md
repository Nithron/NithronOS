# Storage pools

nOS manages data on Btrfs pools. You can create a new pool, import an existing one, and optionally enable encryption.

## Create vs Import
- Create: wipes selected devices and creates a fresh Btrfs filesystem, mounts at your chosen path, and provisions default subvolumes (`data`, `snaps`, `apps`). A plan is shown before any destructive step.
- Import: discovers existing Btrfs filesystems and mounts them without data loss. Labels and UUIDs are detected automatically.

## Defaults
- RAID profiles: if 1 device → `single`; if ≥2 devices → `raid1` (data and metadata). You can choose other safe profiles (single/raid0/raid1/raid10). `raid5/raid6` are blocked by default.
- Mount options: `noatime,compress=zstd:3`.
- Subvolumes: `data/`, `snaps/`, `apps/` created under the mount root.

## Encryption (caveats)
- LUKS2 per-device encryption can be enabled during pool creation. A key file is stored under `/etc/nos/keys/<pool>.key` with mode `0600`.
- The key file path is inserted into `crypttab` and the mapped device is used for the Btrfs filesystem.
- Important: Back up your key file securely. Without it, data is unrecoverable.
- On creation, the plan will show `cryptsetup luksFormat` and `cryptsetup open` steps; ensure you understand these are destructive to the selected devices.

## Safety & Force flags
- Devices with existing signatures are detected (via `wipefs -n`).
- Without `force`, creation is blocked if signatures are found. Set `force=true` to proceed intentionally (still shows a plan before any destructive step).

## Mount options
Recommended `btrfs` mount options by scenario:

| Scenario | Recommended options | Why |
|---|---|---|
| SSD-only | `compress=zstd:3,ssd,discard=async,noatime` | modern compression, SSD hint, async trim, fewer atime writes |
| Mixed/HDD | `compress=zstd:3,noatime` | safe on rotational arrays; avoid discard cost |
| Heavy small files | `compress=zstd:5,autodefrag,noatime` (advanced) | better compression, background defrag |

Notes:
- `discard=async` requires kernel support; periodic `fstrim.timer` is also enabled weekly by default.
- Dangerous/unsupported options are rejected (e.g., `nodatacow`).

## Device operations (add/remove/replace)
nOS supports safe device lifecycle operations using `btrfs` under the hood. The web UI (Pool Details → Devices) provides wizards to plan and apply changes.

- Planning API: `POST /api/v1/pools/{id}/plan-device`
  - Body: `{"action":"add|remove|replace","devices":{...},"targetProfile":{"data":"single|raid1","meta":"single|raid1"},"force":false}`
  - Response includes a dry-run plan with steps and warnings.
  - Safety checks:
    - Add/Replace: new devices must be known and not smaller than existing minimum (or replaced device).
    - Remove: refuses reducing RAID1 below 2 devices (and last device for `single`) unless `force`.
    - Profiles default to current if not specified.

- Apply API: `POST /api/v1/pools/{id}/apply-device`
  - Body: plan steps from the planner response.
  - Execution creates a transaction; logs are streamed under `/api/v1/pools/tx/{tx_id}/log`.
  - When a balance is started, the system polls `btrfs balance status` via the agent status endpoint and logs progress.
  - Replace operations poll `btrfs replace status` similarly.
  - Progress is sourced directly from `btrfs ... status` and may jump or lag on very full pools; this is expected behavior of upstream reporting.

Steps used internally:
- Add: `btrfs device add <devs> <mount>` then `btrfs balance start -dconvert=<profile> -mconvert=<profile> <mount>`
- Remove: `btrfs device remove <devs> <mount>`
- Replace: `btrfs replace start <old> <new> <mount>` (optionally followed by a balance)

### Add device
- Select one or more new devices not already in the pool.
- Size guidance: each new device should be at least ~90% of the smallest existing device in the pool (to ensure effective space usage). Smaller devices may be rejected by the planner.
- Profile selection: defaults to the current pool profile; you may choose `single` or `raid1` as appropriate. Switching to `single` reduces redundancy (a warning is surfaced).
- Applying will trigger a rebalance; large pools or high utilization can increase duration.

### Remove device
- Select one or more existing devices to remove.
- Redundancy checks:
  - `single`: cannot remove the last device without `force`.
  - `raid1`: cannot shrink below 2 devices without `force`.
- If removal would violate redundancy, the planner returns a typed error.

### Replace device
- Pair each old device in the pool with a new device of adequate size (new ≥ old).
- The operation starts `btrfs replace` and optionally balances afterward.
- Progress is visible in the transaction log while the replace runs.

### Rebalance
- Rebalance is automatically started when required (e.g., after adding devices or profile conversion).
- Progress is obtained from `btrfs balance status` via the agent endpoints and may update in jumps.
- Very full pools (>80% used) can experience longer rebalances; warnings are surfaced during planning.
- Limitations: live cancel is not yet exposed in the UI (coming later).

### Destroy pool
- Advanced, destructive action. Requires typing CONFIRM text in the UI.
- Safety: refused unless the mount contains only managed subvols (`data`, `snaps`, `apps`) or `--force` is set.
- Planner and apply endpoints:
  - `POST /api/v1/pools/{id}/plan-destroy` → steps to unmount, remove `fstab`/`crypttab` lines, close LUKS, optional `wipefs`.
  - `POST /api/v1/pools/{id}/apply-destroy` → executes with `confirm: "DESTROY"`.
- After success, the pool record is removed from `pools.json`.


