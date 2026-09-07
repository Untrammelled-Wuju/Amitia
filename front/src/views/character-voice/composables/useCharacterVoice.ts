// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, reactive, inject, onMounted, computed, type Ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { apiClient } from "../../../ui-index";
import { normalizeVoicePitchRatio } from "@/utils/voicePitch";

interface VoicePreset {
  name: string;
  label: string;
  gender: string;
}

export interface TtsConfigSummary {
  id: number;
  name: string;
  apiType?: string;
  resourceId?: string;
  voiceType?: string;
  isActive?: number;
}

interface ClonedVoiceSummary {
  speakerId: string;
  name: string;
  language?: number;
  status?: string;
  createdAt?: string;
  updatedAt?: string;
}

export function useCharacterVoice() {
  const injectedCharacterId = inject<Ref<string | null>>(
    "currentCharacterId",
    ref(null),
  );

  const voicePresets = ref<VoicePreset[]>([]);
  const voiceConfigs = ref<TtsConfigSummary[]>([]);
  const emotions = [
    { value: "", label: "无" },
    { value: "happy", label: "开心" },
    { value: "sad", label: "悲伤" },
    { value: "angry", label: "愤怒" },
    { value: "fearful", label: "恐惧" },
    { value: "surprised", label: "惊讶" },
    { value: "neutral", label: "中性" },
  ];
  const saving = ref(false);
  const previewLoading = ref(false);
  const previewText = ref("你好，我是你的专属角色");
  const previewAudio = ref("");
  const voiceMode = ref<"preset" | "clone">("preset");

  const form = reactive({
    voiceType: "zh_female_vv_uranus_bigtts",
    voiceSpeed: 1.0,
    voicePitch: 1.0,
    voiceVolume: 1.0,
    customVoiceId: "",
    voiceConfigId: "",
    emotion: "",
    emotionScale: 4,
    silenceDuration: 0,
  });

  const originalForm = reactive({
    ...form,
    _mode: "preset" as string,
    emotion: "",
    emotionScale: 4,
    silenceDuration: 0,
  });

  const trainSpeakerId = ref("");
  const trainVoiceName = ref("");
  const cloneFile = ref<File | null>(null);
  const cloneFileList = ref<any[]>([]);
  const trainLoading = ref(false);
  const trainResult = ref("");
  const clonedVoices = ref<ClonedVoiceSummary[]>([]);
  const previewCloneId = ref("");

  const currentVoiceSupportsEmotion = computed(() => {
    if (voiceMode.value !== "preset") return false;
    const v = voicePresets.value.find((p) => p.name === form.voiceType);
    return !!v;
  });

  function onModeChange(mode: string) {
    if (mode !== "preset") {
      form.emotion = "";
      form.emotionScale = 4;
    }
  }

  function selectCloneVoice(speakerId: string) {
    form.customVoiceId = speakerId;
  }

  function onVoiceTypeChange() {
    if (!currentVoiceSupportsEmotion.value) {
      form.emotion = "";
      form.emotionScale = 4;
    }
  }

  function onCloneFileChange(file: any) {
    cloneFile.value = file?.raw || file;
  }

  async function loadVoicePresets() {
    try {
      const r = await apiClient.get("/api/tts/voices");
      const data = r.data?.data || r.data;
      if (Array.isArray(data)) voicePresets.value = data;
    } catch {
      voicePresets.value = [];
    }
  }

  async function loadVoiceConfigs() {
    try {
      const r = await apiClient.get("/api/tts/config-summaries");
      const data = r.data?.data || r.data;
      voiceConfigs.value = Array.isArray(data) ? data : [];
    } catch {
      voiceConfigs.value = [];
    }
  }

  async function loadClonedVoices() {
    try {
      const r = await apiClient.get("/api/tts/voice-clones");
      const data = r.data?.data || r.data;
      clonedVoices.value = Array.isArray(data) ? data : [];
    } catch (err: any) {
      clonedVoices.value = [];
      ElMessage.error(err?.message || "加载复刻音色失败");
    }
  }

  async function loadCharacterVoice() {
    const cid = injectedCharacterId.value;
    if (!cid) return;
    try {
      const r = await apiClient.get(`/api/characters/${cid}`);
      const data = r.data?.data || r.data;
      if (data) {
        form.voiceType = data.voiceType || "zh_female_vv_uranus_bigtts";
        form.voiceSpeed = data.voiceSpeed ?? 1.0;
        form.voicePitch = normalizeVoicePitchRatio(data.voicePitch);
        form.voiceVolume = data.voiceVolume ?? 1.0;
        form.customVoiceId = data.customVoiceId || "";
        form.voiceConfigId = data.voiceConfigId ? String(data.voiceConfigId) : "";
        form.emotion = data.emotion || "";
        form.emotionScale = data.emotionScale || 4;
        form.silenceDuration = data.silenceDuration || 0;

        if (data.voiceMode) {
          voiceMode.value = data.voiceMode as "preset" | "clone";
        } else if (data.customVoiceId) {
          voiceMode.value = "clone";
        } else {
          voiceMode.value = "preset";
        }
        if (!currentVoiceSupportsEmotion.value) {
          form.emotion = "";
          form.emotionScale = 4;
        }

        Object.assign(originalForm, { ...form, _mode: voiceMode.value });
      }
    } catch (err: any) {
      ElMessage.error(err?.message || "加载角色音色配置失败");
    }
  }

  async function submitTrain() {
    const speakerId = trainSpeakerId.value.trim();
    if (!cloneFile.value) {
      ElMessage.warning("请选择音频文件");
      return;
    }
    trainLoading.value = true;
    trainResult.value = "";
    try {
      const displayName = trainVoiceName.value.trim() || speakerId || "角色复刻音色";
      const formData = new FormData();
      formData.append("audio", cloneFile.value);
      formData.append("name", displayName);
      if (speakerId) formData.append("speakerId", speakerId);
      if (form.voiceConfigId.trim()) formData.append("voiceConfigId", form.voiceConfigId.trim());
      formData.append("language", "cn");

      const resp = await apiClient.post("/api/tts/voice-clone", formData);
      const data: any = resp.data?.data || resp.data;
      if (!data?.speakerId) {
        ElMessage.error((resp.data as any)?.message || "复刻失败");
        return;
      }
      await loadClonedVoices();
      form.customVoiceId = String(data.speakerId);
      trainResult.value = `复刻成功: ${data.speakerId}`;
      ElMessage.success("音色复刻成功，已自动选中");
      trainSpeakerId.value = "";
      trainVoiceName.value = "";
      cloneFile.value = null;
      cloneFileList.value = [];
    } catch (err: any) {
      ElMessage.error(err?.message || "复刻失败");
    } finally {
      trainLoading.value = false;
    }
  }

  async function previewClone(speakerId: string) {
    previewCloneId.value = speakerId;
    try {
      const res: any = await apiClient.post("/api/tts/synthesize", {
        speakerId,
        text: "测试",
      });
      const data = res.data?.data || res.data;
      if (data?.audioUrl) {
        previewAudio.value = data.audioUrl;
      } else {
        ElMessage.warning("未能获取音频");
      }
    } catch (err: any) {
      ElMessage.error(err?.message || "试听失败，请检查全局音色配置");
    } finally {
      previewCloneId.value = "";
    }
  }

  async function deleteClone(speakerId: string, name: string) {
    try {
      await ElMessageBox.confirm(`确定删除音色"${name}"吗？`, "确认", {
        type: "warning",
        confirmButtonText: "删除",
      });
      await apiClient.delete("/api/tts/voice-clone", { params: { speakerId } });
      if (form.customVoiceId === speakerId) {
        form.customVoiceId = "";
      }
      await loadClonedVoices();
      ElMessage.success("已删除");
    } catch (err: any) {
      if (err !== "cancel" && err !== "close") {
        ElMessage.error(err?.message || "删除失败");
      }
    }
  }

  async function doPreview() {
    if (!previewText.value.trim()) {
      ElMessage.warning("请输入试听文本");
      return;
    }
    previewLoading.value = true;
    previewAudio.value = "";
    try {
      const res = await apiClient.post("/api/tts/synthesize", {
        characterId: injectedCharacterId.value,
        text: previewText.value,
      });
      const data = res.data?.data || res.data;
      if (data?.audioUrl) {
        previewAudio.value = data.audioUrl;
      } else {
        ElMessage.warning("未能获取音频，请检查全局音色配置");
      }
    } catch (err: any) {
      ElMessage.warning(err?.message || "试听失败，请检查全局音色配置");
    } finally {
      previewLoading.value = false;
    }
  }

  async function saveVoice() {
    const cid = injectedCharacterId.value;
    if (!cid) {
      ElMessage.warning("未找到角色 ID");
      return;
    }
    if (voiceMode.value === "clone" && !form.customVoiceId.trim()) {
      ElMessage.warning("请选择一个复刻音色");
      return;
    }
    saving.value = true;
    try {
      await apiClient.put(`/api/characters/${cid}`, {
        voiceType: form.voiceType,
        voiceSpeed: form.voiceSpeed,
        voicePitch: normalizeVoicePitchRatio(form.voicePitch),
        voiceVolume: form.voiceVolume,
        customVoiceId: form.customVoiceId,
        voiceConfigId: form.voiceConfigId || "",
        voiceMode: voiceMode.value,
        emotion: form.emotion || "",
        emotionScale: form.emotionScale || 0,
        silenceDuration: form.silenceDuration || 0,
      });
      ElMessage.success("音色配置已保存");
      Object.assign(originalForm, { ...form, _mode: voiceMode.value });
    } catch (e: any) {
      ElMessage.error(e?.message || "保存失败");
    } finally {
      saving.value = false;
    }
  }

  function resetForm() {
    Object.assign(form, {
      voiceType: (originalForm as any).voiceType,
      voiceSpeed: (originalForm as any).voiceSpeed,
      voicePitch: Number((originalForm as any).voicePitch),
      voiceVolume: (originalForm as any).voiceVolume,
      customVoiceId: (originalForm as any).customVoiceId,
      voiceConfigId: (originalForm as any).voiceConfigId,
      emotion: (originalForm as any).emotion || "",
      emotionScale: (originalForm as any).emotionScale || 4,
      silenceDuration: (originalForm as any).silenceDuration || 0,
    });
    voiceMode.value = (originalForm as any)._mode || "preset";
    ElMessage.info("已重置为上次保存的值");
  }

  onMounted(() => {
    void Promise.all([
      loadVoicePresets(),
      loadVoiceConfigs(),
      loadClonedVoices(),
      loadCharacterVoice(),
    ]);
  });

  return {
    voicePresets,
    voiceConfigs,
    emotions,
    saving,
    previewLoading,
    previewText,
    previewAudio,
    voiceMode,
    form,
    originalForm,
    trainSpeakerId,
    trainVoiceName,
    cloneFile,
    cloneFileList,
    trainLoading,
    trainResult,
    clonedVoices,
    previewCloneId,
    currentVoiceSupportsEmotion,
    onModeChange,
    selectCloneVoice,
    onVoiceTypeChange,
    onCloneFileChange,
    loadVoicePresets,
    loadVoiceConfigs,
    loadCharacterVoice,
    loadClonedVoices,
    submitTrain,
    previewClone,
    deleteClone,
    doPreview,
    saveVoice,
    resetForm,
  };
}
