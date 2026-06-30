// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
const DOUBAO_API_KEY = "ark-919cb2bc-dcd1-4ef9-b8b5-5f1b42488bf7-9bd5c"
const DOUBAO_BASE_URL = "https://ark.cn-beijing.volces.com/api/v3"
const DOUBAO_MODEL = "doubao-seed-2-0-lite-260428"

export interface DoubaoResponseOutput {
  type: string
  text?: string
}

export interface DoubaoResponse {
  id: string
  model: string
  output: Array<{
    type: string
    role?: string
    content?: DoubaoResponseOutput[]
    summary?: DoubaoResponseOutput[]
  }>
  usage?: {
    input_tokens: number
    output_tokens: number
    total_tokens: number
  }
}

export async function analyzeImage(
  imageBase64: string,
  prompt: string = "请详细描述这张图片的内容"
): Promise<string> {
  try {
    const response = await fetch(DOUBAO_BASE_URL + "/responses", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: "Bearer " + DOUBAO_API_KEY,
      },
      body: JSON.stringify({
        model: DOUBAO_MODEL,
        input: [
          {
            role: "user",
            content: [
              {
                type: "input_image",
                image_url: imageBase64,
              },
              {
                type: "input_text",
                text: prompt,
              },
            ],
          },
        ],
      }),
    })

    const data: DoubaoResponse = await response.json()

    if (!response.ok) {
      throw new Error((data as any)?.error?.message || "请求失败: " + response.status)
    }

    const output = data.output
    if (!output) return "未能获取分析结果"

    const messageItem = output.find((item) => item.type === "message")
    if (messageItem && messageItem.content) {
      const textParts = messageItem.content
        .filter((c: DoubaoResponseOutput) => c.type === "output_text")
        .map((c: DoubaoResponseOutput) => c.text || "")
      return textParts.join("") || "未能获取分析结果"
    }

    return "未能获取分析结果"
  } catch (err: any) {
    throw new Error("豆包AI调用失败: " + (err.message || "未知错误"))
  }
}

export function fileToBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(reader.result as string)
    reader.onerror = () => reject(new Error("文件读取失败"))
    reader.readAsDataURL(file)
  })
}

export function getDoubaoModel(): string {
  return DOUBAO_MODEL
}
