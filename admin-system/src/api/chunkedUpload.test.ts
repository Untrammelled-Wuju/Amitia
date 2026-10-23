import { describe, expect, it, vi } from "vitest"

import { uploadFileInChunks, type ArtifactUploadSession } from "./chunkedUpload"

function memoryStorage() {
  const values = new Map<string, string>()
  return {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => values.set(key, value),
    removeItem: (key: string) => values.delete(key),
  }
}

describe("chunked upload", () => {
  it("uploads ten concurrent files and validates every part", async () => {
    for (let round = 0; round < 10; round += 1) {
      const file = new File(
        [new Uint8Array(2300 + round).map((_, index) => (index * 17 + round) % 251)],
        `release-${round}.apk`,
        { lastModified: 1700000000000 + round },
      )
      const parts = new Map<number, ArrayBuffer>()
      const complete = vi.fn(async () => undefined)
      const create = vi.fn(async (): Promise<ArtifactUploadSession> => ({
        uploadId: `upload_${round}`,
        chunkSize: 256,
        chunkCount: Math.ceil(file.size / 256),
        concurrency: 4,
        uploadedParts: [],
        uploadedBytes: 0,
        completed: false,
      }))
      await uploadFileInChunks(
        file,
        {
          create,
          status: create,
          uploadPart: async (_uploadId, index, data) => {
            await new Promise((resolve) => setTimeout(resolve, 2))
            parts.set(index, data)
          },
          complete,
        },
        {
          storage: memoryStorage(),
          pendingKey: `round-${round}`,
          hashChunk: async () => "a".repeat(64),
        },
      )
      expect(parts.size).toBe(Math.ceil(file.size / 256))
      expect(parts.size).toBeGreaterThan(4)
      expect(complete).toHaveBeenCalledOnce()
    }
  })

  it("resumes only missing parts", async () => {
    const file = new File([new Uint8Array(1024)], "release.apk", {
      lastModified: 1700000000000,
    })
    const storage = memoryStorage()
    storage.setItem(
      "resume",
      JSON.stringify({
        uploadId: "upload_resume",
        fileName: file.name,
        size: file.size,
        lastModified: file.lastModified,
        chunkSize: 256,
        chunkCount: 4,
        concurrency: 2,
      }),
    )
    const uploaded: number[] = []
    await uploadFileInChunks(
      file,
      {
        create: async () => {
          throw new Error("must not create")
        },
        status: async () => ({
          uploadId: "upload_resume",
          chunkSize: 256,
          chunkCount: 4,
          concurrency: 2,
          uploadedParts: [0, 2],
          uploadedBytes: 512,
          completed: false,
        }),
        uploadPart: async (_uploadId, index) => {
          uploaded.push(index)
        },
        complete: async () => undefined,
      },
      {
        storage,
        pendingKey: "resume",
        hashChunk: async () => "b".repeat(64),
      },
    )
    expect(uploaded.sort()).toEqual([1, 3])
    expect(storage.getItem("resume")).toBeNull()
  })
})
