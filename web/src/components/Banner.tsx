
type BannerProps = {
  variant?: 'error' | 'info' | 'success'
  title: string
  message?: string
  action?: { label: string; to: string }
}

export default function Banner({ variant = 'info', title, message, action }: BannerProps) {
  const cls = variant === 'error'
    ? 'bg-status-failed-tint border-status-failed text-ink'
    : variant === 'success'
    ? 'bg-status-ok-tint border-status-ok text-ink'
    : 'border-hairline text-ink'
  return (
    <div className={`w-full border-b ${cls}`}>
      <div className="mx-auto max-w-6xl px-4 py-2 text-sm">
        <div className="flex items-center justify-between gap-4">
          <div>
            <div className="font-medium">{title}</div>
            {message && <div className="opacity-80">{message}</div>}
          </div>
          {action && (
            <a href={action.to} className="btn btn-line whitespace-nowrap text-xs">
              {action.label}
            </a>
          )}
        </div>
      </div>
    </div>
  )
}


