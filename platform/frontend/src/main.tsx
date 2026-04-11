import { StrictMode } from "react"
import { createRoot } from "react-dom/client"

import App from "@/App"
import { initThemeFromStorage } from "@/store/themeStore"

import "./index.css"
import "@/styles/shell.css"
import "@/styles/auth.css"

// Apply theme before first render to avoid a flash.
initThemeFromStorage()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
