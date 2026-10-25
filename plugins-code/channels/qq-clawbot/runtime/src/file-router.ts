// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
const MAGIC_BYTES: Record<string, number[][]> = {
  "image/jpeg": [[0xff, 0xd8, 0xff]],
  "image/png": [[0x89, 0x50, 0x4e, 0x47]],
  "image/gif": [[0x47, 0x49, 0x46, 0x38]],
  "image/webp": [[0x52, 0x49, 0x46, 0x46]],
  "image/bmp": [[0x42, 0x4d]],
  "application/pdf": [[0x25, 0x50, 0x44, 0x46]],
  "application/zip": [[0x50, 0x4b, 0x03, 0x04]],
  "audio/mp3": [[0xff, 0xfb], [0xff, 0xf3], [0xff, 0xf2], [0x49, 0x44, 0x33]],
  "audio/mpeg": [[0xff, 0xfb], [0xff, 0xf3], [0xff, 0xf2], [0x49, 0x44, 0x33]],
  "audio/ogg": [[0x4f, 0x67, 0x67, 0x53]],
  "audio/wav": [[0x52, 0x49, 0x46, 0x46]],
  "video/mp4": [[0x00, 0x00, 0x00, 0x18, 0x66, 0x74, 0x79, 0x70]],
}

export interface FileInfo {
  buffer: Buffer
  fileName: string
  mimeType: string
}

export interface RouteResult {
  handler: string
  data: {
    mimeType?: string
    base64?: string
    fileName?: string
  }
}

type FileHandler = (file: FileInfo) => Promise<RouteResult>

export class FileRouter {
  private handlers = new Map<string, FileHandler>()

  register(type: string, handler: FileHandler): void {
    this.handlers.set(type, handler)
  }

  async route(file: FileInfo): Promise<RouteResult | null> {
    const detectedType = this.detectFileType(file.buffer, file.mimeType)
    const handler = this.handlers.get(detectedType) ?? this.findWildcardHandler(detectedType)
    return handler ? handler(file) : null
  }

  private findWildcardHandler(mimeType: string): FileHandler | undefined {
    const [category] = mimeType.split("/")
    return this.handlers.get(category + "/*")
  }

  detectFileType(buffer: Buffer, hintType?: string): string {
    if (hintType && (hintType.startsWith("image/") || hintType.startsWith("video/") || hintType.startsWith("audio/"))) {
      return hintType
    }
    for (const [mimeType, magicSequences] of Object.entries(MAGIC_BYTES)) {
      for (const sequence of magicSequences) {
        if (buffer.length < sequence.length) continue
        let matched = true
        for (let index = 0; index < sequence.length; index++) {
          if (buffer[index] !== sequence[index]) {
            matched = false
            break
          }
        }
        if (!matched) continue
        if (mimeType === "audio/wav" && buffer.length > 12 && buffer.slice(8, 12).toString("ascii") !== "WAVE") continue
        if (mimeType === "application/zip" && buffer.length > 40 && buffer.slice(30, 40).toString("ascii").includes("mimetype")) continue
        return mimeType
      }
    }
    return hintType || "application/octet-stream"
  }
}

export function createDefaultRouter(): FileRouter {
  const router = new FileRouter()
  router.register("image/*", async (file) => ({
    handler: "image",
    data: { mimeType: file.mimeType, base64: file.buffer.toString("base64"), fileName: file.fileName },
  }))
  router.register("audio/*", async (file) => ({
    handler: "audio",
    data: { mimeType: file.mimeType, base64: file.buffer.toString("base64"), fileName: file.fileName },
  }))
  router.register("video/*", async (file) => ({
    handler: "video",
    data: { mimeType: file.mimeType, base64: file.buffer.toString("base64"), fileName: file.fileName },
  }))
  return router
}
