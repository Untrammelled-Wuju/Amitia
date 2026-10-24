<template>
  <div class="space-identity-page" v-loading="loading">
    <section class="identity-hero">
      <div class="identity-icon"><el-icon :size="24"><Key /></el-icon></div>
      <div class="identity-hero-copy">
        <span class="identity-eyebrow">当前空间</span>
        <h2>聊天与记忆的个人归属</h2>
        <p>Space 身份用于标记你的数据边界。角色、对话和记忆都会保存在这个空间中。</p>
      </div>
      <span class="identity-status"><i></i>{{ identity.spaceId ? "身份可用" : "读取中" }}</span>
    </section>

    <div class="identity-grid">
      <section class="identity-card identity-main" aria-labelledby="identity-main-title">
        <div class="identity-card-heading">
          <div>
            <h3 id="identity-main-title">Space ID</h3>
            <p>当前个人空间的主标识。</p>
          </div>
          <span class="identity-card-mark">SPACE</span>
        </div>
        <div class="identity-field">
          <div class="identity-value-row">
            <code>{{ identity.spaceId || "未读取" }}</code>
            <el-button size="small" :disabled="!identity.spaceId" @click="copyIdentity(identity.spaceId)">复制</el-button>
          </div>
        </div>
      </section>

      <section class="identity-card">
        <div class="identity-card-heading">
          <div>
            <h3>数据归属</h3>
            <p>Space 身份会随当前 Core 一起管理。</p>
          </div>
          <span class="identity-card-mark">LOCAL</span>
        </div>
        <ul class="identity-points">
          <li><span></span>隔离个人资料、角色与对话</li>
          <li><span></span>用于备份、迁移与设备同步</li>
          <li><span></span>不会包含凭据或 API Key</li>
        </ul>
      </section>
    </div>

    <section class="identity-notice">
      <el-icon><Lock /></el-icon>
      <div>
        <strong>仅在需要恢复或迁移时使用标识</strong>
        <p>Space ID 和 Instance ID 属于技术信息，日常使用无需修改或分享。</p>
      </div>
    </section>

    <details class="identity-advanced">
      <summary>实例标识与技术信息</summary>
      <div class="identity-field">
        <span class="identity-label">Instance ID</span>
        <div class="identity-value-row">
          <code>{{ identity.instanceId || "未读取" }}</code>
          <el-button size="small" :disabled="!identity.instanceId" @click="copyIdentity(identity.instanceId)">复制</el-button>
        </div>
      </div>
      <p>实例标识用于区分运行环境，通常无需手动操作。</p>
    </details>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { ElMessage } from "element-plus";
import { Key, Lock } from "@element-plus/icons-vue";
import { apiClient } from "@/composables/useApi";

const loading = ref(false);
const identity = reactive({ spaceId: "", instanceId: "" });

async function loadIdentity() {
  loading.value = true;
  try {
    const res = await apiClient.get("/api/public/core/info");
    const info = res.data?.data || res.data || {};
    identity.spaceId = String(info.spaceId || "");
    identity.instanceId = String(info.instanceId || info.cloudId || "");
  } catch (error: any) {
    ElMessage.error(error?.message || "Space 身份加载失败");
  } finally {
    loading.value = false;
  }
}

async function copyIdentity(value: string) {
  if (!value) return;
  try {
    if (window.amitiaDesktop?.writeClipboardText) {
      await window.amitiaDesktop.writeClipboardText(value);
    } else {
      await navigator.clipboard.writeText(value);
    }
    ElMessage.success("已复制");
  } catch {
    ElMessage.error("复制失败，请手动选择文本");
  }
}

onMounted(loadIdentity);
</script>

<style scoped>
.space-identity-page { width: min(100%, 1040px); min-width: 0; display: grid; gap: 18px; }
.identity-hero { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: 20px; padding: 28px; border-radius: 24px; background: #2C2C2E; }
.identity-icon { width: 58px; height: 58px; flex: 0 0 auto; border-radius: 17px; background: color-mix(in srgb, var(--ac-color-primary) 12%, var(--control-hover-bg)); color: var(--ac-color-primary); display: grid; place-items: center; }
.identity-hero-copy { min-width: 0; }
.identity-eyebrow { color: var(--text-secondary); font-size: 11px; font-weight: 620; letter-spacing: 0.08em; text-transform: uppercase; }
.identity-hero-copy h2 { margin: 6px 0 0; font-size: 21px; line-height: 1.35; }
.identity-hero-copy p { max-width: 680px; margin: 7px 0 0; color: var(--text-secondary); font-size: 13px; line-height: 1.65; }
.identity-status { display: inline-flex; align-items: center; gap: 7px; align-self: flex-start; padding: 6px 10px; border-radius: 999px; background: color-mix(in srgb, var(--workbench-bg) 68%, transparent); color: var(--text-secondary); font-size: 11px; white-space: nowrap; }
.identity-status i { width: 7px; height: 7px; border-radius: 50%; background: var(--ac-color-success); box-shadow: 0 0 0 3px color-mix(in srgb, var(--ac-color-success) 14%, transparent); }
.identity-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 18px; }
.identity-card { min-width: 0; padding: 24px; border-radius: 20px; background: var(--surface-bg-elevated); }
.identity-card-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; margin-bottom: 18px; }
.identity-card-heading h3 { margin: 0; font-size: 15px; }
.identity-card-heading p { margin: 5px 0 0; color: var(--text-secondary); font-size: 12px; line-height: 1.55; }
.identity-card-mark { color: var(--text-secondary); font-size: 10px; font-weight: 650; letter-spacing: 0.08em; }
.identity-field { min-width: 0; }
.identity-label { font-size: 12px; color: var(--text-secondary); display: block; margin-bottom: 8px; }
.identity-value-row { display: flex; align-items: center; gap: 10px; }
.identity-value-row code { min-width: 0; flex: 1; overflow-wrap: anywhere; padding: 13px 14px; border-radius: 11px; background: var(--workbench-bg); color: var(--text-primary); font-size: 13px; }
.identity-value-row .el-button { flex-shrink: 0; }
.identity-points { display: grid; gap: 11px; margin: 0; padding: 0; list-style: none; }
.identity-points li { display: flex; align-items: center; gap: 9px; color: var(--text-secondary); font-size: 12px; }
.identity-points li span { width: 6px; height: 6px; flex: 0 0 auto; border-radius: 50%; background: var(--ac-color-primary); }
.identity-notice { display: flex; align-items: flex-start; gap: 13px; padding: 18px 20px; border-radius: 16px; background: color-mix(in srgb, var(--surface-bg-elevated) 72%, transparent); color: var(--text-secondary); }
.identity-notice > .el-icon { flex: 0 0 auto; margin-top: 2px; color: var(--ac-color-primary); }
.identity-notice strong { color: var(--text-primary); font-size: 13px; }
.identity-notice p { margin: 4px 0 0; font-size: 12px; line-height: 1.55; }
.identity-advanced { padding: 18px 20px; border-radius: 16px; background: var(--surface-bg-elevated); }
.identity-advanced summary { font-size: 13px; cursor: pointer; color: var(--text-secondary); }
.identity-advanced .identity-field { margin-top: 18px; }
.identity-advanced p { margin: 12px 0 0; color: var(--text-secondary); font-size: 12px; line-height: 1.55; }
@media (max-width: 760px) { .identity-hero { grid-template-columns: auto minmax(0, 1fr); padding: 22px; } .identity-status { grid-column: 1 / -1; justify-self: start; } .identity-grid { grid-template-columns: minmax(0, 1fr); } .identity-value-row { align-items: flex-start; } }
</style>
