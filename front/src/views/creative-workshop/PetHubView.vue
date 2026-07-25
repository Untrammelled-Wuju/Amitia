<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <main class="pet-hub">
    <ExtensionPageHeader title="桌宠" description="管理桌宠制作、安装与记录" parent-title="创意工坊" parent-path="/creative-workshop" />

    <section v-if="showDisclaimer" class="disclaimer-overlay">
      <div class="disclaimer-card">
        <div class="disclaimer-icon">
          <el-icon :size="48"><WarningFilled /></el-icon>
        </div>
        <h1 class="disclaimer-title">实验性功能</h1>
        <p class="disclaimer-desc">桌宠功能目前处于实验阶段，正在持续优化中，不确保功能的完整性和可用性。部分功能可能存在异常或不可用的情况。</p>
        <div class="disclaimer-actions">
          <el-checkbox v-model="dontShowAgain" label="不再提示" />
          <div class="disclaimer-buttons">
            <el-button @click="goBackToWorkshop">返回创意工坊</el-button>
            <el-button type="primary" @click="confirmEnter">确认进入</el-button>
          </div>
        </div>
      </div>
    </section>

    <section v-else class="entry-grid">
      <router-link
        v-for="entry in entries"
        :key="entry.to"
        :to="entry.to"
        class="entry-card"
      >
        <el-icon><component :is="entry.icon" /></el-icon>
        <div>
          <h2>{{ entry.title }}</h2>
          <p>{{ entry.description }}</p>
        </div>
        <el-icon><ArrowRight /></el-icon>
      </router-link>
    </section>
  </main>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useRouter } from "vue-router";
import { ArrowRight, Star, Clock, Box, WarningFilled } from "@element-plus/icons-vue";
import ExtensionPageHeader from "../extensions/components/ExtensionPageHeader.vue";

const DISCLAIMER_KEY = "pet-disclaimer-dismissed";

const router = useRouter();
const showDisclaimer = ref(true);
const dontShowAgain = ref(false);

onMounted(() => {
  if (sessionStorage.getItem(DISCLAIMER_KEY) === "1") {
    showDisclaimer.value = false;
  }
});

function confirmEnter() {
  if (dontShowAgain.value) {
    sessionStorage.setItem(DISCLAIMER_KEY, "1");
  }
  showDisclaimer.value = false;
}

function goBackToWorkshop() {
  router.push("/creative-workshop");
}

const entries = [
  {
    to: "/creative-workshop/pet/create",
    title: "制作桌宠",
    description: "创建和定制你的桌面陪伴角色",
    icon: Star,
  },
  {
    to: "/creative-workshop/pet/tasks",
    title: "制作记录",
    description: "查看桌宠生成任务的状态与进度",
    icon: Clock,
  },
  {
    to: "/creative-workshop/pet/installations",
    title: "安装管理",
    description: "管理已安装的桌宠、启用停用与配置",
    icon: Box,
  },
];
</script>

<style scoped>
.pet-hub {
  height: 100%;
  overflow: auto;
}

.disclaimer-overlay {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: calc(100% - 80px);
  padding: 24px;
}

.disclaimer-card {
  max-width: 480px;
  width: 100%;
  padding: 40px 36px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 12px;
  background: var(--ac-color-surface);
  text-align: center;
}

.disclaimer-icon {
  color: var(--el-color-warning);
  margin-bottom: 16px;
}

.disclaimer-title {
  margin: 0 0 16px;
  font-size: 22px;
  font-weight: 700;
  color: var(--console-text);
}

.disclaimer-desc {
  margin: 0 0 28px;
  color: var(--el-text-color-secondary);
  line-height: 1.8;
  font-size: 14px;
}

.disclaimer-actions {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
}

.disclaimer-buttons {
  display: flex;
  gap: 12px;
}

.entry-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 16px;
  margin-top: 24px;
}

.entry-card {
  display: grid;
  grid-template-columns: auto 1fr auto;
  align-items: center;
  gap: 16px;
  min-height: 110px;
  padding: 20px;
  border: 1px solid var(--console-border);
  border-radius: 12px;
  background: var(--ac-color-surface);
  color: var(--console-text);
  text-decoration: none;
  transition:
    border-color 180ms ease,
    background-color 180ms ease;
  box-shadow: none;
}

.entry-card:hover,
.entry-card:focus-visible {
  border-color: var(--el-color-primary);
  background: var(--ac-color-surface-soft);
  outline: none;
}

.entry-card > .el-icon:first-child {
  font-size: 26px;
  color: var(--el-color-primary);
}

.entry-card h2 {
  margin: 0 0 6px;
  font-size: 18px;
}

.entry-card p {
  margin: 0;
  color: var(--console-text-muted);
  line-height: 1.6;
}

@media (max-width: 720px) {
  .disclaimer-card {
    padding: 28px 20px;
  }

  .entry-card {
    min-height: 120px;
  }
}
</style>
