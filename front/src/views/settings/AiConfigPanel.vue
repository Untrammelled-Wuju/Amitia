<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="ai-config-panel">
    <el-card shadow="never" class="section-card">
      <template #header><span>AI回复风格提示词</span></template>
      <el-alert
        type="warning"
        :closable="false"
        show-icon
        style="margin-bottom: 12px"
      >
        <template #title
          >此提示词影响 AI
          的回复风格。默认配置经过优化，<strong>如非必要请勿修改</strong>，修改不当可能导致回复质量下降。</template
        >
      </el-alert>
      <el-form :model="styleForm" label-width="0">
        <el-form-item>
          <el-input
            v-model="styleForm.prompt"
            type="textarea"
            :rows="12"
            placeholder="AI回复风格提示词..."
          />
        </el-form-item>
        <el-form-item>
          <el-button
            type="primary"
            @click="saveStylePrompt"
            :loading="savingPrompt"
            >保存</el-button
          >
          <el-button @click="resetStylePrompt">恢复默认</el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from "vue";
import axios from "axios";
import { ElMessage } from "element-plus";
import { getApiBaseURL } from "../../runtime/runtime-adapter";

const apiBaseUrl = ref("");
const savingPrompt = ref(false);

const DEFAULT_STYLE_PROMPT =
  "你和用户是比较熟悉的长期对话关系，不需要像客服或正式助手一样说话。回复要自然、有反应、有一点态度，可以适当使用「嗯？、喔、奥奥、ok、好、行、确实、懂了」等语气词。用户随口聊，你就自然接话；用户认真问问题，你再认真回答。不要客服腔，不要过度正式，不要每次都完整总结，也不要动不动分点讲大道理。回复格式要像微信连续消息：用户发一句话时，你可以回复 1 到 4 句短句。不要写成一整段长文。整体目标是：像一个熟悉用户、说话自然、有判断力的人。该短就短，该认真就认真，不端着，也不表演过头。回复中不要使用任何emoji表情符号。不能使用markdown格式。";

const styleForm = reactive({ prompt: DEFAULT_STYLE_PROMPT });

async function saveStylePrompt() {
  savingPrompt.value = true;
  try {
    await axios.put(apiBaseUrl.value + "/api/config", {
      settings: { wechat_style_prompt: styleForm.prompt },
    });
    ElMessage.success("AI回复风格提示词已保存");
  } catch (err: any) {
    ElMessage.error("保存失败: " + err.message);
  } finally {
    savingPrompt.value = false;
  }
}

function resetStylePrompt() {
  styleForm.prompt = DEFAULT_STYLE_PROMPT;
}

async function loadStylePrompt() {
  try {
    const { data } = await axios.get(apiBaseUrl.value + "/api/config");
    if (data?.data?.settings?.wechat_style_prompt) {
      styleForm.prompt = data.data.settings.wechat_style_prompt;
    }
  } catch {}
}

onMounted(async () => {
  apiBaseUrl.value = await getApiBaseURL();
  loadStylePrompt();
});
</script>

<style scoped>
.ai-config-panel {
}
.section-card {
  margin-bottom: 12px;
  border: 1px solid var(--ac-color-border-light);
}
</style>
