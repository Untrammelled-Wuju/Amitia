<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="about-panel">
    <el-card shadow="never" class="section-card">
      <template #header><span>关于 Amitia</span></template>
      <div class="about-app">
        <div class="about-logo">
          <img :src="logoUrl" alt="Amitia" />
        </div>
        <div class="about-info">
          <h2 class="about-name">Amitia</h2>
          <p class="about-version">版本 {{ version }}</p>
        </div>
        <div class="about-actions">
          <el-button
            type="primary"
            :loading="checking"
            :disabled="!isDesktop"
            @click="handleCheckUpdate"
          >
            {{ checking ? "检查中..." : "检查版本更新" }}
          </el-button>
          <span v-if="!isDesktop" class="desktop-only-hint"
            >仅桌面端支持版本更新</span
          >
        </div>
      </div>
      <div class="about-channel">
        <div class="about-channel-copy">
          <span class="about-channel-title">接收测试版更新</span>
          <span class="about-channel-hint">
            {{
              allowBetaChannel
                ? "已加入测试通道，测试版与正式版更新都会提示"
                : "开启后检查更新会包含测试版，测试版与正式版更新都会提示"
            }}
          </span>
        </div>
        <el-switch
          :model-value="allowBetaChannel"
          :disabled="!isDesktop || channelSaving"
          @change="handleChannelChange"
        />
      </div>
    </el-card>

    <el-card v-if="releaseNotes" shadow="never" class="section-card">
      <template #header><span>更新内容</span></template>
      <pre class="release-notes">{{ releaseNotes }}</pre>
    </el-card>

    <el-card shadow="never" class="section-card">
      <template #header><span>版权与许可</span></template>
      <el-descriptions :column="1" border size="small">
        <el-descriptions-item label="版权声明">
          Copyright &copy; 2026 彭旭
        </el-descriptions-item>
        <el-descriptions-item label="开源协议">
          <el-link
            type="primary"
            href="https://opensource.org/license/agpl-v3"
            target="_blank"
          >
            GNU Affero General Public License v3.0 (AGPL-3.0)
          </el-link>
        </el-descriptions-item>
      </el-descriptions>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { ElMessage } from "element-plus";
import { isDesktopShell } from "@/runtime/runtime-capabilities";
import { useBrandLogo } from "@/composables/useBrandLogo";

const version = ref("—");

const checking = ref(false);
const releaseNotes = ref("");
const isDesktop = ref(false);
const allowBetaChannel = ref(false);
const channelSaving = ref(false);
const { logoUrl } = useBrandLogo();

let updateHandled = false;

async function handleCheckUpdate() {
  if (!window.amitiaDesktop) return;
  checking.value = true;
  updateHandled = false;
  await window.amitiaDesktop.checkUpdate();
  if (!updateHandled) {
    checking.value = false;
  }
}

async function handleChannelChange(value: string | number | boolean) {
  if (!window.amitiaDesktop) return;
  const next = value === true;
  channelSaving.value = true;
  try {
    const result = await window.amitiaDesktop.setUpdateChannel(next);
    allowBetaChannel.value = result?.allowBetaChannel === true;
    ElMessage.success(
      allowBetaChannel.value ? "已加入测试版通道" : "已回到正式版通道",
    );
  } catch (error) {
  console.error("[original-error]", error);
ElMessage.error(error instanceof Error ? error.message : String(error));
  } finally {
    channelSaving.value = false;
  }
}

onMounted(async () => {
  isDesktop.value = isDesktopShell();

  if (!window.amitiaDesktop) return;

  const api = window.amitiaDesktop;

  try {
    version.value = await api.getCurrentVersion();
  } catch (error) {
  console.error("[original-error]", error);
version.value = "—";
  }

  try {
    const channel = await api.getUpdateChannel();
    allowBetaChannel.value = channel?.allowBetaChannel === true;
  } catch (error) {
  console.error("[original-error]", error);
allowBetaChannel.value = false;
  }

  api.getReleaseNotes().then((notes) => {
    releaseNotes.value = notes;
  });

  api.onUpdateChecking(() => {
    checking.value = true;
  });

  api.onUpdateAvailable(() => {
    checking.value = false;
    updateHandled = true;
    ElMessage.info("发现新版本，请在更新弹窗中继续操作");
  });

  api.onUpdateNotAvailable(() => {
    checking.value = false;
    updateHandled = true;
    ElMessage.success("已是最新版本");
  });

  api.onUpdateError(() => {
    checking.value = false;
    updateHandled = true;
  });
});
</script>

<style scoped>
.about-panel {
}

.section-card {
  margin-bottom: 12px;
  border: 1px solid var(--ac-color-border-light);
}

.about-channel {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-top: 14px;
  padding-top: 14px;
  border-top: 1px solid var(--ac-color-border-light);
}

.about-channel-copy {
  display: grid;
  gap: 4px;
}

.about-channel-title {
  font-size: 14px;
  font-weight: 600;
}

.about-channel-hint {
  color: var(--ac-color-text-muted);
  font-size: 12px;
  line-height: 1.5;
}

.about-app {
  display: flex;
  align-items: center;
  gap: 20px;
  padding: 8px 0;
}

.about-logo {
  width: 72px;
  height: 72px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.about-logo img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.about-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
  flex: 1;
}

.about-name {
  font-size: 22px;
  font-weight: 700;
  color: var(--ac-color-text);
  margin: 0;
}

.about-version {
  font-size: 13px;
  color: var(--ac-color-text-muted);
  margin: 0;
}

.about-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}

.desktop-only-hint {
  font-size: 12px;
  color: var(--ac-color-text-muted);
}

.release-notes {
  font-size: 13px;
  line-height: 1.6;
  color: var(--ac-color-text);
  white-space: pre-wrap;
  word-break: break-word;
  margin: 0;
  padding: 4px 0;
}
</style>
