// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import fs from "node:fs"

const MAGIC_BYTES: Record<string, number[][]> = {
  "image/jpeg": [[0xFF, 0xD8, 0xFF]],
  "image/png": [[0x89, 0x50, 0x4E, 0x47]],
  "image/gif": [[0x47, 0x49, 0x46, 0x38]],
  "image/webp": [[0x52, 0x49, 0x46, 0x46]],
  "image/bmp": [[0x42, 0x4D]],
  "application/pdf": [[0x25, 0x50, 0x44, 0x46]],
  "application/zip": [[0x50, 0x4B, 0x03, 0x04]],
  "audio/mp3": [[0xFF, 0xFB], [0xFF, 0xF3], [0xFF, 0xF2], [0x49, 0x44, 0x33]],
  "audio/mpeg": [[0xFF, 0xFB], [0xFF, 0xF3], [0xFF, 0xF2], [0x49, 0x44, 0x33]],
  "audio/ogg": [[0x4F, 0x67, 0x67, 0x53]],
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
  data: any
}

type FileHandler = (file: FileInfo) => Promise<RouteResult>

export class FileRouter {
  private handlers: Map<string, FileHandler> = new Map()

  register(type: string, handler: FileHandler): void {
    this.handlers.set(type, handler)
    console.log("[FileRouter] 注册处理器: " + type)
  }

  async route(file: FileInfo): Promise<RouteResult | null> {
    const detectedType = this.detectFileType(file.buffer, file.mimeType)
    console.log("[FileRouter] 文件类型检测: fileName=" + file.fileName + " mimeType=" + file.mimeType + " detected=" + detectedType)

    const handler = this.handlers.get(detectedType)
    if (handler) {
      return handler(file)
    }

    const wildcardHandler = this.findWildcardHandler(detectedType)
    if (wildcardHandler) {
      return wildcardHandler(file)
    }

    console.log("[FileRouter] 未找到处理器: " + detectedType)
    return null
  }

  private findWildcardHandler(mimeType: string): FileHandler | undefined {
    const [category] = mimeType.split("/")
    const wildcardKey = category + "/*"
    return this.handlers.get(wildcardKey)
  }

  detectFileType(buffer: Buffer, hintType?: string): string {
    if (hintType && this.isImageType(hintType)) {
      return hintType
    }
    if (hintType && this.isVideoType(hintType)) {
      return hintType
    }
    if (hintType && this.isAudioType(hintType)) {
      return hintType
    }

    for (const [mimeType, magicSeq] of Object.entries(MAGIC_BYTES)) {
      for (const seq of magicSeq) {
        if (buffer.length >= seq.length) {
          let match = true
          for (let i = 0; i < seq.length; i++) {
            if (buffer[i] !== seq[i]) {
              match = false
              break
            }
          }
          if (match) {
            if (mimeType === "audio/wav" && buffer.length > 8) {
              const waveId = buffer.slice(8, 12).toString("ascii")
              if (waveId !== "WAVE") continue
            }
            if (mimeType === "application/zip" && buffer.length > 40) {
              const docType = buffer.slice(30, 40).toString("ascii")
              if (docType.includes("mimetype")) continue
            }
            return mimeType
          }
        }
      }
    }

    return hintType || "application/octet-stream"
  }

  private isImageType(t: string): boolean {
    return t.startsWith("image/")
  }

  private isVideoType(t: string): boolean {
    return t.startsWith("video/")
  }

  private isAudioType(t: string): boolean {
    return t.startsWith("audio/")
  }
}

export function createDefaultRouter(): FileRouter {
  const router = new FileRouter()

  router.register("image/*", async (file: FileInfo): Promise<RouteResult> => {
    const b64 = file.buffer.toString("base64")
    console.log("[FileRouter] 图片文件路由: " + file.fileName + " mime=" + file.mimeType + " size=" + file.buffer.length)
    return {
      handler: "image",
      data: {
        mimeType: file.mimeType,
        base64: b64,
        fileName: file.fileName,
      },
    }
  })

  router.register("audio/*", async (file: FileInfo): Promise<RouteResult> => {
    console.log("[FileRouter] 音频文件路由: " + file.fileName + " mime=" + file.mimeType)
    return {
      handler: "audio",
      data: {
        mimeType: file.mimeType,
        base64: file.buffer.toString("base64"),
        fileName: file.fileName,
      },
    }
  })

  router.register("video/*", async (file: FileInfo): Promise<RouteResult> => {
    console.log("[FileRouter] 视频文件路由: " + file.fileName + " mime=" + file.mimeType)
    return {
      handler: "video",
      data: {
        mimeType: file.mimeType,
        base64: file.buffer.toString("base64"),
        fileName: file.fileName,
      },
    }
  })

  return router
}