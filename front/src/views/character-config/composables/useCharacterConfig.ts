// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, reactive, computed, inject } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useApi } from "../../../composables/useApi";
import { createAuthenticatedFetchInit } from "../../../runtime/request-auth";
import { resolveApiUrl, getDeploymentConfig } from "../../../runtime/runtime-adapter";
import { roleAuthorityConfig } from "../../../runtime/role-authority";
import {
  type TemplateItem,
  DEFAULT_BOUNDARY,
  DEFAULT_PERSONALITY_CONFIG,
  type PersonalityConfig,
} from "./types";

export { type TemplateItem } from "./types";

export function useCharacterConfig() {
  const { get, post, put, del } = useApi();
  const refreshHealth = inject<() => void>("refreshHealth", () => {});
  const defaultPersonalityConfig = DEFAULT_PERSONALITY_CONFIG;

  const avatarUploading = ref(false);

  const templates = ref<any[]>([]);
  const showTemplateDialog = ref(false);
  const templateLoading = ref(false);

  const characters = ref<any[]>([]);
  const selected = ref<any>(null);
  const selectedId = ref("");
  const activeTab = ref("edit");
  const generationSession = ref(0);
  const saving = ref(false);
  const readOnly = ref(false);
  const collectionAuthority = ref("");
  const editorAuthority = ref("");

  async function refreshRolePermission() {
    if ((await getDeploymentConfig()).mode !== "cloud") {
      readOnly.value = false;
      return;
    }
    readOnly.value = true;
    const state = await get<any>("/api/device-mesh/v1/coordination/me");
    readOnly.value = state?.policy?.coordinated === true && state?.canAdminister !== true;
  }

  function allowEditing() {
    if (!readOnly.value) return true;
    ElMessage.warning("当前使用 Core 角色，只有 Core 管理员可以修改");
    return false;
  }
  const showFullPrompt = ref(false);
  const showFullBounds = ref(false);

  const form = reactive({
    name: "",
    avatar: "",
    identity: "",
    personality: "",
    speakingStyle: "",
    relationshipStyle: "",
    characterBase: "",
    boundaryRules: DEFAULT_BOUNDARY,
    isActive: true,
    description: "",
    scenario: "",
    exampleMessages: "",
    alternateGreetingsText: "",
    postHistoryInstructions: "",
    creator: "",
    characterVersion: "",
    tagsText: "",
    basePrompt: "",
    isDefault: false,
    status: "enabled",
    personalityConfig: { ...DEFAULT_PERSONALITY_CONFIG } as PersonalityConfig,
    chatStyleConfig: null as any,
    sceneRules: null as any,
  });
  const cardDataExtra = ref<Record<string, any>>({});

  function parseCardData(value: any): Record<string, any> {
    if (!value) return {};
    if (typeof value === "object") return { ...value };
    try {
      const parsed = JSON.parse(value);
      return parsed && typeof parsed === "object" ? parsed : {};
    } catch {
      return {};
    }
  }

  function normalizePersonalityConfig(value: any): PersonalityConfig {
    const raw = typeof value === "string" ? JSON.parse(value) : value || {};
    return {
      ...defaultPersonalityConfig,
      ...(raw as Partial<PersonalityConfig>),
    };
  }

  const hasOtherActive = computed(() =>
    characters.value.some((c) => c.isActive && c.id !== selectedId.value),
  );

  async function fetchTemplates() {
    templateLoading.value = true;
    try {
      templates.value = (await get<any[]>("/api/character-templates")) || [];
    } catch {
      templates.value = [];
    } finally {
      templateLoading.value = false;
    }
  }

  async function fetchChars() {
    try {
      await refreshRolePermission();
      const authority = await get<{ roleAuthority: string }>("/api/characters/authority");
      roleAuthorityConfig(authority?.roleAuthority);
      const rows = (await get<any[]>("/api/characters")) || [];
      if (rows.some((row) => row.roleAuthority !== authority.roleAuthority)) {
        throw new Error("角色数据归属已变化，请重新加载");
      }
      collectionAuthority.value = authority.roleAuthority;
      characters.value = rows;
    } catch {
      collectionAuthority.value = "";
    }
  }

  function selectChar(c: any) {
    if (saving.value) return;
    editorAuthority.value = c.roleAuthority || "";
    generationSession.value++;
    const cardData = parseCardData(c.cardData);
    cardDataExtra.value = cardData;
    selected.value = c;
    selectedId.value = c.id;
    activeTab.value = "edit";
    form.name = c.name || "";
    form.avatar = c.avatar || "";
    form.identity = c.identity || "";
    form.personality = c.personality || "";
    form.speakingStyle = c.speakingStyle || "";
    form.relationshipStyle = c.relationshipStyle || "";
    form.characterBase = cardData.systemPrompt || c.characterBase || "";
    form.boundaryRules = c.boundaryRules ?? DEFAULT_BOUNDARY;
    form.description = c.description || "";
    form.scenario = cardData.scenario || "";
    form.exampleMessages = cardData.exampleMessages || "";
    form.alternateGreetingsText = Array.isArray(cardData.alternateGreetings)
      ? cardData.alternateGreetings.join("\n")
      : "";
    form.postHistoryInstructions = cardData.postHistoryInstructions || "";
    form.creator = cardData.creator || "";
    form.characterVersion = cardData.characterVersion || "";
    form.tagsText = Array.isArray(cardData.tags) ? cardData.tags.join(", ") : "";
    form.basePrompt = c.basePrompt || "";
    form.isDefault = !!c.isDefault;
    form.status = c.status || "enabled";
    form.personalityConfig = normalizePersonalityConfig(c.personalityConfig);
    form.chatStyleConfig = c.chatStyleConfig || null;
    form.sceneRules = c.sceneRules || null;
    form.isActive = !!c.isActive;
  }

  function createNew() {
    if (saving.value) return;
    if (!allowEditing()) return;
    generationSession.value++;
    editorAuthority.value = collectionAuthority.value;
    cardDataExtra.value = {};
    selected.value = { id: "", name: "", isActive: false };
    selectedId.value = "";
    activeTab.value = "generate";
    form.name = "";
    form.avatar = "";
    form.identity = "";
    form.personality = "";
    form.speakingStyle = "";
    form.relationshipStyle = "";
    form.characterBase = "";
    form.boundaryRules = "";
    form.isActive = true;
    form.description = "";
    form.scenario = "";
    form.exampleMessages = "";
    form.alternateGreetingsText = "";
    form.postHistoryInstructions = "";
    form.creator = "";
    form.characterVersion = "";
    form.tagsText = "";
    form.personalityConfig = { ...DEFAULT_PERSONALITY_CONFIG };
  }

  async function createFromTemplate(tpl: TemplateItem) {
    if (!allowEditing()) return;
    try {
      const result = await post<any>(
        `/api/character-templates/${tpl.id}/create-character`,
        { name: tpl.name },
        roleAuthorityConfig((tpl as any).roleAuthority),
      );
      if (result) {
        showTemplateDialog.value = false;
        await fetchChars();
        selectChar(result);
      }
    } catch (err: any) {
      ElMessage.error(err?.message || "从模板创建角色失败，请重新加载后重试");
    }
  }

  function copyChar(c: any) {
    if (saving.value) return;
    if (!allowEditing()) return;
    const cardData = parseCardData(c.cardData);
    createNew();
    editorAuthority.value = c.roleAuthority || "";
    cardDataExtra.value = cardData;
    form.name = (c.name || "") + " (副本)";
    form.avatar = c.avatar || "";
    form.identity = c.identity || "";
    form.personality = c.personality || "";
    form.speakingStyle = c.speakingStyle || "";
    form.relationshipStyle = c.relationshipStyle || "";
    form.characterBase = cardData.systemPrompt || c.characterBase || "";
    form.boundaryRules = c.boundaryRules ?? DEFAULT_BOUNDARY;
    form.description = c.description || "";
    form.scenario = cardData.scenario || "";
    form.exampleMessages = cardData.exampleMessages || "";
    form.alternateGreetingsText = Array.isArray(cardData.alternateGreetings)
      ? cardData.alternateGreetings.join("\n")
      : "";
    form.postHistoryInstructions = cardData.postHistoryInstructions || "";
    form.creator = cardData.creator || "";
    form.characterVersion = cardData.characterVersion || "";
    form.tagsText = Array.isArray(cardData.tags) ? cardData.tags.join(", ") : "";
    form.basePrompt = c.basePrompt || "";
    form.isDefault = false;
    form.status = "enabled";
    form.personalityConfig = normalizePersonalityConfig(c.personalityConfig);
    form.chatStyleConfig = c.chatStyleConfig || null;
    form.sceneRules = c.sceneRules || null;
    form.isActive = false;
    ElMessage.success("已复制角色，请修改后保存");
  }

  async function saveChar() {
    if (saving.value) return;
    if (!allowEditing()) return;
    if (!form.name.trim()) {
      ElMessage.warning("请输入角色名称");
      return;
    }
    saving.value = true;
    try {
      const intent = roleAuthorityConfig(editorAuthority.value);
      const wasExisting = Boolean(selected.value?.id);
      const cardPayload = JSON.parse(JSON.stringify({
        ...cardDataExtra.value,
        scenario: form.scenario,
        alternateGreetings: form.alternateGreetingsText.split("\n").map((item) => item.trim()).filter(Boolean),
        exampleMessages: form.exampleMessages,
        systemPrompt: form.characterBase,
        postHistoryInstructions: form.postHistoryInstructions,
        creator: form.creator,
        characterVersion: form.characterVersion,
        tags: form.tagsText.split(",").map((item) => item.trim()).filter(Boolean),
      }));
      const payload = JSON.parse(JSON.stringify(form));
      delete (payload as any).scenario;
      delete (payload as any).exampleMessages;
      delete (payload as any).alternateGreetingsText;
      delete (payload as any).postHistoryInstructions;
      delete (payload as any).creator;
      delete (payload as any).characterVersion;
      delete (payload as any).tagsText;
      let targetId = selected.value?.id || "";
      if (selected.value?.id) {
        await put(`/api/characters/${selected.value.id}`, payload, intent);
      } else {
        const created = await post<any>("/api/characters", payload, intent);
        if (created?.id) {
          targetId = created.id;
          selected.value = { ...created };
          selectedId.value = created.id;
        }
      }
      if (targetId) {
        await put(`/api/characters/${targetId}/card-data`, cardPayload, intent);
      }
      ElMessage.success(wasExisting ? "保存成功" : "创建成功");
      await fetchChars();
      if (selectedId.value) {
        const refreshed = characters.value.find(
          (c: any) => c.id === selectedId.value,
        );
        if (refreshed && refreshed.roleAuthority === editorAuthority.value) {
          saving.value = false;
          selectChar(refreshed);
        }
      }
      refreshHealth();
    } catch (err: any) {
      ElMessage.error(err?.message || "角色未完整保存，请重新加载后确认");
    } finally {
      saving.value = false;
    }
  }

  function resetPrompt() {
    ElMessageBox.confirm("恢复默认提示词？当前内容将丢失。", "提示", {
      type: "warning",
    })
      .then(() => {
        form.characterBase = "";
        ElMessage.success("已恢复");
      })
      .catch(() => {});
  }

  function resetBounds() {
    ElMessageBox.confirm("恢复默认边界规则？", "提示", { type: "warning" })
      .then(() => {
        form.boundaryRules = "";
        ElMessage.success("已恢复");
      })
      .catch(() => {});
  }

  function selectCharById(id: string) {
    const found = characters.value.find((c) => c.id === id);
    if (found) selectChar(found);
  }

  async function uploadAvatar(file: File): Promise<string | null> {
    if (!allowEditing()) return null;
    if (!selectedId.value) {
      ElMessage.warning("请先保存角色");
      return null;
    }
    avatarUploading.value = true;
    try {
      const formData = new FormData();
      formData.append("avatar", file);
      const path = `/api/characters/${selectedId.value}/avatar`;
      const [url, init] = await Promise.all([
        resolveApiUrl(path),
        createAuthenticatedFetchInit(path, { method: "POST", body: formData, ...roleAuthorityConfig(editorAuthority.value) }),
      ]);
      const res = await fetch(url, init);
      if (!res.ok) throw new Error("上传失败");
      const data = await res.json();
      const avatarUrl = (data as any)?.data?.avatarUrl || "";
      if (avatarUrl) {
        form.avatar = avatarUrl;
        ElMessage.success("头像上传成功");
        return avatarUrl;
      }
      return null;
    } catch (err: any) {
      ElMessage.error("头像上传失败");
      return null;
    } finally {
      avatarUploading.value = false;
    }
  }

  async function delChar(c: any) {
    if (!allowEditing()) return;
    const target = { ...c };
    if (c.isActive) {
      const others = characters.value.filter((x) => x.id !== c.id);
      if (others.length === 0) {
        ElMessage.warning("不能删除唯一的角色");
        return;
      }
    }
    await ElMessageBox.confirm(
      `确定删除角色「${c.name}」？此操作不可撤销。`,
      "确认删除",
      {
        type: "warning",
        confirmButtonText: "删除",
        confirmButtonClass: "el-button--danger",
      },
    );
    try {
      await del(`/api/characters/${target.id}`, roleAuthorityConfig(target.roleAuthority));
      ElMessage.success("已删除");
      if (selectedId.value === c.id) {
        selected.value = null;
        selectedId.value = "";
      }
      await fetchChars();
      if (selectedId.value) {
        const refreshed = characters.value.find(
          (c: any) => c.id === selectedId.value,
        );
        if (refreshed) selectChar(refreshed);
      }
      refreshHealth();
    } catch (err: any) {
      ElMessage.error(err?.message || "角色删除失败，请重新加载后重试");
    }
  }

  return {
    readOnly,
    generationSession,
    templates,
    showTemplateDialog,
    templateLoading,
    characters,
    selected,
    selectedId,
    activeTab,
    saving,
    showFullPrompt,
    showFullBounds,
    form,
    hasOtherActive,
    fetchTemplates,
    fetchChars,
    selectChar,
    createNew,
    createFromTemplate,
    copyChar,
    saveChar,
    resetPrompt,
    resetBounds,
    delChar,
    selectCharById,
    avatarUploading,
    uploadAvatar,
  };
}
