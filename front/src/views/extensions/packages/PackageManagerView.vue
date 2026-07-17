<template>
  <main class="package-page">
    <ExtensionPageHeader title="本地扩展包" description="导入、验证、导出并管理 workflow 与 instructions Skill 的本地版本。" />

    <el-tabs v-model="tab" class="package-tabs" @tab-change="refreshTab">
      <el-tab-pane label="导入扩展" name="import">
        <section class="panel import-panel">
          <div class="scope-row">
            <el-segmented v-model="scopeType" :options="scopeOptions" aria-label="安装作用域" />
            <div v-if="scopeType === 'character'" class="character-field">
              <label for="package-character-select">导入角色</label>
              <el-select id="package-character-select" v-model="scopeId" filterable :loading="characterLoading" placeholder="请选择角色" no-data-text="暂无可用角色" aria-label="导入角色" @change="onCharacterChange">
                <el-option v-for="character in characters" :key="character.id" :label="character.name" :value="String(character.id)" />
              </el-select>
              <span v-if="characterLoadError" class="field-error" role="alert">{{ characterLoadError }}</span>
            </div>
          </div>
          <div class="drop-zone" @dragover.prevent @drop.prevent="onDrop">
            <el-icon><UploadFilled /></el-icon>
            <strong>选择 .amitiax、AgentSkills ZIP 或目录</strong>
            <span>服务端会重新检测格式并执行全部安全校验</span>
            <div class="actions">
              <el-button type="primary" :disabled="characterSelectionRequired" :loading="loading" @click="chooseFile">选择文件</el-button>
              <el-button :disabled="characterSelectionRequired" :loading="loading" @click="chooseDirectory">选择目录</el-button>
            </div>
            <el-progress v-if="loading" class="upload-progress" :percentage="uploadProgress" :status="uploadProgress === 100 ? 'success' : undefined" />
            <input ref="fileInput" class="sr-only" type="file" accept=".amitiax,.zip,application/zip" @change="onFile" />
            <input ref="directoryInput" class="sr-only" type="file" webkitdirectory multiple @change="onDirectory" />
          </div>
        </section>

        <section v-if="preview" class="preview-grid" aria-live="polite">
          <article class="panel summary-panel">
            <div class="title-row"><div><span class="eyebrow">{{ preview.format }}</span><h2>{{ preview.name }}</h2></div><el-tag :type="preview.compatible ? 'success' : 'danger'">{{ preview.compatible ? '兼容' : '阻止安装' }}</el-tag></div>
            <p>{{ preview.description }}</p>
            <dl class="facts">
              <div><dt>ID</dt><dd>{{ preview.id }}</dd></div><div><dt>版本</dt><dd>{{ preview.version }}</dd></div>
              <div><dt>类型</dt><dd>{{ preview.skillType }}</dd></div><div><dt>许可证</dt><dd>{{ preview.license || '未声明' }}</dd></div>
              <div><dt>来源</dt><dd>{{ preview.source }}</dd></div><div><dt>冲突</dt><dd>{{ preview.conflict }}</dd></div>
              <div><dt>Package Hash</dt><dd class="mono">{{ preview.packageHash }}</dd></div><div><dt>Checksum</dt><dd>{{ preview.checksum.valid ? '通过' : '失败' }}</dd></div>
              <div><dt>签名</dt><dd>{{ preview.signature.status }}</dd></div><div><dt>文件</dt><dd>{{ preview.fileCount }} / {{ formatBytes(preview.totalSize) }}</dd></div>
              <div><dt>签名者</dt><dd class="mono">{{ preview.signature.fingerprint || '无' }}</dd></div><div><dt>作用域</dt><dd>{{ preview.scopeType }}{{ preview.scopeId ? ` / ${preview.scopeId}` : '' }}</dd></div>
              <div><dt>引擎兼容性</dt><dd>{{ preview.compatibility || '通用' }}</dd></div><div><dt>校验结果</dt><dd>{{ preview.testStatus }}</dd></div>
            </dl>
          </article>

          <article class="panel risk-panel">
            <h2>权限与风险</h2>
            <div v-if="preview.capabilities.length" class="tag-list"><el-tag v-for="item in preview.capabilities" :key="item" :type="preview.highRiskCapabilities.includes(item) ? 'danger' : 'info'">{{ item }}</el-tag></div>
            <el-empty v-else description="未声明 Capability" :image-size="54" />
            <ul v-if="preview.risks.length" class="message-list"><li v-for="risk in preview.risks" :key="`${risk.code}-${risk.message}`" :class="`risk-${risk.severity}`"><strong>{{ risk.code }}</strong><span>{{ risk.message }}</span></li></ul>
            <div class="confirmations">
              <el-checkbox v-if="preview.signature.status === 'unsigned'" v-model="confirmUnsigned">我确认来源未验证</el-checkbox>
              <el-checkbox v-if="preview.scripts > 0" v-model="confirmScripts">我确认 {{ preview.scripts }} 个 scripts 永远不会被执行</el-checkbox>
              <el-checkbox v-if="upgradeId" v-model="confirmVersionChange">我确认从当前版本升级到 {{ preview.version }}</el-checkbox>
              <el-checkbox v-if="hasRisk('SIGNER_CHANGED')" v-model="confirmSignerChange">我确认签名者发生变化</el-checkbox>
              <el-checkbox v-if="hasRisk('CONFIG_MIGRATION')" v-model="confirmConfigMigration">我确认迁移现有配置</el-checkbox>
              <el-checkbox-group v-model="confirmedCapabilities"><el-checkbox v-for="capability in requiredCapabilityConfirmations" :key="capability" :value="capability">{{ preview.highRiskCapabilities.includes(capability) ? '确认高风险权限' : '确认新增权限' }}：{{ capability }}</el-checkbox></el-checkbox-group>
            </div>
          </article>

          <article class="panel wide-panel">
            <h2>依赖与检查结果</h2>
            <ul class="message-list"><li v-for="dependency in preview.dependencies" :key="dependency.id" :class="dependency.installed ? 'risk-low' : 'risk-high'"><strong>{{ dependency.id }}</strong><span>{{ dependency.installed ? `已安装 ${dependency.version}` : '缺失，需手动导入' }}</span></li></ul>
            <el-alert v-for="error in preview.errors" :key="error" :title="error" type="error" :closable="false" show-icon />
            <el-alert v-for="warning in preview.warnings" :key="warning" :title="warning" type="warning" :closable="false" show-icon />
          </article>

          <article v-if="preview.workflowSteps?.length || preview.agentSkill" class="panel wide-panel">
            <h2>{{ preview.skillType === 'workflow' ? 'Workflow 摘要' : 'AgentSkills 摘要' }}</h2>
            <div v-if="preview.workflowSteps?.length" class="tag-list"><el-tag v-for="step in preview.workflowSteps" :key="step">{{ step }}</el-tag></div>
            <template v-if="preview.agentSkill"><p>{{ preview.agentSkill.definition.description }}</p><div class="tag-list"><el-tag v-for="mapping in preview.agentSkill.definition.toolMappings" :key="`${mapping.sourceTool}-${mapping.targetSkillId}`" type="info">{{ mapping.sourceTool }} → {{ mapping.targetSkillId || mapping.status }}</el-tag></div></template>
          </article>

          <article v-if="upgradeId" class="panel wide-panel">
            <div class="title-row"><h2>升级差异</h2><span>{{ preview.currentVersion }} → {{ preview.version }} · 回滚点 {{ preview.rollbackVersion }}</span></div>
            <pre class="diff-view">{{ JSON.stringify(preview.upgradeDiff, null, 2) }}</pre>
          </article>

          <article class="panel wide-panel">
            <div class="title-row"><h2>文件树</h2><span>{{ preview.references }} references · {{ preview.assets }} assets · {{ preview.scripts }} scripts</span></div>
            <div class="file-tree"><div v-for="file in preview.files" :key="file.path"><span class="mono">{{ file.path }}</span><span>{{ file.kind }} · {{ formatBytes(file.size) }}</span></div></div>
          </article>

          <footer class="install-bar">
            <span>新安装扩展默认禁用，签名不会产生任何 Capability Grant。</span>
            <el-button type="primary" size="large" :disabled="!canInstall" :loading="installing" @click="installPreview">{{ upgradeId ? '确认升级' : '确认安装' }}</el-button>
          </footer>
        </section>
      </el-tab-pane>

      <el-tab-pane label="版本管理" name="versions">
        <section class="panel manager-panel">
          <div class="manager-toolbar"><el-input v-model="extensionId" placeholder="Extension ID" aria-label="Extension ID" /><el-button type="primary" @click="loadExtension">加载</el-button><el-button @click="chooseUpgrade">选择本地升级包</el-button></div>
          <input ref="upgradeInput" class="sr-only" type="file" accept=".amitiax,.zip" @change="onUpgradeFile" />
          <div v-if="versions.length" class="version-layout">
            <el-table :data="versions" row-key="version">
              <el-table-column prop="version" label="版本" min-width="150"><template #default="{ row }"><strong>{{ row.version }}</strong> <el-tag v-if="row.active" type="success" size="small">当前</el-tag></template></el-table-column>
              <el-table-column prop="source" label="来源" min-width="150" /><el-table-column prop="signatureStatus" label="签名" min-width="130" />
              <el-table-column label="摘要" min-width="150"><template #default="{ row }"><span class="mono">{{ row.packageHash.slice(0, 12) }}</span></template></el-table-column>
              <el-table-column label="操作" width="220"><template #default="{ row }"><el-button link type="primary" @click="selectCompare(row.version)">比较</el-button><el-button link type="warning" :disabled="row.active" @click="rollback(row.version)">回滚</el-button><el-button link @click="exportPackage(row.version, 'amitiax')">导出</el-button></template></el-table-column>
            </el-table>
            <div class="manager-actions"><el-button @click="loadDependencies">查看依赖</el-button><el-button @click="exportPackage('', 'agentskills-zip')">导出 AgentSkills ZIP</el-button><el-button type="danger" plain @click="uninstall">卸载</el-button></div>
            <pre v-if="diff" class="diff-view">{{ JSON.stringify(diff, null, 2) }}</pre>
            <pre v-if="dependencies" class="diff-view">{{ JSON.stringify(dependencies, null, 2) }}</pre>
          </div>
          <el-empty v-else description="输入已安装的本地 Extension ID 查看版本历史" />
        </section>
      </el-tab-pane>

      <el-tab-pane label="操作记录" name="operations"><section class="panel"><el-table :data="operations" row-key="id"><el-table-column prop="operation" label="操作" /><el-table-column prop="extensionId" label="扩展" min-width="220" /><el-table-column prop="targetVersion" label="目标版本" /><el-table-column prop="status" label="状态" /><el-table-column prop="errorCode" label="错误" /><el-table-column prop="createdAt" label="时间" min-width="190" /></el-table></section></el-tab-pane>
      <el-tab-pane label="签名信任" name="signers"><section class="panel"><el-alert title="信任签名者只验证本地来源身份，不会授予 Capability，也不会允许 scripts 执行。" type="info" :closable="false" show-icon /><el-table :data="signers" row-key="fingerprint"><el-table-column prop="displayName" label="名称" /><el-table-column prop="algorithm" label="算法" /><el-table-column prop="fingerprint" label="指纹" min-width="360"><template #default="{ row }"><span class="mono">{{ row.fingerprint }}</span></template></el-table-column><el-table-column label="信任" width="120"><template #default="{ row }"><el-switch :model-value="row.trusted" @change="(value: string | number | boolean) => changeTrust(row.fingerprint, Boolean(value))" /></template></el-table-column></el-table></section></el-tab-pane>
    </el-tabs>
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue"
import { useRoute } from "vue-router"
import { ElMessage, ElMessageBox } from "element-plus"
import { UploadFilled } from "@element-plus/icons-vue"
import type { Character } from "@/types"
import ExtensionPageHeader from "../components/ExtensionPageHeader.vue"
import type { PackageImportPreview, PackageOperation, PackageSigner, PackageVersion } from "../types"
import { comparePackageVersions, downloadExtensionPackage, exportExtensionPackage, fetchCharacterOptions, fetchPackageDependencies, fetchPackageOperations, fetchPackageSigners, fetchPackageVersions, installExtensionPackage, previewExtensionDirectory, previewExtensionPackage, previewPackageUninstall, resolveCharacterId, rollbackExtensionPackage, setPackageSignerTrust, uninstallExtensionPackage } from "../api"

const route = useRoute()
const tab = ref("import")
const scopeType = ref<"global" | "character">("global")
const scopeId = ref("")
const scopeOptions = [{ label: "全局", value: "global" }, { label: "角色", value: "character" }]
const characters = ref<Character[]>([])
const characterLoading = ref(false)
const characterLoadError = ref("")
const fileInput = ref<HTMLInputElement>()
const directoryInput = ref<HTMLInputElement>()
const upgradeInput = ref<HTMLInputElement>()
const preview = ref<PackageImportPreview>()
const loading = ref(false)
const uploadProgress = ref(0)
const installing = ref(false)
const confirmUnsigned = ref(false)
const confirmScripts = ref(false)
const confirmVersionChange = ref(false)
const confirmSignerChange = ref(false)
const confirmConfigMigration = ref(false)
const confirmedCapabilities = ref<string[]>([])
const extensionId = ref(String(route.query.id || ""))
const upgradeId = ref("")
const versions = ref<PackageVersion[]>([])
const operations = ref<PackageOperation[]>([])
const signers = ref<PackageSigner[]>([])
const diff = ref<Record<string, unknown>>()
const dependencies = ref<Record<string, unknown>>()
const compareFrom = ref("")

const requiredCapabilityConfirmations = computed(() => Array.from(new Set([...(preview.value?.highRiskCapabilities || []), ...(preview.value?.capabilityConfirmations || [])])))
const canInstall = computed(() => Boolean(preview.value?.availableActions.length && !preview.value.errors.length && preview.value.compatible && (preview.value.signature.status !== "unsigned" || confirmUnsigned.value) && (!preview.value.scripts || confirmScripts.value) && (!upgradeId.value || confirmVersionChange.value) && (!hasRisk("SIGNER_CHANGED") || confirmSignerChange.value) && (!hasRisk("CONFIG_MIGRATION") || confirmConfigMigration.value) && requiredCapabilityConfirmations.value.every(item => confirmedCapabilities.value.includes(item))))
const characterSelectionRequired = computed(() => scopeType.value === "character" && (!scopeId.value || characterLoading.value))

onMounted(async () => { await loadCharacters(); if (extensionId.value) { tab.value = "versions"; await loadExtension() } })
watch(scopeType, async (value, previous) => { if (value !== previous) resetPreview(); if (value === "character" && !characters.value.length) await loadCharacters() })

function resetPreview() { preview.value = undefined; confirmUnsigned.value = false; confirmScripts.value = false; confirmVersionChange.value = false; confirmSignerChange.value = false; confirmConfigMigration.value = false; confirmedCapabilities.value = [] }
async function loadCharacters() { characterLoading.value = true; characterLoadError.value = ""; try { characters.value = await fetchCharacterOptions(); if (!characters.value.length) { scopeId.value = ""; characterLoadError.value = "暂无可用角色，请先创建角色"; return } if (!characters.value.some(item => String(item.id) === scopeId.value)) scopeId.value = await resolveCharacterId(characters.value) } catch (error: any) { characters.value = []; scopeId.value = ""; characterLoadError.value = error?.response?.data?.detail || "角色列表加载失败，请稍后重试" } finally { characterLoading.value = false } }
function onCharacterChange() { resetPreview() }
function hasRisk(code: string) { return Boolean(preview.value?.risks.some(risk => risk.code === code)) }
async function previewFile(file: File, target = "") { if (scopeType.value === "character" && !scopeId.value) return ElMessage.warning("请先选择导入角色"); loading.value = true; uploadProgress.value = 0; resetPreview(); try { preview.value = await previewExtensionPackage(file, scopeType.value, scopeType.value === "character" ? scopeId.value : "", target, value => { uploadProgress.value = value }); upgradeId.value = target } catch (error: any) { ElMessage.error(error?.response?.data?.detail || error?.message || "预览失败") } finally { loading.value = false } }
function onFile(event: Event) { const file = (event.target as HTMLInputElement).files?.[0]; if (file) void previewFile(file); (event.target as HTMLInputElement).value = "" }
async function chooseFile() { const desktop = window.amitiaDesktop; if (!desktop?.selectExtensionPackage) return fileInput.value?.click(); const selected = await desktop.selectExtensionPackage(); if (!selected) return; const bytes = Uint8Array.from(atob(selected.base64), char => char.charCodeAt(0)); await previewFile(new File([bytes], selected.name, { type: "application/zip" })) }
function onDrop(event: DragEvent) { const file = event.dataTransfer?.files?.[0]; if (file) void previewFile(file) }
async function chooseDirectory() { const desktop = window.amitiaDesktop; if (desktop?.selectAgentSkillDirectory) { const selected = await desktop.selectAgentSkillDirectory(); if (selected) await previewDirectoryPayload(selected.rootName, selected.files.map(item => ({ path: item.path, base64: item.base64 }))); return } directoryInput.value?.click() }
async function onDirectory(event: Event) { const files = Array.from((event.target as HTMLInputElement).files || []); if (!files.length) return; const rootName = files[0].webkitRelativePath.split("/")[0]; const payload = await Promise.all(files.map(async file => ({ path: file.webkitRelativePath.split("/").slice(1).join("/"), base64: await fileBase64(file) }))); await previewDirectoryPayload(rootName, payload); (event.target as HTMLInputElement).value = "" }
async function previewDirectoryPayload(rootName: string, files: Array<{ path: string; base64: string }>) { if (scopeType.value === "character" && !scopeId.value) return ElMessage.warning("请先选择导入角色"); loading.value = true; uploadProgress.value = 0; resetPreview(); try { preview.value = await previewExtensionDirectory(rootName, files, scopeType.value, scopeType.value === "character" ? scopeId.value : "", value => { uploadProgress.value = value }); upgradeId.value = "" } catch (error: any) { ElMessage.error(error?.response?.data?.detail || error?.message || "目录预览失败") } finally { loading.value = false } }
function fileBase64(file: File) { return new Promise<string>((resolve, reject) => { const reader = new FileReader(); reader.onload = () => resolve(String(reader.result).split(",")[1] || ""); reader.onerror = reject; reader.readAsDataURL(file) }) }
async function installPreview() { if (!preview.value || !canInstall.value) return; installing.value = true; try { const result = await installExtensionPackage(preview.value, { unsigned: confirmUnsigned.value, scripts: confirmScripts.value, capabilities: confirmedCapabilities.value, versionChange: confirmVersionChange.value, signerChange: confirmSignerChange.value, configMigration: confirmConfigMigration.value }, upgradeId.value); ElMessage.success(`${result.extensionId} ${result.version} 已安装，当前状态：${result.enabled ? '启用' : '禁用'}`); extensionId.value = result.extensionId; resetPreview(); upgradeId.value = "" } catch (error: any) { ElMessage.error(error?.response?.data?.detail || error?.message || "安装失败") } finally { installing.value = false } }
async function refreshTab(name: string | number) { if (name === "operations") operations.value = await fetchPackageOperations(); if (name === "signers") signers.value = await fetchPackageSigners() }
async function loadExtension() { if (!extensionId.value) return; try { versions.value = await fetchPackageVersions(extensionId.value, scopeType.value, scopeType.value === "character" ? scopeId.value : "") } catch (error: any) { versions.value = []; ElMessage.error(error?.response?.data?.detail || "无法加载版本") } }
function chooseUpgrade() { if (!extensionId.value) return ElMessage.warning("请先输入 Extension ID"); upgradeInput.value?.click() }
function onUpgradeFile(event: Event) { const file = (event.target as HTMLInputElement).files?.[0]; if (file) { tab.value = "import"; void previewFile(file, extensionId.value) } (event.target as HTMLInputElement).value = "" }
async function selectCompare(version: string) { if (!compareFrom.value) { compareFrom.value = version; return ElMessage.info("已选择起始版本，请再选择目标版本") } diff.value = await comparePackageVersions(extensionId.value, compareFrom.value, version, scopeType.value, scopeType.value === "character" ? scopeId.value : ""); compareFrom.value = "" }
async function rollback(version: string) { await ElMessageBox.confirm(`确定回滚到 ${version}？已撤销权限不会恢复。`, "回滚版本", { type: "warning" }); await rollbackExtensionPackage(extensionId.value, version, scopeType.value, scopeType.value === "character" ? scopeId.value : ""); ElMessage.success("回滚完成"); await loadExtension() }
async function exportPackage(version: string, format: "amitiax" | "agentskills-zip") { try { const exported = await exportExtensionPackage(extensionId.value, format, version, scopeType.value, scopeType.value === "character" ? scopeId.value : ""); await ElMessageBox.confirm(`版本：${exported.version}\n格式：${exported.format}\n测试用例：${exported.testsIncluded ? '包含' : '不包含'}\nREADME：${exported.readmeIncluded ? '包含' : '不包含'}\nSBOM：${exported.sbomIncluded ? '包含' : '不包含'}\nscripts：${exported.scriptsIncluded ? '包含但不可执行' : '不包含'}\nSecret 扫描：${exported.secretScan}\n签名：${exported.signatureStatus}\n\n不包含用户配置、Secret、授权记录、聊天或运行历史。AgentSkills ZIP 不包含 Amitia 作用域和启用状态。`, "确认导出", { confirmButtonText: "保存文件" }); await downloadExtensionPackage(extensionId.value, exported) } catch (error: any) { if (error === 'cancel' || error === 'close') return; ElMessage.error(error?.response?.data?.detail || "导出失败") } }
async function loadDependencies() { dependencies.value = await fetchPackageDependencies(extensionId.value, scopeType.value, scopeType.value === "character" ? scopeId.value : "") }
async function uninstall() { const scope = scopeType.value === "character" ? scopeId.value : ""; const detail = await previewPackageUninstall(extensionId.value, scopeType.value, scope); if (detail.dependents.length) return ElMessage.error(`存在反向依赖：${detail.dependents.map(item => item.id).join('、')}`); await ElMessageBox.confirm(`当前版本：${detail.currentVersion}\n启用状态：${detail.enabled ? '启用' : '禁用'}\n自有定时任务：${detail.scheduleCount}\n当前授权：${detail.grants.length ? detail.grants.join('、') : '无'}\n配置：${detail.configPresent ? '将归档' : '无'}\n历史运行：${detail.historicalRuns}\nArtifact：${detail.artifactArchived ? '将归档' : '不处理'}\n\n将清理：${detail.cleanup.join('、')}\n将保留：${detail.preserved.join('、')}`, "卸载扩展", { type: "error", confirmButtonText: "确认卸载" }); await uninstallExtensionPackage(extensionId.value, scopeType.value, scope); versions.value = []; ElMessage.success("扩展已卸载") }
async function changeTrust(fingerprint: string, trusted: boolean) { await setPackageSignerTrust(fingerprint, trusted); signers.value = await fetchPackageSigners() }
function formatBytes(value: number) { if (value < 1024) return `${value} B`; if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`; return `${(value / 1024 / 1024).toFixed(1)} MB` }
</script>

<style scoped>
.package-page { height: 100%; overflow: auto; color: var(--console-text); background: var(--console-bg); }
.page-header, .title-row, .manager-toolbar, .install-bar, .scope-row { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.page-header h1, h2 { margin: 0; }.page-header p, .summary-panel p { margin: 6px 0 0; color: var(--console-text-muted); }.package-tabs { margin-top: 20px; }
.panel { padding: 20px; border: 1px solid var(--console-border); border-radius: 14px; background: var(--console-card-bg); box-shadow: 0 8px 28px rgba(0,0,0,.04); }.import-panel { width: 100%; box-sizing: border-box; }.scope-row { justify-content: flex-start; margin-bottom: 18px; }.character-field { display: grid; grid-template-columns: auto minmax(220px, 320px); align-items: center; gap: 8px 12px; }.character-field label { color: var(--console-text-muted); font-size: 13px; }.character-field .el-select { width: 100%; }.field-error { grid-column: 2; color: var(--el-color-danger); font-size: 12px; }
.drop-zone { min-height: 250px; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 12px; border: 1px dashed var(--el-border-color); border-radius: 12px; background: var(--el-fill-color-lighter); text-align: center; }.drop-zone > .el-icon { font-size: 44px; color: var(--el-color-primary); }.drop-zone span, .install-bar span { color: var(--console-text-muted); }.actions { display: flex; gap: 10px; margin-top: 6px; }.upload-progress { width: min(420px, 80%); }
.preview-grid { display: grid; grid-template-columns: minmax(0, 3fr) minmax(280px, 2fr); gap: 16px; margin-top: 18px; }.wide-panel, .install-bar { grid-column: 1 / -1; }.eyebrow { color: var(--el-color-primary); text-transform: uppercase; font-size: 12px; letter-spacing: .08em; }.facts { display: grid; grid-template-columns: 1fr 1fr; gap: 12px 20px; }.facts div { min-width: 0; }.facts dt { color: var(--console-text-muted); font-size: 12px; }.facts dd { margin: 3px 0 0; overflow-wrap: anywhere; }.mono { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; }
.tag-list { display: flex; flex-wrap: wrap; gap: 8px; margin: 16px 0; }.message-list { list-style: none; margin: 14px 0; padding: 0; display: grid; gap: 8px; }.message-list li { display: flex; justify-content: space-between; gap: 12px; padding: 10px 12px; border-radius: 8px; background: var(--el-fill-color-light); }.risk-high strong { color: var(--el-color-danger); }.risk-medium strong { color: var(--el-color-warning); }.risk-low strong { color: var(--el-color-success); }.confirmations { display: grid; gap: 8px; }.confirmations :deep(.el-checkbox-group) { display: grid; }
.wide-panel .el-alert + .el-alert { margin-top: 8px; }.file-tree { max-height: 320px; overflow: auto; margin-top: 14px; border: 1px solid var(--console-border); border-radius: 10px; }.file-tree > div { display: flex; justify-content: space-between; gap: 16px; padding: 9px 12px; border-bottom: 1px solid var(--console-border); }.file-tree > div:last-child { border-bottom: 0; }.file-tree span:last-child { color: var(--console-text-muted); white-space: nowrap; }.install-bar { position: sticky; bottom: 0; padding: 16px 20px; border: 1px solid var(--console-border); border-radius: 14px; background: color-mix(in srgb, var(--console-card-bg) 92%, transparent); backdrop-filter: blur(14px); z-index: 2; }
.manager-toolbar { justify-content: flex-start; }.manager-toolbar .el-input { max-width: 520px; }.version-layout { margin-top: 18px; }.manager-actions { display: flex; justify-content: flex-end; gap: 10px; margin-top: 14px; }.diff-view { max-height: 420px; overflow: auto; padding: 14px; border-radius: 10px; background: var(--el-fill-color-darker); white-space: pre-wrap; overflow-wrap: anywhere; }.sr-only { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip: rect(0,0,0,0); white-space: nowrap; border: 0; }
@media (max-width: 860px) { .page-header, .install-bar, .manager-toolbar, .scope-row { align-items: stretch; flex-direction: column; }.character-field { grid-template-columns: 1fr; }.field-error { grid-column: 1; }.preview-grid { grid-template-columns: 1fr; }.wide-panel, .install-bar { grid-column: auto; }.facts { grid-template-columns: 1fr; }.file-tree > div { flex-direction: column; gap: 4px; }.manager-toolbar .el-input { max-width: none; } }
@media (prefers-reduced-motion: reduce) { * { scroll-behavior: auto !important; } }
</style>
