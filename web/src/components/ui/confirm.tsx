import { useEffect, useRef, useState, type ReactNode } from 'react'
import { Button } from './button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from './dialog'

/**
 * nOS's own confirmation, in place of the browser's confirm box:
 *
 *   if (!(await confirmAction({ title: 'Delete schedule nightly?', action: 'Delete schedule' }))) return
 *
 * The action button's words say what happens; a destructive action is a
 * line button like every other one. ConfirmProvider is mounted once, in
 * main.tsx.
 */
export interface ConfirmOptions {
  title: string
  description?: ReactNode
  /** The words of the button that goes ahead, e.g. "Delete snapshot". */
  action: string
  cancel?: string
}

type Ask = (options: ConfirmOptions) => Promise<boolean>

let ask: Ask | null = null

export function confirmAction(options: ConfirmOptions): Promise<boolean> {
  if (!ask) return Promise.reject(new Error('ConfirmProvider is not mounted'))
  return ask(options)
}

export function ConfirmProvider() {
  const [options, setOptions] = useState<ConfirmOptions | null>(null)
  const resolver = useRef<((ok: boolean) => void) | null>(null)

  useEffect(() => {
    ask = (o) =>
      new Promise<boolean>((resolve) => {
        resolver.current?.(false)
        resolver.current = resolve
        setOptions(o)
      })
    return () => {
      ask = null
    }
  }, [])

  const close = (ok: boolean) => {
    resolver.current?.(ok)
    resolver.current = null
    setOptions(null)
  }

  return (
    <Dialog open={options !== null} onOpenChange={(open) => !open && close(false)}>
      {options && (
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{options.title}</DialogTitle>
            {options.description && <DialogDescription>{options.description}</DialogDescription>}
          </DialogHeader>
          <DialogFooter className="gap-2">
            <Button variant="outline" onClick={() => close(false)} autoFocus>
              {options.cancel ?? 'Cancel'}
            </Button>
            <Button variant="destructive" onClick={() => close(true)}>
              {options.action}
            </Button>
          </DialogFooter>
        </DialogContent>
      )}
    </Dialog>
  )
}
