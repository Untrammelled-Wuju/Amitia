<!--
Deprecated: Legacy extension architecture.
Do not add new capabilities. This view is retained only for
compatibility, maintenance, testing, and migration to Extension Kernel.
-->
<template>
  <main class="center-page">
    <header>
      <h1>扩展中心</h1>
      <p>集中管理 Skill、Plugin、工坊草案与执行记录。</p>
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
import {
  MagicStick,
  Operation,
  SetUp,
  Tickets,
  ArrowRight,
  DocumentChecked,
  Connection,
} from "@element-plus/icons-vue";
const entries = [
  {
    to: "/extensions/mcp",
    title: "MCP 服务",
    description: "连接外部服务，管理工具、资源、提示词与角色权限",
    icon: Connection,
  },
  {
    to: "/extensions/agent-skills",
    title: "Agent Skills",
    description: "导入和管理通用 SKILL.md 指令包及其文件",
    icon: DocumentChecked,
  },
  {
    to: "/extensions/plugins",
    title: "插件",
    description: "管理受宿主管控的 Plugin Runtime",
    icon: SetUp,
  },
  {
    to: "/extensions/workshop",
    title: "技能制作",
    description: "从自然语言创建安全的声明式 Skill",
    icon: MagicStick,
  },
  {
    to: "/extensions/skills",
    title: "技能管理",
    description: "统一管理模型可调用技能、启用状态与工具适配",
    icon: Operation,
  },
  {
    to: "/extensions/runs",
    title: "执行记录",
    description: "检查 Skill 运行结果、耗时和标准错误",
    icon: Tickets,
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
