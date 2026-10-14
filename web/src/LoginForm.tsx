import { useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { login } from './api'

export default function LoginForm({ onSuccess }: { onSuccess: () => void }) {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const mutation = useMutation({
    // Trimmed because phone keyboards like to append a space after
    // autocomplete; the server matches the login exactly.
    mutationFn: () => login(email.trim(), password),
    onSuccess,
  })

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
      </form>
    </main>
  )
}
