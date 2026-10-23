import axios, { type AxiosError } from "axios"

export interface ApiResponse<T> {
  code: number
  msg: string
  data: T
}

export const apiClient = axios.create({
  baseURL: "/api/v1/admin",
  withCredentials: true,
  timeout: 30 * 60 * 1000,
})

let csrfToken = ""

export function setCsrfToken(value: string) {
  csrfToken = value
}

export function getCsrfToken() {
  return csrfToken
}

apiClient.interceptors.request.use((config) => {
  if (csrfToken && config.method && !["get", "head", "options"].includes(config.method.toLowerCase())) {
    config.headers.set("X-CSRF-Token", csrfToken)
  }
  return config
})

apiClient.interceptors.response.use(
  (response) => {
    const payload = response.data as ApiResponse<unknown> | undefined
    if (payload && typeof payload.code === "number" && payload.code !== 200) {
      return Promise.reject({
        response,
        message: payload.msg || "请求失败",
      })
    }
    return response
  },
  (error) => Promise.reject(error),
)

export function errorMessage(error: unknown) {
  const axiosError = error as AxiosError<ApiResponse<unknown>>
  return axiosError.response?.data?.msg || axiosError.message || "请求失败"
}
