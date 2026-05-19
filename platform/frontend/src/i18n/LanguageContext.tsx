import { createContext, useContext, useState, type ReactNode } from "react"
import { translations, type Lang } from "./translations"

interface LanguageContextType {
  lang: Lang
  setLang: (lang: Lang) => void
  t: (key: string, ...args: any[]) => string
}

const LanguageContext = createContext<LanguageContextType | null>(null)

export function LanguageProvider({ children }: { children: ReactNode }) {
  const [lang, setLang] = useState<Lang>("en")

  function t(key: string, ...args: any[]): string {
    const entry = translations[key]
    if (!entry) return key
    const value = entry[lang]
    if (typeof value === "function") return value(...args)
    return value
  }

  return (
    <LanguageContext.Provider value={{ lang, setLang, t }}>
      {children}
    </LanguageContext.Provider>
  )
}

export function useTranslation() {
  const ctx = useContext(LanguageContext)
  if (!ctx) throw new Error("useTranslation must be used within LanguageProvider")
  return ctx
}

export function LangToggle() {
  const { lang, setLang } = useTranslation()
  return (
    <button
      type="button"
      onClick={() => setLang(lang === "en" ? "fr" : "en")}
      className="rounded-md border border-border px-2 py-1 text-xs font-medium hover:bg-muted transition-colors"
      title={lang === "en" ? "Switch to French" : "Passer en anglais"}
    >
      {lang === "en" ? "FR" : "EN"}
    </button>
  )
}
