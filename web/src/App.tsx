import { useQuery, useQueryClient } from '@tanstack/react-query'
import { getMe } from './api'
import LoginForm from './LoginForm'
import Wardrobe from './Wardrobe'

function App() {
  const queryClient = useQueryClient()
  const me = useQuery({ queryKey: ['me'], queryFn: getMe, retry: false })

  if (me.isLoading) {
    return (
      <main className="min-h-svh flex items-center justify-center text-sm text-gray-500">
        Loading…
      </main>
    )
  }

  const refetchMe = () => queryClient.invalidateQueries({ queryKey: ['me'] })

  if (me.isError || !me.data) {
    return <LoginForm onSuccess={refetchMe} />
  }

  return <Wardrobe me={me.data} onLogout={refetchMe} />
}

export default App
