<template>
  <DeviceOwnedDataView
    title="用户画像"
    description="维护可用于角色理解和个性化回应的用户画像。"
    initial-kind="profile"
    mode="profile"
    semantic-target="profile.page"
    parent-title="角色"
    parent-path="/characters"
  />
</template>
<script setup lang="ts">
import DeviceOwnedDataView from "@/components/DeviceOwnedDataView.vue";
</script>
