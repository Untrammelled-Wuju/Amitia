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
            {{ checking ? '检查中...' : '检查版本更新' }}
          </el-button>
          <span v-if="!isDesktop" class="desktop-only-hint">仅桌面端支持版本更新</span>
        </div>
      </div>
    </el-card>

    <el-card shadow="never" class="section-card">
      <template #header><span>版权与许可</span></template>
      <el-descriptions :column="1" border size="small">
        <el-descriptions-item label="版权声明">
          Copyright &copy; 2026 彭旭
        </el-descriptions-item>
        <el-descriptions-item label="开源协议">
          <el-link type="primary" href="https://opensource.org/license/agpl-v3" target="_blank">
            GNU Affero General Public License v3.0 (AGPL-3.0)
          </el-link>
        </el-descriptions-item>
      </el-descriptions>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue"
import { ElMessage } from "element-plus"
import { isDesktopShell } from "@/runtime/runtime-capabilities"
import { useBrandLogo } from "@/composables/useBrandLogo"

const version = "26.1.0"

const checking = ref(false)
const isDesktop = ref(false)
const { logoUrl } = useBrandLogo()

function handleCheckUpdate() {
  if (!window.amitiaDesktop) return
  checking.value = true
  window.amitiaDesktop.checkUpdate()
}

onMounted(() => {
  isDesktop.value = isDesktopShell()

  if (!window.amitiaDesktop) return

  const api = window.amitiaDesktop

  api.onUpdateChecking(() => {
    checking.value = true
  })

  api.onUpdateAvailable(() => {
    checking.value = false
  })

  api.onUpdateNotAvailable(() => {
    checking.value = false
    ElMessage.success("已是最新版本")
  })

  api.onUpdateError(() => {
    checking.value = false
  })
})
</script>

<style scoped>
.about-panel { }

.section-card {
  margin-bottom: 12px;
  border: 1px solid var(--ac-color-border-light);
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
</style>
