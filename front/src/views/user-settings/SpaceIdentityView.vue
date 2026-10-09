<template>
  <div class="space-identity-page" v-loading="loading">
    <header class="space-page-header">
      <h2>Space 身份</h2>
      <p>用于标识当前数据空间，与登录凭据和设备身份相互独立。</p>
    </header>
    <section class="identity-main" aria-labelledby="identity-main-title">
      <div class="identity-heading">
        <div class="identity-icon"><el-icon :size="22"><Key /></el-icon></div>
        <div>
          <h3 id="identity-main-title">当前 Space</h3>
          <p>你的聊天和角色数据归属于这个空间。</p>
        </div>
      </div>
      <div class="identity-field">
        <span class="identity-label">Space ID</span>
        <div class="identity-value-row">
          <code>{{ identity.spaceId || "未读取" }}</code>
          <el-button size="small" :disabled="!identity.spaceId" @click="copyIdentity(identity.spaceId)">复制</el-button>
        </div>
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
import { Key } from "@element-plus/icons-vue";
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
.space-identity-page { min-width: 0; max-width: 880px; }
.space-page-header { margin-bottom: 24px; }
.space-page-header h2 { margin: 0; font-size: 23px; font-weight: 650; letter-spacing: -0.025em; }
.space-page-header p { margin: 7px 0 0; color: var(--text-secondary); font-size: 13px; line-height: 1.6; }
.identity-main { padding: 26px; border: 1px solid var(--surface-border); border-radius: 14px; background: var(--surface-bg); }
.identity-heading { display: flex; gap: 14px; align-items: center; margin-bottom: 28px; }
.identity-heading h3 { font-size: 17px; margin: 0; }
.identity-heading p, .identity-advanced p { margin: 6px 0 0; color: var(--text-secondary); font-size: 13px; line-height: 1.5; }
.identity-icon { width: 50px; height: 50px; flex: 0 0 auto; border-radius: 14px; background: var(--control-hover-bg); color: var(--text-primary); display: grid; place-items: center; }
.identity-field { min-width: 0; }
.identity-label { font-size: 12px; color: var(--text-secondary); display: block; margin-bottom: 8px; }
.identity-value-row { display: flex; align-items: center; gap: 12px; }
.identity-value-row code { min-width: 0; flex: 1; overflow-wrap: anywhere; padding: 12px 14px; border-radius: 9px; background: var(--control-hover-bg); color: var(--text-primary); font-size: 13px; }
.identity-value-row .el-button { flex-shrink: 0; }
.identity-advanced { margin-top: 18px; padding: 16px 22px; border: 1px solid var(--surface-border); border-radius: 12px; }
.identity-advanced summary { font-size: 13px; cursor: pointer; color: var(--text-secondary); }
.identity-advanced .identity-field { margin-top: 20px; }
@media (max-width: 700px) { .identity-main { padding: 18px; } .identity-value-row { align-items: flex-start; } }
</style>
