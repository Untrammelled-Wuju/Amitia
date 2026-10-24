<template>
  <div class="channel-hub">
    <el-empty class="channel-hub-placeholder" description="请从第二栏选择渠道插件" :image-size="80" />
    <nav class="channel-hub-mobile" aria-label="渠道接入">
      <RouterLink v-for="item in pluginItems" :key="item.id" :to="item.route" class="channel-hub-entry">
        {{ item.label }}
      </RouterLink>
    </nav>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useExtensionUIStore } from "@/stores/extensionUI";
import { collectChannelPluginNavigationItems } from "@/ui-runtime/navigationRegistry";

const store = useExtensionUIStore();
const pluginItems = computed(() => collectChannelPluginNavigationItems(store));
</script>

<style scoped>
.channel-hub { min-height: 100%; }
.channel-hub-placeholder { min-height: 360px; }
.channel-hub-mobile { display: none; }
@media (max-width: 767px) {
  .channel-hub-placeholder { display: none; }
  .channel-hub-mobile { display: grid; gap: 8px; padding-top: 12px; }
  .channel-hub-entry { display: flex; align-items: center; min-height: 48px; padding: 0 14px; border: 1px solid var(--surface-border); border-radius: var(--ac-radius-md); background: var(--surface-bg); color: var(--text-primary); text-decoration: none; }
}
</style>
