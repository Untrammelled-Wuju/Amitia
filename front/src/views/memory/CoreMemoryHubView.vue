<template>
  <div class="memory-hub">
    <ExtensionPageHeader
      title="记忆"
      description="管理记忆总览、知识库、情景记忆、图谱、时间线和用户画像。"
      parent-title="记忆"
      parent-path="/memory"
    />
    <div class="memory-grid">
      <button
        v-for="entry in entries"
        :key="entry.path"
        type="button"
        class="memory-card"
        @click="router.push(entry.path)"
      >
        <el-icon :size="26"><component :is="entry.icon" /></el-icon>
        <span>
          <strong>{{ entry.title }}</strong>
          <small>{{ entry.description }}</small>
        </span>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from "vue-router";
import { Avatar, Collection, DataAnalysis, Film, Share, Timer } from "@element-plus/icons-vue";
import ExtensionPageHeader from "@/components/extension/ExtensionPageHeader.vue";

const router = useRouter();
const entries = [
  { title: "记忆总览", description: "浏览和维护角色记忆与全局记忆。", path: "/memory-manager", icon: DataAnalysis },
  { title: "知识库", description: "管理共享知识源和检索索引。", path: "/knowledge", icon: Collection },
  { title: "情景记忆", description: "查看对话形成的情景记忆。", path: "/episodic", icon: Film },
  { title: "记忆图谱", description: "查看记忆实体和关系。", path: "/graph", icon: Share },
  { title: "时间线", description: "按时间浏览记忆变化。", path: "/memory-timeline", icon: Timer },
  { title: "用户画像", description: "维护用户偏好和画像信息。", path: "/profiles", icon: Avatar },
];
</script>

<style scoped>
.memory-hub { width: min(1120px, 100%); margin: 0 auto; padding: 24px; }
.memory-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); gap: 14px; margin-top: 20px; }
.memory-card { display: flex; align-items: flex-start; gap: 14px; padding: 18px; border: 1px solid var(--surface-border); border-radius: var(--ac-radius-md); background: var(--surface-bg); color: var(--text-primary); text-align: left; cursor: pointer; }
.memory-card:hover { border-color: var(--ac-color-primary); background: var(--control-hover-bg); }
.memory-card span { display: grid; gap: 5px; }
.memory-card strong { font-size: 15px; font-weight: 600; }
.memory-card small { color: var(--text-secondary); font-size: 12px; line-height: 1.5; }
@media (max-width: 767px) { .memory-hub { padding: 16px; } }
</style>
