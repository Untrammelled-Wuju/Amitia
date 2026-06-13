<template>
  <div :class="['chat-message', message.role, { 'is-reminder': isReminder, 'is-tool-result': isToolResult }]">
    <div class="message-avatar">
      <el-avatar :size="36" :src="message.role === 'assistant' ? avatar : undefined">
        {{ message.role === 'user' ? 'U' : 'AI' }}
      </el-avatar>
    </div>
    <div class="message-body">
      <div class="message-header">
        <span class="message-role">{{ message.role === 'user' ? '你' : name }}</span>
        <span v-if="isReminder" class="source-tag reminder-tag">提醒</span>
        <span v-else-if="isToolResult" class="source-tag tool-tag">工具</span>
        <span class="message-time">{{ formatTime(message.createdAt) }}</span>
      </div>
      <div class="message-content">{{ displayContent }}</div>
      <div v-if="message.role === 'assistant'" class="feedback-row">
        <el-button link size="small" @click="submitFeedback('good')" :disabled="feedbackSent">
          👍
        </el-button>
        <el-button link size="small" @click="submitFeedback('too_long')" :disabled="feedbackSent">
          👎
        </el-button>
        <el-popover v-if="!feedbackSent" placement="top" :width="260" trigger="click">
          <template #reference>
            <el-button link size="small">More</el-button>
          </template>
          <div class="feedback-popover">
            <el-radio-group v-model="selectedType" size="small">
              <el-radio v-for="t in feedbackTypes" :key="t.value" :value="t.value" style="display:block;margin:4px 0">{{ t.label }}</el-radio>
            </el-radio-group>
            <el-input v-model="feedbackReason" size="small" placeholder="Optional reason" style="margin-top:6px" />
            <div style="margin-top:8px;text-align:right">
              <el-button size="small" type="primary" @click="submitFeedback(selectedType)">Submit</el-button>
            </div>
          </div>
        </el-popover>
        <span v-if="feedbackSent" class="feedback-done">Thanks!</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from "vue"
import { ElMessage } from "element-plus"
import { request } from "../composables/request"
import type { Message } from "@/types"

const props = defineProps<{
  message: Message
  name?: string
  avatar?: string
}>()

const isReminder = computed(() => {
  return props.message.source === "reminder" || props.message.msgType === "reminder"
})

const isToolResult = computed(() => {
  return props.message.source === "tool_result" || props.message.msgType === "tool_result"
})





const displayContent = computed(() => props.message.content || "")

const feedbackSent = ref(false)
const selectedType = ref("good")
const feedbackReason = ref("")

const feedbackTypes = [
  { value: "good", label: "Good reply" },
  { value: "too_long", label: "Too long" },
  { value: "too_cold", label: "Too cold" },
  { value: "too_exaggerated", label: "Too exaggerated" },
  { value: "not_understand", label: "Did not understand" },
  { value: "unsafe", label: "Inappropriate" },
  { value: "other", label: "Other" },
]

async function submitFeedback(type: string) {
  try {
    await request.post(`/api/messages/${props.message.id}/feedback`, { feedbackType: type, reason: feedbackReason.value })
    feedbackSent.value = true
    ElMessage.success("Feedback submitted")
  } catch (err: any) {
    ElMessage.error(err?.message || "Failed")
  }
}

function formatTime(dateStr: string): string {
  if (!dateStr) return ""
  const d = new Date(dateStr)
  return d.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" })
}
</script>

<style scoped>
.chat-message { display: flex; gap: 14px; padding: 14px 36px; align-items: flex-start; }
.chat-message.user { flex-direction: row-reverse; }
.chat-message.user .message-body { text-align: right; }
.chat-message.user .message-header { justify-content: flex-end; }
.chat-message.assistant { background: var(--el-fill-color-light); }
.chat-message.is-reminder { background: #fafbfc; border-left: 3px solid #d0d5dd; }
.chat-message.is-tool-result { background: #f8f9fb; border-left: 3px solid #c8cdd5; }
.chat-message.is-typing { border-left: 3px solid var(--el-color-primary); }
.message-avatar { flex-shrink: 0; }
.message-body { flex: 1; min-width: 0; }
.message-header { display: flex; align-items: center; gap: 8px; margin-bottom: 4px; }
.message-role { font-weight: 600; font-size: 13px; }
.message-time { font-size: 11px; color: var(--el-text-color-secondary); }
.message-content { font-size: 14px; line-height: 1.6; white-space: pre-wrap; word-break: break-word; }
.feedback-row { display: flex; align-items: center; gap: 2px; margin-top: 6px; opacity: 0; transition: opacity 0.2s; }
.chat-message:hover .feedback-row { opacity: 1; }
.feedback-done { font-size: 12px; color: var(--ac-color-text-muted); }
.feedback-popover { padding: 4px 0; }

.source-tag {
  font-size: 10px;
  padding: 0 6px;
  border-radius: 3px;
  line-height: 1.8;
  text-transform: none;
}
.reminder-tag {
  color: #8b5e3c;
  background: #fef6e8;
  border: 1px solid #f0dba8;
}
.tool-tag {
  color: #4a6fa5;
  background: #eef3fa;
  border: 1px solid #c8d6e5;
}
</style>
