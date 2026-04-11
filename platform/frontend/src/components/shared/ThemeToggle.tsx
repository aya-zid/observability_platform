import { Moon, Sun } from "lucide-react"

import { Button } from "@/components/ui/button"
import { useThemeStore } from "@/store/themeStore"

export default function ThemeToggle({
  variant = "outline",
}: {
  variant?: "default" | "outline" | "secondary" | "ghost" | "destructive" | "link"
}) {
  const theme = useThemeStore((s) => s.theme)
  const toggle = useThemeStore((s) => s.toggle)

  return (
    <Button variant={variant} size="icon" onClick={toggle} aria-label="Toggle theme">
      {theme === "dark" ? <Moon /> : <Sun />}
    </Button>
  )
}

