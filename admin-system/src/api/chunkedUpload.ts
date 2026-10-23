export interface ArtifactUploadSession {
  uploadId: string
  chunkSize: number
  chunkCount: number
  concurrency: number
  uploadedParts: number[]
  uploadedBytes: number
  completed: boolean
}

export interface ChunkedUploadHooks {
  create: () => Promise<ArtifactUploadSession>
  status: (uploadId: string) => Promise<ArtifactUploadSession>
  uploadPart: (uploadId: string, index: number, data: ArrayBuffer, sha256: string) => Promise<void>
  complete: (uploadId: string) => Promise<void>
}

export interface ChunkedUploadOptions {
  onProgress?: (percent: number) => void
  retries?: number
  hashChunk?: (data: ArrayBuffer) => Promise<string>
  pendingKey?: string
  storage?: Pick<Storage, "getItem" | "setItem" | "removeItem">
}

interface PendingUpload {
  uploadId: string
  fileName: string
  size: number
  lastModified: number
  chunkSize: number
  chunkCount: number
  concurrency: number
}

async function browserChunkHash(data: ArrayBuffer): Promise<string> {
  const digest = await crypto.subtle.digest("SHA-256", data)
  return Array.from(new Uint8Array(digest), (value) => value.toString(16).padStart(2, "0")).join("")
}

function readPending(storage: ChunkedUploadOptions["storage"], key: string | undefined, file: File) {
  if (!storage || !key) return null
  try {
    const raw = storage.getItem(key)
    if (!raw) return null
    const parsed = JSON.parse(raw) as PendingUpload
    if (
      parsed.fileName !== file.name ||
      parsed.size !== file.size ||
      parsed.lastModified !== file.lastModified
    ) {
      storage.removeItem(key)
      return null
    }
    return parsed
  } catch {
    if (storage && key) storage.removeItem(key)
    return null
  }
}

function writePending(storage: ChunkedUploadOptions["storage"], key: string | undefined, pending: PendingUpload) {
  if (!storage || !key) return
  storage.setItem(key, JSON.stringify(pending))
}

function wait(ms: number) {
  return new Promise<void>((resolve) => setTimeout(resolve, ms))
}

function readBlob(blob: Blob): Promise<ArrayBuffer> {
  if (typeof blob.arrayBuffer === "function") return blob.arrayBuffer()
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(reader.result as ArrayBuffer)
    reader.onerror = () => reject(reader.error ?? new Error("读取上传分片失败"))
    reader.readAsArrayBuffer(blob)
  })
}

async function withRetry(action: () => Promise<void>, retries: number) {
  let lastError: unknown
  for (let attempt = 0; attempt <= retries; attempt += 1) {
    try {
      await action()
      return
    } catch (error) {
      lastError = error
      if (attempt < retries) {
        await wait(250 * 2 ** attempt)
      }
    }
  }
  throw lastError
}

export async function uploadFileInChunks(
  file: File,
  hooks: ChunkedUploadHooks,
  options: ChunkedUploadOptions = {},
) {
  const storage =
    options.storage ??
    (typeof localStorage === "undefined" ? undefined : localStorage)
  const key = options.pendingKey
  const hashChunk = options.hashChunk ?? browserChunkHash
  const retries = Math.max(0, options.retries ?? 2)
  let pending = readPending(storage, key, file)
  let session: ArtifactUploadSession

  if (pending) {
    try {
      session = await hooks.status(pending.uploadId)
      if (
        session.chunkSize !== pending.chunkSize ||
        session.chunkCount !== pending.chunkCount
      ) {
        throw new Error("upload session changed")
      }
    } catch {
      storage?.removeItem(key ?? "")
      pending = null
      session = await hooks.create()
    }
  } else {
    session = await hooks.create()
  }

  if (!session.uploadId || session.chunkSize <= 0 || session.chunkCount <= 0) {
    throw new Error("上传会话响应无效")
  }
  if (session.chunkCount !== Math.ceil(file.size / session.chunkSize)) {
    throw new Error("上传分片配置与文件大小不匹配")
  }
  const concurrency = Math.min(
    8,
    Math.max(1, session.concurrency || 4),
    session.chunkCount,
  )
  const pendingRecord: PendingUpload = {
    uploadId: session.uploadId,
    fileName: file.name,
    size: file.size,
    lastModified: file.lastModified,
    chunkSize: session.chunkSize,
    chunkCount: session.chunkCount,
    concurrency,
  }
  writePending(storage, key, pendingRecord)

  const completed = new Set(
    session.uploadedParts.filter(
      (index) => Number.isInteger(index) && index >= 0 && index < session.chunkCount,
    ),
  )
  const updateProgress = () => {
    if (!options.onProgress) return
    let uploadedBytes = 0
    for (const index of completed) {
      uploadedBytes += Math.min(
        session.chunkSize,
        file.size - index * session.chunkSize,
      )
    }
    options.onProgress(Math.min(100, Math.round((uploadedBytes / file.size) * 100)))
  }
  updateProgress()

  let nextIndex = 0
  const worker = async () => {
    while (true) {
      const index = nextIndex
      nextIndex += 1
      if (index >= session.chunkCount) return
      if (completed.has(index)) continue
      const start = index * session.chunkSize
      const end = Math.min(file.size, start + session.chunkSize)
      const data = await readBlob(file.slice(start, end))
      const sha256 = await hashChunk(data)
      await withRetry(
        () => hooks.uploadPart(session.uploadId, index, data, sha256),
        retries,
      )
      completed.add(index)
      updateProgress()
    }
  }
  await Promise.all(Array.from({ length: concurrency }, () => worker()))
  if (completed.size !== session.chunkCount) {
    throw new Error("上传分片未全部完成")
  }
  await hooks.complete(session.uploadId)
  if (storage && key) storage.removeItem(key)
  options.onProgress?.(100)
  return session
}

export function pendingUploadKey(releaseId: number, kind: string) {
  return `amitia-admin-upload:${releaseId}:${kind}`
}
