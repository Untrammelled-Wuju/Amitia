<template>
  <el-card shadow="never" class="section-card">
    <template #header>聊天界面风格</template>
    <el-radio-group :model-value="messageStyle" aria-label="聊天界面风格" @update:model-value="update">
      <el-radio-button value="flow">流式消息</el-radio-button>
      <el-radio-button value="bubble">气泡消息</el-radio-button>
    </el-radio-group>
    <p>{{ messageStyle === 'flow' ? '保留当前的连续阅读布局。' : '用户消息靠右，AI 消息靠左，以气泡区分。' }} 两种风格都支持实时生成。</p>
  </el-card>
</template>

<script setup lang="ts">
import { ElMessage } from "element-plus";
import { useChatAppearancePreference, type ChatMessageStyle } from "@/composables/useChatAppearancePreference";
const { messageStyle, setMessageStyle } = useChatAppearancePreference();
function update(value: string | number | boolean | undefined) {
  try { setMessageStyle(value as ChatMessageStyle); }
  catch { ElMessage.error("保存聊天界面风格失败，请重试"); }
}
</script>

<style scoped>
.section-card { margin-bottom: 14px; }
p { margin: 12px 0 0; color: var(--ac-color-text-secondary); font-size: 13px; line-height: 1.6; }
</style>
