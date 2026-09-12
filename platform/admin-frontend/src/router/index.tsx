import { createBrowserRouter } from "react-router-dom"

import { AdminLayout } from "@/components/layout/AdminLayout"
import LoginPage from "@/pages/auth/LoginPage"
import SignupPage from "@/pages/auth/SignupPage"
import DashboardPage from "@/pages/dashboard/DashboardPage"
import SubscriptionsPage from "@/pages/subscriptions/SubscriptionsPage"
import TenantsPage from "@/pages/tenants/TenantsPage"
import TenantDetailPage from "@/pages/tenants/TenantDetailPage"
import PacksPage from "@/pages/packs/PacksPage"
import SettingsPage from "@/pages/settings/SettingsPage"
import { ProtectedRoute, PublicRoute } from "@/router/guards"

export const router = createBrowserRouter([
  {
    element: <PublicRoute />,
    children: [
      { path: "/login", element: <LoginPage /> },
      { path: "/signup", element: <SignupPage /> },
    ],
  },
  {
    element: <ProtectedRoute />,
    children: [
      {
        element: <AdminLayout />,
        children: [
          { index: true, element: <DashboardPage /> },
          { path: "dashboard", element: <DashboardPage /> },
          { path: "subscriptions", element: <SubscriptionsPage /> },
          { path: "tenants", element: <TenantsPage /> },
          { path: "tenants/:id", element: <TenantDetailPage /> },
          { path: "packs", element: <PacksPage /> },
          { path: "settings", element: <SettingsPage /> },
        ],
      },
    ],
  },
  { path: "*", element: <LoginPage /> },
])
