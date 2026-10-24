<template>
  <DeviceOwnedDataView
    title="记忆总览"
    description="浏览、检索和维护角色记忆与全局记忆。"
    initial-kind="memory"
    mode="list"
    semantic-target="memory.page"
    parent-title="记忆"
    parent-path="/memory"
  />
</template>
<script setup lang="ts">
import DeviceOwnedDataView from "@/components/DeviceOwnedDataView.vue";
</script>
