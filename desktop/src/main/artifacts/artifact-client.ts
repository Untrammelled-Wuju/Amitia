import { BrowserWindow } from "electron";
import { getAccessToken } from "../auth-token-store";
import {
  ArtifactMetadata,
  ArtifactKind,
  ArtifactStatus,
  UploadProgress,
  UploadState,
  UploadTask,
  buildArtifactUri,
} from "../../shared/artifact-types";

export interface ArtifactClientOptions {
  baseURL: string;
  tokenProvider?: () => string | null;
  mainWindow?: BrowserWindow;
}

export interface CreateArtifactInput {
  filename: string;
  kind: ArtifactKind;
  mimeType?: string;
  source?: string;
}

export class BusinessCoreArtifactClient {
  constructor(private readonly options: ArtifactClientOptions) {}

  private get baseURL(): string {
    return this.options.baseURL.replace(/\/$/, "");
  }

  private get token(): string | null {
    if (this.options.tokenProvider) {
      return this.options.tokenProvider();
    }
    return getAccessToken();
  }

  private headers(extra?: Record<string, string>): Record<string, string> {
    const headers: Record<string, string> = { ...extra };
    const token = this.token;
    if (token) {
      headers["Authorization"] = `Bearer ${token}`;
    }
    return headers;
  }

  async getMetadata(artifactId: string): Promise<ArtifactMetadata> {
    const res = await fetch(`${this.baseURL}/api/artifacts/v1/${artifactId}`, {
      method: "GET",
      headers: this.headers(),
    });
    if (!res.ok) {
      throw new Error(`artifact_metadata_failed: ${res.status}`);
    }
    const data = await res.json();
    return data.artifact as ArtifactMetadata;
  }

  async delete(artifactId: string): Promise<void> {
    const res = await fetch(`${this.baseURL}/api/artifacts/v1/${artifactId}`, {
      method: "DELETE",
      headers: this.headers(),
    });
    if (!res.ok) {
      throw new Error(`artifact_delete_failed: ${res.status}`);
    }
  }

  openContentUrl(artifactId: string): string {
    return `${this.baseURL}/api/artifacts/v1/${artifactId}/content`;
  }

  async uploadArtifact(
    input: CreateArtifactInput,
    filePath: string,
    onProgress?: (progress: UploadProgress) => void,
  ): Promise<ArtifactMetadata> {
    const fs = await import("node:fs");
    const path = await import("node:path");
    const stat = await fs.promises.stat(filePath);
    const fileSize = stat.size;
    const stream = fs.createReadStream(filePath);
    return this.uploadStream(input, stream, fileSize, path.basename(filePath), onProgress);
  }

  async uploadStream(
    input: CreateArtifactInput,
    stream: NodeJS.ReadableStream,
    totalBytes: number,
    filename?: string,
    onProgress?: (progress: UploadProgress) => void,
  ): Promise<ArtifactMetadata> {
    const fd = new FormData();
    fd.append("kind", input.kind);
    if (input.mimeType) {
      fd.append("mimeType", input.mimeType);
    }
    if (input.source) {
      fd.append("source", input.source);
    }
    fd.append("file", {
      stream,
      filename: filename || input.filename,
      knownLength: totalBytes,
    } as any);

    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      xhr.open("POST", `${this.baseURL}/api/artifacts/v1`);
      const token = this.token;
      if (token) {
        xhr.setRequestHeader("Authorization", `Bearer ${token}`);
      }

      xhr.upload.onprogress = (event) => {
        if (event.lengthComputable && onProgress) {
          onProgress({ loaded: event.loaded, total: event.total });
        }
      };

      xhr.onload = () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          try {
            const data = JSON.parse(xhr.responseText);
            resolve(data.artifact as ArtifactMetadata);
          } catch (e) {
            reject(new Error("invalid_artifact_response"));
          }
        } else {
          reject(new Error(`artifact_upload_failed: ${xhr.status}`));
        }
      };

      xhr.onerror = () => reject(new Error("artifact_upload_network_error"));
      xhr.onabort = () => reject(new Error("artifact_upload_aborted"));
      xhr.timeout = 600000;
      xhr.ontimeout = () => reject(new Error("artifact_upload_timeout"));

      xhr.send(fd);
    });
  }
}

export function createUploadTask(
  file: File,
  kind: ArtifactKind,
): UploadTask {
  return {
    id: `upload_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`,
    file,
    kind,
    state: "queued",
    progress: { loaded: 0, total: file.size },
  };
}

export function deriveArtifactKind(file: File): ArtifactKind {
  if (file.type.startsWith("image/")) return "image";
  if (file.type.startsWith("audio/")) return "audio";
  if (file.type.startsWith("video/")) return "video";
  return "file";
}

export { ArtifactMetadata, ArtifactKind, ArtifactStatus, buildArtifactUri };
