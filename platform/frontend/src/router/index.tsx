import { Navigate, createBrowserRouter } from "react-router-dom"

import AppLayout from "@/components/layout/AppLayout"
import LoginPage from "@/pages/auth/LoginPage"
import SignupPage from "@/pages/auth/SignupPage"
import AlertsPage from "@/pages/alerts/AlertsPage"
import IntegrationsPage from "@/pages/integrations/IntegrationsPage"
import IntegrationServicePage from "@/pages/integrations/IntegrationServicePage"
import { NginxDashboardPage } from "@/pages/integrations/NginxDashboardPage"
import { NginxLogsPage } from "@/pages/integrations/NginxLogsPage"
import LogsPage from "@/pages/logs/LogsPage"
import MetricsPage from "@/pages/metrics/MetricsPage"
import OverviewPage from "@/pages/overview/OverviewPage"
import ProfilePage from "@/pages/profile/ProfilePage"
import ApiKeysPage from "@/pages/settings/ApiKeysPage"
import TracesPage from "@/pages/traces/TracesPage"
import WelcomePage from "@/pages/welcome/WelcomePage"
import { ProtectedRoute, PublicRoute } from "@/router/guards"

export const router = createBrowserRouter([
  {
    element: <PublicRoute />,
    children: [
      { path: "/", element: <WelcomePage /> },
      { path: "/login", element: <LoginPage /> },
      { path: "/signup", element: <SignupPage /> },
    ],
  },
  {
    element: <ProtectedRoute />,
    children: [
      {
        element: <AppLayout />,
        children: [
          { path: "/", element: <Navigate to="/overview" replace /> },
          { path: "/overview", element: <OverviewPage /> },
          { path: "/metrics", element: <MetricsPage /> },
          { path: "/logs", element: <LogsPage /> },
          { path: "/traces", element: <TracesPage /> },
          { path: "/alerts", element: <AlertsPage /> },
          { path: "/integrations", element: <IntegrationsPage /> },
          { path: "/integrations/services/:id", element: <IntegrationServicePage /> },
          { path: "/integrations/nginx-dashboard/:id", element: <NginxDashboardPage /> },
          { path: "/integrations/nginx-logs/:id", element: <NginxLogsPage /> },
          { path: "/settings/api-keys", element: <ApiKeysPage /> },
          { path: "/profile", element: <ProfilePage /> },
        ],
      },
    ],
  },
  { path: "*", element: <Navigate to="/" replace /> },
])
