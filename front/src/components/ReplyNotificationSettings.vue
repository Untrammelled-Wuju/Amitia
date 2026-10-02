<template>
  <el-card shadow="never" class="reply-settings">
    <template #header>对话通知</template>
    <div class="setting-row">
      <div>
        <strong>回复完成通知</strong>
        <p>AI 回复完成且应用处于后台时发送系统通知。前台、失败或取消的回复不通知。</p>
      </div>
      <el-switch :model-value="enabled" :disabled="busy" aria-label="回复完成通知" @update:model-value="update" />
    </div>
    <div class="setting-row sound-row">
      <div>
        <strong>回复提示音</strong>
        <p>仅后台正常完成回复时播放，可独立于通知开启。音量由系统控制。</p>
      </div>
      <el-switch :model-value="soundEnabled" :disabled="busy" aria-label="回复提示音" @update:model-value="updateSound" />
    </div>
  </el-card>
</template>
<script setup lang="ts">
import { ref } from "vue";
import { ElMessage } from "element-plus";
import { useReplyNotifications } from "../composables/useReplyNotifications";
const { enabled, soundEnabled, setEnabled, setSoundEnabled } = useReplyNotifications();
const busy = ref(false);
function updateSound(value: boolean | string | number) {
  return update(value, true);
}
async function update(value: boolean | string | number, sound = false) {
  busy.value = true;
  try {
    await (sound ? setSoundEnabled(Boolean(value)) : setEnabled(Boolean(value)));
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "保存通知设置失败");
  } finally {
    busy.value = false;
  }
}
</script>
<style scoped>
.reply-settings {
  margin-bottom: 16px;
}
.setting-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}
.sound-row {
  margin-top: 20px;
}
p {
  color: var(--ac-color-text-secondary);
  font-size: 13px;
  line-height: 1.6;
  margin-bottom: 0;
}
</style>
