export interface AdminUser {
  id: number
  username: string
  role: "admin" | "publisher"
}

export interface AuthPayload {
  user: AdminUser
  csrfToken: string
  expiresAt?: string
}

export type Product = "desktop" | "android"
export type Channel = "stable" | "beta" | "alpha"
export type ReleaseStatus = "draft" | "ready" | "published" | "paused" | "failed"

export interface ReleaseArtifact {
  id: number
  releaseId: number
  kind: "installer" | "blockmap" | "apk"
  originalName: string
  size: number
  sha256: string
  sha512: string
  sha512Base64: string
  createdAt: string
  updatedAt: string
}

export interface UpdateRelease {
  id: number
  product: Product
  channel: Channel
  version: string
  versionCode: number
  status: ReleaseStatus
  releaseName: string
  releaseNotes: string
  mandatory: boolean
  minSupportedVersionCode: number
  rolloutPercentage: number
  abi: string
  packageName: string
  publishPath: string
  createdBy: number
  publishedAt: string | null
  createdAt: string
  updatedAt: string
  artifacts: ReleaseArtifact[]
}

export interface CreateReleasePayload {
  product: Product
  channel: Channel
  version: string
  versionCode: number
  releaseName: string
  releaseNotes: string
  mandatory: boolean
  minSupportedVersionCode: number
  rolloutPercentage: number
  abi: string
}

export interface ReleaseValidation {
  ready: boolean
  errors: string[]
  artifacts: string[]
}

export interface AuditLog {
  id: number
  actorId: number
  actorName: string
  action: string
  product: string
  channel: string
  releaseId: number
  target: string
  result: string
  message: string
  createdAt: string
}

export interface OverviewData {
  releaseCount: number
  publishedCount: number
  androidCount: number
  auditCount: number
  latestReleases: UpdateRelease[]
  desktopPublish: string
  androidPublish: string
  publicBaseURL: string
  signingKeyReady: boolean
}

export interface AdminSettings {
  desktopPublishDir: string
  androidPublishDir: string
  publicBaseURL: string
  androidPackageName: string
  desktopClientURL: string
  androidClientBaseURL: string
  signingKeyConfigured: boolean
  cookieSecure: boolean
}
