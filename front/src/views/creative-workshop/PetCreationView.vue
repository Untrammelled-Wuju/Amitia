<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <main class="center-page">
    <header>
      <h1>桌宠制作</h1>
      <p>创建和定制你的桌面陪伴角色</p>
    </header>
    <section class="placeholder">
      <el-icon><Star /></el-icon>
      <p>功能开发中，敬请期待</p>
    </section>
  </main>
</template>
<script setup lang="ts">
import { Star } from "@element-plus/icons-vue";
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
header p {
  margin: 0;
  color: var(--console-text-muted);
  line-height: 1.6;
}
.placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 16px;
  margin-top: 60px;
  color: var(--console-text-muted);
}
.placeholder .el-icon {
  font-size: 48px;
}
</style>
