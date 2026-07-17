<template>
  <el-dialog v-model="visible" :title="`${subjectLabel}权限`" width="760px" destroy-on-close @closed="emit('close')">
    <el-alert
      v-if="hasHighRisk"
      title="包含高风险能力"
      description="高风险能力可能写入记忆、访问网络或影响当前会话。请只授予必要范围，并优先使用一次、会话或角色授权。"
      type="warning"
      show-icon
      :closable="false"
      class="risk-alert"
    />
    <div class="permission-list" role="list" aria-label="技能能力授权列表">
      <section v-for="row in rows" :key="row.capability" class="permission-row" role="listitem">
        <div class="permission-copy">
          <div class="permission-title">
            <code>{{ row.capability }}</code>
            <el-tag :type="riskType(row.risk)" size="small">{{ riskLabel(row.risk) }}</el-tag>
          </div>
          <p>{{ row.description }}</p>
        </div>
        <div class="permission-controls">
          <label :for="`decision-${row.capability}`">授权方式</label>
          <el-select :id="`decision-${row.capability}`" v-model="row.decision" @change="normalizeScope(row)">
            <el-option label="拒绝" value="deny" />
            <el-option label="仅一次" value="allow_once" />
            <el-option label="当前会话" value="allow_session" />
            <el-option label="当前角色" value="allow_character" />
            <el-option label="始终允许" value="allow_always" />
          </el-select>
          <template v-if="row.decision === 'allow_session'">
            <label :for="`scope-${row.capability}`">会话 ID</label>
            <el-input :id="`scope-${row.capability}`" v-model="row.scopeId" placeholder="输入当前会话 ID" />
          </template>
          <template v-else-if="row.decision === 'allow_once'">
            <label :for="`scope-${row.capability}`">授权角色</label>
            <el-input :id="`scope-${row.capability}`" v-model="row.scopeId" readonly />
          </template>
        </div>
      </section>
    </div>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="submit">保存授权</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue"
import { ElMessage } from "element-plus"
import type { CapabilityDefinition, PermissionGrant } from "../types"

const props = defineProps<{
  modelValue: boolean
  capabilityNames: string[]
  catalog: CapabilityDefinition[]
  grants: PermissionGrant[]
  characterId: string
  saving?: boolean
  subjectLabel?: string
}>()

const emit = defineEmits<{
  "update:modelValue": [value: boolean]
  save: [grants: PermissionGrant[]]
  close: []
}>()

const visible = computed({ get: () => props.modelValue, set: (value) => emit("update:modelValue", value) })
const subjectLabel = computed(() => props.subjectLabel || "技能")
const rows = ref<PermissionGrant[]>([])
const hasHighRisk = computed(() => rows.value.some((row) => row.risk === "high"))

watch(
  () => [props.modelValue, props.grants, props.catalog, props.capabilityNames] as const,
  () => {
    if (!props.modelValue) return
    const catalog = new Map(props.catalog.map((item) => [item.name, item]))
    const grants = new Map(props.grants.map((item) => [item.capability, item]))
    rows.value = props.capabilityNames.map((name) => {
      const capability = catalog.get(name)
      const grant = grants.get(name)
      return {
        capability: name,
        risk: capability?.risk || "high",
        description: capability?.description || "未知能力",
        decision: grant?.decision || "deny",
        scopeType: grant?.scopeType || "global",
        scopeId: grant?.scopeId || "",
        expiresAt: grant?.expiresAt,
      }
    })
  },
  { immediate: true, deep: true },
)

function normalizeScope(row: PermissionGrant) {
  if (row.decision === "allow_always" || row.decision === "deny") {
    row.scopeType = "global"
    row.scopeId = ""
  } else if (row.decision === "allow_session") {
    row.scopeType = "session"
    row.scopeId = ""
  } else {
    row.scopeType = "character"
    row.scopeId = props.characterId
  }
}

function submit() {
  const missingSession = rows.value.find((row) => row.decision === "allow_session" && !row.scopeId.trim())
  if (missingSession) {
    ElMessage.warning(`请填写 ${missingSession.capability} 的会话 ID`)
    return
  }
  emit("save", rows.value.map((row) => ({ ...row })))
}

function riskType(risk: string) {
  if (risk === "high") return "danger"
  if (risk === "medium") return "warning"
  return "success"
}

function riskLabel(risk: string) {
  if (risk === "high") return "高风险"
  if (risk === "medium") return "中风险"
  return "低风险"
}
</script>

<style scoped>
.risk-alert {
  margin-bottom: 16px;
}

.permission-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  max-height: 58vh;
  overflow-y: auto;
}

.permission-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 240px;
  gap: 20px;
  padding: 16px;
  border: 1px solid var(--ac-color-border);
  border-radius: var(--ac-radius-md);
  background: var(--ac-color-surface);
}

.permission-title {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.permission-title code {
  color: var(--ac-color-text);
  overflow-wrap: anywhere;
}

.permission-copy p {
  margin-top: 8px;
  color: var(--ac-color-text-secondary);
  line-height: 1.6;
}

.permission-controls {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.permission-controls label {
  color: var(--ac-color-text-secondary);
  font-size: var(--ac-font-size-sm);
}

@media (max-width: 720px) {
  .permission-row {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
