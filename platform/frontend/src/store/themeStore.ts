import { create } from "zustand"
import { persist } from "zustand/middleware"

export type Theme = "dark" | "light"

type ThemeState = {
  theme: Theme
  setTheme: (theme: Theme) => void
  toggle: () => void
}

function applyThemeToDocument(theme: Theme) {
  document.documentElement.classList.toggle("dark", theme === "dark")
}

export const useThemeStore = create<ThemeState>()(
  persist(
    (set, get) => ({
      theme: "dark",
      setTheme: (theme) => {
        applyThemeToDocument(theme)
        set({ theme })
      },
      toggle: () => {
        const next: Theme = get().theme === "dark" ? "light" : "dark"
        applyThemeToDocument(next)
        set({ theme: next })
      },
    }),
    { name: "theme" }
  )
)

export function initThemeFromStorage() {
  try {
    const raw = localStorage.getItem("theme")
    if (!raw) {
      applyThemeToDocument("dark")
      return
    }
    const parsed = JSON.parse(raw) as { state?: { theme?: Theme } } | Theme
    const theme =
      typeof parsed === "string" ? parsed : (parsed.state?.theme ?? "dark")
    applyThemeToDocument(theme)
  } catch {
    applyThemeToDocument("dark")
  }
}

