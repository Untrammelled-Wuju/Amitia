<template>
  <el-table :data="rows" empty-text="暂无数据" stripe>
    <el-table-column v-for="column in columns" :key="column.key" :prop="column.key" :label="column.label" min-width="120" show-overflow-tooltip />
  </el-table>
</template>

<script setup lang="ts">
import type { SurfaceField } from "../types"
defineProps<{ columns: SurfaceField[]; rows: Record<string, unknown>[] }>()
</script>
