<template>
  <section class="section-card">
    <div class="section-heading">
      <div>
        <h3>AI 表情策略</h3>
        <p>概率只决定是否检索表情；具体表情由用户填写的语义匹配。</p>
      </div>
      <el-switch
        v-model="form.enabled"
        :disabled="!characterId"
        @change="save"
      />
    </div>
    <div class="emote-settings" :class="{ disabled: !form.enabled }">
      <label
        ><span
          >基础发送概率
          <strong>{{ Math.round(form.baseProbability * 100) }}%</strong></span
        ><el-slider
          v-model="basePercent"
          :min="0"
          :max="30"
          :disabled="!form.enabled"
          @change="save"
      /></label>
      <label
        ><span
          >最大发送概率
          <strong>{{ Math.round(form.maxProbability * 100) }}%</strong></span
        ><el-slider
          v-model="maxPercent"
          :min="0"
          :max="50"
          :disabled="!form.enabled"
          @change="save"
      /></label>
      <label
        ><span>每小时最大发送数量</span
        ><el-input-number
          v-model="form.maxPerHour"
          :min="0"
          :max="20"
          :disabled="!form.enabled"
          @change="save"
      /></label>
      <label
        ><span>最小回复间隔</span
        ><el-input-number
          v-model="form.minReplyGap"
          :min="0"
          :max="20"
          :disabled="!form.enabled"
          @change="save"
        /><small>两次 AI 表情之间至少间隔的文字回复数</small></label
      >
      <label
        ><span>同一表情冷却时间</span
        ><el-input-number
          v-model="form.sameEmoteCooldownMinutes"
          :min="0"
          :max="1440"
          :disabled="!form.enabled"
          @change="save"
        /><small>分钟</small></label
      >
      <label
        ><span>允许纯表情回复</span
        ><el-switch
          v-model="form.allowEmoteOnly"
          :disabled="!form.enabled"
          @change="save"
        /><small>MVP 默认仍优先在文字后发送</small></label
      >
    </div>
    <p v-if="savedAt" class="saved" aria-live="polite">已保存 {{ savedAt }}</p>
  </section>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import { useApi } from "@/composables/useApi";

const props = defineProps<{ characterId: string }>();
const { get, put } = useApi();
const loading = ref(false);
const savedAt = ref("");
const form = reactive({
  enabled: true,
  baseProbability: 0.1,
  maxProbability: 0.3,
  maxPerHour: 5,
  minReplyGap: 3,
  sameEmoteCooldownMinutes: 30,
  allowEmoteOnly: false,
});
const basePercent = computed({
  get: () => Math.round(form.baseProbability * 100),
  set: (value) => {
    form.baseProbability = Number(value) / 100;
  },
});
const maxPercent = computed({
  get: () => Math.round(form.maxProbability * 100),
  set: (value) => {
    form.maxProbability = Number(value) / 100;
  },
});

async function load() {
  if (!props.characterId) return;
  loading.value = true;
  try {
    const data = await get<any>(
      `/api/characters/${props.characterId}/emote-settings`,
    );
    Object.assign(form, {
      ...data,
      enabled: !!data.enabled,
      allowEmoteOnly: !!data.allowEmoteOnly,
    });
  } finally {
    loading.value = false;
  }
}
async function save() {
  if (!props.characterId || loading.value) return;
  if (form.baseProbability > form.maxProbability) {
    form.maxProbability = form.baseProbability;
    ElMessage.warning("最大发送概率已同步为基础概率");
  }
  try {
    await put(`/api/characters/${props.characterId}/emote-settings`, form);
    savedAt.value = new Date().toLocaleTimeString([], {
      hour: "2-digit",
      minute: "2-digit",
    });
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.msg || "表情策略保存失败");
  }
}
watch(() => props.characterId, load, { immediate: true });
</script>

<style scoped>
.section-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}
.section-heading h3 {
  margin: 0;
  font-size: 14px;
}
.section-heading p {
  margin: 4px 0 0;
  color: var(--ac-color-text-muted);
  font-size: 11px;
}
.emote-settings {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px 24px;
  margin-top: 16px;
}
.emote-settings label {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.emote-settings label > span {
  display: flex;
  justify-content: space-between;
  color: var(--ac-color-text-secondary);
  font-size: 12px;
}
.emote-settings strong {
  color: var(--ac-color-primary);
}
.emote-settings small {
  color: var(--ac-color-text-muted);
  font-size: 10px;
}
.emote-settings.disabled {
  opacity: 0.65;
}
.saved {
  margin: 10px 0 0;
  text-align: right;
  color: var(--el-color-success);
  font-size: 11px;
}
@media (max-width: 700px) {
  .emote-settings {
    grid-template-columns: 1fr;
  }
}
</style>
