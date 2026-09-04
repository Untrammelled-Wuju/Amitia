<template>
  <el-descriptions :column="2" border>
    <el-descriptions-item label="生命周期">{{
      health?.lifecycle || "—"
    }}</el-descriptions-item>
    <el-descriptions-item label="健康状态"
      ><el-tag :type="health?.health === 'healthy' ? 'success' : 'warning'">{{
        health?.health || "unknown"
      }}</el-tag></el-descriptions-item
    >
    <el-descriptions-item label="兼容性">{{
      health?.compatible ? "兼容" : "不兼容"
    }}</el-descriptions-item>
    <el-descriptions-item label="最近错误">{{
      health?.lastErrorCode || "—"
    }}</el-descriptions-item>
  </el-descriptions>
</template>

<script setup lang="ts">
import type { PluginHealth } from "../types";
defineProps<{ health?: PluginHealth }>();
</script>
