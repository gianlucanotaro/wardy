import { useEffect, useState } from 'react'

type Health = { status: string }

function App() {
  const [health, setHealth] = useState<Health | 'loading' | 'error'>('loading')

  useEffect(() => {
    fetch('/healthz')
      .then((res) => res.json())
      .then((data: Health) => setHealth(data))
      .catch(() => setHealth('error'))
  }, [])

  const statusText =
    health === 'loading' ? 'checking…' : health === 'error' ? 'unreachable' : health.status

  return (
    <main className="min-h-svh flex flex-col items-center justify-center gap-2 px-4 text-center">
      <h1 className="text-2xl font-medium">Wardy</h1>
      <p className="text-sm text-gray-500">
        Backend: <span className="font-mono">{statusText}</span>
      </p>
    </main>
  )
}

export default App
