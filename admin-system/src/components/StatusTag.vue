<script setup lang="ts">
import { computed } from "vue"

import type { ReleaseStatus } from "@/api/types"

const props = defineProps<{
  status: ReleaseStatus
}>()

const label = computed(() => {
  return {
    draft: "草稿",
    ready: "待发布",
    published: "已发布",
    paused: "已暂停",
    failed: "失败",
  }[props.status]
})

const type = computed(() => {
  return {
    draft: "info",
    ready: "warning",
    published: "success",
    paused: "warning",
    failed: "danger",
  }[props.status] as "info" | "warning" | "success" | "danger"
})
</script>

<template>
  <el-tag :type="type" effect="plain" round>{{ label }}</el-tag>
</template>
