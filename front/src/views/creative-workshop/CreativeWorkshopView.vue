<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<!--
Creative Workshop is the user-facing creation hub. Legacy Skill creation is
retained for compatibility; Workflow creation below is backed by Extension Kernel.
-->
<template>
  <main class="center-page">
    <header>
      <h1>创意工坊</h1>
      <p>在这里创作、调试和优化你的 AI 角色与内容</p>
    </header>
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
import { ArrowRight, MagicStick, Share, Star } from "@element-plus/icons-vue";
const entries = [
  {
    to: "/creative-workshop/workflows",
    title: "工作流",
    description: "可视化拖拽创建 DAG 自动化工作流",
    icon: Share,
  },
  {
    to: "/creative-workshop/skills",
    title: "技能制作",
    description: "创建声明式 Skill",
    icon: MagicStick,
  },
  {
    to: "/creative-workshop/pet",
    title: "桌宠",
    description: "管理桌宠制作、安装与记录",
    icon: Star,
  },
];
</script>
<style scoped>
.center-page {
  height: 100%;
  overflow: auto;
}
header h1 {
  margin: 0 0 6px;
  color: var(--console-text);
  font-size: 24px;
}
header p,
.entry-card p {
  margin: 0;
  color: var(--console-text-muted);
  line-height: 1.6;
  font-size: 13px;
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
@media (max-width: 720px) {
  .entry-card {
    min-height: 120px;
  }
}
</style>
