<script setup lang="ts">
import { computed, ref } from "vue";
import ExtensionSlot from "@/components/extension/ExtensionSlot.vue";
import { apiClient } from "@/composables/useApi";

const props = defineProps<{
  characterId: string;
  conversationId: string;
  channel: string;
  platform: string;
  conversationState: "idle" | "generating" | "offline";
  capabilities: string[];
}>();

const draft = ref<string>("");
const composerContext = computed(() => ({
  characterId: props.characterId,
  conversationId: props.conversationId,
  channel: props.channel,
  platform: props.platform,
  conversationState: props.conversationState,
  capabilities: props.capabilities,
  scope: "composer",
}));

async function invokeComposerAction(actionId: string) {
  try {
    await apiClient.post(`/api/extensions/ui/composer/action/${actionId}`, {
      draft: draft.value,
      context: composerContext.value,
    });
  } catch (e) {
    console.error("Composer action failed:", e);
  }
}
</script>

<template>
  <div class="composer-extension-host">
    <div class="composer-extension-host__actions">
      <ExtensionSlot
        slot-id="chat.composer.action"
        :context="composerContext"
        fallback="none"
        layout="inline"
      />
    </div>
    <div class="composer-extension-host__attachments">
      <ExtensionSlot
        slot-id="chat.composer.attachment"
        :context="composerContext"
        fallback="none"
        layout="inline"
      />
    </div>
    <div class="composer-extension-host__hint">
      <ExtensionSlot
        slot-id="chat.composer.hint"
        :context="composerContext"
        fallback="none"
        layout="inline"
      />
    </div>
  </div>
</template>

<style scoped>
.composer-extension-host {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.composer-extension-host__actions,
.composer-extension-host__attachments {
  display: flex;
  gap: 4px;
  align-items: center;
}
.composer-extension-host__hint {
  margin-left: auto;
  opacity: 0.7;
}
</style>
