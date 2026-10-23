import { apiClient, type ApiResponse } from "./client"

export interface UserFeedbackSummary {
  id: number
  ticketId: string
  category: string
  title: string
  platform: string
  clientType: string
  appVersion: string
  coreVersion: string
  runtimeMode: string
  osInfo: string
  contact: string
  deviceHash: string
  logLineCount: number
  errorLineCount: number
  redactedCount: number
  hasScreenshot: boolean
  screenshotName: string
  screenshotSize: number
  clientCreatedAt: string
  receivedAt: string
  resolved: boolean
  resolvedAt: string | null
  resolvedBy: string
}

export interface UserFeedbackDetail {
  id: number
  ticketId: string
  receivedAt: string
  payload: Record<string, any>
  hasScreenshot: boolean
  screenshotName: string
  screenshotSize: number
  screenshotURL: string
  resolved: boolean
  resolvedAt: string | null
  resolvedBy: string
}

export interface FeedbackResolveResult {
  id: number
  resolved: boolean
  resolvedAt: string | null
  resolvedBy: string
}

export interface FeedbackQuery {
  page?: number
  pageSize?: number
  category?: string
  platform?: string
  keyword?: string
  resolved?: string
}

export async function fetchFeedbacks(params: FeedbackQuery) {
  const response = await apiClient.get<
    ApiResponse<{ items: UserFeedbackSummary[]; total: number; page: number; pageSize: number }>
  >("/feedbacks", { params })
  return response.data.data
}

export async function fetchFeedback(id: number) {
  const response = await apiClient.get<ApiResponse<UserFeedbackDetail>>(`/feedbacks/${id}`)
  return response.data.data
}

export async function updateFeedbackResolved(id: number, resolved: boolean) {
  const response = await apiClient.patch<ApiResponse<FeedbackResolveResult>>(`/feedbacks/${id}`, { resolved })
  return response.data.data
}

export function feedbackScreenshotURL(id: number) {
  return `/api/v1/admin/feedbacks/${id}/screenshot`
}

export const FEEDBACK_EXPORT_URL = "/api/v1/admin/feedbacks-export"
