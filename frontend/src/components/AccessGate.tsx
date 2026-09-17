import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react'
import { createAuthSession, deleteAuthSession, getAuthStatus, UNAUTHORIZED_EVENT } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Field, FieldDescription, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'

type GateState = 'checking' | 'open' | 'locked' | 'unreachable'

interface AccessContextValue {
  /** required is true when this installation runs with LAN_AUTH_TOKEN set. */
  required: boolean
  /** forget clears this device's access cookie and shows the token screen. */
  forget: () => Promise<void>
}

const AccessContext = createContext<AccessContextValue>({ required: false, forget: async () => {} })

// oxlint-disable-next-line react/only-export-components -- the hook belongs with its provider
export function useAccess(): AccessContextValue {
  return useContext(AccessContext)
}

// AccessGate stands between the app and a backend running in token mode
// (issue #181). In default mode the status check answers "not required"
// and the app renders as if the gate were not there. In token mode a device
// without access sees the token screen instead, enters the token once, and
// the backend's HttpOnly cookie carries access from then on — for fetches
// and for the PDF links and logo images a header could never reach. The
// URL is left untouched, so whatever page was opened (a shared link, a deep
// link) renders once access is granted.
export default function AccessGate({ children }: { children: ReactNode }) {
  const [state, setState] = useState<GateState>('checking')
  const [required, setRequired] = useState(false)

  const check = useCallback(() => {
    getAuthStatus()
      .then((status) => {
        setRequired(status.required)
        setState(!status.required || status.authenticated ? 'open' : 'locked')
      })
      .catch(() => setState('unreachable'))
  }, [])

  useEffect(() => {
    check()
    const onUnauthorized = () => setState('locked')
    window.addEventListener(UNAUTHORIZED_EVENT, onUnauthorized)
    return () => window.removeEventListener(UNAUTHORIZED_EVENT, onUnauthorized)
  }, [check])

  const forget = useCallback(async () => {
    await deleteAuthSession()
    setState('locked')
  }, [])

  if (state === 'checking') return null
  if (state === 'unreachable') {
    return (
      <GateFrame>
        <p role="alert" className="font-medium text-destructive">
          Can&apos;t reach the Sumisura backend.
        </p>
        <Button type="button" className="mt-4" onClick={check}>
          Try again
        </Button>
      </GateFrame>
    )
  }
  if (state === 'locked') {
    return <AccessTokenForm onGranted={() => setState('open')} />
  }
  return <AccessContext.Provider value={{ required, forget }}>{children}</AccessContext.Provider>
}

function GateFrame({ children }: { children: ReactNode }) {
  return (
    <main className="mx-auto max-w-[420px] px-6 pt-16 pb-12 sm:px-4">
      <img src="/logo-lockup.svg" alt="Sumisura" className="mb-8 h-8 w-auto dark:hidden" />
      <img src="/logo-lockup-dark.svg" alt="" aria-hidden className="mb-8 hidden h-8 w-auto dark:block" />
      {children}
    </main>
  )
}

function AccessTokenForm({ onGranted }: { onGranted: () => void }) {
  const [token, setToken] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      await createAuthSession(token.trim())
      onGranted()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <GateFrame>
      <h1>Enter access token</h1>
      <form onSubmit={handleSubmit}>
        {error && (
          <p role="alert" className="mb-4 font-medium text-destructive">
            {error}
          </p>
        )}
        <FieldGroup>
          <Field>
            <FieldLabel htmlFor="access-token">Access token</FieldLabel>
            <Input
              id="access-token"
              type="password"
              autoComplete="current-password"
              value={token}
              onChange={(e) => setToken(e.target.value)}
              required
            />
            <FieldDescription>
              The <code>LAN_AUTH_TOKEN</code> value from this installation&apos;s <code>.env</code>. You enter it
              once per device.
            </FieldDescription>
          </Field>
        </FieldGroup>
        <Button type="submit" className="mt-6" disabled={submitting || token.trim() === ''}>
          {submitting ? 'Checking…' : 'Continue'}
        </Button>
      </form>
    </GateFrame>
  )
}

/** ForgetDeviceButton renders only in token mode. */
export function ForgetDeviceButton() {
  const { required, forget } = useAccess()
  if (!required) return null
  return (
    <button
      type="button"
      className="text-xs text-muted-foreground underline-offset-2 hover:text-foreground hover:underline"
      onClick={() => {
        forget().catch(() => {
          // The cookie is HttpOnly, so only the backend can clear it; if that
          // failed there is nothing more the page can do.
        })
      }}
    >
      Forget this device
    </button>
  )
}
