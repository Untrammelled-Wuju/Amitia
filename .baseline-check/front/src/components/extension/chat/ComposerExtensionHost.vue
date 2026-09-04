<script setup lang="ts">
import { computed } from "vue";
import ExtensionSlot from "@/components/extension/ExtensionSlot.vue";
import { apiClient } from "@/composables/useApi";

const props = defineProps<{
  characterId: string;
  conversationId: string;
  channel: string;
  platform: string;
  conversationState: "idle" | "generating" | "offline";
  capabilities: string[];
  draft: string;
}>();

const composerContext = computed(() => ({
  characterId: props.characterId,
  conversationId: props.conversationId,
  channel: props.channel,
  platform: props.platform,
  conversationState: props.conversationState,
  capabilities: props.capabilities,
  draft: props.draft,
  scope: "composer",
}));

async function invokeComposerAction(actionId: string) {
  try {
    await apiClient.post(`/api/extension/composer/action/${actionId}`, {
      draft: props.draft,
      context: composerContext.value,
    });
  } catch (e) {
    console.error("Composer action failed:", e);
  }
}
</script>

<template>
  <div class="composer-extension-host">
    <div class="composer-extension-host__main">
      <div class="composer-extension-host__attachments">
        <ExtensionSlot
          slot-id="chat.composer.attachment"
          :context="composerContext"
          fallback="none"
          layout="inline"
          surface-role="composer"
        />
      </div>
      <div class="composer-extension-host__actions">
        <ExtensionSlot
          slot-id="chat.composer.action"
          :context="composerContext"
          fallback="none"
          layout="inline"
          surface-role="composer"
        />
        <ExtensionSlot
          slot-id="chat.composer.hint"
          :context="composerContext"
          fallback="none"
          layout="inline"
          surface-role="composer"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
.composer-extension-host {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}
.composer-extension-host__main {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}
.composer-extension-host__attachments {
  display: flex;
  gap: 4px;
  align-items: center;
  flex-wrap: wrap;
}
.composer-extension-host__actions {
  display: flex;
  gap: 4px;
  align-items: center;
  justify-content: space-between;
  opacity: 0.85;
}
</style>
