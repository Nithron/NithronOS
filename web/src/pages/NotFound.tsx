import { useLocation, useNavigate } from 'react-router-dom'
import { Button } from '@/components/ui/button'

/** A route nOS doesn't have: say which, and offer the way back. */
export function NotFound() {
  const navigate = useNavigate()
  const { pathname } = useLocation()

  return (
    <div className="mx-auto max-w-xl py-16">
      <h1 className="screen-title">Page not found</h1>
      <p className="mt-4 text-ink-2">
        nOS has no page at <span className="font-mono text-ink">{pathname}</span>.
      </p>
      <div className="mt-8 flex gap-3">
        <Button onClick={() => navigate('/')}>Go to the dashboard</Button>
        <Button variant="outline" onClick={() => navigate(-1)}>
          Go back
        </Button>
      </div>
    </div>
  )
}
