import { Toaster, toast as hotToast } from 'react-hot-toast'
import { FailedIcon, NoticeIcon, OkIcon, WarningIcon, CloseIcon, type IconComponent } from '@/components/icons'

/**
 * The one toast of nOS: a mark and one sentence that says what happened.
 * "Done" leaves after about five seconds; a failure stays until it is
 * dismissed (and is also shown where it happened). react-hot-toast only
 * places and times them; ToastProvider is mounted once, in main.tsx.
 */
export const ToastProvider = () => <Toaster position="top-right" gutter={8} />

/** Kept for the shell, which used to render toasts itself. */
export const Toasts = () => null

type Kind = 'success' | 'error' | 'info' | 'warning'

interface ToastOptions {
  /** A second line: what went wrong, or what to do. */
  description?: string
}

const kinds: Record<Kind, { icon: IconComponent; tone: string; duration: number }> = {
  success: { icon: OkIcon, tone: 'text-status-ok', duration: 5000 },
  info: { icon: NoticeIcon, tone: 'text-ink', duration: 5000 },
  warning: { icon: WarningIcon, tone: 'text-status-warning', duration: 8000 },
  error: { icon: FailedIcon, tone: 'text-status-failed', duration: Infinity },
}

function show(kind: Kind, message: string, options?: ToastOptions): void {
  const { icon: Icon, tone, duration } = kinds[kind]
  hotToast.custom(
    (t) => (
      <div
        role={kind === 'error' ? 'alert' : 'status'}
        className="flex max-w-sm items-start gap-3 border border-edge bg-page px-4 py-3 text-ink"
      >
        <Icon className={`mt-0.5 h-4 w-4 shrink-0 ${tone}`} />
        <div className="flex-1 text-sm">
          <p>{message}</p>
          {options?.description && <p className="mt-1 text-ink-2">{options.description}</p>}
        </div>
        <button
          type="button"
          onClick={() => hotToast.dismiss(t.id)}
          className="text-ink-2 hover:text-ink focus-visible:outline focus-visible:outline-2 focus-visible:outline-ink"
          aria-label="Dismiss"
        >
          <CloseIcon className="h-4 w-4" />
        </button>
      </div>
    ),
    { duration },
  )
}

export const toast = {
  success: (message: string, options?: ToastOptions) => show('success', message, options),
  error: (message: string, options?: ToastOptions) => show('error', message, options),
  info: (message: string, options?: ToastOptions) => show('info', message, options),
  warning: (message: string, options?: ToastOptions) => show('warning', message, options),
}

// Compatibility wrapper for existing code - a real function
export function pushToast(message: string, type: Kind = 'success'): void {
  toast[type](message)
}
