
type BannerProps = {
  variant?: 'error' | 'info' | 'success'
  title: string
  message?: string
  action?: { label: string; to: string }
}

export default function Banner({ variant = 'info', title, message, action }: BannerProps) {
  const cls = variant === 'error'
    ? 'bg-red-950/40 border-red-700 text-red-200'
    : variant === 'success'
    ? 'bg-green-950/40 border-green-700 text-green-200'
    : 'bg-blue-950/40 border-blue-700 text-blue-200'
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


