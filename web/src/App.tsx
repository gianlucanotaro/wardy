import { useQuery, useQueryClient } from '@tanstack/react-query'
import { getMe } from './api'
import LoginForm from './LoginForm'
import Shell from './Shell'

function App() {
  const queryClient = useQueryClient()
  const me = useQuery({ queryKey: ['me'], queryFn: getMe, retry: false })

  if (me.isLoading) {
    return <main className="min-h-svh flex items-center justify-center wy-lbl">LOADING…</main>
  }

  const refetchMe = () => queryClient.invalidateQueries({ queryKey: ['me'] })

  if (me.isError || !me.data) {
    return <LoginForm onSuccess={refetchMe} />
  }

  return <Shell me={me.data} onLogout={refetchMe} />
}

export default App
