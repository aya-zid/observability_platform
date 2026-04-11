import { create } from "zustand"

export type AuthStatus = "unknown" | "authenticated" | "unauthenticated"

type AuthState = {
  status: AuthStatus
  apiKey: string | null
  setStatus: (status: AuthStatus) => void
  setApiKey: (apiKey: string | null) => void
  clearSession: () => void
}

export const useAuthStore = create<AuthState>((set) => ({
  status: "unknown",
  apiKey: null,
  setStatus: (status) => set({ status }),
  setApiKey: (apiKey) => set({ apiKey }),
  clearSession: () => {
    set({ status: "unauthenticated", apiKey: null })
  },
}))
