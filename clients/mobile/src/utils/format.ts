const UNITS = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
const STEP = 1024;

/** 1536 → "1.5 KB" */
export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) {
    return '0 B';
  }
  const i = Math.min(
    Math.floor(Math.log(bytes) / Math.log(STEP)),
    UNITS.length - 1,
  );
  return `${parseFloat((bytes / Math.pow(STEP, i)).toFixed(1))} ${UNITS[i]}`;
}

/** The last part of a path: "Photos/2026/a.jpg" → "a.jpg" */
export function fileName(path: string): string {
  return path.split('/').pop() || path;
}

/** "3 items", "1 item" */
export function countOf(n: number, one: string, many: string): string {
  return `${n} ${n === 1 ? one : many}`;
}
