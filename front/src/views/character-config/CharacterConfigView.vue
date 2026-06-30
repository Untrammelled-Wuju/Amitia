<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="char-config-page">
    <el-alert type="warning" :closable="false" show-icon style="margin-bottom:12px">
      <template #title>
        安全提示：角色不能声称自己是真人、真实恋人，不能诱导依赖、索要隐私、代替回复微信好友，不能输出成人化、操控式、威胁式或危险内容。
      </template>
    </el-alert>

    <div class="char-layout">
      <CharacterSidebar
        :characters="characters"
        :selected-id="selectedId"
        @create="createNew"
        @open-templates="showTemplateDialog = true"
        @select="onSelectChar"
        @copy="copyChar"
        @delete="delChar"
      />

      <div class="char-main" :class="{ empty: !selected }">
        <template v-if="selected">
          <CharacterEditForm
            v-model:active-tab="activeTab"
            v-model:name="form.name"
            v-model:avatar="form.avatar"
            v-model:identity="form.identity"
            v-model:personality="form.personality"
            v-model:speaking-style="form.speakingStyle"
            v-model:relationship-style="form.relationshipStyle"
            v-model:system-prompt="form.systemPrompt"
            v-model:boundary-rules="form.boundaryRules"
            v-model:personality-config="form.personalityConfig"
            v-model:is-active="form.isActive"
            :has-other-active="hasOtherActive"
            :saving="saving"
            :selected-id="selectedId"
            @show-full-prompt="showFullPrompt = true"
            @show-full-bounds="showFullBounds = true"
            @reset-prompt="resetPrompt"
            @reset-bounds="resetBounds"
            @save="saveChar"
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
        </template>

        <div v-else class="char-main-empty">
          <el-empty description="请选择一个角色或创建新角色" :image-size="60" />
        </div>
      </div>
    </div>

    <div class="page-actions">
      <el-button :loading="exportingPack" @click="onExportPack" :disabled="!selected">
        导出角色包
      </el-button>
      <el-button @click="showImportDialog = true">导入角色包</el-button>
    </div>

    <el-dialog v-model="showFullPrompt" title="完整 Prompt" fullscreen destroy-on-close>
      <el-input
        v-model="form.systemPrompt"
        type="textarea"
        :rows="30"
        placeholder="编写角色的 System Prompt..."
      />
    </el-dialog>

    <el-dialog v-model="showFullBounds" title="完整边界规则" fullscreen destroy-on-close>
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
    />
  </div>
</template>

<script setup lang="ts">
import { onMounted } from "vue"
import { useCharacterConfig } from "./composables/useCharacterConfig"
import { useCharacterTestChat } from "./composables/useCharacterTestChat"
import { useCharacterImportExport } from "./composables/useCharacterImportExport"
import CharacterSidebar from "./components/CharacterSidebar.vue"
import CharacterEditForm from "./components/CharacterEditForm.vue"
import CharacterTestChat from "./components/CharacterTestChat.vue"
import TemplatePickerDialog from "./components/TemplatePickerDialog.vue"
import ImportPackDialog from "./components/ImportPackDialog.vue"

const {
  templates, showTemplateDialog, templateLoading,
  characters, selected, selectedId, activeTab, saving,
  showFullPrompt, showFullBounds,
  form, hasOtherActive,
  fetchTemplates, fetchChars,
  selectChar, createNew, createFromTemplate,
  copyChar, saveChar, resetPrompt, resetBounds, delChar,
  selectCharById,
} = useCharacterConfig()

const {
  testMessages, testMsg, testLoading, sendTest, clearTestMessages,
} = useCharacterTestChat()

const {
  exportingPack, showImportDialog,
  importPackName, importPreview, importPreviewing,
  importConfirmText, importing, packHistory,
  exportPack, previewImport, confirmImport, loadPackHistory,
  cancelImportPreview,
} = useCharacterImportExport()

function onSelectChar(c: any) {
  selectChar(c)
  clearTestMessages()
}

function onSendTest(text: string) {
  sendTest(selectedId.value, text)
}

function onExportPack() {
  exportPack(selectedId.value, selected.value?.name || "")
}

async function onConfirmImport() {
  const d = await confirmImport()
  if (d?.characterId) selectCharById(d.characterId)
  if (d) await fetchChars()
}

onMounted(async () => {
  await loadPackHistory()
  await fetchTemplates()
  await fetchChars()
})
</script>

<style scoped>
.char-config-page {
  padding: 20px;
  height: 100%;
  display: flex;
  flex-direction: column;
}

.char-layout {
  display: flex;
  gap: 16px;
  flex: 1;
  min-height: 0;
}

.char-main {
  flex: 1;
  overflow-y: auto;
  min-width: 0;
}

.char-main.empty {
  display: flex;
  align-items: center;
  justify-content: center;
}

.char-main-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
}

.page-actions {
  display: flex;
  gap: 8px;
  padding-top: 12px;
  border-top: 1px solid var(--ac-color-border-light);
  margin-top: 12px;
}

@media (max-width: 768px) {
  .char-layout {
    flex-direction: column;
  }
}
</style>
