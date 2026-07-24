import { Outlet } from "react-router-dom"

import Sidebar from "@/components/layout/Sidebar"
import TopBar from "@/components/layout/TopBar"

export default function AppLayout() {
  return (
    <div className="shell shell-grid h-screen overflow-hidden">
      <div className="flex h-full">
        <Sidebar />
        <div className="flex flex-col flex-1 min-w-0">
          <TopBar />
          <main className="flex-1 overflow-y-auto p-4 md:p-6">
            <Outlet />
          </main>
        </div>
      </div>
    </div>
  )
}
