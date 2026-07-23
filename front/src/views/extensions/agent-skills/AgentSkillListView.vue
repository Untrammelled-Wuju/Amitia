<template>
  <main class="agent-page">
    <ExtensionPageHeader
      title="Agent Skills"
      description="导入并管理通用 SKILL.md。脚本始终只能查看，不能执行。"
    >
      <template #actions
        ><div class="header-actions">
          <el-button :icon="Refresh" :loading="loading" @click="load"
            >刷新</el-button
          ><el-button type="primary" :icon="Upload" @click="importDialog = true"
            >导入 Agent Skill</el-button
          >
        </div></template
      >
    </ExtensionPageHeader>

    <el-alert
      title="安全边界"
      type="info"
      :closable="false"
      show-icon
      description="allowed-tools 不会授予权限；MCP 依赖必须展示计划并经用户确认；scripts 不会执行。"
    />

    <el-card shadow="never">
      <el-form :inline="true" label-position="top" @submit.prevent>
        <el-form-item label="搜索"
          ><el-input
            v-model="filters.query"
            clearable
            placeholder="名称或描述"
            @keyup.enter="load"
        /></el-form-item>
        <el-form-item label="作用域"
          ><el-select v-model="filters.scope" clearable placeholder="全部"
            ><el-option label="用户全局" value="global" /><el-option
              label="当前角色"
              value="character" /></el-select
        ></el-form-item>
        <el-form-item label="兼容性"
          ><el-select v-model="filters.status" clearable placeholder="全部"
            ><el-option
              v-for="item in statusOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value" /></el-select
        ></el-form-item>
        <el-form-item label=" "
          ><el-button type="primary" @click="load"
            >筛选</el-button
          ></el-form-item
        >
      </el-form>
    </el-card>

    <el-alert
      v-if="errorText"
      :title="errorText"
      type="error"
      show-icon
      :closable="false"
      role="alert"
    />
    <el-card shadow="never" class="table-card">
      <el-table
        v-loading="loading"
        :data="items"
        row-key="extensionId"
        empty-text="暂无 Agent Skill"
        stripe
      >
        <el-table-column label="名称" min-width="220"
          ><template #default="{ row }"
            ><button
              class="name-button"
              type="button"
              @click="openDetail(row.extensionId)"
            >
              <strong>{{ row.displayName || row.name }}</strong
              ><code>${{ row.name }}</code>
            </button></template
          ></el-table-column
        >
        <el-table-column
          prop="description"
          label="说明"
          min-width="320"
          show-overflow-tooltip
        />
        <el-table-column label="作用域" width="110"
          ><template #default="{ row }"
            ><el-tag type="info">{{
              row.scope === "character" ? "当前角色" : "用户全局"
            }}</el-tag></template
          ></el-table-column
        >
        <el-table-column label="兼容性" width="170"
          ><template #default="{ row }"
            ><el-tag :type="statusType(row.compatibilityStatus)">{{
              statusLabel(row.compatibilityStatus)
            }}</el-tag></template
          ></el-table-column
        >
        <el-table-column label="脚本" width="90"
          ><template #default="{ row }"
            ><el-tag v-if="scriptCount(row)" type="warning"
              >{{ scriptCount(row) }} 个，禁用</el-tag
            ><span v-else class="muted">无</span></template
          ></el-table-column
        >
        <el-table-column label="状态" width="100"
          ><template #default="{ row }"
            ><el-tag :type="row.enabled ? 'success' : 'info'">{{
              row.enabled ? "已启用" : "已禁用"
            }}</el-tag></template
          ></el-table-column
        >
        <el-table-column label="操作" width="210" fixed="right"
          ><template #default="{ row }"
            ><el-button @click="openDetail(row.extensionId)">详情</el-button
            ><el-button
              :type="row.enabled ? 'danger' : 'primary'"
              plain
              :loading="changing === row.extensionId"
              :disabled="row.compatibilityStatus === 'blocked' && !row.enabled"
              @click="toggle(row)"
              >{{ row.enabled ? "禁用" : "启用" }}</el-button
            ></template
          ></el-table-column
        >
      </el-table>
    </el-card>

    <el-dialog
      v-model="importDialog"
      title="导入 Agent Skill"
      width="min(760px, 94vw)"
      destroy-on-close
    >
      <el-steps :active="preview ? 1 : 0" finish-status="success" simple
        ><el-step title="选择来源" /><el-step title="检查并确认"
      /></el-steps>
      
      <div class="scope-row">
        <el-segmented
          v-model="installScope"
          :options="scopeOptions"
          aria-label="安装作用域"
        />
        <div v-if="installScope === 'character'" class="character-field">
          <label for="agent-character-select">导入角色</label>
          <el-select
            id="agent-character-select"
            v-model="selectedCharacterId"
            filterable
            :loading="characterLoading"
            placeholder="请选择角色"
            no-data-text="暂无可用角色"
            aria-label="导入角色"
          >
            <el-option
              v-for="ch in characters"
              :key="String(ch.id)"
              :label="ch.name"
              :value="String(ch.id)"
            />
          </el-select>
          <span v-if="characterLoadError" class="field-error" role="alert">{{ characterLoadError }}</span>
        </div>
      </div><div v-if="!preview" class="source-grid">
        <button class="source-card" type="button" @click="zipInput?.click()">
          <el-icon><Files /></el-icon><strong>选择 ZIP</strong
          ><span>适用于 Web 和桌面端</span>
        </button>
        <button class="source-card" type="button" @click="chooseDirectory">
          <el-icon><FolderOpened /></el-icon><strong>选择文件夹</strong
          ><span>目录内容通过受控选择器传递，并由服务端重新校验</span>
        </button>
        <input
          ref="zipInput"
          hidden
          type="file"
          accept=".zip,application/zip"
          @change="selectZIP"
        />
        <input
          ref="directoryInput"
          hidden
          type="file"
          webkitdirectory
          multiple
          @change="selectDirectory"
        />
      </div>
      <div v-else class="preview-layout">
        <section class="preview-heading">
          <div>
            <h2>
              {{ preview.definition.displayName || preview.definition.name }}
            </h2>
            <p>{{ preview.definition.description }}</p>
          </div>
          <el-tag :type="statusType(preview.compatibilityReport.status)">{{
            statusLabel(preview.compatibilityReport.status)
          }}</el-tag>
        </section>
        <el-alert
          v-if="preview.files.some((item) => item.kind === 'script')"
          title="此 Skill 含 scripts：只允许查看源码，任何情况下都不会执行"
          type="warning"
          show-icon
          :closable="false"
        />
        <el-form label-position="top"
          ><el-form-item label="安装作用域"
            ><el-segmented
              v-model="installScope"
              :options="scopeOptions"
              aria-label="安装作用域"
            /></el-form-item
          ><el-form-item v-if="installScope === 'character'" label="选择角色">
            <el-select v-model="selectedCharacterId" placeholder="当前活跃角色" clearable style="width: 100%">
              <el-option
                v-for="ch in characters"
                :key="String(ch.id)"
                :label="ch.name"
                :value="String(ch.id)"
              />
            </el-select>
          ></el-form-item
          ></el-form
        >
        <section
          v-if="preview.definition.mcpDependencies?.length"
          class="dependency-plan"
        >
          <div class="dependency-heading">
            <div>
              <h3>MCP 依赖计划</h3>
              <p>
                只复用或创建配置，不会自动下载程序。连接外部服务和启动本地进程需要分别确认。
              </p>
            </div>
            <el-tag
              :type="dependencyPlan?.requiredMissing ? 'danger' : 'warning'"
              >{{
                dependencyPlan?.requiredMissing
                  ? "存在缺失依赖"
                  : `${preview.definition.mcpDependencies.length} 项`
              }}</el-tag
            >
          </div>
          <el-table
            v-loading="dependencyLoading"
            :data="dependencyPlan?.items || []"
            size="small"
          >
            <el-table-column label="依赖" min-width="170"
              ><template #default="{ row }"
                ><strong>{{ row.dependency.id }}</strong>
                <div class="muted">
                  {{ row.dependency.required ? "必需" : "可选" }}
                </div></template
              ></el-table-column
            >
            <el-table-column label="方式" width="120"
              ><template #default="{ row }">{{
                row.dependency.transport === "stdio"
                  ? "本地 stdio"
                  : "远程 HTTP"
              }}</template></el-table-column
            >
            <el-table-column label="状态" min-width="160"
              ><template #default="{ row }"
                ><el-tag
                  :type="
                    row.installed
                      ? 'success'
                      : row.needsUserConfiguration
                        ? 'danger'
                        : 'info'
                  "
                  >{{
                    row.installed
                      ? "复用已有服务"
                      : row.needsUserConfiguration
                        ? "需要补充配置"
                        : row.authorizationRequired
                          ? "安装后授权"
                          : "可安装"
                  }}</el-tag
                ></template
              ></el-table-column
            >
            <el-table-column prop="riskLevel" label="风险" width="90" />
          </el-table>
          <el-checkbox v-if="hasHTTPDependency" v-model="confirmHTTP"
            >我确认连接计划中列出的远程 MCP 服务</el-checkbox
          >
          <el-checkbox v-if="hasStdioDependency" v-model="confirmStdio"
            >我确认启动计划中列出的本地程序，且程序已经由我安装</el-checkbox
          >
          <el-checkbox v-if="hasOptionalDependency" v-model="installOptional"
            >同时安装可选依赖</el-checkbox
          >
        </section>
        <el-tabs>
          <el-tab-pane label="兼容报告"
            ><ul class="issue-list">
              <li
                v-for="warning in preview.compatibilityReport.warnings"
                :key="warning.code + warning.path"
              >
                <strong>{{ warning.code }}</strong
                ><span>{{ warning.message }} {{ warning.path }}</span>
              </li>
              <li
                v-if="!preview.compatibilityReport.warnings.length"
                class="muted"
              >
                未发现兼容性警告
              </li>
            </ul></el-tab-pane
          >
          <el-tab-pane label="文件树"
            ><el-table :data="preview.files" size="small"
              ><el-table-column
                prop="path"
                label="路径"
                min-width="280"
              /><el-table-column
                prop="kind"
                label="类型"
                width="120"
              /><el-table-column
                prop="size"
                label="大小"
                width="100"
              /><el-table-column label="可执行" width="90"
                ><template #default>否</template></el-table-column
              ></el-table
            ></el-tab-pane
          >
          <el-tab-pane label="工具映射"
            ><el-table
              :data="preview.compatibilityReport.toolMappings"
              size="small"
              ><el-table-column
                prop="sourceTool"
                label="来源工具" /><el-table-column
                prop="status"
                label="状态" /><el-table-column
                prop="reason"
                label="说明"
                min-width="300" /></el-table
          ></el-tab-pane>
        </el-tabs>
      </div>
      <template #footer
        ><el-button v-if="preview" @click="resetPreview">重新选择</el-button
        ><el-button @click="importDialog = false">取消</el-button
        ><el-button
          v-if="preview"
          type="primary"
          :loading="installing"
          :disabled="installBlocked"
          @click="install"
          >确认安装（默认禁用）</el-button
        ></template
      >
    </el-dialog>

    <el-drawer
      v-model="detailOpen"
      title="Agent Skill 详情"
      size="min(840px, 96vw)"
      destroy-on-close
    >
      <div v-if="detail" class="detail-stack">
        <section class="detail-header">
          <div>
            <h2>
              {{ detail.definition.displayName || detail.definition.name }}
            </h2>
            <code>{{ detail.definition.extensionId }}</code>
          </div>
          <el-tag :type="statusType(detail.definition.compatibilityStatus)">{{
            statusLabel(detail.definition.compatibilityStatus)
          }}</el-tag>
        </section>
        <p>{{ detail.definition.description }}</p>
        <el-descriptions :column="2" border
          ><el-descriptions-item label="来源">{{
            detail.definition.source
          }}</el-descriptions-item
          ><el-descriptions-item label="作用域">{{
            detail.definition.scope
          }}</el-descriptions-item
          ><el-descriptions-item label="License">{{
            detail.definition.license || "未声明"
          }}</el-descriptions-item
          ><el-descriptions-item label="Content Hash"
            ><code>{{
              detail.definition.contentHash
            }}</code></el-descriptions-item
          ><el-descriptions-item label="allowed-tools" :span="2">{{
            detail.definition.allowedTools || "未声明"
          }}</el-descriptions-item></el-descriptions
        >
        <el-alert
          v-if="scriptCount(detail.definition)"
          title="scripts 不可执行"
          type="warning"
          show-icon
          :closable="false"
          description="可以查看文件名和只读源码，但不会进入 PATH、调用解释器或安装依赖。"
        />
        <el-tabs>
          <el-tab-pane label="SKILL.md">
            <pre class="safe-markdown">{{ detail.definition.rawSkillMd }}</pre>
          </el-tab-pane>
          <el-tab-pane label="资源"
            ><el-table :data="detail.definition.resources" size="small"
              ><el-table-column
                prop="path"
                label="路径"
                min-width="300"
              /><el-table-column prop="kind" label="类型" /><el-table-column
                prop="mimeType"
                label="MIME"
              /><el-table-column label="可执行"
                ><template #default>否</template></el-table-column
              ></el-table
            ></el-tab-pane
          >
          <el-tab-pane label="工具映射"
            ><el-table :data="detail.definition.toolMappings" size="small"
              ><el-table-column
                prop="sourceTool"
                label="来源工具" /><el-table-column
                prop="status"
                label="状态" /><el-table-column
                prop="reason"
                label="说明"
                min-width="260" /></el-table
          ></el-tab-pane>
          <el-tab-pane label="最近激活"
            ><el-table :data="detail.activations" size="small"
              ><el-table-column
                prop="createdAt"
                label="时间"
                min-width="170" /><el-table-column
                prop="triggerType"
                label="方式" /><el-table-column
                prop="loadedTokens"
                label="Body Tokens" /><el-table-column
                prop="resourceReads"
                label="资源读取" /><el-table-column
                prop="status"
                label="状态" /></el-table
          ></el-tab-pane>
        </el-tabs>
        <div class="detail-actions">
          <el-button
            :type="detail.definition.enabled ? 'danger' : 'primary'"
            @click="toggle(detail.definition)"
            >{{ detail.definition.enabled ? "禁用" : "启用" }}</el-button
          ><el-button type="danger" plain @click="removeCurrent"
            >移除</el-button
          >
        </div>
      </div>
    </el-drawer>
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { Files, FolderOpened, Refresh, Upload } from "@element-plus/icons-vue";
import ExtensionPageHeader from "../components/ExtensionPageHeader.vue";
import {
  fetchAgentSkill,
  fetchAgentSkills,
  fetchCharacterOptions,
  installAgentSkill,
  installAgentSkillMCPDependencies,
  previewAgentSkillDirectory,
  previewAgentSkillMCPDependencies,
  previewAgentSkillZIP,
  removeAgentSkill,
  removeAgentSkillMCPDependencies,
  resolveCharacterId,
  setAgentSkillEnabled,
} from "../api";
import type {
  AgentSkillCompatibilityStatus,
  AgentSkillDefinition,
  AgentSkillDetail,
  AgentSkillPreview,
  AgentSkillScope,
} from "../types";

const loading = ref(false),
  installing = ref(false),
  importDialog = ref(false),
  detailOpen = ref(false);
const errorText = ref(""),
  changing = ref(""),
  characterId = ref("");
const characters = ref<Array<{ id: string | number; name: string }>>([]);
const selectedCharacterId = ref("");
const zipInput = ref<HTMLInputElement>(),
  directoryInput = ref<HTMLInputElement>();
const items = ref<AgentSkillDefinition[]>([]),
  preview = ref<AgentSkillPreview | null>(null),
  detail = ref<AgentSkillDetail | null>(null);
const installScope = ref<AgentSkillScope>("global");
const scopeOptions = [
  { label: "全局", value: "global" },
  { label: "角色", value: "character" },
];
const characterLoading = ref(false);
const characterLoadError = ref("");
const dependencyPlan = ref<any>(null),
  dependencyLoading = ref(false),
  confirmHTTP = ref(false),
  confirmStdio = ref(false),
  installOptional = ref(false);
const filters = reactive({ query: "", scope: "", status: "" });
const hasHTTPDependency = computed(() =>
  dependencyPlan.value?.items?.some(
    (item: any) =>
      !item.installed && item.dependency.transport === "streamable_http",
  ),
);
const hasStdioDependency = computed(() =>
  dependencyPlan.value?.items?.some(
    (item: any) => !item.installed && item.dependency.transport === "stdio",
  ),
);
const hasOptionalDependency = computed(
  () =>
    preview.value?.definition.mcpDependencies?.some((item) => !item.required) ||
    false,
);
const installBlocked = computed(
  () =>
    preview.value?.compatibilityReport.status === "blocked" ||
    dependencyLoading.value ||
    !!dependencyPlan.value?.requiredMissing ||
    (hasHTTPDependency.value && !confirmHTTP.value) ||
    (hasStdioDependency.value && !confirmStdio.value),
);
const statusOptions = [
  { value: "compatible", label: "兼容" },
  { value: "compatible_with_warnings", label: "兼容（有警告）" },
  { value: "partially_compatible", label: "部分兼容" },
  { value: "blocked", label: "已阻止" },
];
async function load() {
  loading.value = true;
  errorText.value = "";
  try {
    if (!characterId.value) characterId.value = await resolveCharacterId();
    const page = await fetchAgentSkills(characterId.value, filters);
    items.value = page.items || [];
  } catch (error: any) {
    errorText.value =
      error?.response?.data?.detail || error?.message || "加载失败";
  } finally {
    loading.value = false;
  }
}
async function selectZIP(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0];
  if (!file) return;
  await runPreview(() => previewAgentSkillZIP(file));
  (event.target as HTMLInputElement).value = "";
}
async function selectDirectory(event: Event) {
  const list = Array.from((event.target as HTMLInputElement).files || []);
  if (!list.length) return;
  const firstPath =
    (list[0] as File & { webkitRelativePath?: string }).webkitRelativePath ||
    list[0].name;
  const root = firstPath.split("/")[0];
  const files = list.map((file) => ({
    path: (
      (file as File & { webkitRelativePath?: string }).webkitRelativePath ||
      file.name
    )
      .split("/")
      .slice(1)
      .join("/"),
    file,
  }));
  await runPreview(() => previewAgentSkillDirectory(root, files));
  (event.target as HTMLInputElement).value = "";
}
async function chooseDirectory() {
  if (!window.amitiaDesktop?.selectAgentSkillDirectory) {
    directoryInput.value?.click();
    return;
  }
  try {
    const selected = await window.amitiaDesktop.selectAgentSkillDirectory();
    if (!selected) return;
    const files = selected.files.map((item) => {
      const binary = atob(item.base64);
      const bytes = new Uint8Array(binary.length);
      for (let index = 0; index < binary.length; index++)
        bytes[index] = binary.charCodeAt(index);
      return { path: item.path, file: new File([bytes], item.name) };
    });
    await runPreview(() =>
      previewAgentSkillDirectory(selected.rootName, files),
    );
  } catch (error: any) {
    ElMessage.error(error?.message || "目录读取失败");
  }
}
async function runPreview(action: () => Promise<AgentSkillPreview>) {
  loading.value = true;
  try {
    preview.value = await action();
    await loadDependencyPlan();
  } catch (error: any) {
    ElMessage.error(
      error?.response?.data?.detail || error?.message || "导入检查失败",
    );
  } finally {
    loading.value = false;
  }
}
async function loadDependencyPlan() {
  dependencyPlan.value = null;
  confirmHTTP.value = false;
  confirmStdio.value = false;
  if (!preview.value?.definition.mcpDependencies?.length) return;
  dependencyLoading.value = true;
  try {
    dependencyPlan.value = await previewAgentSkillMCPDependencies(
      preview.value.definition.extensionId,
      installScope.value === "character" ? (selectedCharacterId.value || characterId.value) : "",
      preview.value.definition.mcpDependencies,
    );
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.detail || "MCP 依赖计划生成失败");
  } finally {
    dependencyLoading.value = false;
  }
}
function resetPreview() {
  preview.value = null;
  dependencyPlan.value = null;
  confirmHTTP.value = false;
  confirmStdio.value = false;
  installOptional.value = false;
}
async function install() {
  if (!preview.value) return;
  installing.value = true;
  let installed: AgentSkillDefinition | null = null;
  try {
    installed = await installAgentSkill(
      preview.value.previewId,
      installScope.value,
      selectedCharacterId.value || characterId.value,
    );
    if (dependencyPlan.value) {
      dependencyPlan.value.agentSkillExtensionId = installed.extensionId;
      const result = await installAgentSkillMCPDependencies(
        dependencyPlan.value,
        {
          installOptional: installOptional.value,
          confirmHTTP: confirmHTTP.value,
          confirmStdio: confirmStdio.value,
        },
      );
      if (result.status === "awaiting_authorization")
        ElMessage.warning("Agent Skill 已安装，请前往 MCP 服务页面完成授权");
      else ElMessage.success("已安装，MCP 依赖已按确认计划处理");
    } else ElMessage.success("已安装，默认处于禁用状态");
    importDialog.value = false;
    resetPreview();
    await load();
  } catch (error: any) {
    if (installed) {
      await removeAgentSkillMCPDependencies(installed.extensionId).catch(
        () => undefined,
      );
      await removeAgentSkill(installed.extensionId, characterId.value).catch(
        () => undefined,
      );
    }
    ElMessage.error(
      error?.response?.data?.detail || error?.message || "安装失败，已回滚",
    );
  } finally {
    installing.value = false;
  }
}
async function openDetail(id: string) {
  try {
    detail.value = await fetchAgentSkill(id, characterId.value);
    detailOpen.value = true;
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.detail || "详情加载失败");
  }
}
async function toggle(item: AgentSkillDefinition) {
  changing.value = item.extensionId;
  try {
    await setAgentSkillEnabled(
      item.extensionId,
      !item.enabled,
      characterId.value,
    );
    ElMessage.success(item.enabled ? "已禁用" : "已启用");
    await load();
    if (detailOpen.value)
      detail.value = await fetchAgentSkill(item.extensionId, characterId.value);
  } finally {
    changing.value = "";
  }
}
async function removeCurrent() {
  if (!detail.value) return;
  await ElMessageBox.confirm(
    `移除 ${detail.value.definition.name}？Artifact 将归档，MCP 服务配置和历史审计保留。`,
    "确认移除",
    { type: "warning", confirmButtonText: "移除", cancelButtonText: "取消" },
  );
  const dependencyResult = await removeAgentSkillMCPDependencies(
    detail.value.definition.extensionId,
  );
  await removeAgentSkill(
    detail.value.definition.extensionId,
    characterId.value,
  );
  detailOpen.value = false;
  detail.value = null;
  ElMessage.success("已移除，未删除可能共享的 MCP 服务");
  if (dependencyResult?.unreferencedServerIds?.length)
    ElMessage.warning(
      `以下 MCP 服务已无依赖引用，可在 MCP 服务页确认删除：${dependencyResult.unreferencedServerIds.join("、")}`,
    );
  await load();
}
function scriptCount(item: AgentSkillDefinition) {
  return (item.resources || []).filter((resource) => resource.kind === "script")
    .length;
}
function statusLabel(status: AgentSkillCompatibilityStatus) {
  return {
    compatible: "兼容",
    compatible_with_warnings: "兼容（有警告）",
    partially_compatible: "部分兼容",
    blocked: "已阻止",
  }[status];
}
function statusType(status: AgentSkillCompatibilityStatus) {
  return status === "compatible"
    ? "success"
    : status === "blocked"
      ? "danger"
      : "warning";
}
onMounted(load);
watch(importDialog, async (open) => {
  if (open) {
    characters.value = [];
    selectedCharacterId.value = "";
    characterLoading.value = true;
    characterLoadError.value = "";
    try {
      characters.value = await fetchCharacterOptions();
      if (!characters.value.length) characterLoadError.value = "暂无可用角色，请先创建角色";
    } catch {
      characterLoadError.value = "角色列表加载失败，请稍后重试";
    } finally {
      characterLoading.value = false;
    }
  }
});
watch(installScope, () => {
  if (preview.value?.definition.mcpDependencies?.length)
    void loadDependencyPlan();
});
</script>

<style scoped>
.agent-page,
.detail-stack {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-width: 0;
}
.page-header,
.header-actions,
.preview-heading,
.detail-header,
.detail-actions {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}
.page-header h1,
.detail-header h2,
.preview-heading h2 {
  margin: 0;
  color: var(--ac-color-text);
}
.page-header p,
.preview-heading p,
.detail-stack > p {
  margin: 6px 0 0;
  color: var(--ac-color-text-secondary);
  line-height: 1.6;
}
.table-card :deep(.el-card__body) {
  padding: 0;
  overflow: auto;
}
.name-button {
  display: flex;
  min-height: 48px;
  flex-direction: column;
  align-items: flex-start;
  justify-content: center;
  gap: 4px;
  border: 0;
  background: transparent;
  color: var(--ac-color-text);
  cursor: pointer;
  text-align: left;
}
.name-button:focus-visible,
.source-card:focus-visible {
  outline: 3px solid var(--ac-color-primary);
  outline-offset: 2px;
}
.name-button code,
.muted {
  color: var(--ac-color-text-muted);
}
.source-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
  margin-top: 20px;
}
.source-card {
  display: flex;
  min-height: 150px;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  gap: 10px;
  padding: 20px;
  border: 1px solid var(--ac-color-border);
  border-radius: var(--ac-radius-md);
  background: var(--ac-color-surface);
  color: var(--ac-color-text);
  cursor: pointer;
  transition:
    border-color 180ms ease,
    box-shadow 180ms ease;
}
.source-card:hover {
  border-color: var(--ac-color-primary);
  box-shadow: var(--ac-shadow-sm);
}
.source-card .el-icon {
  font-size: 30px;
  color: var(--ac-color-primary);
}
.source-card span {
  color: var(--ac-color-text-secondary);
  text-align: center;
  line-height: 1.5;
}
.preview-layout {
  display: flex;
  flex-direction: column;
  gap: 16px;
  margin-top: 20px;
}
.issue-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.issue-list li {
  display: grid;
  grid-template-columns: minmax(150px, auto) 1fr;
  gap: 12px;
  padding: 10px;
  border-bottom: 1px solid var(--ac-color-border-light);
}
.safe-markdown {
  max-height: 480px;
  margin: 0;
  padding: 16px;
  overflow: auto;
  border: 1px solid var(--ac-color-border);
  border-radius: var(--ac-radius-sm);
  background: var(--ac-color-bg-secondary);
  color: var(--ac-color-text);
  font: 13px/1.7 var(--ac-font-family-mono, monospace);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.detail-actions {
  justify-content: flex-end;
  padding-top: 8px;
  border-top: 1px solid var(--ac-color-border-light);
}
code {
  overflow-wrap: anywhere;
}
@media (max-width: 720px) {
  .page-header,
  .preview-heading,
  .detail-header {
    align-items: stretch;
    flex-direction: column;
  }
  .header-actions {
    justify-content: stretch;
  }
  .header-actions .el-button {
    flex: 1;
  }
  .source-grid {
    grid-template-columns: 1fr;
  }
  .issue-list li {
    grid-template-columns: 1fr;
  }
  .agent-page {
    gap: 12px;
  }
}
.dependency-plan {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 14px;
  border: 1px solid var(--ac-color-border);
  border-radius: var(--ac-radius-md);
  background: var(--ac-color-bg-secondary);
}
.dependency-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}
.dependency-heading h3 {
  margin: 0;
  color: var(--ac-color-text);
  font-size: 16px;
}
.dependency-heading p {
  margin: 4px 0 0;
  color: var(--ac-color-text-secondary);
  font-size: 13px;
  line-height: 1.5;
}
@media (prefers-reduced-motion: reduce) {
  .source-card {
    transition: none;
  }
}
.scope-row {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: 16px;
  margin-bottom: 18px;
}
.character-field {
  display: grid;
  grid-template-columns: auto minmax(220px, 320px);
  align-items: center;
  gap: 8px 12px;
}
.character-field label {
  color: var(--console-text-muted);
  font-size: 13px;
}
.character-field .el-select {
  width: 100%;
}
.field-error {
  grid-column: 2;
  color: var(--el-color-danger);
  font-size: 12px;
}
</style>
