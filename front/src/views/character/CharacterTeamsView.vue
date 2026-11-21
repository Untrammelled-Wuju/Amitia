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
    <el-alert v-if="!canWrite" :title="coreData.access.value.reason || '当前设备只能查看统筹中心团队数据'" type="info" :closable="false" show-icon />
    <details v-if="coreData.error.value"><summary>查看原始错误</summary><pre>{{ coreData.error.value }}</pre></details>

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
            <strong>{{ coreOwnershipLabel("团队", team.name) }}</strong>
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
                <h2>{{ coreOwnershipLabel("团队", selectedTeam.team.name) }}</h2>
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
              <el-button size="small" :loading="dispatching" @click="dispatchTeam">调度设备角色</el-button>
            </div>
          </header>

          <div class="team-grid">
            <article class="team-card">
              <h3>角色成员</h3>
              <div v-for="member in selectedTeam.members || []" :key="member.id" class="member-row">
                <div>
                  <strong>{{ memberName(member.memberId) }}</strong>
                  <small>{{ memberOwnerLabel(member) }} · {{ member.memberType }} · {{ member.responderMode }}</small>
                </div>
                <el-tag size="small" :type="member.role === 'lead' ? 'warning' : 'info'">{{ member.role }}</el-tag>
              </div>
              <div class="inline-form">
                <el-select
                  v-model="memberForm.source"
                  style="width: 132px"
                  aria-label="角色来源"
                  @change="onMemberSourceChange"
                >
                  <el-option label="统筹中心角色" value="center" />
                  <el-option label="设备角色" value="device" />
                </el-select>
                <el-select
                  v-if="memberForm.source === 'device'"
                  v-model="memberForm.deviceId"
                  placeholder="选择设备"
                  style="width: 148px"
                  aria-label="目标设备"
                  :loading="devicesLoading"
                  @change="loadDeviceRoles"
                >
                  <el-option
                    v-for="device in devices"
                    :key="device.deviceId"
                    :label="device.name"
                    :value="device.deviceId"
                  />
                </el-select>
                <el-select
                  v-model="memberForm.roleId"
                  placeholder="选择角色"
                  :loading="deviceRolesLoading"
                >
                  <el-option
                    v-for="role in memberRoleOptions"
                    :key="role.id"
                    :label="role.name"
                    :value="role.id"
                  />
                </el-select>
                <el-select v-model="memberForm.role" style="width: 108px">
                  <el-option label="负责人" value="lead" />
                  <el-option label="成员" value="member" />
                </el-select>
                <el-button :loading="addingMember" @click="addMember">加入</el-button>
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
                <el-button :loading="addingContext" @click="addContext">添加共享上下文</el-button>
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
        <el-form-item label="设备角色成员">
          <div class="device-member-picker">
            <el-select
              v-model="createForm.deviceId"
              clearable
              placeholder="选择设备"
              style="width: 180px"
              aria-label="设备角色来源设备"
              :loading="devicesLoading"
              @change="onCreateDeviceChange"
            >
              <el-option
                v-for="device in devices"
                :key="device.deviceId"
                :label="device.name"
                :value="device.deviceId"
              />
            </el-select>
            <el-select
              v-model="createForm.deviceMemberIds"
              multiple
              filterable
              collapse-tags
              placeholder="选择该设备的角色"
              style="flex: 1"
              aria-label="设备角色成员"
              :loading="deviceRolesLoading"
              :disabled="!createForm.deviceId"
            >
              <el-option
                v-for="role in deviceRoles"
                :key="role.id"
                :label="role.name"
                :value="role.id"
              />
            </el-select>
          </div>
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
import { coreOwnershipLabel } from "@/runtime/ownership-label";
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
import type { AgentProfile, Team } from "@/api/role-types";
import { originalErrorMessage } from "@/runtime/original-error";
import { apiClient } from "@/composables/useApi";
import { deviceDisplayName } from "@/runtime/device-display-name";
import { buildTeamMemberPayload } from "@/runtime/team-member-payload";
import { useCoreDataAccess } from "@/composables/useCoreDataAccess";

interface TeamDetail {
  team: Team;
  members?: Array<Record<string, any>>;
  contexts?: Array<Record<string, any>>;
  handoffs?: Array<Record<string, any>>;
}

const route = useRoute();
const coreData = useCoreDataAccess();
const canWrite = computed(() => coreData.access.value.write);
function requireWrite() {
  if (canWrite.value) return true;
  ElMessage.warning(coreData.access.value.reason || "当前设备只能查看统筹中心团队数据");
  return false;
}
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
const dispatching = ref(false);
const createVisible = ref(false);
const editVisible = ref(false);

const createForm = reactive({
  templateId: "",
  name: "",
  description: "",
  leadAgentId: "",
  memberIds: [] as string[],
  deviceId: "",
  deviceMemberIds: [] as string[],
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
const memberForm = reactive({
  source: "center" as "center" | "device",
  deviceId: "",
  roleId: "",
  role: "member",
});
const contextForm = reactive({ kind: "instruction", content: "" });
const devices = ref<Array<{ deviceId: string; name: string }>>([]);
const devicesLoading = ref(false);
const deviceRoles = ref<Array<{ id: string; name: string; revision: number }>>([]);
const deviceRolesLoading = ref(false);
const deviceRoleNames = ref<Map<string, string>>(new Map());

const memberRoleOptions = computed(() => {
  const members = selectedTeam.value?.members || [];
  if (memberForm.source === "device") {
    if (!memberForm.deviceId) return [];
    return deviceRoles.value.filter(
      (role) =>
        !members.some(
          (member) =>
            String(member.ownerType || "") === "device" &&
            String(member.ownerId || "") === memberForm.deviceId &&
            String(member.memberId || "") === role.id,
        ),
    );
  }
  const centerMemberIDs = new Set(
    members
      .filter((member) => String(member.ownerType || "center") !== "device")
      .map((member) => String(member.memberId)),
  );
  return roles.value.filter((role) => !centerMemberIDs.has(role.id));
});

function onMemberSourceChange() {
  memberForm.roleId = "";
  deviceRoles.value = [];
  if (memberForm.source === "device") {
    void loadDevices();
    if (memberForm.deviceId) void loadDeviceRoles();
  }
}

async function loadDevices() {
  devicesLoading.value = true;
  try {
    const response = await apiClient.get("/api/device-mesh/v1/devices");
    const payload = (response as any)?.data?.data ?? (response as any)?.data ?? response;
    const list = Array.isArray(payload?.devices) ? payload.devices : [];
    devices.value = list.map((device: Record<string, unknown>) => ({
      deviceId: String(device.deviceId || ""),
      name: deviceDisplayName(device as any, list as any),
    }));
  } catch (cause) {
    console.error("[original-error]", cause, (cause as { stack?: string })?.stack);
    ElMessage.error(originalErrorMessage(cause));
  } finally {
    devicesLoading.value = false;
  }
}

async function fetchDeviceRoles(deviceId: string) {
  if (!deviceId) return [] as Array<{ id: string; name: string; revision: number }>;
  const response = await apiClient.get("/api/roles", {
    params: { device: deviceId, domain: "local" },
  });
  const payload = (response as any)?.data?.data ?? (response as any)?.data ?? response;
  const list = Array.isArray(payload?.roles) ? payload.roles : [];
  const roles = list.map((role: Record<string, unknown>) => ({
    id: String(role.id || ""),
    name: String(role.name || role.id || ""),
    revision: Number(role.revision) || 0,
  }));
  const names = new Map(deviceRoleNames.value);
  for (const role of roles) names.set(role.id, role.name);
  deviceRoleNames.value = names;
  return roles;
}

async function loadDeviceRoles() {
  memberForm.roleId = "";
  deviceRoles.value = [];
  if (!memberForm.deviceId) return;
  deviceRolesLoading.value = true;
  try {
    deviceRoles.value = await fetchDeviceRoles(memberForm.deviceId);
  } catch (cause) {
    console.error("[original-error]", cause, (cause as { stack?: string })?.stack);
    ElMessage.error(originalErrorMessage(cause));
  } finally {
    deviceRolesLoading.value = false;
  }
}

async function onCreateDeviceChange() {
  createForm.deviceMemberIds = [];
  deviceRoles.value = [];
  if (!createForm.deviceId) return;
  deviceRolesLoading.value = true;
  try {
    deviceRoles.value = await fetchDeviceRoles(createForm.deviceId);
  } catch (cause) {
    console.error("[original-error]", cause, (cause as { stack?: string })?.stack);
    ElMessage.error(originalErrorMessage(cause));
  } finally {
    deviceRolesLoading.value = false;
  }
}

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
  if (!requireWrite()) return;
  Object.assign(createForm, {
    templateId: "",
    name: "",
    description: "",
    leadAgentId: "",
    memberIds: [],
    deviceId: "",
    deviceMemberIds: [],
    sharedMemory: true,
    sharedWorkspace: true,
  });
  createVisible.value = true;
  void loadDevices();
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
  if (!requireWrite()) return;
  if (!createForm.name.trim()) { ElMessage.warning("请先填写团队名称"); return; }
  creating.value = true;
  try {
    await coreData.requireWrite();
    const deviceMembers = createForm.deviceMemberIds.map((roleId) =>
      buildTeamMemberPayload({
        source: "device",
        deviceId: createForm.deviceId,
        roleId,
        role: "member",
        roleRevision: deviceRoles.value.find((role) => role.id === roleId)?.revision,
      }),
    );
    const payload: Record<string, unknown> = {
      templateId: createForm.templateId,
      name: createForm.name,
      description: createForm.description,
      leadAgentId: createForm.leadAgentId,
      memberIds: createForm.memberIds.filter((id) => id !== createForm.leadAgentId),
      sharedMemory: createForm.sharedMemory,
      sharedWorkspace: createForm.sharedWorkspace,
    };
    if (deviceMembers.length) payload.members = deviceMembers;
    const team = createForm.templateId
      ? await createRoleTeamFromTemplate(payload)
      : await createRoleTeam(payload);
    if (createForm.templateId && deviceMembers.length) {
      for (const member of deviceMembers) await addRoleToTeam(team.id, member);
    }
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
  if (!requireWrite()) return;
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
  if (!requireWrite()) return;
  if (!selectedTeam.value) { ElMessage.warning("请先选择团队"); return; }
  if (!editForm.name.trim()) { ElMessage.warning("请先填写团队名称"); return; }
  saving.value = true;
  try {
    await coreData.requireWrite();
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
  if (!requireWrite()) return;
  if (!memberForm.roleId) { ElMessage.warning("请先选择角色"); return; }
  if (memberForm.source === "device" && !memberForm.deviceId) {
    ElMessage.warning("请先选择设备");
    return;
  }
  if (!selectedTeam.value || !memberForm.roleId) return;
  addingMember.value = true;
  try {
    await coreData.requireWrite();
    const role = deviceRoles.value.find((item) => item.id === memberForm.roleId);
    const payload = buildTeamMemberPayload({
      source: memberForm.source,
      deviceId: memberForm.deviceId,
      roleId: memberForm.roleId,
      role: memberForm.role === "lead" ? "lead" : "member",
      roleRevision: role?.revision,
    });
    await addRoleToTeam(selectedTeam.value.team.id, payload);
    await selectTeam(selectedTeam.value.team.id);
    ElMessage.success("角色已加入团队");
  } catch (cause) {
    failure(cause);
  } finally {
    addingMember.value = false;
  }
}

async function addContext() {
  if (!requireWrite()) return;
  if (!contextForm.content.trim()) { ElMessage.warning("请先填写共享上下文"); return; }
  if (!selectedTeam.value || !contextForm.content.trim()) return;
  addingContext.value = true;
  try {
    await coreData.requireWrite();
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
  if (!requireWrite()) return;
  if (!selectedTeam.value) return;
  const result = await ElMessageBox.prompt("输入一条团队测试消息", "团队消息路由").catch((cause) => {
    if (cause === "cancel" || cause === "close") return null;
    throw cause;
  });
  if (!result) return;
  routing.value = true;
  try {
    await coreData.requireWrite();
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

async function dispatchTeam() {
  if (!requireWrite()) return;
  if (!selectedTeam.value) return;
  const result = await ElMessageBox.prompt("输入要调度给设备角色的消息", "调度设备角色").catch((cause) => {
    if (cause === "cancel" || cause === "close") return null;
    throw cause;
  });
  if (!result) return;
  dispatching.value = true;
  try {
    await coreData.requireWrite();
    const response = await apiClient.post(`/api/device-mesh/v1/business/teams/${encodeURIComponent(selectedTeam.value.team.id)}/dispatch`, { message: result.value });
    const payload = (response as any)?.data?.data ?? (response as any)?.data ?? response;
    const lines = (payload?.targets || []).map((item: Record<string, any>) => `${memberOwnerLabel({ ownerType: item.ownerId ? "device" : "center", ownerId: item.ownerId })} · ${memberName(String(item.roleId || ""))} · ${item.status}${item.message ? ` · ${item.message}` : ""}${item.text ? `\n${item.text}` : ""}`);
    await ElMessageBox.alert(lines.join("\n\n") || "没有可调度的目标", "调度结果");
  } catch (cause) {
    failure(cause);
  } finally {
    dispatching.value = false;
  }
}

function memberName(id: string) {
  return (
    roles.value.find((role) => role.id === id)?.name ||
    deviceRoleNames.value.get(id) ||
    id ||
    "系统"
  );
}

function memberOwnerLabel(member: Record<string, any>) {
  if (String(member.ownerType || "") !== "device") return "统筹中心";
  const ownerID = String(member.ownerId || "");
  const device = devices.value.find((item) => item.deviceId === ownerID);
  return `设备${device?.name || ownerID}`;
}

onMounted(async () => {
  await load();
  void loadDevices();
});
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
.inline-form { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; margin-top: 10px; }
.inline-form > .el-select { flex: 1 1 140px; min-width: 0; }
.inline-form > .el-button { flex: 0 0 auto; }
.device-member-picker { display: flex; flex-wrap: wrap; gap: 8px; width: 100%; }
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
