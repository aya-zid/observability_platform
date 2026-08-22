import { create } from "zustand"

export type AuthStatus = "unknown" | "authenticated" | "unauthenticated" | "inactive" | "subscription_expired" | "account_suspended"

type AuthState = {
  status: AuthStatus
  apiKey: string | null
  role: string | null
  setStatus: (status: AuthStatus) => void
  setApiKey: (apiKey: string | null) => void
  setRole: (role: string | null) => void
  clearSession: () => void
}

export const useAuthStore = create<AuthState>((set) => ({
  status: "unknown",
  apiKey: null,
  role: null,
  setStatus: (status) => set({ status }),
  setApiKey: (apiKey) => set({ apiKey }),
  setRole: (role) => set({ role }),
  clearSession: () => {
    set({ status: "unauthenticated", apiKey: null, role: null })
  },
}))
