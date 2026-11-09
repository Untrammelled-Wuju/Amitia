<template>
  <div class="memory-core-page">
    <ExtensionSlot
      slot-id="memory.page.section"
      :context="{ route: '/memory' }"
      fallback="none"
      layout="stack"
      surface-role="main"
    />
    <DeviceOwnedDataView
      title="记忆总览"
      description="浏览、检索和维护角色记忆与全局记忆。"
      initial-kind="memory"
      mode="list"
      semantic-target="memory.page"
      parent-title="记忆"
      parent-path="/memory"
    />
  </div>
</template>
<script setup lang="ts">
import DeviceOwnedDataView from "@/components/DeviceOwnedDataView.vue";
import ExtensionSlot from "@/components/extension/ExtensionSlot.vue";
</script>
