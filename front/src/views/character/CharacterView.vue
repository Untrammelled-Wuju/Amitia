<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="char-layout">
    <div class="char-sidebar">
      <div class="sidebar-header">
        <h3>角色</h3>
        <div class="sidebar-actions">
          <el-button size="small" @click="openTemplates">模板</el-button>
          <el-button size="small" @click="showImportDialog = true">导入</el-button>
          <el-button size="small" type="primary" @click="openCreate">+</el-button>
        </div>
      </div>
      <div class="char-list">
        <div
          v-for="c in characters"
          :key="c.id"
          class="char-item"
          :class="{ active: selectedId === String(c.id) }"
          @click="selectChar(c)"
        >
          <el-avatar
            :size="28"
            :src="c.avatar || undefined"
            style="flex-shrink: 0; margin-right: 6px"
            >{{ c.name?.charAt(0) }}</el-avatar
          >
          <div style="flex: 1; min-width: 0">
            <span class="char-name">{{ c.name }}</span>
          </div>
        </div>
        <el-empty
          v-if="!characters.length"
          description="暂无角色"
          :image-size="40"
        />
      </div>
      <ExtensionSlot
        slot-id="character.sidebar.card"
        :context="characterExtensionContext"
        fallback="none"
        layout="stack"
        surface-role="sidebar"
      />
    </div>

    <div class="char-main">
      <template v-if="selectedId">
        <div class="detail-top">
          <h2>{{ selectedChar?.name }}</h2>
          <el-button size="small" @click="editCurrent">编辑</el-button>
          <el-button size="small" @click="copyCurrentCharacter">复制</el-button>
          <el-button size="small" @click="goToChatLogs">聊天记录</el-button>
          <el-button size="small" :loading="exportingPack" @click="exportCurrentCharacter">导出角色卡</el-button>
          <el-button size="small" type="danger" @click="deleteCurrent"
            >删除</el-button
          >
          <ExtensionSlot
            slot-id="character.detail.action"
            :context="characterExtensionContext"
            fallback="none"
            layout="inline"
            surface-role="header"
          />
        </div>
        <el-tabs
          :model-value="activeTab"
          @tab-change="onTabChange"
          type="border-card"
        >
          <el-tab-pane label="生活规则" name="life-rules">
            <AiCharacterSettingsView
              v-if="activeTab === 'life-rules'"
              :key="`life-${selectedId}`"
            />
          </el-tab-pane>
          <el-tab-pane label="拟态语音" name="voice">
            <CharacterVoiceView
              v-if="activeTab === 'voice'"
              :key="`voice-${selectedId}`"
            />
          </el-tab-pane>
          <el-tab-pane label="记忆" name="memory">
            <MemoryManagerView
              v-if="activeTab === 'memory'"
              :key="`memory-${selectedId}`"
            />
          </el-tab-pane>
          <el-tab-pane label="时间线" name="timeline">
            <MemoryTimeline
              v-if="activeTab === 'timeline'"
              :key="`timeline-${selectedId}`"
            />
          </el-tab-pane>
          <el-tab-pane label="主动消息" name="proactive">
            <ProactiveRulesView
              v-if="activeTab === 'proactive'"
              :key="`pro-${selectedId}`"
            />
          </el-tab-pane>
          <el-tab-pane label="调试" name="debug">
            <CompanionDebugView
              v-if="activeTab === 'debug'"
              :key="`dbg-${selectedId}`"
            />
          </el-tab-pane>
          <el-tab-pane label="心理状态" name="psyche">
            <CharacterPsycheView
              v-if="activeTab === 'psyche'"
              :key="`psyche-${selectedId}`"
            />
          </el-tab-pane>
        </el-tabs>
        <ExtensionSlot
          slot-id="character.detail.tab"
          :context="characterExtensionContext"
          fallback="none"
          layout="tabs"
          surface-role="main"
          class="character-detail-slot"
        />
      </template>
      <el-empty
        v-else
        description="左侧选择一个角色"
        :image-size="60"
        style="margin-top: 80px"
      />
    </div>

    <el-dialog
      v-model="showDialog"
      :title="editingId ? '编辑角色' : '创建角色'"
      width="640px"
    >
      <el-form :model="form" label-width="80px">
        <el-form-item label="头像">
          <div class="avatar-upload-row">
            <div
              class="avatar-preview"
              @click="triggerAvatarUpload"
              :title="form.avatar ? '点击更换头像' : '点击上传头像'"
            >
              <img
                v-if="form.avatar"
                :src="form.avatar"
                class="avatar-img"
                @error="onAvatarImgError"
              />
              <span v-else class="avatar-placeholder">+</span>
            </div>
            <input
              ref="avatarInputRef"
              type="file"
              accept="image/*"
              hidden
              @change="onAvatarFileChange"
            />
            <el-input
              v-model="form.avatar"
              placeholder="或输入URL"
              size="small"
              style="flex: 1"
            />
          </div>
        </el-form-item>
        <el-form-item label="名称"
          ><el-input v-model="form.name"
        /></el-form-item>
        <el-form-item label="描述"
          ><el-input v-model="form.description" type="textarea" :rows="2"
        /></el-form-item>
        <el-form-item label="性格"
          ><el-input v-model="form.personality" type="textarea" :rows="3"
        /></el-form-item>
        <el-form-item label="身份设定">
          <el-input v-model="form.identity" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item label="说话风格">
          <el-input v-model="form.speakingStyle" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item label="关系风格">
          <el-input v-model="form.relationshipStyle" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item label="提示词">
          <el-input v-model="form.characterBase" type="textarea" :rows="4" />
        </el-form-item>
        <el-form-item label="边界规则">
          <el-input v-model="form.boundaryRules" type="textarea" :rows="3" />
        </el-form-item>
        <el-form-item label="基础 Prompt">
          <el-input v-model="form.basePrompt" type="textarea" :rows="3" />
        </el-form-item>

        <el-divider content-position="left">高级配置</el-divider>
        <el-form-item label="性格 JSON">
          <el-input v-model="form.personalityConfig" type="textarea" :rows="4" placeholder='{"openness": 50}' />
        </el-form-item>
        <el-form-item label="聊天风格">
          <el-input v-model="form.chatStyleConfig" type="textarea" :rows="4" placeholder="JSON 对象" />
        </el-form-item>
        <el-form-item label="场景规则">
          <el-input v-model="form.sceneRules" type="textarea" :rows="4" placeholder="JSON 对象" />
        </el-form-item>

        <el-divider content-position="left">语音配置</el-divider>

        <el-form-item label="音色">
          <el-select
            v-model="form.voiceType"
            style="width: 100%"
            filterable
            placeholder="选择音色"
            @change="onVoiceTypeChange"
          >
            <el-option
              v-for="v in voicePresets"
              :key="v.name"
              :label="v.label"
              :value="v.name"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="语速">
          <el-slider
            v-model="form.voiceSpeed"
            :min="0.5"
            :max="2.0"
            :step="0.1"
            show-input
            :format-tooltip="(v: number) => v.toFixed(1) + 'x'"
            style="width: 70%"
          />
        </el-form-item>
        <el-form-item label="音调">
          <el-slider
            v-model="form.voicePitch"
            :min="0.5"
            :max="2.0"
            :step="0.05"
            show-input
            :format-tooltip="(v: number) => v.toFixed(2) + 'x'"
            style="width: 70%"
          />
        </el-form-item>
        <el-form-item label="试听">
          <el-button size="small" @click="testVoice" :loading="testingVoice"
            >试听</el-button
          >
          <audio
            v-if="testAudioUrl"
            :src="testAudioUrl"
            controls
            autoplay
            style="width: 260px; margin-left: 10px; height: 30px"
          />
        </el-form-item>

        <el-divider content-position="left">声音复刻</el-divider>

        <el-form-item label="复刻音色ID">
          <el-input
            v-model="form.customVoiceId"
            placeholder="输入音色ID，如 S_xxxxxxxx"
            style="width: 240px"
            clearable
          />
          <span
            style="
              font-size: 11px;
              color: var(--ac-color-text-muted);
              margin-left: 8px;
            "
            >在火山控制台训练后填入</span
          >
        </el-form-item>
        <el-form-item label="试听" v-if="form.customVoiceId">
          <el-button
            size="small"
            @click="previewClone"
            :loading="previewCloneLoading"
            >试听</el-button
          >
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showDialog = false">取消</el-button>
        <el-button type="primary" @click="saveCharacter" :loading="saving"
          >保存</el-button
        >
      </template>
    </el-dialog>

    <TemplatePickerDialog
      v-model="showTemplateDialog"
      :templates="templates"
      :loading="templateLoading"
      @select="createFromTemplate"
    />

    <ImportPackDialog
      v-model="showImportDialog"
      v-model:pack-name="importPackName"
      v-model:confirm-text="importConfirmText"
      :preview="importPreview"
      :previewing="importPreviewing"
      :importing="importing"
      :history="packHistory"
      @preview="previewImport"
      @cancel-preview="cancelImportPreview"
      @confirm="confirmImportAndReload"
      @file-selected="setSelectedFile"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, watch, provide } from "vue";
import { useRouter, useRoute } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import { apiClient } from "@/composables/useApi";
import {
  AiCharacterSettingsView,
  CharacterVoiceView,
  ProactiveRulesView,
  CompanionDebugView,
} from "../../ui-index";
import CharacterPsycheView from "./CharacterPsycheView.vue";
import MemoryManagerView from "@/views/memory-manager/MemoryManagerView.vue";
import MemoryTimeline from "@/views/memory-timeline/MemoryTimeline.vue";
import ImportPackDialog from "@/views/character-config/components/ImportPackDialog.vue";
import TemplatePickerDialog from "@/views/character-config/components/TemplatePickerDialog.vue";
import ExtensionSlot from "@/components/extension/ExtensionSlot.vue";
import type { TemplateItem } from "@/views/character-config/composables/types";
import { normalizeVoicePitchRatio } from "@/utils/voicePitch";
import { useCharacterImportExport } from "@/views/character-config/composables/useCharacterImportExport";

const router = useRouter();
const route = useRoute();
const selectedId = ref<string | null>(null);
const selectedChar = ref<any>(null);
const {
  exportingPack,
  showImportDialog,
  importPackName,
  importPreview,
  importPreviewing,
  importConfirmText,
  importing,
  packHistory,
  exportPack,
  previewImport,
  confirmImport,
  loadPackHistory,
  cancelImportPreview,
  setSelectedFile,
} = useCharacterImportExport();

const currentCharacterId = computed(() => selectedId.value);
provide("currentCharacterId", currentCharacterId);

function goToChatLogs() {
  if (!selectedId.value) return;
  router.push({ path: "/logs", query: { characterId: selectedId.value } });
}

async function exportCurrentCharacter() {
  if (!selectedId.value) return;
  await exportPack(selectedId.value, selectedChar.value?.name || "character");
}

async function confirmImportAndReload() {
  const result = await confirmImport();
  if (!result) return;
  await loadCharacters();
  const importedId = String(
    result?.characterId || result?.id || result?.character?.id || "",
  );
  const imported = importedId
    ? characters.value.find((item: any) => String(item.id) === importedId)
    : characters.value.at(-1);
  if (imported) selectChar(imported);
}

const characters = ref<any[]>([]);
const templates = ref<TemplateItem[]>([]);
const showTemplateDialog = ref(false);
const templateLoading = ref(false);
const showDialog = ref(false);
const editingId = ref<string | null>(null);
const saving = ref(false);
const voicePresets = ref<any[]>([]);
const avatarInputRef = ref<HTMLInputElement>();

function triggerAvatarUpload() {
  avatarInputRef.value?.click();
}
function onAvatarFileChange(e: Event) {
  const input = e.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;
  if (!file.type.startsWith("image/")) {
    ElMessage.warning("请选择图片文件");
    return;
  }
  const reader = new FileReader();
  reader.onload = () => {
    form.avatar = reader.result as string;
  };
  reader.readAsDataURL(file);
  input.value = "";
}
function onAvatarImgError(e: Event) {
  const img = e.target as HTMLImageElement;
  img.style.display = "none";
}
const currentVoiceSupportsEmotion = computed(() => {
  const v = voicePresets.value.find((p: any) => p.name === form.voiceType);
  return v?.supportsEmotion ?? false;
});
const globalApiKey = ref("");

const form = reactive({
  name: "",
  description: "",
  personality: "",
  avatar: "",
  identity: "",
  speakingStyle: "",
  relationshipStyle: "",
  characterBase: "",
  boundaryRules: "",
  basePrompt: "",
  personalityConfig: "{}",
  chatStyleConfig: "{}",
  sceneRules: "{}",
  voiceType: "zh_female_vv_uranus_bigtts",
  voiceSpeed: 1.0,
  voicePitch: 1.0,
  voiceVolume: 1.0,
  customVoiceId: "",
  emotion: "",
  emotionScale: 0,
  silenceDuration: 0,
});

const testingVoice = ref(false);
const testAudioUrl = ref("");
const cloneFile = ref<File | null>(null);
const cloneName = ref("");
const cloneLoading = ref(false);
const cloneResult = ref("");
const previewCloneLoading = ref(false);


const activeTab = computed(() => {
  const p = route.path;
  if (p.endsWith("/voice")) return "voice";
  if (p.endsWith("/memory")) return "memory";
  if (p.endsWith("/timeline")) return "timeline";
  if (p.endsWith("/proactive")) return "proactive";
  if (p.endsWith("/debug")) return "debug";
  if (p.endsWith("/psyche")) return "psyche";
  return "life-rules";
});

const characterExtensionContext = computed(() => ({
  characterId: selectedId.value,
  characterName: selectedChar.value?.name ?? "",
  activeTab: activeTab.value,
  surface: "character-detail",
}));

onMounted(async () => {
  await Promise.allSettled([loadPackHistory(), loadTemplates()]);
  await loadVoices();
  await loadGlobalApiKey();
  await loadCharacters();
  const id = route.params.id as string;
  if (id) {
    selectedId.value = id;
    const c = characters.value.find((x: any) => String(x.id) === id);
    if (c) selectedChar.value = c;
  }
});

watch(
  () => characters.value,
  () => {
    const id = route.params.id as string;
    if (id) {
      const c = characters.value.find((x: any) => String(x.id) === id);
      if (c) selectedChar.value = c;
    }
  },
);


async function loadTemplates() {
  templateLoading.value = true;
  try {
    const r = await apiClient.get("/api/character-templates");
    const data = r.data?.data || r.data || [];
    templates.value = Array.isArray(data) ? data : [];
  } catch {
    templates.value = [];
  } finally {
    templateLoading.value = false;
  }
}

async function openTemplates() {
  if (!templates.value.length) await loadTemplates();
  showTemplateDialog.value = true;
}

async function createFromTemplate(tpl: TemplateItem) {
  try {
    const r = await apiClient.post(`/api/character-templates/${tpl.id}/create-character`, { name: tpl.name });
    const created = r.data?.data || r.data;
    showTemplateDialog.value = false;
    await loadCharacters();
    const id = String(created?.id || created?.characterId || "");
    const target = characters.value.find((c: any) => String(c.id) === id) || created;
    if (target?.id) selectChar(target);
    ElMessage.success("已从模板创建角色");
  } catch (err: any) {
    ElMessage.error(err?.message || "从模板创建失败");
  }
}

async function loadVoices() {
  try {
    voicePresets.value = await apiClient
      .get("/api/tts/voices")
      .then((r) => (Array.isArray(r.data) ? r.data : []));
  } catch {
    voicePresets.value = [];
  }
}

async function loadGlobalApiKey() {
  try {
    const configs = await apiClient
      .get("/api/tts/configs")
      .then((r) => (Array.isArray(r.data) ? r.data : []));
    const active = configs.find((c: any) => c.isActive);
    if (active) globalApiKey.value = active.apiKey || "";
  } catch {}
}

async function loadCharacters() {
  try {
    const r = await apiClient.get("/api/characters");
    characters.value = r.data?.data || r.data || [];
  } catch {}
}

function selectChar(c: any) {
  selectedId.value = String(c.id);
  selectedChar.value = c;
  router.push(`/character/${c.id}/life-rules`);
}

function onTabChange(tab: string) {
  if (selectedId.value) router.push(`/character/${selectedId.value}/${tab}`);
}

function openCreate() {
  editingId.value = null;
  form.name = "";
  form.description = "";
  form.personality = "";
  form.avatar = "";
  form.identity = "";
  form.speakingStyle = "";
  form.relationshipStyle = "";
  form.characterBase = "";
  form.boundaryRules = "";
  form.basePrompt = "";
  form.personalityConfig = "{}";
  form.chatStyleConfig = "{}";
  form.sceneRules = "{}";
  form.voiceType = "zh_female_vv_uranus_bigtts";
  form.voiceSpeed = 1.0;
  form.voicePitch = 1.0;
  form.voiceVolume = 1.0;
  form.customVoiceId = "";
  form.emotion = "";
  form.emotionScale = 0;
  form.silenceDuration = 0;
  cloneFile.value = null;
  cloneName.value = "";
  cloneResult.value = "";
  showDialog.value = true;
}

function editCurrent() {
  if (!selectedChar.value) return;
  editingId.value = selectedChar.value.id;
  form.name = selectedChar.value.name || "";
  form.description = selectedChar.value.description || "";
  form.avatar = selectedChar.value.avatar || "";
  form.personality = selectedChar.value.personality || "";
  form.identity = selectedChar.value.identity || "";
  form.speakingStyle = selectedChar.value.speakingStyle || "";
  form.relationshipStyle = selectedChar.value.relationshipStyle || "";
  form.characterBase = selectedChar.value.characterBase || "";
  form.boundaryRules = selectedChar.value.boundaryRules || "";
  form.basePrompt = selectedChar.value.basePrompt || "";
  form.personalityConfig = prettyJson(selectedChar.value.personalityConfig);
  form.chatStyleConfig = prettyJson(selectedChar.value.chatStyleConfig);
  form.sceneRules = prettyJson(selectedChar.value.sceneRules);
  form.voiceType = selectedChar.value.voiceType || "zh_female_vv_uranus_bigtts";
  form.voiceSpeed = selectedChar.value.voiceSpeed ?? 1.0;
  form.voicePitch = normalizeVoicePitchRatio(selectedChar.value.voicePitch);
  form.voiceVolume = selectedChar.value.voiceVolume ?? 1.0;
  form.customVoiceId = selectedChar.value.customVoiceId || "";
  form.emotion = selectedChar.value.emotion || "";
  form.emotionScale = selectedChar.value.emotionScale ?? 0;
  form.silenceDuration = selectedChar.value.silenceDuration ?? 0;
  cloneFile.value = null;
  cloneName.value = "";
  cloneResult.value = "";
  showDialog.value = true;
}


function prettyJson(value: unknown): string {
  if (value == null || value === "") return "{}";
  try {
    const parsed = typeof value === "string" ? JSON.parse(value) : value;
    return JSON.stringify(parsed ?? {}, null, 2);
  } catch {
    return String(value);
  }
}

function parseJsonObject(value: string, label: string): Record<string, unknown> {
  try {
    const parsed = JSON.parse(value || "{}");
    if (!parsed || Array.isArray(parsed) || typeof parsed !== "object") {
      throw new Error(`${label} 必须是 JSON 对象`);
    }
    return parsed as Record<string, unknown>;
  } catch (err: any) {
    throw new Error(err?.message?.includes(label) ? err.message : `${label} JSON 格式错误`);
  }
}

function copyCurrentCharacter() {
  if (!selectedChar.value) return;
  const source = selectedChar.value;
  openCreate();
  form.name = `${source.name || "角色"} (副本)`;
  form.avatar = source.avatar || "";
  form.description = source.description || "";
  form.identity = source.identity || "";
  form.personality = source.personality || "";
  form.speakingStyle = source.speakingStyle || "";
  form.relationshipStyle = source.relationshipStyle || "";
  form.characterBase = source.characterBase || "";
  form.boundaryRules = source.boundaryRules || "";
  form.basePrompt = source.basePrompt || "";
  form.personalityConfig = prettyJson(source.personalityConfig);
  form.chatStyleConfig = prettyJson(source.chatStyleConfig);
  form.sceneRules = prettyJson(source.sceneRules);
  form.voiceType = source.voiceType || "zh_female_vv_uranus_bigtts";
  form.voiceSpeed = source.voiceSpeed ?? 1.0;
  form.voicePitch = normalizeVoicePitchRatio(source.voicePitch);
  form.voiceVolume = source.voiceVolume ?? 1.0;
  form.customVoiceId = source.customVoiceId || "";
  form.emotion = source.emotion || "";
  form.emotionScale = source.emotionScale ?? 0;
  form.silenceDuration = source.silenceDuration ?? 0;
  ElMessage.success("已复制角色配置，请保存为新角色");
}

function onVoiceTypeChange() {
  const v = voicePresets.value.find((p: any) => p.name === form.voiceType);
  if (v) {
    if (v.supportsEmotion) {
      if (!form.emotion) form.emotion = "happy";
    } else {
      form.emotion = "";
    }
  }
}

async function testVoice() {
  testingVoice.value = true;
  testAudioUrl.value = "";
  try {
    const res = await apiClient.post("/api/tts/synthesize", {
      voiceType: form.voiceType,
      text: "你好，我是你的AI伙伴",
      speedRatio: form.voiceSpeed,
      pitchRatio: form.voicePitch,
      volumeRatio: form.voiceVolume,
      emotion: form.emotion || undefined,
      emotionScale: form.emotionScale || undefined,
      silenceDuration: form.silenceDuration || undefined,
    });
    const json: any = res.data;
    testAudioUrl.value = json?.audioUrl || json?.data?.audioUrl || "";
  } catch {
  } finally {
    testingVoice.value = false;
  }
}

async function ensureTtsConfig() {
  if (!globalApiKey.value) return;
  const configs = await apiClient
    .get("/api/tts/configs")
    .then((r) => (Array.isArray(r.data) ? r.data : []));
  const existing = configs.find((c: any) => c.isActive);
  if (existing) {
    if (!existing.hasApiKey)
      await apiClient.put(`/api/tts/configs/${existing.id}`, {
        apiKey: globalApiKey.value,
      });
  } else {
    await apiClient.post("/api/tts/configs", {
      name: "默认配置",
      apiKey: globalApiKey.value,
      voiceType: form.voiceType,
      isActive: 1,
    });
  }
}

async function submitClone() {
  if (!cloneFile.value || !cloneName.value.trim()) return;
  if (!globalApiKey.value) {
    ElMessage.warning("请先设置API Key");
    return;
  }
  cloneLoading.value = true;
  cloneResult.value = "";
  try {
    const fd = new FormData();
    fd.append("audio", cloneFile.value);
    fd.append("name", cloneName.value.trim());
    fd.append("language", "cn");

    const url =
      "/api/tts/voice-clone?apiKey=" + encodeURIComponent(globalApiKey.value);
    const resp = await apiClient.post(url, fd);
    const json: any = resp.data;
    const speakerId = json?.speakerId || json?.data?.speakerId || "";
    if (!speakerId) {
      ElMessage.error(json?.message || "复刻失败");
      return;
    }
    form.customVoiceId = speakerId;
    cloneResult.value = "复刻成功: " + speakerId;
    ElMessage.success("声音复刻成功");
  } catch (err: any) {
    ElMessage.error(err?.message || "复刻失败");
  } finally {
    cloneLoading.value = false;
  }
}

async function previewClone() {
  if (!form.customVoiceId) return;
  previewCloneLoading.value = true;
  testAudioUrl.value = "";
  try {
    const configs = await apiClient
      .get("/api/tts/configs")
      .then((r) => (Array.isArray(r.data) ? r.data : []));
    const cfg = configs.find((c: any) => c.isActive) || configs[0];
    if (!cfg) {
      ElMessage.warning("未找到音色配置");
      return;
    }
    await apiClient.put(`/api/tts/configs/${cfg.id}`, {
      voiceType: form.customVoiceId,
    });
    const res = await apiClient.post("/api/tts/synthesize", {
      speakerId: form.customVoiceId,
      text: "复刻音色试听",
    });
    testAudioUrl.value =
      (res as any)?.data?.audioUrl || (res as any)?.audioUrl || "";
  } catch (err: any) {
    ElMessage.error(err?.message || "试听失败");
  } finally {
    previewCloneLoading.value = false;
  }
}

async function saveCharacter() {
  saving.value = true;
  try {
    if (!form.name.trim()) {
      ElMessage.warning("请输入角色名称");
      return;
    }
    const personalityConfig = parseJsonObject(form.personalityConfig, "性格配置");
    parseJsonObject(form.chatStyleConfig, "聊天风格配置");
    parseJsonObject(form.sceneRules, "场景规则");
    const payload: any = {
      name: form.name,
      avatar: form.avatar,
      description: form.description,
      personality: form.personality,
      identity: form.identity,
      speakingStyle: form.speakingStyle,
      relationshipStyle: form.relationshipStyle,
      characterBase: form.characterBase,
      boundaryRules: form.boundaryRules,
      basePrompt: form.basePrompt,
      personalityConfig,
      chatStyleConfig: form.chatStyleConfig || "{}",
      sceneRules: form.sceneRules || "{}",
      voiceType: form.voiceType,
      voiceSpeed: form.voiceSpeed,
      voicePitch: normalizeVoicePitchRatio(form.voicePitch),
      voiceVolume: form.voiceVolume,
      customVoiceId: form.customVoiceId,
      emotion: form.emotion || "",
      emotionScale: form.emotionScale || 0,
      silenceDuration: form.silenceDuration || 0,
    };

    if (editingId.value) {
      await apiClient.put(`/api/characters/${editingId.value}`, payload);
    } else {
      const r = await apiClient.post("/api/characters", payload);
      const created = r.data?.data || r.data;
      if (created) {
        selectedId.value = String(created.id);
        selectedChar.value = created;
        router.push(`/character/${created.id}/life-rules`);
      }
    }
    ElMessage.success("已保存");
    showDialog.value = false;
    await loadCharacters();
    window.dispatchEvent(
      new CustomEvent("character-updated", {
        detail: {
          id: editingId.value || selectedId.value,
          avatar: form.avatar,
        },
      }),
    );
  } catch (err: any) {
    ElMessage.error(err?.message || "保存失败");
  } finally {
    saving.value = false;
  }
}

async function deleteCurrent() {
  if (!selectedChar.value) return;
  try {
    await ElMessageBox.confirm(
      "确定删除「" + selectedChar.value.name + "」？",
      "确认",
      { type: "warning" },
    );
    await apiClient.delete(`/api/characters/${selectedChar.value.id}`);
    ElMessage.success("已删除");
    selectedId.value = null;
    selectedChar.value = null;
    router.push("/character");
    await loadCharacters();
  } catch {}
}
</script>

<style scoped>
.avatar-upload-row {
  display: flex;
  align-items: center;
  gap: 8px;
}
.avatar-preview {
  width: 64px;
  height: 64px;
  border-radius: 50%;
  border: 2px dashed var(--ac-color-border);
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  overflow: hidden;
  flex-shrink: 0;
  background: var(--ac-color-surface);
  transition: border-color 0.2s;
}
.avatar-preview:hover {
  border-color: var(--ac-color-primary);
}
.avatar-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.avatar-placeholder {
  font-size: 28px;
  color: var(--ac-color-text-muted);
}
.char-layout {
  display: flex;
  height: 100%;
  min-height: 100%;
  gap: 0;
}
.char-sidebar {
  width: 200px;
  flex-shrink: 0;
  border-right: 1px solid var(--el-border-color-light);
  background: var(--el-bg-color);
  display: flex;
  flex-direction: column;
}
.sidebar-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}
.sidebar-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}

.sidebar-header h3 {
  font-size: 15px;
  font-weight: 600;
  margin: 0;
}
.char-list {
  flex: 1;
  overflow-y: auto;
  padding: 4px;
}
.char-item {
  padding: 10px 12px;
  cursor: pointer;
  border-radius: 6px;
  margin: 2px 0;
  display: flex;
  align-items: center;
  gap: 8px;
  transition: background 0.15s;
}
.char-item:hover {
  background: var(--el-fill-color-light);
}
.char-item.active {
  background: var(--el-color-primary-light-9);
}
.char-name {
  font-size: 14px;
  font-weight: 500;
}
.char-main {
  flex: 1;
  overflow-y: auto;
  padding: 16px 20px;
}
.detail-top {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
}
.detail-top h2 {
  font-size: 18px;
  font-weight: 600;
  margin: 0;
  flex: 1;
}
</style>
