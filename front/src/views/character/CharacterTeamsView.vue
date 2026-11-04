<template>
  <div class="team-page">
    <ExtensionPageHeader
      title="团队"
      description="管理角色团队、负责人、成员协作、共享上下文和任务交接。"
      parent-title="角色"
      parent-path="/characters"
      breadcrumb-label="角色页面层级"
      class="team-breadcrumb"
    />

    <div class="team-layout">
      <aside class="team-sidebar">
        <div class="team-sidebar-head">
          <strong>全部团队</strong>
          <el-button size="small" type="primary" @click="openCreate">新建</el-button>
        </div>
        <button
          v-for="team in teams"
          :key="team.id"
          type="button"
          class="team-list-item"
          :class="{ active: selectedTeamId === team.id }"
          @click="selectTeam(team.id)"
        >
          <span class="team-mark">{{ team.name.slice(0, 1) }}</span>
          <span class="team-list-copy">
            <strong>{{ team.name }}</strong>
            <small>{{ team.description || "暂无团队说明" }}</small>
          </span>
          <el-tag size="small" type="success">{{ team.status }}</el-tag>
        </button>
        <div v-if="!loading && teams.length === 0" class="team-empty">暂无团队</div>
      </aside>

      <main class="team-main">
        <div v-if="error" class="team-error" role="alert">{{ error }}</div>
        <div v-if="loading" class="team-loading">正在加载团队…</div>
        <template v-else-if="selectedTeam">
          <header class="team-detail-head">
            <div>
              <div class="team-title-line">
                <h2>{{ selectedTeam.team.name }}</h2>
                <el-tag size="small">{{ selectedTeam.team.status }}</el-tag>
                <span>revision {{ selectedTeam.team.revision }}</span>
              </div>
              <p>{{ selectedTeam.team.description || "暂无团队说明" }}</p>
              <div class="team-tags">
                <span>{{ selectedTeam.team.sharedMemory ? "共享记忆" : "独立记忆" }}</span>
                <span>{{ selectedTeam.team.sharedWorkspace ? "共享文件" : "独立文件" }}</span>
                <span>{{ selectedTeam.members?.length || 0 }} 名成员</span>
                <span>{{ selectedTeam.contexts?.length || 0 }} 条共享上下文</span>
              </div>
            </div>
            <div class="team-actions">
              <el-button size="small" @click="openEdit">编辑</el-button>
              <el-button size="small" :loading="routing" @click="testRoute">测试路由</el-button>
            </div>
          </header>

          <div class="team-grid">
            <article class="team-card">
              <h3>角色成员</h3>
              <div v-for="member in selectedTeam.members || []" :key="member.id" class="member-row">
                <div>
                  <strong>{{ memberName(member.memberId) }}</strong>
                  <small>{{ member.memberType }} · {{ member.responderMode }}</small>
                </div>
                <el-tag size="small" :type="member.role === 'lead' ? 'warning' : 'info'">{{ member.role }}</el-tag>
              </div>
              <div class="inline-form">
                <el-select v-model="memberForm.roleId" placeholder="选择角色">
                  <el-option
                    v-for="role in availableRoles"
                    :key="role.id"
                    :label="role.name"
                    :value="role.id"
                  />
                </el-select>
                <el-select v-model="memberForm.role" style="width: 108px">
                  <el-option label="负责人" value="lead" />
                  <el-option label="成员" value="member" />
                </el-select>
                <el-button :disabled="!memberForm.roleId" :loading="addingMember" @click="addMember">加入</el-button>
              </div>
            </article>

            <article class="team-card">
              <h3>共享上下文</h3>
              <div v-for="context in selectedTeam.contexts || []" :key="context.id" class="context-row">
                <div class="context-meta">
                  <el-tag size="small">{{ context.kind }}</el-tag>
                  <small>{{ context.visibility }}</small>
                </div>
                <p>{{ context.content }}</p>
              </div>
              <div class="context-editor">
                <el-select v-model="contextForm.kind" style="width: 112px">
                  <el-option label="指令" value="instruction" />
                  <el-option label="决策" value="decision" />
                  <el-option label="知识" value="knowledge" />
                </el-select>
                <el-input v-model="contextForm.content" type="textarea" :rows="3" placeholder="添加团队规则、决策或共享知识" />
                <el-button :disabled="!contextForm.content.trim()" :loading="addingContext" @click="addContext">添加共享上下文</el-button>
              </div>
            </article>

            <article class="team-card">
              <h3>任务交接</h3>
              <div v-for="handoff in selectedTeam.handoffs || []" :key="handoff.id" class="member-row">
                <div>
                  <strong>{{ handoff.summary || handoff.taskId }}</strong>
                  <small>{{ memberName(handoff.fromAgentId) }} → {{ memberName(handoff.toAgentId) }}</small>
                </div>
                <el-tag size="small">{{ handoff.status }}</el-tag>
              </div>
              <div v-if="!(selectedTeam.handoffs || []).length" class="team-empty">暂无任务交接</div>
            </article>

          </div>
        </template>
        <div v-else class="team-empty team-empty-main">选择左侧团队查看详情</div>
      </main>
    </div>

    <el-dialog v-model="createVisible" title="新建团队" width="560px">
      <el-form label-position="top">
        <el-form-item label="团队模板">
          <el-select v-model="createForm.templateId" clearable style="width: 100%" @change="applyTemplate">
            <el-option v-for="template in templates" :key="template.id" :label="template.name" :value="template.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="团队名称">
          <el-input v-model="createForm.name" />
        </el-form-item>
        <el-form-item label="说明">
          <el-input v-model="createForm.description" type="textarea" :rows="3" />
        </el-form-item>
        <el-form-item label="负责人">
          <el-select v-model="createForm.leadAgentId" clearable style="width: 100%">
            <el-option v-for="role in roles" :key="role.id" :label="role.name" :value="role.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="团队成员">
          <el-select v-model="createForm.memberIds" multiple filterable collapse-tags style="width: 100%" placeholder="选择一个或多个角色成员">
            <el-option v-for="role in roles" :key="role.id" :label="role.name" :value="role.id" />
          </el-select>
        </el-form-item>
        <el-checkbox v-model="createForm.sharedMemory">共享团队记忆</el-checkbox>
        <el-checkbox v-model="createForm.sharedWorkspace">共享团队文件</el-checkbox>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="createTeam">创建</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="editVisible" title="编辑团队" width="560px">
      <el-form label-position="top">
        <el-form-item label="团队名称">
          <el-input v-model="editForm.name" />
        </el-form-item>
        <el-form-item label="说明">
          <el-input v-model="editForm.description" type="textarea" :rows="3" />
        </el-form-item>
        <el-form-item label="负责人">
          <el-select v-model="editForm.leadAgentId" clearable style="width: 100%">
            <el-option v-for="role in roles" :key="role.id" :label="role.name" :value="role.id" />
          </el-select>
        </el-form-item>
        <el-checkbox v-model="editForm.sharedMemory">共享团队记忆</el-checkbox>
        <el-checkbox v-model="editForm.sharedWorkspace">共享团队文件</el-checkbox>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveTeam">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { useRoute } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import ExtensionPageHeader from "@/views/extensions/components/ExtensionPageHeader.vue";
import {
  addRoleToTeam,
  createRoleTeam,
  createRoleTeamContext,
  createRoleTeamFromTemplate,
  getRoleTeam,
  listRoleRuntimeProfiles,
  listRoleTeams,
  listRoleTeamTemplates,
  routeRoleTeamMessage,
  updateRoleTeam,
} from "@/api/role-runtime";
import type { AgentProfile, Team } from "@/api/agent-platform";
import { originalErrorMessage } from "@/runtime/original-error";

interface TeamDetail {
  team: Team;
  members?: Array<Record<string, any>>;
  contexts?: Array<Record<string, any>>;
  handoffs?: Array<Record<string, any>>;
}

const route = useRoute();
const teams = ref<Team[]>([]);
const roles = ref<AgentProfile[]>([]);
const templates = ref<Record<string, any>[]>([]);
const selectedTeamId = ref("");
const selectedTeam = ref<TeamDetail | null>(null);
const loading = ref(false);
const error = ref("");
const creating = ref(false);
const saving = ref(false);
const addingMember = ref(false);
const addingContext = ref(false);
const routing = ref(false);
const createVisible = ref(false);
const editVisible = ref(false);

const createForm = reactive({
  templateId: "",
  name: "",
  description: "",
  leadAgentId: "",
  memberIds: [] as string[],
  sharedMemory: true,
  sharedWorkspace: true,
});
const editForm = reactive({
  name: "",
  description: "",
  leadAgentId: "",
  sharedMemory: true,
  sharedWorkspace: true,
});
const memberForm = reactive({ roleId: "", role: "member" });
const contextForm = reactive({ kind: "instruction", content: "" });

const availableRoles = computed(() => {
  const members = new Set((selectedTeam.value?.members || []).map((member) => String(member.memberId)));
  return roles.value.filter((role) => !members.has(role.id));
});

function failure(cause: unknown) {
  console.error("[original-error]", cause);
  error.value = originalErrorMessage(cause);
  ElMessage.error(error.value);
}

async function load() {
  loading.value = true;
  error.value = "";
  try {
    const [teamValues, roleValues, templateValues] = await Promise.all([
      listRoleTeams(),
      listRoleRuntimeProfiles(),
      listRoleTeamTemplates(),
    ]);
    teams.value = teamValues;
    roles.value = roleValues;
    templates.value = templateValues;
    const requested = String(route.query.teamId || "");
    const nextID = teamValues.some((team) => team.id === requested) ? requested : teamValues[0]?.id || "";
    if (nextID) await selectTeam(nextID);
    else {
      selectedTeamId.value = "";
      selectedTeam.value = null;
    }
  } catch (cause) {
    failure(cause);
  } finally {
    loading.value = false;
  }
}

async function selectTeam(id: string) {
  selectedTeamId.value = id;
  const detail = await getRoleTeam(id);
  selectedTeam.value = detail as TeamDetail;
  memberForm.roleId = "";
  contextForm.content = "";
}

function openCreate() {
  Object.assign(createForm, {
    templateId: "",
    name: "",
    description: "",
    leadAgentId: "",
    memberIds: [],
    sharedMemory: true,
    sharedWorkspace: true,
  });
  createVisible.value = true;
}

function applyTemplate(templateId: string) {
  const template = templates.value.find((item) => item.id === templateId);
  if (!template) return;
  createForm.name = String(template.name || "");
  createForm.description = String(template.description || "");
  createForm.sharedMemory = Boolean(template.sharedMemory);
  createForm.sharedWorkspace = Boolean(template.sharedWorkspace);
}

async function createTeam() {
  if (!createForm.name.trim()) return;
  creating.value = true;
  try {
    const payload = {
      ...createForm,
      memberIds: createForm.memberIds.filter((id) => id !== createForm.leadAgentId),
    };
    const team = createForm.templateId
      ? await createRoleTeamFromTemplate(payload)
      : await createRoleTeam(payload);
    createVisible.value = false;
    await load();
    await selectTeam(team.id);
    ElMessage.success("团队已创建");
  } catch (cause) {
    failure(cause);
  } finally {
    creating.value = false;
  }
}

function openEdit() {
  if (!selectedTeam.value) return;
  Object.assign(editForm, {
    name: selectedTeam.value.team.name,
    description: selectedTeam.value.team.description,
    leadAgentId: selectedTeam.value.team.leadAgentId,
    sharedMemory: selectedTeam.value.team.sharedMemory,
    sharedWorkspace: selectedTeam.value.team.sharedWorkspace,
  });
  editVisible.value = true;
}

async function saveTeam() {
  if (!selectedTeam.value || !editForm.name.trim()) return;
  saving.value = true;
  try {
    await updateRoleTeam(selectedTeam.value.team.id, {
      ...editForm,
      expectedRevision: selectedTeam.value.team.revision,
    });
    editVisible.value = false;
    await load();
    await selectTeam(selectedTeam.value.team.id);
    ElMessage.success("团队已更新");
  } catch (cause) {
    failure(cause);
  } finally {
    saving.value = false;
  }
}

async function addMember() {
  if (!selectedTeam.value || !memberForm.roleId) return;
  addingMember.value = true;
  try {
    await addRoleToTeam(selectedTeam.value.team.id, {
      memberType: "agent",
      memberId: memberForm.roleId,
      role: memberForm.role,
      responderMode: memberForm.role === "lead" ? "lead" : "mention_or_lead",
      capabilities: memberForm.role === "lead" ? ["*"] : [],
    });
    await selectTeam(selectedTeam.value.team.id);
    ElMessage.success("角色已加入团队");
  } catch (cause) {
    failure(cause);
  } finally {
    addingMember.value = false;
  }
}

async function addContext() {
  if (!selectedTeam.value || !contextForm.content.trim()) return;
  addingContext.value = true;
  try {
    await createRoleTeamContext(selectedTeam.value.team.id, {
      kind: contextForm.kind,
      key: "",
      content: contextForm.content.trim(),
      visibility: "team",
    });
    await selectTeam(selectedTeam.value.team.id);
    ElMessage.success("共享上下文已添加");
  } catch (cause) {
    failure(cause);
  } finally {
    addingContext.value = false;
  }
}

async function testRoute() {
  if (!selectedTeam.value) return;
  const result = await ElMessageBox.prompt("输入一条团队测试消息", "团队消息路由").catch((cause) => {
    if (cause === "cancel" || cause === "close") return null;
    throw cause;
  });
  if (!result) return;
  routing.value = true;
  try {
    const value = await routeRoleTeamMessage(selectedTeam.value.team.id, {
      message: result.value,
      conversationId: `team-route-${crypto.randomUUID()}`,
    });
    await ElMessageBox.alert(JSON.stringify(value, null, 2), "路由结果");
  } catch (cause) {
    failure(cause);
  } finally {
    routing.value = false;
  }
}

function memberName(id: string) {
  return roles.value.find((role) => role.id === id)?.name || id || "系统";
}

onMounted(load);
</script>

<style scoped>
.team-page {
  display: flex;
  flex-direction: column;
  min-height: 100%;
  color: var(--text-primary);
}
.team-breadcrumb { flex-shrink: 0; padding: 20px 24px 16px; }
.team-layout { display: grid; grid-template-columns: 250px minmax(0, 1fr); flex: 1; min-height: 0; }
.team-sidebar { min-height: 0; padding: 10px; border-right: 1px solid var(--surface-border); overflow-y: auto; background: var(--el-bg-color); }
.team-sidebar-head { display: flex; align-items: center; justify-content: space-between; gap: 10px; padding: 4px 4px 12px; }
.team-list-item { display: flex; align-items: center; width: 100%; gap: 9px; padding: 9px; border: 0; border-radius: 8px; background: transparent; color: inherit; cursor: pointer; text-align: left; }
.team-list-item:hover, .team-list-item.active { background: var(--workbench-sidebar-active); }
.team-mark { display: grid; place-items: center; width: 32px; height: 32px; flex: 0 0 32px; border-radius: 9px; background: var(--ac-color-primary-light-9); color: var(--ac-color-primary); font-weight: 700; }
.team-list-copy { display: grid; min-width: 0; flex: 1; gap: 2px; }
.team-list-copy strong, .team-list-copy small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.team-list-copy small { color: var(--text-muted); font-size: 11px; }
.team-main { min-width: 0; padding: 16px 20px 28px; overflow-y: auto; }
.team-detail-head { display: flex; justify-content: space-between; gap: 18px; padding: 18px; border: 1px solid var(--surface-border); border-radius: 14px; background: var(--surface-bg); }
.team-title-line, .team-actions, .team-tags, .context-meta, .inline-form { display: flex; align-items: center; gap: 8px; }
.team-title-line h2 { margin: 0; font-size: 22px; }
.team-title-line span, .team-detail-head p, .member-row small, .context-row p { color: var(--text-muted); }
.team-detail-head p { margin: 8px 0 0; }
.team-tags { flex-wrap: wrap; margin-top: 12px; }
.team-tags span { padding: 5px 8px; border-radius: 7px; background: var(--workbench-sidebar-hover); font-size: 12px; }
.team-actions { align-self: flex-start; flex-wrap: wrap; justify-content: flex-end; }
.team-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; margin-top: 14px; }
.team-card { min-width: 0; padding: 15px; border: 1px solid var(--surface-border); border-radius: 12px; background: var(--surface-bg); }
.team-card h3 { margin: 0 0 12px; font-size: 15px; }
.member-row { display: flex; align-items: center; justify-content: space-between; gap: 10px; padding: 9px 0; border-top: 1px solid var(--surface-border); }
.member-row > div { min-width: 0; }
.member-row strong, .member-row small { display: block; overflow-wrap: anywhere; }
.inline-form { display: grid; grid-template-columns: minmax(0, 1fr) 108px auto; margin-top: 10px; }
.context-row { padding: 9px 0; border-top: 1px solid var(--surface-border); }
.context-row p { margin: 7px 0 0; white-space: pre-wrap; line-height: 1.55; }
.context-editor { display: grid; gap: 8px; margin-top: 10px; }
.team-error { padding: 10px 12px; border: 1px solid var(--ac-color-danger); border-radius: 8px; color: var(--ac-color-danger); white-space: pre-wrap; overflow-wrap: anywhere; user-select: text; }
.team-loading, .team-empty { padding: 28px 16px; color: var(--text-muted); text-align: center; }
.team-empty-main { place-self: center; }
@media (max-width: 900px) {
  .team-layout { grid-template-columns: 200px minmax(0, 1fr); }
  .team-grid { grid-template-columns: 1fr; }
  .team-detail-head { flex-direction: column; }
  .team-actions { justify-content: flex-start; }
}
</style>
