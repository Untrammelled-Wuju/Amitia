<template>
  <main class="center-page">
    <header>
      <h1>扩展中心</h1>
      <p>统一管理扩展包、系统插件、Skill 与外部能力。</p>
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
  ArrowRight,
  Box,
  Connection,
  DocumentChecked,
  Operation,
  SetUp,
  Tickets,
} from "@element-plus/icons-vue";

const entries = [
  { to: "/extensions/packages", title: "本地扩展包", description: "安装和管理 .amitiax 扩展包", icon: Box },
  { to: "/extensions/mcp", title: "MCP 服务", description: "连接外部 MCP 服务", icon: Connection },
  { to: "/extensions/agent-skills", title: "Agent Skills", description: "管理 SKILL.md 指令包", icon: DocumentChecked },
  { to: "/extensions/plugins", title: "系统插件", description: "管理系统 Plugin Runtime", icon: SetUp },
  { to: "/extensions/skills", title: "兼容技能", description: "管理旧 Skill 能力", icon: Operation },
  { to: "/extensions/runs", title: "执行记录", description: "查看运行结果和耗时", icon: Tickets },
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
