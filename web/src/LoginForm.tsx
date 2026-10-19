import { useEffect, useState } from 'react'
import { useMutation, useQuery } from '@tanstack/react-query'
import { getAuthProviders, login } from './api'

// Why an SSO attempt (#51) came back to the login screen — the server
// redirects to /?sso=<reason>.
const SSO_ERRORS: Record<string, string> = {
  unknown: 'That account has no Wardy login yet. Ask Gianluca to link it.',
  expired: 'Sign-in took too long or was interrupted. Try again.',
  denied: 'Sign-in was cancelled.',
  unavailable: 'The sign-in service is unreachable. Use your password for now.',
  ratelimited: 'Too many attempts. Wait a few minutes and try again.',
  failed: 'Sign-in failed. Try again, or use your password.',
}

// Query flags the login screen reads once, then drops from the address bar
// so a reload doesn't repeat them: ?sso=<reason> (an SSO attempt came back
// failed), ?signedout (just logged out) and ?password (asked for the
// password form). Any of them stops the automatic SSO redirect — otherwise
// a failure would loop and logging out would log straight back in.
function takeLoginParams(): { ssoError: string | null; stay: boolean } {
  const params = new URLSearchParams(window.location.search)
  const reason = params.get('sso')
  const stay = reason !== null || params.has('signedout') || params.has('password')
  for (const key of ['sso', 'signedout', 'password']) params.delete(key)
  const rest = params.toString()
  window.history.replaceState(null, '', window.location.pathname + (rest ? `?${rest}` : ''))
  return { ssoError: reason === null ? null : (SSO_ERRORS[reason] ?? SSO_ERRORS.failed), stay }
}

export default function LoginForm({ onSuccess }: { onSuccess: () => void }) {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [{ ssoError, stay }] = useState(takeLoginParams)
  const providers = useQuery({ queryKey: ['auth-providers'], queryFn: getAuthProviders, staleTime: Infinity })
  // With SSO on, skip this screen: the provider remembers who's logged in,
  // so one login there covers every app (#51).
  const autoSSO = !stay && providers.data?.sso_enabled === true
  useEffect(() => {
    if (autoSSO) window.location.assign('/api/auth/oidc/start')
  }, [autoSSO])
  const mutation = useMutation({
    // Trimmed because phone keyboards like to append a space after
    // autocomplete; the server matches the login exactly.
    mutationFn: () => login(email.trim(), password),
    onSuccess,
  })

  if (autoSSO || (!stay && providers.isLoading)) {
    return <main className="min-h-svh flex items-center justify-center wy-lbl">SIGNING IN…</main>
  }

  return (
    <main className="min-h-svh flex items-center justify-center px-4 bg-(--color-bg) text-(--color-ink)">
      <form
        className="w-full max-w-xs flex flex-col gap-3"
        onSubmit={(e) => {
          e.preventDefault()
          mutation.mutate()
        }}
      >
        <h1 className="font-extrabold text-3xl tracking-tight text-center mb-3">WARDY</h1>
        <label className="flex flex-col gap-1 text-sm" htmlFor="email">
          <span className="wy-lbl">EMAIL OR USERNAME</span>
          <input
            id="email"
            type="text"
            required
            autoComplete="username"
            autoCapitalize="none"
            autoCorrect="off"
            spellCheck={false}
            className="border border-(--color-line) bg-(--color-surface) px-3 py-2 focus:outline-2 focus:outline-(--color-accent)"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
        </label>
        <label className="flex flex-col gap-1 text-sm" htmlFor="password">
          <span className="wy-lbl">PASSWORD</span>
          <input
            id="password"
            type="password"
            required
            autoComplete="current-password"
            className="border border-(--color-line) bg-(--color-surface) px-3 py-2 focus:outline-2 focus:outline-(--color-accent)"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </label>
        <button type="submit" disabled={mutation.isPending} data-primary="1" className="wy-act mt-2 text-center">
          {mutation.isPending ? 'SIGNING IN…' : 'SIGN IN'}
        </button>
        {mutation.isError && <p className="text-sm text-(--color-accent-700)">Invalid login or password.</p>}
        {providers.data?.sso_enabled && (
          <>
            <div className="flex items-center gap-3 my-1 text-xs text-(--color-neutral-700)" aria-hidden="true">
              <span className="flex-1 border-t border-(--color-line)" />
              OR
              <span className="flex-1 border-t border-(--color-line)" />
            </div>
            {/* A full navigation, not fetch: the server redirects to the provider. */}
            <a href="/api/auth/oidc/start" data-outline="1" className="wy-act flex items-center">
              SIGN IN WITH {providers.data.sso_name.toUpperCase()}
            </a>
          </>
        )}
        {ssoError && <p className="text-sm text-(--color-accent-700)">{ssoError}</p>}
      </form>
    </main>
  )
}
