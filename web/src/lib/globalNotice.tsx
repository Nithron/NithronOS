import { createContext, useContext, useEffect, useMemo, useRef, useState } from 'react'
import http, { ProxyMisconfiguredError } from '@/lib/nos-client'

export type GlobalNotice = {
  kind: 'error' | 'info' | 'success'
  title: string
  message?: string
  action?: { label: string; to: string }
}

type Ctx = {
  notice: GlobalNotice | null
  setNotice: (n: GlobalNotice | null) => void
}

const Ctx = createContext<Ctx | null>(null)

export function GlobalNoticeProvider({ children }: { children: React.ReactNode }) {
  const [notice, setNotice] = useState<GlobalNotice | null>(null)
  const stopRef = useRef(false)

  // Health checker: ask /api/v1/setup/state every 10 s and on window focus
  useEffect(() => {
    let timeout: any
    async function check() {
      try {
        await http.setup.getState()
        if (!stopRef.current) setNotice((n) => (n && n.title.includes('Backend unreachable') ? null : n))
      } catch (e: any) {
        if (e instanceof ProxyMisconfiguredError) {
          if (!stopRef.current) setNotice({
            kind: 'error',
            title: 'Backend unreachable or proxy misconfigured',
            message: 'The server didn’t return JSON. This usually means the reverse proxy is serving HTML at /api/* (try Caddy config).',
            action: { label: 'Troubleshooting', to: '/help/proxy' },
          })
        }
      } finally {
        if (!stopRef.current) timeout = setTimeout(check, 10000)
      }
    }
    check()
    const onFocus = () => { check() }
    window.addEventListener('focus', onFocus)
    return () => { stopRef.current = true; clearTimeout(timeout); window.removeEventListener('focus', onFocus) }
  }, [])

  const value = useMemo(() => ({ notice, setNotice }), [notice])
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>
}

export function useGlobalNotice() {
  const ctx = useContext(Ctx)
  if (!ctx) throw new Error('useGlobalNotice must be used within GlobalNoticeProvider')
  return ctx
}


