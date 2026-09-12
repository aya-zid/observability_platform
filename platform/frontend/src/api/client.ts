import axios, { type AxiosError, type AxiosInstance } from "axios"

import { useAuthStore } from "@/store/authStore"

type RetriableConfig = {
  _retry?: boolean
  url?: string
} & NonNullable<Parameters<AxiosInstance["request"]>[0]>

const API_BASE_URL =
  import.meta.env.VITE_API_URL?.toString() ?? "http://localhost:8080"

const client = axios.create({
  baseURL: API_BASE_URL,
  withCredentials: true, // cookie-based auth: access_token + refresh_token
  headers: { "Content-Type": "application/json" },
  timeout: 15000, // 15s timeout so mutations don't hang forever
})

function isAuthEndpoint(url?: string) {
  if (!url) return false
  return (
    url.includes("/login") ||
    url.includes("/signup") ||
    url.includes("/refresh") ||
    url.includes("/logout")
  )
}

// Auto-refresh when access token expires (401), then replay the original request.
let isRefreshing = false
let queue: Array<{
  resolve: () => void
  reject: (e: unknown) => void
}> = []

client.interceptors.response.use(
  (res) => res,
  async (error: AxiosError) => {
    const status = error.response?.status
    const orig = (error.config ?? {}) as RetriableConfig

    // A deactivated user is rejected on every call. Transition the whole app
    // to the "inactive" screen regardless of where the error occurred.
    if (status === 403 && apiErrorCode(error) === ERROR_CODES.USER_DEACTIVATED) {
      useAuthStore.getState().setStatus("inactive")
    }

    if (status === 403 && apiErrorCode(error) === ERROR_CODES.SUBSCRIPTION_EXPIRED) {
      useAuthStore.getState().setStatus("subscription_expired")
    }

    if (status === 403 && apiErrorCode(error) === ERROR_CODES.ACCOUNT_SUSPENDED) {
      useAuthStore.getState().setStatus("account_suspended")
    }

    if (status === 403 && apiErrorCode(error) === ERROR_CODES.TENANT_BANNED) {
      useAuthStore.getState().setStatus("account_suspended")
    }

    if (
      status === 401 &&
      !orig._retry &&
      !isAuthEndpoint(orig.url) // never refresh in a refresh loop
    ) {
      if (isRefreshing) {
        return new Promise<void>((resolve, reject) => queue.push({ resolve, reject }))
          .then(() => client.request(orig))
          .catch((e) => Promise.reject(e))
      }

      orig._retry = true
      isRefreshing = true

      try {
        await client.post("/refresh")
        queue.forEach((p) => p.resolve())
        queue = []
        return client.request(orig)
      } catch (e) {
        queue.forEach((p) => p.reject(e))
        queue = []
        return Promise.reject(e)
      } finally {
        isRefreshing = false
      }
    }

    return Promise.reject(error)
  }
)

export default client

// apiErrorCode extracts the backend error code (e.g. "USER_DEACTIVATED")
// from an axios error, when present.
export function apiErrorCode(error: unknown): string | undefined {
  if (axios.isAxiosError(error)) {
    const code = (error.response?.data as { code?: string } | undefined)?.code
    if (typeof code === "string" && code.length > 0) return code
  }
  return undefined
}

export const ERROR_CODES = {
  USER_DEACTIVATED: "USER_DEACTIVATED",
  SUBSCRIPTION_EXPIRED: "SUBSCRIPTION_EXPIRED",
  ACCOUNT_SUSPENDED: "ACCOUNT_SUSPENDED",
  TENANT_BANNED: "TENANT_BANNED",
} as const
