import { Outlet } from "react-router-dom"

import Sidebar from "@/components/layout/Sidebar"
import TopBar from "@/components/layout/TopBar"

export default function AppLayout() {
  return (
    <div className="shell shell-grid min-h-dvh grid grid-cols-1 md:grid-cols-[260px_1fr]">
      <Sidebar />
      <div className="min-w-0 relative">
        <TopBar />
        <main className="relative p-4 md:p-6">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
