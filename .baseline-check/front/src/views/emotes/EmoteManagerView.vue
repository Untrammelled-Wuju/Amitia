<template>
  <main class="emote-page">
    <input
      ref="fileInput"
      hidden
      type="file"
      multiple
      accept=".png,.jpg,.jpeg,.gif,.webp"
      @change="previewFiles"
    />
    <input
      ref="folderInput"
      hidden
      type="file"
      multiple
      webkitdirectory
      accept=".png,.jpg,.jpeg,.gif,.webp"
      @change="previewFiles"
    />
    <section class="workspace">
      <aside class="groups" aria-label="表情分组">
        <button
          v-for="view in views"
          :key="view.key"
          type="button"
          :class="{ active: activeView === view.key && !activeGroup }"
          @click="selectView(view.key)"
        >
          <el-icon><component :is="view.icon" /></el-icon>{{ view.label }}
        </button>
        <div class="section-title">
          <span>自定义分组</span
          ><el-button
            text
            circle
            :icon="Plus"
            aria-label="新建分组"
            @click="createGroup"
          />
        </div>
        <div v-if="!groups.length" class="muted">还没有分组</div>
        <div
          v-for="(group, index) in groups"
          :key="group.id"
          class="group"
          :class="{ active: activeGroup === group.id }"
        >
          <button type="button" @click="selectGroup(group.id)">
            <span class="cover"
              ><img
                v-if="coverUrl(group)"
                :src="assetUrl(coverUrl(group))"
                alt="" /></span
            ><span>{{ group.name }}</span>
          </button>
          <el-dropdown
            ><el-button
              text
              circle
              :icon="MoreFilled"
              :aria-label="`${group.name} 操作`"
            /><template #dropdown
              ><el-dropdown-menu
                ><el-dropdown-item @click="renameGroup(group)"
                  >重命名</el-dropdown-item
                ><el-dropdown-item
                  :disabled="index === 0"
                  @click="moveGroup(index, -1)"
                  >上移</el-dropdown-item
                ><el-dropdown-item
                  :disabled="index === groups.length - 1"
                  @click="moveGroup(index, 1)"
                  >下移</el-dropdown-item
                ><el-dropdown-item
                  :disabled="selectedIds.length !== 1"
                  @click="setCover(group)"
                  >设置封面</el-dropdown-item
                ><el-dropdown-item divided @click="deleteGroup(group)"
                  >删除分组</el-dropdown-item
                ></el-dropdown-menu
              ></template
            ></el-dropdown
          >
        </div>
      </aside>
      <section class="list-panel" @dragover.prevent @drop.prevent="dropFiles">
        <div class="toolbar">
          <el-input
            v-model="search"
            :prefix-icon="Search"
            clearable
            placeholder="搜索名称、含义或关键词"
            @input="debouncedLoad"
          /><el-button @click="folderInput?.click()">导入文件夹</el-button
          ><el-button type="primary" @click="fileInput?.click()"
            >导入表情</el-button
          ><el-button
            :icon="Refresh"
            circle
            aria-label="刷新"
            @click="loadEmotes"
          />
        </div>
        <div v-if="selectedIds.length" class="bulk">
          <span>已选 {{ selectedIds.length }} 项</span>
          <el-select
            v-model="bulkGroup"
            placeholder="加入分组"
            @change="addSelectedToGroup"
            ><el-option
              v-for="group in groups"
              :key="group.id"
              :label="group.name"
              :value="group.id"
          /></el-select>
          <el-button @click="bulkAI(true)">允许 AI</el-button
          ><el-button @click="bulkAI(false)">禁止 AI</el-button>
          <el-button v-if="activeGroup" @click="removeSelectedFromGroup"
            >移出分组</el-button
          ><el-button type="danger" plain @click="deleteSelected"
            >删除</el-button
          >
        </div>
        <div class="select-row">
          <el-checkbox
            :model-value="allSelected"
            :indeterminate="someSelected"
            @change="toggleAll"
            >全选当前页</el-checkbox
          ><span>{{ total }} 个表情</span>
        </div>
        <div v-loading="loading" class="grid">
          <button
            v-for="item in emotes"
            :key="item.id"
            type="button"
            class="card"
            :class="{
              selected: selectedIds.includes(item.id),
              focused: focusedId === item.id,
            }"
            @click="focusItem(item)"
            @dblclick="toggleSelection(item.id)"
          >
            <el-checkbox
              class="check"
              :model-value="selectedIds.includes(item.id)"
              :aria-label="`选择 ${item.name}`"
              @click.stop
              @change="toggleSelection(item.id)"
            />
            <img
              :src="
                assetUrl(
                  hoveredId === item.id ? item.filePath : item.thumbnailPath,
                )
              "
              :alt="item.meaning || item.name"
              loading="lazy"
              @mouseenter="hoveredId = item.id"
              @mouseleave="hoveredId = ''"
            />
            <strong>{{ item.name }}</strong
            ><span
              ><small v-if="item.isAnimated">动图</small
              ><small :class="{ enabled: item.aiEnabled }">AI</small></span
            >
          </button>
          <div v-if="!loading && !emotes.length" class="empty">
            <el-icon><Picture /></el-icon><strong>暂无表情</strong
            ><span>拖入图片，或点击上方按钮开始导入。</span>
          </div>
        </div>
      </section>
      <aside class="detail">
        <template v-if="focused">
          <img
            class="preview"
            :src="assetUrl(focused.filePath)"
            :alt="focused.meaning || focused.name"
          />
          <el-form label-position="top">
            <el-form-item label="名称"
              ><el-input v-model="detailForm.name"
            /></el-form-item>
            <el-form-item label="含义"
              ><el-input
                v-model="detailForm.meaning"
                type="textarea"
                :rows="3"
              /><small>AI 可用时含义不能为空。</small></el-form-item
            >
            <el-form-item label="关键词"
              ><el-input v-model="detailForm.keywords" placeholder="用逗号分隔"
            /></el-form-item>
            <el-form-item label="允许 AI 使用"
              ><el-switch v-model="detailForm.aiEnabled"
            /></el-form-item>
            <fieldset>
              <legend>适用角色</legend>
              <el-radio-group v-model="detailForm.roleScope"
                ><el-radio value="all_characters">全部角色</el-radio
                ><el-radio value="selected_characters"
                  >指定角色</el-radio
                ></el-radio-group
              ><el-select
                v-if="detailForm.roleScope === 'selected_characters'"
                v-model="detailForm.characterIds"
                multiple
                placeholder="选择角色"
                ><el-option
                  v-for="character in characters"
                  :key="character.id"
                  :label="character.name"
                  :value="character.id"
              /></el-select>
            </fieldset>
            <fieldset>
              <legend>所在分组</legend>
              <el-select
                v-model="detailForm.groupIds"
                multiple
                placeholder="未分组"
                ><el-option
                  v-for="group in groups"
                  :key="group.id"
                  :label="group.name"
                  :value="group.id"
              /></el-select>
            </fieldset>
            <div class="metadata">
              <span>{{ focused.fileExtension.toUpperCase() }}</span
              ><span>{{ formatBytes(focused.fileSize) }}</span
              ><span>{{ focused.width }} × {{ focused.height }}</span
              ><span>{{
                focused.isAnimated ? `${focused.frameCount} 帧` : "静态"
              }}</span
              ><span
                >平台降级：{{ focused.fallbackPath ? "可用" : "不可用" }}</span
              >
            </div>
            <div class="detail-actions">
              <el-button type="primary" :loading="saving" @click="saveDetail"
                >保存</el-button
              ><el-button type="danger" plain @click="deleteFocused"
                >删除</el-button
              >
            </div>
          </el-form>
        </template>
        <div v-else class="empty-detail">选择一个表情查看详情</div>
      </aside>
    </section>
    <el-dialog
      v-model="showImport"
      title="导入预览"
      width="min(1060px, 94vw)"
      :close-on-click-modal="false"
    >
      <section class="defaults">
        <div class="defaults-head">
          <strong>批量设置</strong><span>填写后应用到全部表情</span>
        </div>
        <div class="defaults-row">
          <el-input v-model="defaults.meaning" placeholder="批量含义" />
          <el-input
            v-model="defaults.keywords"
            placeholder="批量关键词（中文或英文逗号分隔）"
          />
          <el-select
            v-model="defaults.groupIds"
            multiple
            collapse-tags
            placeholder="批量分组"
            ><el-option
              v-for="group in groups"
              :key="group.id"
              :label="group.name"
              :value="group.id"
          /></el-select>
          <el-switch v-model="defaults.aiEnabled" active-text="允许 AI" />
          <el-button @click="applyDefaults">应用到全部</el-button>
        </div>
      </section>
      <div class="import-list-head">
        <strong>待导入表情</strong><span>{{ importItems.length }} 个文件</span>
      </div>
      <div class="import-list">
        <div
          v-for="(item, index) in importItems"
          :key="item.key"
          class="import-item"
        >
          <img :src="item.preview" :alt="item.name" />
          <div class="import-main">
            <div class="import-row primary-row">
              <el-input
                v-model="item.name"
                aria-label="名称"
                placeholder="名称"
              />
              <el-input
                v-model="item.meaning"
                aria-label="含义"
                placeholder="含义；为空时自动关闭 AI"
              />
            </div>
            <div class="import-row settings-row">
              <el-input
                v-model="item.keywords"
                aria-label="关键词"
                placeholder="关键词（中文或英文逗号分隔）"
              />
              <el-select
                v-model="item.groupIds"
                multiple
                collapse-tags
                placeholder="分组"
                ><el-option
                  v-for="group in groups"
                  :key="group.id"
                  :label="group.name"
                  :value="group.id"
              /></el-select>
              <el-select v-model="item.roleScope"
                ><el-option label="全部角色" value="all_characters" /><el-option
                  label="指定角色"
                  value="selected_characters"
              /></el-select>
              <el-select
                v-model="item.characterIds"
                multiple
                collapse-tags
                :disabled="item.roleScope !== 'selected_characters'"
                :placeholder="
                  item.roleScope === 'selected_characters'
                    ? '选择角色'
                    : '全部角色'
                "
                ><el-option
                  v-for="character in characters"
                  :key="character.id"
                  :label="character.name"
                  :value="character.id"
              /></el-select>
              <el-switch v-model="item.aiEnabled" active-text="AI" />
            </div>
          </div>
          <div class="import-item-actions">
            <span class="status" :data-status="item.status">{{
              item.status
            }}</span
            ><el-button
              text
              circle
              :icon="Delete"
              aria-label="移除"
              @click="removeImport(index)"
            />
          </div>
        </div>
      </div>
      <template #footer
        ><div class="dialog-footer">
          <span>{{ importItems.length }} 个待导入文件</span
          ><el-button @click="showImport = false">取消</el-button
          ><el-button type="primary" :loading="importing" @click="submitImport"
            >确认导入</el-button
          >
        </div></template
      >
    </el-dialog>
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  Collection,
  Delete,
  MoreFilled,
  Picture,
  Plus,
  Refresh,
  Search,
  Timer,
} from "@element-plus/icons-vue";
import { apiClient, useApi } from "@/composables/useApi";
import { useAssetUrl } from "@/composables/useAssetUrl";

const { get, post, put, del } = useApi();
const { assetUrl } = useAssetUrl();
const fileInput = ref<HTMLInputElement>();
const folderInput = ref<HTMLInputElement>();
const groups = ref<any[]>([]);
const emotes = ref<any[]>([]);
const characters = ref<any[]>([]);
const total = ref(0);
const loading = ref(false);
const saving = ref(false);
const importing = ref(false);
const activeView = ref("all");
const activeGroup = ref("");
const search = ref("");
const selectedIds = ref<string[]>([]);
const focusedId = ref("");
const hoveredId = ref("");
const bulkGroup = ref("");
const showImport = ref(false);
const importItems = ref<any[]>([]);
const defaults = reactive({
  meaning: "",
  keywords: "",
  groupIds: [] as string[],
  aiEnabled: false,
});
const detailForm = reactive({
  name: "",
  meaning: "",
  keywords: "",
  aiEnabled: false,
  roleScope: "all_characters",
  characterIds: [] as string[],
  groupIds: [] as string[],
});
const views = [
  { key: "all", label: "全部表情", icon: Collection },
  { key: "recent", label: "最近使用", icon: Timer },
  { key: "unassigned", label: "未分组", icon: Picture },
];
const focused = computed(() =>
  emotes.value.find((item) => item.id === focusedId.value),
);
const allSelected = computed(
  () =>
    !!emotes.value.length &&
    emotes.value.every((item) => selectedIds.value.includes(item.id)),
);
const someSelected = computed(
  () =>
    !allSelected.value &&
    emotes.value.some((item) => selectedIds.value.includes(item.id)),
);
let searchTimer: ReturnType<typeof setTimeout> | undefined;

function debouncedLoad() {
  clearTimeout(searchTimer);
  searchTimer = setTimeout(loadEmotes, 250);
}
async function loadGroups() {
  groups.value = await get<any[]>("/api/emote-groups");
}
async function loadCharacters() {
  characters.value = await get<any[]>("/api/characters?includeDisabled=true");
}
async function loadEmotes() {
  loading.value = true;
  try {
    const data = await get<any>("/api/emotes", {
      groupId: activeGroup.value || undefined,
      view: activeGroup.value ? undefined : activeView.value,
      q: search.value,
      pageSize: 200,
    });
    emotes.value = data.items || [];
    total.value = data.total || 0;
    selectedIds.value = selectedIds.value.filter((id) =>
      emotes.value.some((item) => item.id === id),
    );
    if (
      focusedId.value &&
      !emotes.value.some((item) => item.id === focusedId.value)
    )
      focusedId.value = "";
  } finally {
    loading.value = false;
  }
}
function selectView(view: string) {
  activeView.value = view;
  activeGroup.value = "";
  selectedIds.value = [];
  loadEmotes();
}
function selectGroup(id: string) {
  activeGroup.value = id;
  selectedIds.value = [];
  loadEmotes();
}
function focusItem(item: any) {
  focusedId.value = item.id;
  Object.assign(detailForm, {
    name: item.name,
    meaning: item.meaning,
    keywords: (item.keywords || []).join("，"),
    aiEnabled: !!item.aiEnabled,
    roleScope: item.roleScope,
    characterIds: [...(item.characterIds || [])],
    groupIds: [...(item.groupIds || [])],
  });
}
function toggleSelection(id: string) {
  selectedIds.value = selectedIds.value.includes(id)
    ? selectedIds.value.filter((item) => item !== id)
    : [...selectedIds.value, id];
}
function toggleAll(value: any) {
  selectedIds.value = value ? emotes.value.map((item) => item.id) : [];
}
function coverUrl(group: any) {
  return (
    emotes.value.find((item) => item.id === group.coverEmoteId)
      ?.thumbnailPath || ""
  );
}
async function createGroup() {
  const { value } = await ElMessageBox.prompt(
    "分组只用于整理和浏览。",
    "新建分组",
    { inputPattern: /\S+/, inputErrorMessage: "请输入分组名称" },
  );
  await post("/api/emote-groups", { name: value });
  await loadGroups();
}
async function renameGroup(group: any) {
  const { value } = await ElMessageBox.prompt(
    "输入新的分组名称",
    "重命名分组",
    { inputValue: group.name, inputPattern: /\S+/ },
  );
  await put(`/api/emote-groups/${group.id}`, { name: value });
  await loadGroups();
}
async function deleteGroup(group: any) {
  await ElMessageBox.confirm("只删除分组，分组内的表情会被保留。", "删除分组", {
    type: "warning",
  });
  await del(`/api/emote-groups/${group.id}`);
  if (activeGroup.value === group.id) selectView("all");
  await loadGroups();
  await loadEmotes();
}
async function moveGroup(index: number, delta: number) {
  const copy = [...groups.value];
  const [moved] = copy.splice(index, 1);
  copy.splice(index + delta, 0, moved);
  groups.value = copy;
  await post("/api/emote-groups/reorder", { ids: copy.map((item) => item.id) });
}
async function setCover(group: any) {
  const item = emotes.value.find((value) => value.id === selectedIds.value[0]);
  if (!item) return;
  await put(`/api/emote-groups/${group.id}`, { coverEmoteId: item.id });
  await loadGroups();
}
async function saveDetail() {
  if (!focused.value) return;
  saving.value = true;
  try {
    const item = await put<any>(`/api/emotes/${focused.value.id}`, {
      name: detailForm.name,
      meaning: detailForm.meaning,
      keywords: splitKeywords(detailForm.keywords),
      aiEnabled: detailForm.aiEnabled,
      roleScope: detailForm.roleScope,
      characterIds: detailForm.characterIds,
      groupIds: detailForm.groupIds,
    });
    const index = emotes.value.findIndex((value) => value.id === item.id);
    if (index >= 0) emotes.value[index] = item;
    focusItem(item);
    ElMessage.success("已保存");
  } finally {
    saving.value = false;
  }
}
async function deleteFocused() {
  if (!focused.value) return;
  await ElMessageBox.confirm("删除后将停止 AI 使用并清理文件。", "删除表情", {
    type: "warning",
  });
  await del(`/api/emotes/${focused.value.id}`);
  focusedId.value = "";
  await loadEmotes();
}
async function bulkAI(aiEnabled: boolean) {
  await post("/api/emotes/batch-update", {
    ids: selectedIds.value,
    update: { aiEnabled },
  });
  await loadEmotes();
}
async function addSelectedToGroup() {
  if (!bulkGroup.value) return;
  await post(`/api/emote-groups/${bulkGroup.value}/emotes`, {
    emoteIds: selectedIds.value,
  });
  bulkGroup.value = "";
  ElMessage.success("已加入分组");
  await loadEmotes();
}
async function removeSelectedFromGroup() {
  for (const id of selectedIds.value)
    await del(`/api/emote-groups/${activeGroup.value}/emotes/${id}`);
  await loadEmotes();
}
async function deleteSelected() {
  await ElMessageBox.confirm(
    `确认删除 ${selectedIds.value.length} 个表情？`,
    "批量删除",
    { type: "warning" },
  );
  for (const id of selectedIds.value) await del(`/api/emotes/${id}`);
  selectedIds.value = [];
  await loadEmotes();
}
function previewFiles(event: Event) {
  const input = event.target as HTMLInputElement;
  openFiles(Array.from(input.files || []));
  input.value = "";
}
function dropFiles(event: DragEvent) {
  openFiles(Array.from(event.dataTransfer?.files || []));
}
function openFiles(files: File[]) {
  const allowed = /\.(png|jpe?g|gif|webp)$/i;
  importItems.value = files
    .filter((file) => allowed.test(file.name))
    .map((file) => {
      const relative = (file as any).webkitRelativePath || file.name;
      const parts = relative.split("/");
      return {
        key: crypto.randomUUID(),
        file,
        preview: URL.createObjectURL(file),
        name: file.name.replace(/\.[^.]+$/, ""),
        meaning: "",
        keywords: "",
        groupIds: activeGroup.value ? [activeGroup.value] : [],
        aiEnabled: false,
        roleScope: "all_characters",
        characterIds: [],
        folderGroup: parts.length > 1 ? parts[0] : "",
        relativePath: relative,
        status: "待导入",
      };
    });
  if (importItems.value.length) showImport.value = true;
  else ElMessage.warning("没有可导入的图片");
}
function applyDefaults() {
  for (const item of importItems.value) {
    item.meaning = defaults.meaning;
    item.keywords = defaults.keywords;
    item.groupIds = [...defaults.groupIds];
    item.aiEnabled = defaults.aiEnabled;
  }
}
function removeImport(index: number) {
  URL.revokeObjectURL(importItems.value[index].preview);
  importItems.value.splice(index, 1);
}
async function submitImport() {
  if (!importItems.value.length) return;
  importing.value = true;
  try {
    const form = new FormData();
    for (const item of importItems.value)
      form.append("files", item.file, item.file.name);
    form.append(
      "configs",
      JSON.stringify(
        importItems.value.map((item) => ({
          sourceName: item.file.name,
          relativePath: item.relativePath,
          name: item.name,
          meaning: item.meaning,
          keywords: splitKeywords(item.keywords),
          groupIds: item.groupIds,
          aiEnabled: item.aiEnabled,
          roleScope: item.roleScope,
          characterIds: item.characterIds,
          folderGroup: item.folderGroup,
        })),
      ),
    );
    const response = await apiClient.post("/api/emotes/batch-upload", form, {
      timeout: 180000,
    });
    const data = response.data;
    const byName = new Map(
      (data.items || []).map((item: any) => [item.sourceName, item]),
    );
    for (const item of importItems.value) {
      const result: any = byName.get(item.file.name);
      item.status =
        result?.status === "success"
          ? "成功"
          : result?.status === "duplicate"
            ? "重复"
            : result?.errorMessage || "失败";
    }
    ElMessage.success(
      `成功 ${data.summary?.success || 0}，重复 ${data.summary?.duplicates || 0}，失败 ${data.summary?.failed || 0}，自动关闭 AI ${data.summary?.aiDisabled || 0}`,
    );
    await loadEmotes();
  } finally {
    importing.value = false;
  }
}
function splitKeywords(value: string) {
  return value
    .split(/[，,；;\n]/)
    .map((item) => item.trim())
    .filter(Boolean);
}
function formatBytes(value: number) {
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / 1024 / 1024).toFixed(1)} MB`;
}
onMounted(async () => {
  await Promise.all([loadGroups(), loadCharacters()]);
  await loadEmotes();
});
</script>

<style scoped>
.emote-page {
  height: 100%;
  min-height: 100%;
  display: flex;
  flex-direction: column;
  padding: 0;
  min-width: 0;
}
.toolbar,
.bulk,
.detail-actions,
.dialog-footer {
  display: flex;
  gap: 8px;
  align-items: center;
}
.workspace {
  flex: 1;
  min-height: 0;
  display: grid;
  grid-template-columns: 210px minmax(380px, 1fr) 300px;
  border: 1px solid var(--ac-color-border-light);
  /* border-radius: var(--ac-radius-lg); */
  overflow: hidden;
  background: var(--tp-glass-bg-strong);
  backdrop-filter: blur(var(--tp-glass-blur));
}
.groups,
.detail {
  min-height: 0;
  overflow: auto;
  padding: 14px;
  background: var(--ac-color-surface);
}
.groups {
  border-right: 1px solid var(--ac-color-border-light);
}
.detail {
  border-left: 1px solid var(--ac-color-border-light);
}
.groups > button,
.group > button {
  border: 0;
  background: transparent;
  color: var(--ac-color-text);
  cursor: pointer;
}
.groups > button {
  width: 100%;
  height: 42px;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 12px;
  border-radius: 8px;
}
.groups > button:hover,
.groups > button.active,
.group.active {
  background: var(--nav-active-bg);
  color: var(--nav-active-color);
}
.section-title {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin: 14px 4px 6px;
  color: var(--ac-color-text-muted);
  font-size: 12px;
}
.muted {
  padding: 12px;
  color: var(--ac-color-text-muted);
  font-size: 12px;
}
.group {
  display: flex;
  align-items: center;
  border-radius: 8px;
}
.group > button {
  flex: 1;
  min-width: 0;
  height: 42px;
  display: flex;
  align-items: center;
  gap: 8px;
  text-align: left;
}
.group > button span:last-child {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.cover {
  width: 26px;
  height: 26px;
  border-radius: 7px;
  background: var(--ac-color-bg-secondary);
  overflow: hidden;
}
.cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.list-panel {
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  padding: 14px;
}
.toolbar .el-input {
  flex: 1;
}
.toolbar {
  flex-wrap: wrap;
}
.bulk {
  flex-wrap: wrap;
  padding: 10px 0;
}
.bulk .el-select {
  width: 150px;
}
.select-row {
  display: flex;
  justify-content: space-between;
  padding: 10px 2px;
  color: var(--ac-color-text-muted);
  font-size: 12px;
}
.grid {
  overflow: auto;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(112px, 1fr));
  gap: 10px;
  align-content: start;
}
.card {
  position: relative;
  min-height: 142px;
  padding: 8px;
  border: 1px solid var(--ac-color-border-light);
  border-radius: 10px;
  background: var(--ac-color-surface);
  color: var(--ac-color-text);
  cursor: pointer;
  text-align: left;
  transition:
    border-color 0.2s,
    background 0.2s;
}
.card:hover,
.card.focused {
  border-color: var(--ac-color-primary);
}
.card.selected {
  background: var(--nav-active-bg);
}
.card img {
  width: 100%;
  height: 94px;
  object-fit: contain;
  border-radius: 7px;
  background: var(--ac-color-bg-secondary);
}
.check {
  position: absolute;
  top: 12px;
  left: 12px;
  z-index: 1;
  padding: 3px;
  border-radius: 5px;
  background: var(--ac-color-surface);
}
.card strong {
  display: block;
  margin-top: 6px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
}
.card span {
  display: flex;
  gap: 4px;
}
.card small,
.metadata span {
  padding: 1px 4px;
  border-radius: 4px;
  background: var(--ac-color-bg-secondary);
  color: var(--ac-color-text-muted);
  font-size: 10px;
}
.card small.enabled {
  color: var(--el-color-success);
}
.empty,
.empty-detail {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: var(--ac-color-text-muted);
}
.empty {
  grid-column: 1/-1;
  min-height: 280px;
}
.empty-detail {
  height: 100%;
}
.preview {
  display: block;
  width: 100%;
  height: 210px;
  object-fit: contain;
  border-radius: 10px;
  background: var(--ac-color-bg-secondary);
  margin-bottom: 14px;
}
.detail small {
  color: var(--ac-color-text-muted);
}
fieldset {
  border: 0;
  margin: 0 0 14px;
  padding: 0;
}
legend {
  margin-bottom: 8px;
  color: var(--ac-color-text-secondary);
  font-size: 13px;
}
fieldset .el-select {
  width: 100%;
  margin-top: 8px;
}
.metadata {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin: 8px 0 16px;
}
.detail-actions {
  justify-content: space-between;
}
.defaults-head,
.import-list-head,
.import-item-actions {
  display: flex;
  align-items: center;
}
.defaults-head,
.import-list-head {
  justify-content: space-between;
  gap: 12px;
}
.defaults-head span,
.import-list-head span {
  color: var(--ac-color-text-muted);
  font-size: 12px;
}
.defaults {
  padding-bottom: 14px;
  border-bottom: 1px solid var(--ac-color-border-light);
}
.defaults-row {
  display: grid;
  grid-template-columns: minmax(140px, 1fr) minmax(210px, 1.35fr) minmax(
      150px,
      0.8fr
    ) auto auto;
  gap: 8px;
  margin-top: 10px;
  align-items: center;
}
.import-list-head {
  margin: 14px 2px 6px;
}
.import-list {
  max-height: 58vh;
  overflow: auto;
  padding-right: 4px;
}
.import-item {
  display: grid;
  grid-template-columns: 72px minmax(0, 1fr) auto;
  gap: 12px;
  align-items: center;
  padding: 12px 2px;
  border-bottom: 1px solid var(--ac-color-border-light);
}
.import-item img {
  width: 72px;
  height: 72px;
  object-fit: contain;
  border-radius: 8px;
  background: var(--ac-color-bg-secondary);
}
.import-main {
  display: grid;
  min-width: 0;
  gap: 8px;
}
.import-row {
  display: grid;
  min-width: 0;
  gap: 8px;
  align-items: center;
}
.primary-row {
  grid-template-columns: minmax(160px, 0.8fr) minmax(240px, 1.5fr);
}
.settings-row {
  grid-template-columns: minmax(180px, 1.2fr) minmax(130px, 0.8fr) minmax(
      110px,
      0.7fr
    ) minmax(130px, 0.8fr) auto;
}
.import-row .el-select {
  width: 100%;
}
.import-item-actions {
  flex-direction: column;
  gap: 4px;
}
.status {
  padding: 4px 8px;
  border-radius: 999px;
  background: var(--ac-color-bg-secondary);
  font-size: 12px;
  color: var(--ac-color-text-muted);
  white-space: nowrap;
}
.status[data-status="成功"] {
  background: color-mix(in srgb, var(--el-color-success) 12%, transparent);
  color: var(--el-color-success);
}
.status[data-status="失败"] {
  background: color-mix(in srgb, var(--el-color-danger) 12%, transparent);
  color: var(--el-color-danger);
}
.status[data-status="重复"] {
  background: color-mix(in srgb, var(--el-color-warning) 12%, transparent);
  color: var(--el-color-warning);
}
.dialog-footer {
  justify-content: flex-end;
}
.dialog-footer span {
  margin-right: auto;
}
@media (max-width: 1100px) {
  .workspace {
    grid-template-columns: 180px 1fr;
  }
  .detail {
    display: none;
  }
  .defaults-row,
  .settings-row {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
@media (max-width: 720px) {
  .toolbar .el-input {
    flex-basis: 100%;
  }
  .workspace {
    grid-template-columns: 1fr;
  }
  .groups {
    display: none;
  }
  .grid {
    grid-template-columns: repeat(3, minmax(90px, 1fr));
  }
  .defaults-row,
  .primary-row,
  .settings-row {
    grid-template-columns: 1fr;
  }
  .import-item {
    grid-template-columns: 60px minmax(0, 1fr) auto;
    gap: 10px;
    align-items: start;
  }
  .import-item img {
    width: 60px;
    height: 60px;
  }
}
</style>
