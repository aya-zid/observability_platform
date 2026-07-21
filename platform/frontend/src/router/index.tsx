import { Navigate, createBrowserRouter } from "react-router-dom"

import AppLayout from "@/components/layout/AppLayout"
import LoginPage from "@/pages/auth/LoginPage"
import SignupPage from "@/pages/auth/SignupPage"
import AcceptInvitePage from "@/pages/auth/AcceptInvitePage"
import AlertsPage from "@/pages/alerts/AlertsPage"
import ServicesPage from "@/pages/service/ServicesPage"
import ServiceConfigPage from "@/pages/service/ServiceConfigPage"
import { NginxDashboardPage } from "@/pages/service/nginx/NginxDashboardPage"
import { NginxLogsPage } from "@/pages/service/nginx/NginxLogsPage"
import { NginxSlowRequestsPage } from "@/pages/service/nginx/NginxSlowRequestsPage"
import { ApacheDashboardPage } from "@/pages/service/apache/ApacheDashboardPage"
import { ApacheLogsPage } from "@/pages/service/apache/ApacheLogsPage"
import { ApacheSlowRequestsPage } from "@/pages/service/apache/ApacheSlowRequestsPage"
import { PostgresDashboardPage } from "@/pages/service/postgresql/PostgresDashboardPage"
import { PostgresConfigPage } from "@/pages/service/postgresql/PostgresConfigPage"
import { PostgresLogsPage } from "@/pages/service/postgresql/PostgresLogsPage"
import { PHPFPMDashboardPage } from "@/pages/service/phpfpm/PHPFPMDashboardPage"
import { PHPFPMLogsPage } from "@/pages/service/phpfpm/PHPFPMLogsPage"
import { RedisDashboardPage } from "@/pages/service/redis/RedisDashboardPage"
import { RedisLogsPage } from "@/pages/service/redis/RedisLogsPage"
import { MariaDBDashboardPage } from "@/pages/service/mariadb/MariaDBDashboardPage"
import { MariaDBLogsPage } from "@/pages/service/mariadb/MariaDBLogsPage"
import HostsPage from "@/pages/hosts/HostsPage"
import HostDetailPage from "@/pages/hosts/HostDetailPage"
import ContainersPage from "@/pages/container/ContainersPage"
import { ContainerDashboardPage } from "@/pages/container/ContainerDashboardPage"
import { ContainerLogsPage } from "@/pages/container/ContainerLogsPage"
import { HostLogsPage } from "@/pages/hosts/HostLogsPage"
import OverviewPage from "@/pages/overview/OverviewPage"
import ProfilePage from "@/pages/profile/ProfilePage"
import SettingsLayout from "@/pages/settings/SettingsLayout"
import OrgSettingsPage from "@/pages/settings/OrgSettingsPage"
import TeamPage from "@/pages/settings/TeamPage"
import ApiKeysPage from "@/pages/settings/ApiKeysPage"
import NotificationsPage from "@/pages/settings/NotificationsPage"
import BillingPage from "@/pages/settings/BillingPage"
import WelcomePage from "@/pages/welcome/WelcomePage"
import { ProtectedRoute, PublicRoute } from "@/router/guards"

export const router = createBrowserRouter([
  {
    element: <PublicRoute />,
    children: [
      { path: "/", element: <WelcomePage /> },
      { path: "/login", element: <LoginPage /> },
      { path: "/signup", element: <SignupPage /> },
      { path: "/invite/:token", element: <AcceptInvitePage /> },
    ],
  },
  {
    element: <ProtectedRoute />,
    children: [
      {
        element: <AppLayout />,
        children: [
          { index: true, element: <Navigate to="/overview" replace /> },
          { path: "/overview", element: <OverviewPage /> },
          { path: "/alerts", element: <AlertsPage /> },
          { path: "/hosts", element: <HostsPage /> },
          { path: "/hosts/:machineId", element: <HostDetailPage /> },
          { path: "/hosts/:machineId/logs", element: <HostLogsPage /> },
          { path: "/containers", element: <ContainersPage /> },
          { path: "/containers/:id/dashboard", element: <ContainerDashboardPage /> },
          { path: "/containers/:id/logs", element: <ContainerLogsPage /> },
          { path: "/services", element: <ServicesPage /> },
          { path: "/services/:id", element: <ServiceConfigPage /> },
          { path: "/services/nginx-dashboard/:id", element: <NginxDashboardPage /> },
          { path: "/services/nginx-dashboard/:id/slow-requests", element: <NginxSlowRequestsPage /> },
          { path: "/services/nginx-logs/:id", element: <NginxLogsPage /> },
          { path: "/services/apache-dashboard/:id", element: <ApacheDashboardPage /> },
          { path: "/services/apache-dashboard/:id/slow-requests", element: <ApacheSlowRequestsPage /> },
          { path: "/services/apache-logs/:id", element: <ApacheLogsPage /> },
          { path: "/services/postgres-dashboard/:id", element: <PostgresDashboardPage /> },
          { path: "/services/postgres-config/:id", element: <PostgresConfigPage /> },
          { path: "/services/postgres-logs/:id", element: <PostgresLogsPage /> },
          { path: "/services/phpfpm-dashboard/:id", element: <PHPFPMDashboardPage /> },
          { path: "/services/phpfpm-logs/:id", element: <PHPFPMLogsPage /> },
          { path: "/services/redis-dashboard/:id", element: <RedisDashboardPage /> },
          { path: "/services/redis-logs/:id", element: <RedisLogsPage /> },
          { path: "/services/mariadb-dashboard/:id", element: <MariaDBDashboardPage /> },
          { path: "/services/mariadb-logs/:id", element: <MariaDBLogsPage /> },
          { path: "/profile", element: <ProfilePage /> },
          {
            path: "/settings",
            element: <SettingsLayout />,
            children: [
              { index: true, element: <Navigate to="/settings/general" replace /> },
              { path: "general", element: <OrgSettingsPage /> },
              { path: "team", element: <TeamPage /> },
              { path: "api-keys", element: <ApiKeysPage /> },
              { path: "notifications", element: <NotificationsPage /> },
              { path: "billing", element: <BillingPage /> },
            ],
          },
        ],
      },
    ],
  },
  { path: "*", element: <Navigate to="/" replace /> },
])
