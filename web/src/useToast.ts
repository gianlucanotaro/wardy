import { createContext, useContext } from 'react'

// Lives apart from Toast.tsx so that file exports only a component and
// keeps Vite's fast refresh working.
export const ToastContext = createContext<(message: string) => void>(() => {})

// Returns flash(message): shows the app-wide toast (see ToastProvider).
export function useToast() {
  return useContext(ToastContext)
}
