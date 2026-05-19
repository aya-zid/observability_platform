import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { RouterProvider } from "react-router-dom"

import AuthBootstrap from "@/components/auth/AuthBootstrap"
import { LanguageProvider } from "@/i18n/LanguageContext"
import { router } from "@/router"

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      // Always treat data as stale for real-time dashboards after browser suspend
      staleTime: 0,
      // Refetch when window regains focus (browser wakes from suspend)
      refetchOnWindowFocus: true,
      // Refetch when network reconnects
      refetchOnReconnect: true,
    },
  },
})

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <LanguageProvider>
        <AuthBootstrap>
          <RouterProvider router={router} />
        </AuthBootstrap>
      </LanguageProvider>
    </QueryClientProvider>
  )
}
