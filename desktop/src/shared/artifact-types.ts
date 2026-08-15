export type ArtifactKind = "image" | "audio" | "video" | "file";

export type ArtifactStatus = "uploading" | "ready" | "deleted";

export interface ArtifactMetadata {
  id: string;
  owner_user_id: string;
  workspace_id: string;
  kind: ArtifactKind;
  blob_digest: string;
  size_bytes: number;
  mime_type: string;
  filename: string;
  file_extension: string;
  status: ArtifactStatus;
  source: string;
  width: number;
  height: number;
  duration_ms: number;
  revision: number;
  created_at: string;
  updated_at: string;
}

export const ARTIFACT_URI_PREFIX = "amitia://artifacts/";

export function buildArtifactUri(artifactId: string): string {
  return `${ARTIFACT_URI_PREFIX}${artifactId}`;
}

export function parseArtifactUri(uri: string): string | null {
  if (!uri.startsWith(ARTIFACT_URI_PREFIX)) {
    return null;
  }
  const id = uri.slice(ARTIFACT_URI_PREFIX.length);
  if (!id || id.includes("/") || id.includes("..")) {
    return null;
  }
  return id;
}

export interface UploadProgress {
  loaded: number;
  total: number;
}

export type UploadState = "queued" | "uploading" | "uploaded" | "failed";

export interface UploadTask {
  id: string;
  file: File;
  kind: ArtifactKind;
  state: UploadState;
  progress: UploadProgress;
  error?: string;
  artifact?: ArtifactMetadata;
}
