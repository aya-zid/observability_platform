import axios, { type AxiosError, type AxiosInstance } from "axios"
import { useAdminAuthStore } from "@/store/adminAuthStore"

type RetriableConfig = {
  _retry?: boolean
  url?: string
} & NonNullable<Parameters<AxiosInstance["request"]>[0]>

const API_BASE_URL =
  import.meta.env.VITE_API_URL?.toString() ?? "http://localhost:8080"

const client = axios.create({
  baseURL: `${API_BASE_URL}/admin/api`,
  withCredentials: true,
  headers: { "Content-Type": "application/json" },
  timeout: 15000,
})

function isAuthEndpoint(url?: string) {
  if (!url) return false
  return url.includes("/auth/login") || url.includes("/auth/signup") || url.includes("/auth/refresh") || url.includes("/auth/logout")
}

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

    if (
      status === 401 &&
      !orig._retry &&
      !isAuthEndpoint(orig.url)
    ) {
      if (isRefreshing) {
        return new Promise<void>((resolve, reject) => queue.push({ resolve, reject }))
          .then(() => client.request(orig))
          .catch((e) => Promise.reject(e))
      }

      orig._retry = true
      isRefreshing = true

      try {
        await client.post("/auth/refresh")
        queue.forEach((p) => p.resolve())
        queue = []
        return client.request(orig)
      } catch (e) {
        queue.forEach((p) => p.reject(e))
        queue = []
        useAdminAuthStore.getState().clearSession()
        return Promise.reject(e)
      } finally {
        isRefreshing = false
      }
    }

    return Promise.reject(error)
  }
)

export default client
