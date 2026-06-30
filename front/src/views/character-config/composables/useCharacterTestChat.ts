// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref } from "vue"
import { useApi } from "../../../composables/useApi"

export function useCharacterTestChat() {
  const { post } = useApi()

  const testMessages = ref<{ role: string; content: string }[]>([])
  const testMsg = ref("")
  const testLoading = ref(false)

  async function sendTest(characterId: string, text: string) {
    const msg = text.trim()
    if (!msg || testLoading.value || !characterId) return
    testMessages.value.push({ role: "user", content: msg })
    testMsg.value = ""
    testLoading.value = true
    try {
      const result = await post<any>(`/api/characters/${characterId}/test`, { message: msg })
      testMessages.value.push({ role: "assistant", content: result?.reply || "(无回复)" })
    } catch {
      testMessages.value.push({ role: "assistant", content: "测试失败，请检查模型配置" })
    } finally {
      testLoading.value = false
    }
  }

  function clearTestMessages() {
    testMessages.value = []
  }

  return {
    testMessages,
    testMsg,
    testLoading,
    sendTest,
    clearTestMessages,
  }
}
