<template>
  <section class="channel-plugin-workspace">
    <header class="channel-plugin-tabs" role="tablist" aria-label="渠道插件页面">
      <button
        type="button"
        role="tab"
        :aria-selected="activeTab === 'plugin'"
        :class="{ active: activeTab === 'plugin' }"
        @click="selectTab('plugin')"
      >
        插件页面
      </button>
      <button
        type="button"
        role="tab"
        :aria-selected="activeTab === 'messages'"
        :class="{ active: activeTab === 'messages' }"
        @click="selectTab('messages')"
      >
        消息统计
      </button>
    </header>

    <div v-show="activeTab === 'plugin'" class="channel-plugin-content" data-amitia-ui="channels.plugin-page">
      <UIProviderHost
        :capability="capability"
        :provider-id="providerId || undefined"
        :fallback="DefaultFallback"
        :context="context"
        frameless
      />
    </div>

    <ChannelHostMessagesPanel
      v-if="activeTab === 'messages'"
      :channel-id="channelId"
      class="channel-host-messages"
    />
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import UIProviderHost from "./UIProviderHost.vue";
import type { UIProviderCapability } from "@/ui-runtime/types";
import { useExtensionUIStore } from "@/stores/extensionUI";
import { collectChannelPluginNavigationItems } from "@/ui-runtime/navigationRegistry";
import ChannelHostMessagesPanel from "@/views/channels/ChannelHostMessagesPanel.vue";

defineProps<{
  providerId: string;
  capability: UIProviderCapability;
  context: Record<string, unknown>;
}>();

const DefaultFallback = { template: '<div class="channel-plugin-unavailable">渠道插件页面暂不可用</div>' };
const route = useRoute();
const router = useRouter();
const store = useExtensionUIStore();
const activeTab = ref(route.query.tab === "messages" ? "messages" : "plugin");

const pluginItem = computed(() => collectChannelPluginNavigationItems(store).find(
  (item) => route.path === item.route || route.path.startsWith(`${item.route}/`),
));
const channelId = computed(() => pluginItem.value?.channelId || "");

function selectTab(tab: "plugin" | "messages") {
  activeTab.value = tab;
  void router.replace({
    path: route.path,
    query: { ...route.query, tab: tab === "messages" ? "messages" : undefined },
    hash: route.hash,
  });
}

watch(() => route.query.tab, (tab) => {
  activeTab.value = tab === "messages" ? "messages" : "plugin";
});
</script>

<style scoped>
.channel-plugin-workspace { height: 100%; min-height: 0; display: flex; flex-direction: column; }
.channel-plugin-tabs { flex: 0 0 auto; display: flex; align-items: center; gap: 4px; min-height: 46px; padding: 6px 18px 0; border-bottom: 1px solid var(--surface-border); background: var(--workbench-bg); }
.channel-plugin-tabs button { align-self: stretch; padding: 0 12px; border: 0; border-bottom: 2px solid transparent; background: transparent; color: var(--text-secondary); cursor: pointer; font: inherit; font-size: var(--ac-font-size-sm); }
.channel-plugin-tabs button:hover { color: var(--text-primary); }
.channel-plugin-tabs button.active { border-bottom-color: var(--ac-color-primary); color: var(--text-primary); font-weight: 600; }
.channel-plugin-tabs button:focus-visible { outline: 2px solid var(--ac-color-primary); outline-offset: -2px; }
.channel-plugin-content { min-height: 0; flex: 1; overflow: hidden; }
.channel-host-messages { min-height: 0; flex: 1; }
.channel-plugin-unavailable { padding: 24px; color: var(--text-secondary); }
@media (max-width: 720px) {
  .channel-plugin-tabs { padding-left: 10px; padding-right: 10px; }
}
</style>
