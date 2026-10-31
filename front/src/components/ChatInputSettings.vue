<template>
  <el-card shadow="never" class="section-card" data-amitia-ui="component.card">
    <template #header>对话输入</template>
    <div class="input-setting">
      <div><div>回车发送</div><p>开启时 Enter 发送，Shift + Enter 换行；关闭时 Enter 换行。发送按钮始终可用。</p></div>
      <el-switch data-amitia-ui="component.switch" :model-value="sendOnEnter" aria-label="回车发送" @update:model-value="update" />
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { ElMessage } from "element-plus";
import { useChatInputPreference } from "@/composables/useChatInputPreference";
const { sendOnEnter, setSendOnEnter } = useChatInputPreference();
function update(value: boolean) {
  try { setSendOnEnter(value); }
  catch (error) {
  console.error("[original-error]", error);
ElMessage.error(error instanceof Error ? error.message : String(error)); }
}
</script>

<style scoped>
.section-card { margin-bottom: 12px; border: 1px solid var(--ac-color-border-light); }
.input-setting { display: flex; align-items: center; justify-content: space-between; gap: 20px; }
p { color: var(--ac-color-text-secondary); font-size: 13px; line-height: 1.6; margin-bottom: 0; }
</style>
