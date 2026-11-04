<template>
  <el-dialog
    :model-value="visible"
    title="新建团队"
    width="min(560px, calc(100vw - 32px))"
    align-center
    destroy-on-close
    :close-on-click-modal="!creating"
    :close-on-press-escape="!creating"
    @update:model-value="$emit('update:visible', $event)"
  >
    <el-form label-position="top">
      <el-form-item label="团队模板">
        <el-select v-model="form.templateId" clearable style="width: 100%" :loading="loading" @change="applyTemplate">
          <el-option v-for="template in templates" :key="template.id" :label="template.name" :value="template.id" />
        </el-select>
      </el-form-item>
      <el-form-item label="团队名称" required>
        <el-input v-model="form.name" maxlength="120" placeholder="输入团队名称" />
      </el-form-item>
      <el-form-item label="说明">
        <el-input v-model="form.description" type="textarea" :rows="3" />
      </el-form-item>
      <el-form-item label="负责人">
        <el-select v-model="form.leadAgentId" clearable style="width: 100%" :loading="loading">
          <el-option v-for="role in roles" :key="role.id" :label="role.name" :value="role.id" />
        </el-select>
      </el-form-item>
      <el-form-item label="团队成员">
        <el-select v-model="form.memberIds" multiple filterable collapse-tags style="width: 100%" placeholder="选择一个或多个角色成员">
          <el-option v-for="role in roles" :key="role.id" :label="role.name" :value="role.id" />
        </el-select>
      </el-form-item>
      <el-checkbox v-model="form.sharedMemory">共享团队记忆</el-checkbox>
      <el-checkbox v-model="form.sharedWorkspace">共享团队文件</el-checkbox>
    </el-form>
    <div v-if="error" class="team-create-error" role="alert">{{ error }}</div>
    <template #footer>
      <el-button :disabled="creating" @click="$emit('update:visible', false)">取消</el-button>
      <el-button type="primary" :loading="creating" :disabled="!form.name.trim()" @click="create">创建团队</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import {
  createRoleTeam,
  createRoleTeamFromTemplate,
  listRoleRuntimeProfiles,
  listRoleTeamTemplates,
} from "@/api/role-runtime";
import type { AgentProfile, Team } from "@/api/agent-platform";
import { originalErrorMessage } from "@/runtime/original-error";

const props = defineProps<{ visible: boolean }>();
const emit = defineEmits<{
  "update:visible": [value: boolean];
  created: [team: Team];
}>();

const roles = ref<AgentProfile[]>([]);
const templates = ref<Record<string, any>[]>([]);
const loading = ref(false);
const creating = ref(false);
const error = ref("");
const form = reactive({
  templateId: "",
  name: "",
  description: "",
  leadAgentId: "",
  memberIds: [] as string[],
  sharedMemory: true,
  sharedWorkspace: true,
});

watch(
  () => props.visible,
  (visible) => {
    if (!visible) return;
    Object.assign(form, {
      templateId: "",
      name: "",
      description: "",
      leadAgentId: "",
      memberIds: [],
      sharedMemory: true,
      sharedWorkspace: true,
    });
    error.value = "";
    void loadOptions();
  },
);

async function loadOptions() {
  loading.value = true;
  error.value = "";
  try {
    [roles.value, templates.value] = await Promise.all([
      listRoleRuntimeProfiles(),
      listRoleTeamTemplates(),
    ]);
  } catch (cause) {
    console.error("[original-error]", cause);
    error.value = originalErrorMessage(cause);
  } finally {
    loading.value = false;
  }
}

function applyTemplate(id: string) {
  const template = templates.value.find((item) => item.id === id);
  if (!template) return;
  form.name = String(template.name || "");
  form.description = String(template.description || "");
  form.sharedMemory = Boolean(template.sharedMemory);
  form.sharedWorkspace = Boolean(template.sharedWorkspace);
}

async function create() {
  if (!form.name.trim() || creating.value) return;
  creating.value = true;
  error.value = "";
  try {
    const payload = {
      ...form,
      memberIds: form.memberIds.filter((id) => id !== form.leadAgentId),
    };
    const team = form.templateId
      ? await createRoleTeamFromTemplate(payload)
      : await createRoleTeam(payload);
    emit("created", team);
    emit("update:visible", false);
    ElMessage.success("团队已创建");
  } catch (cause) {
    console.error("[original-error]", cause);
    error.value = originalErrorMessage(cause);
    ElMessage.error(error.value);
  } finally {
    creating.value = false;
  }
}
</script>

<style scoped>
.team-create-error {
  margin-top: 10px;
  padding: 9px 11px;
  border: 1px solid var(--ac-color-danger);
  border-radius: 8px;
  color: var(--ac-color-danger);
  font-size: var(--ac-font-size-xs);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
</style>
