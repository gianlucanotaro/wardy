import { useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { login } from './api'

export default function LoginForm({ onSuccess }: { onSuccess: () => void }) {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const mutation = useMutation({
    mutationFn: () => login(email, password),
    onSuccess,
  })

  return (
    <main className="min-h-svh flex items-center justify-center px-4">
      <form
        className="w-full max-w-xs flex flex-col gap-3"
        onSubmit={(e) => {
          e.preventDefault()
          mutation.mutate()
        }}
      >
        <h1 className="text-xl font-medium text-center mb-2">Wardy</h1>
        <label className="flex flex-col gap-1 text-sm" htmlFor="email">
          Email
          <input
            id="email"
            type="email"
            required
            autoComplete="email"
            className="border rounded px-3 py-2"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
        </label>
        <label className="flex flex-col gap-1 text-sm" htmlFor="password">
          Password
          <input
            id="password"
            type="password"
            required
            autoComplete="current-password"
            className="border rounded px-3 py-2"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </label>
        <button
          type="submit"
          disabled={mutation.isPending}
          className="bg-black text-white rounded px-3 py-2 mt-2 disabled:opacity-50"
        >
          {mutation.isPending ? 'Signing in…' : 'Sign in'}
        </button>
        {mutation.isError && <p className="text-sm text-red-600">Invalid email or password.</p>}
      </form>
    </main>
  )
}
