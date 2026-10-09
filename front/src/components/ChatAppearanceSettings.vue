<template>
  <el-card shadow="never" class="section-card">
    <template #header>聊天界面风格</template>
    <el-radio-group :model-value="messageStyle" aria-label="聊天界面风格" @update:model-value="update">
      <el-radio-button value="flow">流式消息</el-radio-button>
      <el-radio-button value="bubble">气泡消息</el-radio-button>
    </el-radio-group>
    <p>{{ messageStyle === 'flow' ? '保留当前的连续阅读与实时生成。' : '用户消息靠右，AI 消息靠左；按完整消息推送，思考、工具和多媒体各自独立显示。' }}</p>
    <div class="glass-setting">
      <div><span>显示 AI 头像</span><p>在 AI 消息旁显示角色头像，两种聊天风格均适用。</p></div>
      <el-switch :model-value="aiAvatarEnabled" aria-label="显示 AI 头像" @update:model-value="updateAvatar" />
    </div>
    <div class="glass-setting avatar-shape-setting">
      <div class="avatar-shape-header"><span>AI 头像形状</span><span class="avatar-preview" :style="{ borderRadius: aiAvatarRadius }" aria-label="AI 头像形状预览">AI</span></div>
      <el-radio-group :model-value="aiAvatarShape" aria-label="AI 头像形状" @update:model-value="updateAvatarShape">
        <el-radio-button value="circle">圆形</el-radio-button>
        <el-radio-button value="rounded">圆角</el-radio-button>
        <el-radio-button value="custom">自定义</el-radio-button>
      </el-radio-group>
      <template v-if="aiAvatarShape === 'custom'">
        <p>圆润度 {{ aiAvatarRoundness }}%，从直角调整至圆形。</p>
        <el-slider :model-value="aiAvatarRoundness" :min="0" :max="100" :step="1" aria-label="AI 头像圆润度" @update:model-value="updateAvatarRoundness" />
      </template>
    </div>
    <div class="glass-setting">
      <div><span>显示 AI 名称</span><p>在 AI 消息中显示角色名称，与头像独立控制。</p></div>
      <el-switch :model-value="aiNameEnabled" aria-label="显示 AI 名称" @update:model-value="updateName" />
    </div>
    <div class="glass-setting">
      <div><span>用户消息磨砂玻璃</span><p>以半透明背景和局部模糊显示用户消息，两种聊天风格均适用。</p></div>
      <el-switch :model-value="userMessageGlass" aria-label="用户消息磨砂玻璃" @update:model-value="updateGlass" />
    </div>
    <div class="glass-setting">
      <div><span>用户消息水玻璃</span><p>更通透的轻模糊与边缘高光，开启后自动关闭磨砂玻璃。</p></div>
      <el-switch :model-value="userMessageWaterGlass" aria-label="用户消息水玻璃" @update:model-value="updateWaterGlass" />
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { ElMessage } from "element-plus";
import { useChatAppearancePreference, type ChatMessageStyle, type AiAvatarShape } from "@/composables/useChatAppearancePreference";
const { messageStyle, setMessageStyle, userMessageGlass, setUserMessageGlass, userMessageWaterGlass, setUserMessageWaterGlass, aiAvatarEnabled, setAiAvatarEnabled, aiNameEnabled, setAiNameEnabled } = useChatAppearancePreference();
const { aiAvatarShape, aiAvatarRoundness, aiAvatarRadius, setAiAvatarShape } = useChatAppearancePreference();
function updateAvatarShape(value: string | number | boolean | undefined) {
  try { setAiAvatarShape(value as AiAvatarShape); }
  catch { ElMessage.error("保存 AI 头像形状失败，请重试"); }
}
function updateAvatarRoundness(value: number | number[]) {
  try { setAiAvatarShape("custom", Number(value)); }
  catch { ElMessage.error("保存 AI 头像圆润度失败，请重试"); }
}
function updateName(value: string | number | boolean) {
  try { setAiNameEnabled(Boolean(value)); }
  catch { ElMessage.error("保存 AI 名称设置失败，请重试"); }
}
function updateAvatar(value: string | number | boolean) {
  try { setAiAvatarEnabled(Boolean(value)); }
  catch { ElMessage.error("保存 AI 头像设置失败，请重试"); }
}
function updateWaterGlass(value: string | number | boolean) {
  try { setUserMessageWaterGlass(Boolean(value)); }
  catch { ElMessage.error("保存用户消息水玻璃设置失败，请重试"); }
}
function updateGlass(value: string | number | boolean) {
  try { setUserMessageGlass(Boolean(value)); }
  catch { ElMessage.error("保存用户消息磨砂玻璃设置失败，请重试"); }
}
function update(value: string | number | boolean | undefined) {
  try { setMessageStyle(value as ChatMessageStyle); }
  catch { ElMessage.error("保存聊天界面风格失败，请重试"); }
}
</script>

<style scoped>
.section-card { margin-bottom: 14px; }
p { margin: 12px 0 0; color: var(--ac-color-text-secondary); font-size: 13px; line-height: 1.6; }
.glass-setting { display: flex; align-items: center; justify-content: space-between; gap: 20px; margin-top: 20px; padding-top: 16px; border-top: 1px solid var(--ac-color-border); }
.glass-setting p { margin-top: 6px; }
.avatar-shape-setting { flex-direction: column; align-items: stretch; gap: 12px; }
.avatar-shape-header { display: flex; align-items: center; justify-content: space-between; }
.avatar-preview { display: grid; place-items: center; width: 40px; height: 40px; background: var(--ac-color-primary); color: var(--tp-text-on-primary); font-size: 13px; }
</style>
