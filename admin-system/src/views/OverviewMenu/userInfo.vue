<script setup lang="ts">
import { computed } from 'vue'
import { useAuthStore } from '@/stores/authStore'
const authStore = useAuthStore()
const initial = computed(() => (authStore.user?.username || '?').slice(0, 1).toUpperCase())
</script>
<template>
  <div class="userInfo">
    <span class="userAvatar" aria-hidden="true">{{ initial }}</span>
    <div class="userName">{{ authStore.user?.username }}</div>
  </div>
</template>
<style scoped>
.userInfo {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 10px 4px 4px;
  border-radius: 999px;
  cursor: pointer;
  transition: background-color var(--transitionTime) ease;
  &:hover {
    background-color: color-mix(in srgb, var(--Main_font_SecondColor) 7%, transparent);
  }
  .userAvatar {
    display: grid;
    width: 28px;
    height: 28px;
    place-items: center;
    border-radius: 50%;
    background: var(--BrandPrimarySoft);
    color: var(--BrandPrimary);
    font-size: 13px;
    font-weight: 600;
  }
  .userName {
    font-size: 14px;
    font-weight: 500;
    color: var(--Main_font_SecondColor);
  }
}
</style>
