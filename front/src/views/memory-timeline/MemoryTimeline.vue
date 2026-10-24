<template>
  <DeviceOwnedDataView
    title="时间线"
    description="按时间顺序回顾角色和记忆的变化。"
    initial-kind="memory"
    mode="timeline"
    :kinds="TIMELINE_KINDS"
    parent-title="记忆"
    parent-path="/memory"
  />
</template>
<script setup lang="ts">
import DeviceOwnedDataView from "@/components/DeviceOwnedDataView.vue";

const TIMELINE_KINDS = ["memory", "working", "episodic", "fact", "profile", "graph", "summary"];
</script>
