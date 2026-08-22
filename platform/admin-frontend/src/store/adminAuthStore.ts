import { create } from "zustand"

export type AuthStatus = "unknown" | "authenticated" | "unauthenticated"

type AuthState = {
  status: AuthStatus
  role: string | null
  setStatus: (status: AuthStatus) => void
  setRole: (role: string | null) => void
  clearSession: () => void
}

export const useAdminAuthStore = create<AuthState>((set) => ({
  status: "unknown",
  role: null,
  setStatus: (status) => set({ status }),
  setRole: (role) => set({ role }),
  clearSession: () => {
    set({ status: "unauthenticated", role: null })
  },
}))
