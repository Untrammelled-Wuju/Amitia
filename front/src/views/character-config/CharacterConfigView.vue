<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="char-config-page">
    <ExtensionPageHeader
      title="角色卡工坊"
      description="创建和编辑角色卡，导入酒馆角色卡，并导出标准 CHARX 角色包。"
      parent-title="创意工坊"
      parent-path="/creative-workshop"
    >
      <template #actions>
        <el-button @click="showImportDialog = true">导入角色卡</el-button>
        <el-button
          :loading="exportingPack"
          :disabled="!selected"
          @click="onExportPack"
        >
          导出 CHARX
        </el-button>
      </template>
    </ExtensionPageHeader>
    <div class="char-layout">
      <div class="char-sidebar-stack">
        <CharacterSidebar
          :characters="characters"
          :selected-id="selectedId"
          :creating="!!selected && !selectedId"
          @create="createNew"
          @open-templates="showTemplateDialog = true"
          @select="onSelectChar"
          @copy="copyChar"
          @delete="delChar"
        />
        <ExtensionSlot
          slot-id="character.sidebar.card"
          :context="characterExtensionContext"
          fallback="none"
          layout="stack"
          surface-role="sidebar"
        />
      </div>

      <div
        class="char-main"
        :class="{ empty: !selected, 'test-mode': activeTab === 'test' }"
      >
        <template v-if="selected">
          <ExtensionSlot
            slot-id="character.detail.action"
            :context="characterExtensionContext"
            fallback="none"
            layout="inline"
            surface-role="header"
            class="character-action-slot"
          />
          <CharacterEditForm
            v-model:active-tab="activeTab"
            v-model:name="form.name"
            v-model:avatar="form.avatar"
            v-model:identity="form.identity"
            v-model:personality="form.personality"
            v-model:speaking-style="form.speakingStyle"
            v-model:relationship-style="form.relationshipStyle"
            v-model:character-base="form.characterBase"
            v-model:boundary-rules="form.boundaryRules"
            v-model:description="form.description"
            v-model:scenario="form.scenario"
            v-model:example-messages="form.exampleMessages"
            v-model:alternate-greetings-text="form.alternateGreetingsText"
            v-model:post-history-instructions="form.postHistoryInstructions"
            v-model:creator="form.creator"
            v-model:character-version="form.characterVersion"
            v-model:tags-text="form.tagsText"
            v-model:personality-config="form.personalityConfig"
            v-model:is-active="form.isActive"
            :has-other-active="hasOtherActive"
            :saving="saving"
            :selected-id="selectedId"
            :uploading-avatar="avatarUploading"
            @show-full-prompt="showFullPrompt = true"
            @show-full-bounds="showFullBounds = true"
            @reset-prompt="resetPrompt"
            @reset-bounds="resetBounds"
            @save="saveChar"
            @upload-avatar="onUploadAvatar"
          >
            <template #test>
              <CharacterTestChat
                :messages="testMessages"
                :loading="testLoading"
                v-model:msg="testMsg"
                :char-name="selected?.name || ''"
                @send="onSendTest"
              />
            </template>
          </CharacterEditForm>
          <ExtensionSlot
            slot-id="character.detail.tab"
            :context="characterExtensionContext"
            fallback="none"
            layout="tabs"
            surface-role="main"
            class="character-detail-slot"
          />
        </template>

        <div v-else class="char-main-empty">
          <el-empty description="请选择一个角色或创建新角色" :image-size="60" />
        </div>
      </div>
    </div>

    <el-dialog
      v-model="showFullPrompt"
      title="完整 Prompt"
      fullscreen
      destroy-on-close
    >
      <el-input
        v-model="form.characterBase"
        type="textarea"
        :rows="30"
        placeholder="编写角色的 System Prompt..."
      />
    </el-dialog>

    <el-dialog
      v-model="showFullBounds"
      title="完整边界规则"
      fullscreen
      destroy-on-close
    >
      <el-input
        v-model="form.boundaryRules"
        type="textarea"
        :rows="30"
        placeholder="每行一条规则..."
      />
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
      :preview="importPreview"
      :previewing="importPreviewing"
      v-model:confirm-text="importConfirmText"
      :importing="importing"
      :history="packHistory"
      @preview="previewImport"
      @cancel-preview="cancelImportPreview"
      @confirm="onConfirmImport"
      @file-selected="setSelectedFile"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from "vue";
import { useCharacterConfig } from "./composables/useCharacterConfig";
import { useCharacterTestChat } from "./composables/useCharacterTestChat";
import { useCharacterImportExport } from "./composables/useCharacterImportExport";
import CharacterSidebar from "./components/CharacterSidebar.vue";
import CharacterEditForm from "./components/CharacterEditForm.vue";
import CharacterTestChat from "./components/CharacterTestChat.vue";
import TemplatePickerDialog from "./components/TemplatePickerDialog.vue";
import ImportPackDialog from "./components/ImportPackDialog.vue";
import ExtensionSlot from "@/components/extension/ExtensionSlot.vue";
import ExtensionPageHeader from "@/views/extensions/components/ExtensionPageHeader.vue";

const {
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
} = useCharacterConfig();

const { testMessages, testMsg, testLoading, sendTest, clearTestMessages } =
  useCharacterTestChat();

const characterExtensionContext = computed(() => ({
  characterId: selectedId.value,
  characterName: selected.value?.name ?? "",
  activeTab: activeTab.value,
  surface: "character-detail",
}));

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
  setSelectedFile,
  loadPackHistory,
  cancelImportPreview,
} = useCharacterImportExport();

function onSelectChar(c: any) {
  selectChar(c);
  clearTestMessages();
}

function onSendTest(text: string) {
  sendTest(selectedId.value, text);
}

function onExportPack() {
  exportPack(selectedId.value, selected.value?.name || "");
}

async function onUploadAvatar(file: File) {
  await uploadAvatar(file);
}

async function onConfirmImport() {
  const d = await confirmImport();
  if (d?.characterId) selectCharById(d.characterId);
  if (d) await fetchChars();
}

onMounted(async () => {
  await loadPackHistory();
  await fetchTemplates();
  await fetchChars();
});
</script>

<style scoped>
.char-config-page {
  width: 100%;
  height: 100%;
  min-width: 0;
  min-height: 0;
  margin: 0 auto;
  padding: 0;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  gap: clamp(12px, 1.5vw, 18px);
  overflow: hidden;
}

.char-layout {
  width: 100%;
  display: grid;
  grid-template-columns: clamp(220px, 22vw, 260px) minmax(0, 1fr);
  gap: clamp(12px, 1.5vw, 18px);
  flex: 1;
  min-width: 0;
  min-height: 0;
  align-items: stretch;
}

.char-sidebar-stack {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
}
.char-sidebar-stack :deep(.extension-slot) { max-height: 36%; overflow: auto; }
.character-action-slot { margin-bottom: 10px; }
.character-detail-slot { margin-top: 12px; }

.char-main {
  width: 100%;
  min-width: 0;
  min-height: 0;
  padding: 20px;
  box-sizing: border-box;
  container-type: inline-size;
  overflow: auto;
  border: 1px solid var(--ac-color-border);
  border-radius: 14px;
  background: var(--ac-color-surface);
  box-shadow: var(--ac-shadow-sm);
}

.char-main.empty {
  display: flex;
  align-items: center;
  justify-content: center;
}

.char-main.test-mode {
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.char-main.test-mode :deep(.el-tabs) {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.char-main.test-mode :deep(.el-tabs__content) {
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.char-main.test-mode :deep(.el-tab-pane) {
  height: 100%;
}

.char-main-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
}

@media (max-width: 1080px) {
  .char-main {
    padding: 16px;
  }
}

@media (max-width: 820px) {
  .char-config-page {
    height: auto;
    overflow: visible;
  }

  .char-layout {
    grid-template-columns: 1fr;
  }

  .char-sidebar-stack {
    max-height: 360px;
  }

  .char-main {
    min-height: 520px;
    overflow: visible;
  }
}
</style>
