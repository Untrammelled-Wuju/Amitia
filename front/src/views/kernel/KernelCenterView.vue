<template>
  <div class="kernel-redirect">
    <el-icon class="redirect-icon" :size="32"><Loading /></el-icon>
    <p>正在跳转到扩展中心...</p>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from "vue";
import { useRouter } from "vue-router";
import { Loading } from "@element-plus/icons-vue";

const router = useRouter();

onMounted(() => {
  router.replace("/extensions/packages");
});
</script>

<style scoped>
.kernel-redirect {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  height: 100%;
  color: var(--console-text-muted);
}
.redirect-icon {
  animation: spin 1.5s linear infinite;
}
@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
</style>
