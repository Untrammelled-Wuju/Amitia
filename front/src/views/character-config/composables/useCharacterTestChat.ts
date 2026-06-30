// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, nextTick } from "vue"
import { useApi } from "../../../composables/useApi"

export function useCharacterTestChat() {
  const { post } = useApi()

  const testMessages = ref<{ role: string; content: string }[]>([])
  const testMsg = ref("")
  const testLoading = ref(false)

  async function sendTest(characterId: string, testChatRef: HTMLElement | null) {
    const msg = testMsg.value.trim()
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
      if (testChatRef) {
        nextTick(() => {
          testChatRef.scrollTop = testChatRef.scrollHeight
        })
      }
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
