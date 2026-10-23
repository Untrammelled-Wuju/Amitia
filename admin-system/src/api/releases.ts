import { apiClient, type ApiResponse } from "./client"
import type {
  AdminSettings,
  AuditLog,
  AuthPayload,
  CreateReleasePayload,
  OverviewData,
  ReleaseValidation,
  UpdateRelease,
} from "./types"
import { pendingUploadKey, uploadFileInChunks, type ArtifactUploadSession } from "./chunkedUpload"

export async function login(username: string, password: string) {
  const response = await apiClient.post<ApiResponse<AuthPayload>>("/auth/login", {
    username,
    password,
  })
  return response.data.data
}

export async function loadMe() {
  const response = await apiClient.get<ApiResponse<AuthPayload>>("/auth/me")
  return response.data.data
}

export async function logout() {
  await apiClient.post("/auth/logout")
}

export async function changePassword(currentPassword: string, newPassword: string) {
  await apiClient.post("/auth/password", {
    currentPassword,
    newPassword,
  })
}

export async function fetchOverview() {
  const response = await apiClient.get<ApiResponse<OverviewData>>("/overview")
  return response.data.data
}

export async function fetchReleases(params: {
  product?: string
  channel?: string
  status?: string
}) {
  const response = await apiClient.get<ApiResponse<UpdateRelease[]>>("/releases", { params })
  return response.data.data
}

export async function fetchRelease(id: number) {
  const response = await apiClient.get<ApiResponse<UpdateRelease>>(`/releases/${id}`)
  return response.data.data
}

export async function createRelease(payload: CreateReleasePayload) {
  const response = await apiClient.post<ApiResponse<UpdateRelease>>("/releases", payload)
  return response.data.data
}

export async function deleteRelease(id: number) {
  await apiClient.delete(`/releases/${id}`)
}

export async function uploadArtifact(
  id: number,
  kind: string,
  file: File,
  onProgress?: (percent: number) => void,
) {
  if (file.size >= 4 * 1024 * 1024) {
    return uploadArtifactInChunks(id, kind, file, onProgress)
  }
  const form = new FormData()
  form.append("kind", kind)
  form.append("file", file)
  const response = await apiClient.post<ApiResponse<unknown>>(`/releases/${id}/artifacts`, form, {
    onUploadProgress: (event) => {
      if (event.total && onProgress) {
        onProgress(Math.round((event.loaded / event.total) * 100))
      }
    },
  })
  return response.data.data
}

export async function uploadArtifactInChunks(
  id: number,
  kind: string,
  file: File,
  onProgress?: (percent: number) => void,
) {
  const session = await uploadFileInChunks(file, {
    create: async () => {
      const response = await apiClient.post<ApiResponse<ArtifactUploadSession>>(
        `/releases/${id}/upload-sessions`,
        { kind, fileName: file.name, size: file.size },
      )
      return response.data.data
    },
    status: async (uploadId) => {
      const response = await apiClient.get<ApiResponse<ArtifactUploadSession>>(
        `/releases/${id}/upload-sessions/${encodeURIComponent(uploadId)}`,
      )
      return response.data.data
    },
    uploadPart: async (uploadId, index, data, sha256) => {
      await apiClient.put(
        `/releases/${id}/upload-sessions/${encodeURIComponent(uploadId)}/parts/${index}`,
        data,
        {
          headers: {
            "Content-Type": "application/octet-stream",
            "X-Chunk-SHA256": sha256,
          },
        },
      )
    },
    complete: async (uploadId) => {
      await apiClient.post(
        `/releases/${id}/upload-sessions/${encodeURIComponent(uploadId)}/complete`,
      )
    },
  }, {
    onProgress,
    pendingKey: pendingUploadKey(id, kind),
  })
  const response = await apiClient.get<ApiResponse<{ artifacts: unknown[] }>>(`/releases/${id}`)
  return response.data.data.artifacts.at(-1) ?? session
}

export async function validateRelease(id: number) {
  const response = await apiClient.post<ApiResponse<ReleaseValidation>>(`/releases/${id}/validate`)
  return response.data.data
}

export async function publishRelease(id: number) {
  const response = await apiClient.post<ApiResponse<UpdateRelease>>(`/releases/${id}/publish`)
  return response.data.data
}

export async function updateRollout(id: number, rolloutPercentage: number) {
  const response = await apiClient.post<ApiResponse<UpdateRelease>>(`/releases/${id}/rollout`, {
    rolloutPercentage,
  })
  return response.data.data
}

export async function rollbackDesktop(id: number) {
  const response = await apiClient.post<ApiResponse<UpdateRelease>>(`/releases/${id}/rollback`)
  return response.data.data
}

export async function fetchAuditLogs() {
  const response = await apiClient.get<ApiResponse<AuditLog[]>>("/audit-logs")
  return response.data.data
}

export async function fetchSettings() {
  const response = await apiClient.get<ApiResponse<AdminSettings>>("/settings")
  return response.data.data
}
