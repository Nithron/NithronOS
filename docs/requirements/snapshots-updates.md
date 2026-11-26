## Snapshots Before Updates and Rollback (v0)

### Goal
On every system or app update, take point-in-time backups so an admin can quickly roll back if something goes wrong. Prefer Btrfs read-only snapshots; if a target is not on Btrfs, fall back to a tar.gz archive. Provide a simple rollback flow per target.

### Scope (v0)
- Targets:
  - nOS config and data directories (for example: /etc/nos and /srv/nos-web if present).
  - App data roots that live under Btrfs pools (for example: /srv/apps/*). Each app directory is treated as an individual target when possible.
  - Optional pool subvolumes the user selects in the UI (admin chooses which subvolumes are protected during updates).
- Out of scope for v0:
  - Root filesystem rollback (requires boot integration and stable root-on-Btrfs). Consider only if root is Btrfs in a later milestone.

### Safety and Operational Rules
- Quiesce I/O:
  - Stop or pause services that write to a target before snapshotting or rolling back. For app targets this implies stopping the relevant Compose stack or containers.
- Confirmation:
  - All apply and rollback operations require a Confirm: yes header. No destructive or state-changing operation proceeds without it.
- Authentication:
  - Two-factor authentication is strongly recommended for these operations in v0, but not strictly required. Future milestones may enforce 2FA based on system policy.

### Snapshot and Archive Behavior
- Preferred path: Btrfs read-only snapshots created atomically for any target that is a Btrfs subvolume.
- Fallback path: tar.gz archive for targets that are not on Btrfs; write to a durable location on a Btrfs pool if possible, otherwise to a configured staging directory.
- Naming and metadata:
  - Each snapshot/archive gets a unique ID and metadata including timestamp, target path, type (btrfs or tar), and optional notes.

### Artifacts and Retention
- Index storage:
  - Store snapshot index in /var/lib/nos/snapshots/index.json. The index contains entries for each target and its snapshot history with IDs, timestamps, type, and paths.
- Retention policy:
  - Keep the last N = 5 snapshots per target by default. Older snapshots are pruned in FIFO order. Retention should be configurable in a later milestone.

### Rollback
- For Btrfs snapshots: perform a safe rollback by replacing the live subvolume with the selected snapshot (or by send/receive into a staging subvolume and swapping mount points).
- For tar.gz archives: stop the service, restore files into the target directory, fix ownership and permissions, then restart the service.
- Always restart services after rollback and verify basic health.

### Observability and Audit
- Log every snapshot and rollback with target path, snapshot ID, actor, and result.
- Expose APIs to list snapshots per target, trigger snapshot, and perform rollback.
- Surface results and errors clearly in the UI with explicit confirmations.

### Acceptance Criteria
- A snapshot is taken automatically before any system or app update touching a protected target.
- If a target is on Btrfs, a read-only snapshot is created; otherwise a tar.gz fallback is created.
- All snapshot and rollback actions require a Confirm: yes header.
- Admin can list available snapshots per target and initiate a rollback successfully.
- After rollback, the associated services are restarted and the system indicates success or error.
- Index at /var/lib/nos/snapshots/index.json is updated consistently and prunes entries beyond the last 5 per target.

## Optional follow-ups (post-v0)
- **Rootfs rollback on Btrfs:** integrate with GRUB and maintain `@`/`@home` snapshot pairs; boot entry for previous snapshot.
- **Per-app hooks:** pre/post update hooks to stop only the affected containers/services, not all docker apps.
- **Advanced retention:** prune snapshots by age and/or total size caps (in addition to “keep N”).

## Post-v0 follow-ups
- Rootfs rollback on Btrfs (GRUB integration, @ snapshot pair)
- Per-app pre/post hooks to stop only affected containers/services
- Retention by age/size in addition to "keep N"

