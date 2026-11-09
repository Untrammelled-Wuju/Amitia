<template>
  <div class="page">
    <el-alert v-if="!checking && !canConfigure" title="模型由统筹中心统一管理。当前设备没有管理员权限，请在中心设备修改。" type="info" :closable="false" show-icon />
    <el-tabs v-if="canConfigure" :model-value="activeTab" @tab-change="onTabChange">
      <el-tab-pane label="普通模型" name="llm" />
      <el-tab-pane label="语音模型" name="voice" />
      <el-tab-pane label="语音识别" name="asr" />
      <el-tab-pane label="视觉模型" name="vision" />
      <el-tab-pane label="向量模型" name="embedding" />
      <el-tab-pane label="生图模型" name="imagegen" />
    </el-tabs>

    <div v-if="checking" v-loading="true" class="access-loading" />
    <component :is="activeModelView" v-else-if="canConfigure" />
  </div>
</template>

<script setup lang="ts">
import { computed, defineAsyncComponent, onMounted, type Component } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useCoreConfigurationAccess } from "@/composables/useCoreConfigurationAccess";

const route = useRoute();
const router = useRouter();
const { checking, canConfigure } = useCoreConfigurationAccess();

const views: Record<string, Component> = {
  llm: defineAsyncComponent(() => import("@/views/model-config/ModelConfigLlmView.vue")),
  voice: defineAsyncComponent(() => import("@/views/model-config/VoiceModelConfigView.vue")),
  asr: defineAsyncComponent(() => import("@/views/model-config/AsrModelConfigView.vue")),
  embedding: defineAsyncComponent(() => import("@/views/model-config/VectorModelConfigView.vue")),
  vision: defineAsyncComponent(() => import("@/views/model-config/VisionModelConfigView.vue")),
  imagegen: defineAsyncComponent(() => import("@/views/model-config/ImageGenModelConfigView.vue")),
};

const activeTab = computed(() => {
  const value = route.path.split("/").filter(Boolean).at(-1) ?? "llm";
  return views[value] ? value : "llm";
});
const activeModelView = computed(() => views[activeTab.value]);

function onTabChange(name: string | number) {
  if (views[String(name)]) void router.push(`/settings/model/${name}`);
}

onMounted(() => {
  if (route.path === "/settings/model") void router.replace("/settings/model/llm");
});
</script>

<style scoped>
.page {
  margin: 0 auto;
  padding: 20px 16px;
}
.access-loading { min-height: 120px; }
</style>
