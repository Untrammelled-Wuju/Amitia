<template>
  <div class="wasm-runtime-view">
    <div class="wasm-header">
      <div class="header-left">
        <h2>WASM Runtime</h2>
        <p class="subtitle">WebAssembly Runtime — 模块管理、实例监控、调用测试</p>
      </div>
      <div class="header-right">
        <el-button @click="refreshAll" :icon="Refresh">刷新</el-button>
      </div>
    </div>

    <el-tabs v-model="activeTab" class="wasm-tabs">
      <el-tab-pane label="定义" name="definitions">
        <div class="tab-toolbar">
          <el-button type="primary" size="small" @click="showCreateDialog = true">新建定义</el-button>
        </div>
        <el-table :data="definitions" v-loading="defLoading" border size="small">
          <el-table-column prop="runtime_definition_id" label="定义ID" min-width="200" show-overflow-tooltip />
          <el-table-column prop="module_id" label="模块ID" min-width="140" show-overflow-tooltip />
          <el-table-column prop="extension_id" label="扩展ID" min-width="140" show-overflow-tooltip />
          <el-table-column prop="engine_type" label="引擎" width="90" />
          <el-table-column prop="abi" label="ABI" width="80" />
          <el-table-column label="策略" width="120">
            <template #default="{ row }">
              <el-tag size="small">{{ row.instance_policy }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="确定性" width="80">
            <template #default="{ row }">
              <el-tag :type="row.deterministic ? 'success' : 'info'" size="small">
                {{ row.deterministic ? '是' : '否' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="version" label="版本" width="80" />
          <el-table-column label="操作" width="100" fixed="right">
            <template #default="{ row }">
              <el-button size="small" type="danger" text @click="doDeleteDef(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <el-tab-pane label="模块" name="modules">
        <div class="tab-toolbar">
          <el-upload
            :auto-upload="false"
            :show-file-list="false"
            accept=".wasm"
            :on-change="onModuleFileChange"
          >
            <el-button type="primary" size="small" :icon="Upload">选择WASM文件</el-button>
          </el-upload>
          <el-input
            v-if="selectedModuleFile"
            v-model="uploadModuleId"
            placeholder="模块ID"
            style="width: 240px"
            size="small"
          />
          <el-button
            v-if="selectedModuleFile"
            type="success"
            size="small"
            :loading="uploadLoading"
            @click="doUploadModule"
          >上传</el-button>
        </div>
        <el-table :data="modules" v-loading="modLoading" border size="small">
          <el-table-column prop="module_id" label="模块ID" min-width="180" show-overflow-tooltip />
          <el-table-column prop="path" label="路径" min-width="200" show-overflow-tooltip />
          <el-table-column prop="hash" label="哈希" min-width="120" show-overflow-tooltip />
          <el-table-column prop="size" label="大小" width="100">
            <template #default="{ row }">{{ formatSize(row.size) }}</template>
          </el-table-column>
          <el-table-column label="有效" width="80">
            <template #default="{ row }">
              <el-tag :type="row.valid ? 'success' : 'danger'" size="small">
                {{ row.valid ? '是' : '否' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="100" fixed="right">
            <template #default="{ row }">
              <el-button size="small" type="danger" text @click="doDeleteModule(row)">卸载</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <el-tab-pane label="实例" name="instances">
        <el-table :data="instances" v-loading="instLoading" border size="small">
          <el-table-column prop="instance_id" label="实例ID" min-width="200" show-overflow-tooltip />
          <el-table-column prop="identity.module_id" label="模块ID" min-width="140" show-overflow-tooltip />
          <el-table-column label="状态" width="100">
            <template #default="{ row }">
              <el-tag :type="stateTagType(row.identity.state)" size="small">
                {{ row.identity.state }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="stats.invocations" label="调用次数" width="90" />
          <el-table-column prop="stats.traps" label="陷阱" width="70" />
          <el-table-column prop="stats.timeouts" label="超时" width="70" />
          <el-table-column label="内存" width="100">
            <template #default="{ row }">{{ formatSize(row.stats.memory_used) }}</template>
          </el-table-column>
          <el-table-column prop="stats.last_error" label="最后错误" min-width="160" show-overflow-tooltip />
        </el-table>
      </el-tab-pane>

      <el-tab-pane label="调用测试" name="invoke">
        <el-form label-width="100px" class="invoke-form">
          <el-form-item label="模块ID">
            <el-select v-model="invokeModuleId" placeholder="选择模块" filterable style="width: 320px">
              <el-option
                v-for="m in modules"
                :key="m.module_id"
                :label="m.module_id"
                :value="m.module_id"
              />
            </el-select>
          </el-form-item>
          <el-form-item label="输入JSON">
            <el-input
              v-model="invokeInput"
              type="textarea"
              :rows="6"
              placeholder='{"key": "value"}'
            />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="invokeLoading" @click="doInvoke">执行调用</el-button>
          </el-form-item>
        </el-form>
        <div v-if="invokeResult" class="invoke-result">
          <el-descriptions :column="3" border size="small" title="调用结果">
            <el-descriptions-item label="耗时">{{ invokeResult.duration }}</el-descriptions-item>
            <el-descriptions-item label="Fuel">{{ invokeResult.fuel_used }}</el-descriptions-item>
            <el-descriptions-item label="缓存">
              <el-tag :type="invokeResult.cached ? 'success' : 'info'" size="small">
                {{ invokeResult.cached ? '命中' : '未命中' }}
              </el-tag>
            </el-descriptions-item>
          </el-descriptions>
          <div class="result-output">
            <pre>{{ JSON.stringify(invokeResult.output, null, 2) }}</pre>
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane label="校验" name="validate">
        <div class="tab-toolbar">
          <el-upload
            :auto-upload="false"
            :show-file-list="false"
            accept=".wasm"
            :on-change="onValidateFileChange"
          >
            <el-button type="primary" size="small" :icon="Upload">选择WASM文件</el-button>
          </el-upload>
          <el-button
            v-if="validateFile"
            type="success"
            size="small"
            :loading="validateLoading"
            @click="doValidate"
          >校验</el-button>
        </div>
        <div v-if="validationReport" class="validate-result">
          <el-descriptions :column="2" border size="small" title="校验报告">
            <el-descriptions-item label="有效">
              <el-tag :type="validationReport.valid ? 'success' : 'danger'" size="small">
                {{ validationReport.valid ? '通过' : '未通过' }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="大小">{{ formatSize(validationReport.module_size) }}</el-descriptions-item>
            <el-descriptions-item label="哈希" :span="2">{{ validationReport.module_hash }}</el-descriptions-item>
          </el-descriptions>
          <div v-if="validationReport.exports.length > 0" class="validate-section">
            <h4>导出函数</h4>
            <el-tag v-for="exp in validationReport.exports" :key="exp" size="small" class="export-tag">{{ exp }}</el-tag>
          </div>
          <div v-if="validationReport.imports.length > 0" class="validate-section">
            <h4>导入函数</h4>
            <el-tag v-for="imp in validationReport.imports" :key="imp" size="small" type="warning" class="export-tag">{{ imp }}</el-tag>
          </div>
          <div v-if="validationReport.errors.length > 0" class="validate-section">
            <h4>错误</h4>
            <el-alert
              v-for="(err, i) in validationReport.errors"
              :key="i"
              :title="err"
              type="error"
              :closable="false"
              class="validate-alert"
            />
          </div>
          <div v-if="validationReport.warnings.length > 0" class="validate-section">
            <h4>警告</h4>
            <el-alert
              v-for="(warn, i) in validationReport.warnings"
              :key="i"
              :title="warn"
              type="warning"
              :closable="false"
              class="validate-alert"
            />
          </div>
        </div>
      </el-tab-pane>
    </el-tabs>

    <el-dialog v-model="showCreateDialog" title="新建WASM Runtime定义" width="680px">
      <el-form :model="createForm" label-width="140px" size="small">
        <el-form-item label="模块ID" required>
          <el-input v-model="createForm.module_id" placeholder="如: echo-module" />
        </el-form-item>
        <el-form-item label="扩展ID">
          <el-input v-model="createForm.extension_id" placeholder="如: com.example.echo" />
        </el-form-item>
        <el-form-item label="模块路径" required>
          <el-input v-model="createForm.module_path" placeholder="如: modules/echo.wasm" />
        </el-form-item>
        <el-form-item label="模块哈希(SHA256)" required>
          <el-input v-model="createForm.module_hash" placeholder="SHA256 hex" />
        </el-form-item>
        <el-form-item label="引擎类型">
          <el-select v-model="createForm.engine_type" style="width: 200px">
            <el-option label="wazero" value="wazero" />
          </el-select>
        </el-form-item>
        <el-form-item label="ABI">
          <el-select v-model="createForm.abi" style="width: 200px">
            <el-option label="raw" value="raw" />
            <el-option label="wit" value="wit" />
            <el-option label="amitia" value="amitia" />
          </el-select>
        </el-form-item>
        <el-form-item label="入口导出">
          <el-input v-model="createForm.entry_export" placeholder="amitia_invoke" />
        </el-form-item>
        <el-form-item label="内存限制(Bytes)">
          <el-input-number v-model="createForm.memory_limit_bytes" :min="65536" :max="1073741824" />
        </el-form-item>
        <el-form-item label="Fuel限制">
          <el-input-number v-model="createForm.fuel_limit" :min="1" />
        </el-form-item>
        <el-form-item label="实例策略">
          <el-select v-model="createForm.instance_policy" style="width: 200px">
            <el-option label="per_invocation" value="per_invocation" />
            <el-option label="pooled" value="pooled" />
            <el-option label="singleton_per_module" value="singleton_per_module" />
          </el-select>
        </el-form-item>
        <el-form-item label="确定性">
          <el-switch v-model="createForm.deterministic" />
        </el-form-item>
        <el-form-item label="最大输出(Bytes)">
          <el-input-number v-model="createForm.max_output_bytes" :min="1" />
        </el-form-item>
        <el-form-item label="最大主机调用">
          <el-input-number v-model="createForm.max_host_calls" :min="1" />
        </el-form-item>
        <el-form-item label="调用超时(ms)">
          <el-input-number v-model="createForm.call_timeout" :min="100" />
        </el-form-item>
        <el-form-item label="允许的导入">
          <el-checkbox-group v-model="createForm.allowed_imports">
            <el-checkbox label="amitia.log">log</el-checkbox>
            <el-checkbox label="amitia.time">time</el-checkbox>
            <el-checkbox label="amitia.random">random</el-checkbox>
            <el-checkbox label="amitia.storage_get">storage_get</el-checkbox>
            <el-checkbox label="amitia.storage_cas">storage_cas</el-checkbox>
            <el-checkbox label="amitia.resource_read">resource_read</el-checkbox>
            <el-checkbox label="amitia.artifact_write">artifact_write</el-checkbox>
            <el-checkbox label="amitia.tool_invoke">tool_invoke</el-checkbox>
          </el-checkbox-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateDialog = false">取消</el-button>
        <el-button type="primary" :loading="createLoading" @click="doCreate">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { Upload, Refresh } from "@element-plus/icons-vue";
import type { UploadFile } from "element-plus";
import {
  listDefinitions,
  createDefinition,
  deleteDefinition,
  listModules,
  uploadModule,
  deleteModule,
  invokeModule,
  listInstances,
  validateModule,
  type WasmRuntimeDefinition,
  type WasmModule,
  type WasmInstance,
  type InvokeResult,
  type ValidationReport,
} from "./wasm-api";

const activeTab = ref("definitions");

const definitions = ref<WasmRuntimeDefinition[]>([]);
const defLoading = ref(false);
const modules = ref<WasmModule[]>([]);
const modLoading = ref(false);
const instances = ref<WasmInstance[]>([]);
const instLoading = ref(false);

const selectedModuleFile = ref<File | null>(null);
const uploadModuleId = ref("");
const uploadLoading = ref(false);

const invokeModuleId = ref("");
const invokeInput = ref('{"message": "hello"}');
const invokeLoading = ref(false);
const invokeResult = ref<InvokeResult | null>(null);

const validateFile = ref<File | null>(null);
const validateLoading = ref(false);
const validationReport = ref<ValidationReport | null>(null);

const showCreateDialog = ref(false);
const createLoading = ref(false);
const createForm = ref({
  module_id: "",
  extension_id: "",
  module_path: "",
  module_hash: "",
  engine_type: "wazero",
  abi: "raw",
  entry_export: "amitia_invoke",
  memory_limit_bytes: 16777216,
  fuel_limit: 1000000,
  instance_policy: "per_invocation",
  deterministic: false,
  max_output_bytes: 1048576,
  max_host_calls: 128,
  call_timeout: 5000,
  allowed_imports: ["amitia.log"] as string[],
});

async function loadDefinitions() {
  defLoading.value = true;
  try {
    definitions.value = await listDefinitions();
  } catch (e: any) {
    ElMessage.error("加载定义失败: " + (e?.message || e));
  } finally {
    defLoading.value = false;
  }
}

async function loadModules() {
  modLoading.value = true;
  try {
    modules.value = await listModules();
  } catch (e: any) {
    ElMessage.error("加载模块失败: " + (e?.message || e));
  } finally {
    modLoading.value = false;
  }
}

async function loadInstances() {
  instLoading.value = true;
  try {
    instances.value = await listInstances();
  } catch (e: any) {
    ElMessage.error("加载实例失败: " + (e?.message || e));
  } finally {
    instLoading.value = false;
  }
}

async function refreshAll() {
  await Promise.all([loadDefinitions(), loadModules(), loadInstances()]);
}

function onModuleFileChange(file: UploadFile) {
  if (file.raw) {
    selectedModuleFile.value = file.raw;
    if (!uploadModuleId.value) {
      uploadModuleId.value = file.name.replace(/\.wasm$/, "");
    }
  }
}

async function doUploadModule() {
  if (!selectedModuleFile.value || !uploadModuleId.value) {
    ElMessage.warning("请选择文件并填写模块ID");
    return;
  }
  uploadLoading.value = true;
  try {
    await uploadModule(uploadModuleId.value, selectedModuleFile.value);
    ElMessage.success("模块上传成功");
    selectedModuleFile.value = null;
    uploadModuleId.value = "";
    await loadModules();
  } catch (e: any) {
    ElMessage.error("上传失败: " + (e?.message || e));
  } finally {
    uploadLoading.value = false;
  }
}

async function doDeleteModule(row: WasmModule) {
  try {
    await ElMessageBox.confirm(`确定卸载模块 ${row.module_id}？`, "卸载确认", { type: "warning" });
    await deleteModule(row.module_id);
    ElMessage.success("已卸载");
    await loadModules();
  } catch (e: any) {
    if (e !== "cancel") ElMessage.error("卸载失败: " + (e?.message || e));
  }
}

async function doDeleteDef(row: WasmRuntimeDefinition) {
  try {
    await ElMessageBox.confirm(`确定删除定义 ${row.runtime_definition_id}？`, "删除确认", { type: "warning" });
    await deleteDefinition(row.runtime_definition_id);
    ElMessage.success("已删除");
    await loadDefinitions();
  } catch (e: any) {
    if (e !== "cancel") ElMessage.error("删除失败: " + (e?.message || e));
  }
}

async function doInvoke() {
  if (!invokeModuleId.value) {
    ElMessage.warning("请选择模块");
    return;
  }
  let parsedInput: unknown;
  try {
    parsedInput = JSON.parse(invokeInput.value);
  } catch {
    ElMessage.error("输入JSON格式错误");
    return;
  }
  invokeLoading.value = true;
  invokeResult.value = null;
  try {
    invokeResult.value = await invokeModule(invokeModuleId.value, parsedInput);
    ElMessage.success("调用成功");
  } catch (e: any) {
    ElMessage.error("调用失败: " + (e?.message || e));
  } finally {
    invokeLoading.value = false;
  }
}

function onValidateFileChange(file: UploadFile) {
  if (file.raw) {
    validateFile.value = file.raw;
    validationReport.value = null;
  }
}

async function doValidate() {
  if (!validateFile.value) {
    ElMessage.warning("请选择WASM文件");
    return;
  }
  validateLoading.value = true;
  try {
    validationReport.value = await validateModule(validateFile.value);
  } catch (e: any) {
    ElMessage.error("校验失败: " + (e?.message || e));
  } finally {
    validateLoading.value = false;
  }
}

async function doCreate() {
  if (!createForm.value.module_id || !createForm.value.module_path || !createForm.value.module_hash) {
    ElMessage.warning("请填写必填字段");
    return;
  }
  createLoading.value = true;
  try {
    await createDefinition({
      ...createForm.value,
      call_timeout: createForm.value.call_timeout * 1000000,
    } as any);
    ElMessage.success("定义创建成功");
    showCreateDialog.value = false;
    await loadDefinitions();
  } catch (e: any) {
    ElMessage.error("创建失败: " + (e?.message || e));
  } finally {
    createLoading.value = false;
  }
}

function formatSize(bytes: number): string {
  if (!bytes) return "0 B";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function stateTagType(state: string): "success" | "warning" | "danger" | "info" {
  switch (state) {
    case "ready": return "success";
    case "running": return "warning";
    case "trapped": return "danger";
    default: return "info";
  }
}

onMounted(() => {
  refreshAll();
});
</script>

<style scoped>
.wasm-runtime-view {
  padding: 24px;
  max-width: 1200px;
  margin: 0 auto;
}

.wasm-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.wasm-header h2 {
  margin: 0 0 4px 0;
  font-size: 22px;
}

.subtitle {
  margin: 0;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.wasm-tabs {
  margin-top: 8px;
}

.tab-toolbar {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
  align-items: center;
}

.invoke-form {
  max-width: 640px;
}

.invoke-result {
  margin-top: 20px;
}

.result-output {
  margin-top: 12px;
  background: var(--el-fill-color-light);
  border-radius: 4px;
  padding: 12px;
  overflow-x: auto;
}

.result-output pre {
  margin: 0;
  font-size: 13px;
  white-space: pre-wrap;
  word-break: break-all;
}

.validate-result {
  margin-top: 16px;
}

.validate-section {
  margin-top: 16px;
}

.validate-section h4 {
  margin: 0 0 8px 0;
  font-size: 14px;
}

.export-tag {
  margin: 2px 4px 2px 0;
}

.validate-alert {
  margin-bottom: 8px;
}
</style>
