<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <main class="pet-hub">
    <ExtensionPageHeader title="桌宠" description="管理桌宠制作、安装与记录" parent-title="创意工坊" parent-path="/creative-workshop" />

    <section class="entry-grid">
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
import { ArrowRight, Star, Clock, Box } from "@element-plus/icons-vue";
import ExtensionPageHeader from "../extensions/components/ExtensionPageHeader.vue";


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
  .entry-card {
    min-height: 120px;
  }
}
</style>
