<script setup lang="ts">
import { computed } from "vue";
import Reminders from "@/views/reminders/Reminders.vue";

const props = withDefaults(defineProps<{ context?: Record<string, unknown> }>(), {
  context: () => ({}),
});

const characterId = computed(() => String(props.context.characterId || props.context.id || ""));
</script>

<template>
  <Reminders :initial-character-id="characterId" />
</template>
