<template>
  <SecondaryWorkspace :title="title" :return-to="returnTo">
    <template #navigation>
      <nav class="account-navigation" :aria-label="`${title}导航`">
        <router-link v-for="section in sections" :key="section.id"
          :to="{ path: route.path, query: route.query, hash: `#${section.id}` }"
          class="account-navigation-item" :class="{ 'is-active': activeSection === section.id }"
          :aria-current="activeSection === section.id ? 'location' : undefined">
          {{ section.label }}
        </router-link>
      </nav>
    </template>
    <div ref="content" class="account-content"><slot /></div>
  </SecondaryWorkspace>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from "vue";
import { useRoute } from "vue-router";
import SecondaryWorkspace from "./SecondaryWorkspace.vue";
import { resolveSecondaryPage } from "../navigation/secondary-workspace";

defineProps<{ title: string; returnTo: string }>();
const route = useRoute();
const content = ref<HTMLElement>();
const sections = computed(() => resolveSecondaryPage(route.path)?.sections ?? []);
const activeSection = computed(() => sections.value.find(section => `#${section.id}` === route.hash)?.id ?? sections.value[0]?.id);

watch(() => route.hash, async (hash) => {
  const section = sections.value.find(item => `#${item.id}` === hash);
  if (!section) return;
  await nextTick();
  content.value?.querySelector(`#${section.id}`)?.scrollIntoView({ block: "start" });
}, { immediate: true, flush: "post" });
</script>

<style scoped>
.account-navigation { display: grid; align-content: start; gap: 4px; overflow-y: auto; padding: 0 16px 24px; }
.account-navigation-item { display: flex; align-items: center; min-height: 34px; padding: 6px 10px; box-sizing: border-box; border-radius: var(--ac-radius-sm); color: var(--text-secondary); font-size: var(--ac-font-size-sm); line-height: 1.4; text-decoration: none; transition: background-color var(--ac-transition-fast), color var(--ac-transition-fast); }
.account-navigation-item:hover { background: var(--control-hover-bg); color: var(--text-primary); }
.account-navigation-item.is-active { background: var(--control-active-bg); color: var(--text-primary); font-weight: 600; }
.account-navigation-item:active { background: var(--control-active-bg); transition: none; }
.account-navigation-item:focus-visible { outline: 2px solid var(--ac-color-primary); outline-offset: -2px; }
.account-content :deep([id]) { scroll-margin-top: 16px; }
@media (max-width: 767px) {
  .account-navigation { display: flex; overflow-x: auto; padding: 0 16px 12px; }
  .account-navigation-item { flex-shrink: 0; }
}
</style>
