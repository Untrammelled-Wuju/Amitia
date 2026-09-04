<template>
  <div class="temporal-settings">
    <div class="page-header">
      <div>
        <h2>时间与地区</h2>
        <p>管理用户当地时间、安静时段、节日地区和可被角色感知的日期。</p>
      </div>
      <el-button type="primary" :loading="saving" @click="saveProfile"
        >保存设置</el-button
      >
    </div>

    <el-alert
      v-if="profile.pendingTimezoneSuggestion"
      type="info"
      show-icon
      :closable="false"
      class="suggestion-alert"
    >
      <template #title
        >检测到设备时区变为 {{ profile.pendingTimezoneSuggestion }}</template
      >
      <div class="suggestion-actions">
        <el-button size="small" type="primary" @click="resolveSuggestion(true)"
          >接受变更</el-button
        >
        <el-button size="small" @click="resolveSuggestion(false)"
          >暂不变更</el-button
        >
      </div>
    </el-alert>

    <el-tabs v-model="activeTab">
      <el-tab-pane label="时间与地区" name="profile">
        <el-card shadow="never" class="settings-card" v-loading="loading">
          <el-form label-position="top" :model="profile" class="form-grid">
            <el-form-item label="时间感知">
              <el-switch
                v-model="profile.enabled"
                active-text="开启"
                inactive-text="关闭"
              />
              <div class="field-help">
                关闭后不会向普通聊天 Prompt 注入时间上下文。
              </div>
            </el-form-item>
            <el-form-item label="时区模式">
              <el-radio-group v-model="profile.timezoneMode">
                <el-radio-button value="follow_device"
                  >跟随设备</el-radio-button
                >
                <el-radio-button value="fixed">固定时区</el-radio-button>
              </el-radio-group>
            </el-form-item>
            <el-form-item label="IANA 时区" required>
              <el-select
                v-model="profile.timezone"
                filterable
                allow-create
                default-first-option
                class="full-width"
                placeholder="例如 Asia/Shanghai"
              >
                <el-option
                  v-for="zone in timezoneOptions"
                  :key="zone"
                  :label="zone"
                  :value="zone"
                />
              </el-select>
              <div class="field-help">
                使用 IANA 标识以正确处理夏令时和历史规则。
              </div>
            </el-form-item>
            <el-form-item label="自动检测设备时区">
              <el-switch v-model="profile.autoDetectTimezone" />
              <el-button class="detect-button" @click="detectDeviceTimezone"
                >立即检测</el-button
              >
            </el-form-item>
            <el-form-item label="语言与地区">
              <el-select v-model="profile.locale" class="full-width">
                <el-option label="简体中文" value="zh-CN" />
                <el-option label="繁體中文" value="zh-TW" />
                <el-option label="English (US)" value="en-US" />
                <el-option label="日本語" value="ja-JP" />
              </el-select>
            </el-form-item>
            <el-form-item label="每周起始日">
              <el-select v-model="profile.weekStart" class="full-width">
                <el-option label="星期一" :value="1" />
                <el-option label="星期日" :value="0" />
              </el-select>
            </el-form-item>
            <el-form-item label="所在半球">
              <el-select v-model="profile.hemisphere" class="full-width">
                <el-option label="未知" value="unknown" />
                <el-option label="北半球" value="north" />
                <el-option label="南半球" value="south" />
              </el-select>
            </el-form-item>
            <el-form-item label="安静时段" class="wide-item">
              <div class="quiet-hours">
                <el-switch v-model="quietHours.enabled" active-text="启用" />
                <el-time-picker
                  v-model="quietHours.start"
                  value-format="HH:mm"
                  format="HH:mm"
                  placeholder="开始"
                />
                <span>至</span>
                <el-time-picker
                  v-model="quietHours.end"
                  value-format="HH:mm"
                  format="HH:mm"
                  placeholder="结束"
                />
              </div>
              <div class="field-help">
                主动消息是否可投递始终按用户当地时间判断。
              </div>
            </el-form-item>
            <el-form-item label="节日感知">
              <el-switch v-model="profile.holidayAwareness" />
            </el-form-item>
            <el-form-item label="时段感知">
              <el-switch v-model="profile.daypartAwareness" />
            </el-form-item>
            <el-form-item label="纪念日感知">
              <el-switch v-model="profile.anniversaryAwareness" />
            </el-form-item>
            <el-form-item label="记忆时间共振">
              <el-switch v-model="profile.memoryResonance" />
            </el-form-item>
            <el-form-item label="允许共享日期提及">
              <el-switch v-model="profile.allowSharedDateMention" />
            </el-form-item>
          </el-form>
        </el-card>

        <el-card shadow="never" class="settings-card">
          <template #header>
            <div class="card-header">
              <span>当前时间快照</span
              ><el-button :loading="snapshotLoading" @click="loadSnapshot"
                >刷新</el-button
              >
            </div>
          </template>
          <el-descriptions v-if="snapshot" :column="2" border>
            <el-descriptions-item label="用户当地时间">{{
              formatCivil(snapshot.userTime)
            }}</el-descriptions-item>
            <el-descriptions-item label="角色后备时间">{{
              formatCivil(snapshot.characterTime)
            }}</el-descriptions-item>
            <el-descriptions-item label="当前时段">{{
              snapshot.userTime.daypart || "已关闭"
            }}</el-descriptions-item>
            <el-descriptions-item label="季节">{{
              snapshot.userTime.season
            }}</el-descriptions-item>
            <el-descriptions-item label="安静时段">{{
              snapshot.signals.quietHours ? "是" : "否"
            }}</el-descriptions-item>
            <el-descriptions-item label="时区来源"
              >{{ profile.source }} /
              {{ profile.confidence }}%</el-descriptions-item
            >
          </el-descriptions>
          <el-empty v-else description="暂无时间快照" :image-size="52" />
        </el-card>
      </el-tab-pane>

      <el-tab-pane label="时间锚点" name="anchors">
        <el-card shadow="never" class="settings-card">
          <template #header>
            <div class="card-header">
              <span>生日、纪念日与计划</span
              ><el-button type="primary" @click="openAnchorDialog()"
                ><el-icon><Plus /></el-icon>新增锚点</el-button
              >
            </div>
          </template>
          <el-table :data="anchors" v-loading="anchorsLoading" stripe>
            <el-table-column prop="title" label="名称" min-width="160" />
            <el-table-column prop="anchorType" label="类型" width="140" />
            <el-table-column label="时间" min-width="180"
              ><template #default="{ row }">{{
                anchorTimeLabel(row)
              }}</template></el-table-column
            >
            <el-table-column prop="status" label="状态" width="100"
              ><template #default="{ row }"
                ><el-tag :type="row.status === 'active' ? 'success' : 'info'">{{
                  row.status
                }}</el-tag></template
              ></el-table-column
            >
            <el-table-column label="提及" width="100"
              ><template #default="{ row }">{{
                row.allowPromptMention ? "可提及" : "不提及"
              }}</template></el-table-column
            >
            <el-table-column label="操作" width="210" fixed="right">
              <template #default="{ row }">
                <el-button
                  v-if="row.status === 'candidate'"
                  text
                  type="success"
                  @click="confirmAnchor(row)"
                  >确认</el-button
                >
                <el-button text type="primary" @click="openAnchorDialog(row)"
                  >编辑</el-button
                >
                <el-button text type="danger" @click="removeAnchor(row)"
                  >删除</el-button
                >
              </template>
            </el-table-column>
          </el-table>
          <el-empty
            v-if="!anchorsLoading && anchors.length === 0"
            description="还没有保存时间锚点"
            :image-size="52"
          />
        </el-card>
      </el-tab-pane>
    </el-tabs>

    <el-dialog
      v-model="anchorDialogVisible"
      :title="anchorForm.id ? '编辑时间锚点' : '新增时间锚点'"
      width="min(620px, 92vw)"
      destroy-on-close
    >
      <el-form
        ref="anchorFormRef"
        :model="anchorForm"
        :rules="anchorRules"
        label-position="top"
      >
        <div class="dialog-grid">
          <el-form-item label="名称" prop="title"
            ><el-input v-model="anchorForm.title" maxlength="100"
          /></el-form-item>
          <el-form-item label="类型" prop="anchorType"
            ><el-select v-model="anchorForm.anchorType" class="full-width"
              ><el-option
                v-for="item in anchorTypes"
                :key="item.value"
                :label="item.label"
                :value="item.value" /></el-select
          ></el-form-item>
          <el-form-item label="时间语义" prop="timeKind"
            ><el-select v-model="anchorForm.timeKind" class="full-width"
              ><el-option label="周期规则" value="recurring" /><el-option
                label="年度日期" value="annual_date" /><el-option
                label="一次性日期"
                value="local_date" /><el-option
                label="当地日期时间"
                value="local_datetime" /><el-option
                label="UTC 瞬间"
                value="instant" /><el-option
                label="UTC 时间范围"
                value="range" /></el-select
          ></el-form-item>
          <el-form-item
            v-if="anchorForm.timeKind === 'annual_date'"
            label="月-日"
            prop="localDate"
            ><el-input v-model="anchorForm.localDate" placeholder="08-16"
          /></el-form-item>
          <el-form-item
            v-else-if="anchorForm.timeKind !== 'instant' && anchorForm.timeKind !== 'range'"
            label="当地日期"
            prop="localDate"
            ><el-date-picker
              v-model="anchorForm.localDate"
              type="date"
              value-format="YYYY-MM-DD"
              format="YYYY-MM-DD"
              class="full-width"
          /></el-form-item>
          <el-form-item
            v-if="anchorForm.timeKind !== 'instant' && anchorForm.timeKind !== 'range'"
            label="当地时间"
            ><el-time-picker
              v-model="anchorForm.localTime"
              value-format="HH:mm"
              format="HH:mm"
              class="full-width"
          /></el-form-item>
          <el-form-item
            v-if="anchorForm.timeKind === 'instant' || anchorForm.timeKind === 'range'"
            :label="anchorForm.timeKind === 'range' ? '开始 UTC' : 'UTC 瞬间'"
            prop="instantAtUtc"
            ><el-date-picker
              v-model="anchorForm.instantAtUtc"
              type="datetime"
              value-format="YYYY-MM-DDTHH:mm:ss[Z]"
              class="full-width"
          /></el-form-item>
          <el-form-item v-if="anchorForm.timeKind === 'range'" label="结束 UTC" prop="endAtUtc"
            ><el-date-picker
              v-model="anchorForm.endAtUtc"
              type="datetime"
              value-format="YYYY-MM-DDTHH:mm:ss[Z]"
              class="full-width"
          /></el-form-item>
          <el-form-item v-if="anchorForm.timeKind === 'recurring'" label="RRULE" prop="rrule">
            <el-input v-model="anchorForm.rrule" placeholder="FREQ=WEEKLY;BYDAY=MO,WE,FR" />
          </el-form-item>
          <el-form-item label="IANA 时区"
            ><el-select
              v-model="anchorForm.timezone"
              filterable
              allow-create
              class="full-width"
              ><el-option
                v-for="zone in timezoneOptions"
                :key="zone"
                :label="zone"
                :value="zone" /></el-select
          ></el-form-item>
          <el-form-item label="重要度"
            ><el-slider v-model="anchorForm.importance" :min="0" :max="100"
          /></el-form-item>
          <el-form-item label="置信度"
            ><el-slider v-model="anchorForm.confidence" :min="0" :max="100"
          /></el-form-item>
          <el-form-item label="允许进入聊天上下文"
            ><el-switch v-model="anchorForm.allowPromptMention"
          /></el-form-item>
          <el-form-item label="需要确认"
            ><el-switch v-model="anchorForm.requiresConfirmation"
          /></el-form-item>
          <el-form-item label="描述" class="wide-item"
            ><el-input
              v-model="anchorForm.description"
              type="textarea"
              :rows="3"
              maxlength="500"
          /></el-form-item>
        </div>
      </el-form>
      <template #footer
        ><el-button @click="anchorDialogVisible = false">取消</el-button
        ><el-button type="primary" :loading="anchorSaving" @click="saveAnchor"
          >保存</el-button
        ></template
      >
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref, watch } from "vue";
import {
  ElMessage,
  ElMessageBox,
  type FormInstance,
  type FormRules,
} from "element-plus";
import { Plus } from "@element-plus/icons-vue";
import {
  acceptTemporalTimezoneSuggestion,
  confirmTemporalAnchor,
  createTemporalAnchor,
  deleteTemporalAnchor,
  getTemporalProfile,
  getTemporalSnapshot,
  listTemporalAnchors,
  rejectTemporalTimezoneSuggestion,
  suggestTemporalTimezone,
  updateTemporalAnchor,
  updateTemporalProfile,
  type TemporalAnchor,
  type TemporalProfile,
  type TemporalSnapshot,
} from "@/api/temporal";

const activeTab = ref("profile");
const loading = ref(false);
const saving = ref(false);
const snapshotLoading = ref(false);
const anchorsLoading = ref(false);
const anchorSaving = ref(false);
const snapshot = ref<TemporalSnapshot | null>(null);
const anchors = ref<TemporalAnchor[]>([]);
const anchorDialogVisible = ref(false);
const anchorFormRef = ref<FormInstance>();

const profile = reactive<TemporalProfile>({
  id: "",
  ownerType: "user",
  ownerId: "",
  timezoneMode: "follow_device",
  timezone: "Asia/Shanghai",
  locale: "zh-CN",
  calendarSystem: "gregorian",
  weekStart: 1,
  holidayRegion: "",
  hemisphere: "unknown",
  daypartConfigJson: "{}",
  quietHoursJson: "{}",
  autoDetectTimezone: true,
  travelMode: false,
  awarenessLevel: 70,
  source: "fallback",
  confidence: 30,
  enabled: true,
  holidayAwareness: true,
  daypartAwareness: true,
  anniversaryAwareness: true,
  memoryResonance: true,
  allowSharedDateMention: true,
  version: 1,
  createdAtUtc: "",
  updatedAtUtc: "",
});
const quietHours = reactive({ enabled: true, start: "23:00", end: "07:00" });
const timezoneOptions = [
  "Asia/Shanghai",
  "Asia/Tokyo",
  "Asia/Hong_Kong",
  "Asia/Singapore",
  "Europe/London",
  "Europe/Paris",
  "America/New_York",
  "America/Chicago",
  "America/Denver",
  "America/Los_Angeles",
  "Australia/Sydney",
  "UTC",
];
const anchorTypes = [
  { label: "生日", value: "birthday" },
  { label: "纪念日", value: "anniversary" },
  { label: "关系纪念日", value: "relationship_anniversary" },
  { label: "首次相识", value: "first_meeting" },
  { label: "共同经历", value: "shared_memory" },
  { label: "节日", value: "holiday" },
  { label: "截止日期", value: "deadline" },
  { label: "预约", value: "appointment" },
  { label: "考试", value: "exam" },
  { label: "旅行", value: "travel" },
  { label: "工作事件", value: "work_event" },
  { label: "课程事件", value: "class_event" },
  { label: "自定义", value: "custom" },
];

function emptyAnchor(): Partial<TemporalAnchor> {
  return {
    id: "",
    scopeType: "user",
    characterId: "",
    anchorType: "custom",
    title: "",
    description: "",
    timeKind: "annual_date",
    instantAtUtc: "",
    endAtUtc: "",
    localDate: "",
    localTime: "09:00",
    timezone: profile.timezone,
    rrule: "",
    durationSeconds: 0,
    preWindowSeconds: 259200,
    postWindowSeconds: 86400,
    importance: 70,
    confidence: 100,
    sensitivityLevel: "internal",
    allowPromptMention: true,
    allowProactiveMention: false,
    requiresConfirmation: false,
    source: "manual",
    status: "active",
  };
}
const anchorForm = reactive<Partial<TemporalAnchor>>(emptyAnchor());
const anchorRules: FormRules = {
  title: [{ required: true, message: "请输入锚点名称", trigger: "blur" }],
  anchorType: [{ required: true, message: "请选择类型", trigger: "change" }],
  timeKind: [{ required: true, message: "请选择时间语义", trigger: "change" }],
  localDate: [
    {
      validator: (_rule, value, callback) => {
        if (anchorForm.timeKind !== "instant" && anchorForm.timeKind !== "range" && !value)
          callback(new Error("请选择或输入日期"));
        else callback();
      },
      trigger: "change",
    },
  ],
  instantAtUtc: [
    {
      validator: (_rule, value, callback) => {
        if ((anchorForm.timeKind === "instant" || anchorForm.timeKind === "range") && !value)
          callback(new Error("请选择 UTC 开始时间"));
        else callback();
      },
      trigger: "change",
    },
  ],
  endAtUtc: [
    {
      validator: (_rule, value, callback) => {
        if (anchorForm.timeKind !== "range") return callback();
        if (!value) return callback(new Error("请选择 UTC 结束时间"));
        const start = Date.parse(String(anchorForm.instantAtUtc || ""));
        const end = Date.parse(String(value));
        if (!Number.isFinite(start) || !Number.isFinite(end) || end <= start)
          callback(new Error("结束时间必须晚于开始时间"));
        else callback();
      },
      trigger: "change",
    },
  ],
  rrule: [
    {
      validator: (_rule, value, callback) => {
        if (anchorForm.timeKind === "recurring" && !value)
          callback(new Error("请输入 RRULE"));
        else callback();
      },
      trigger: "blur",
    },
  ],
};

async function loadProfile() {
  loading.value = true;
  try {
    Object.assign(profile, await getTemporalProfile());
    try {
      Object.assign(quietHours, JSON.parse(profile.quietHoursJson || "{}"));
    } catch {}
    await detectDeviceTimezone(true);
  } catch {
    ElMessage.error("加载时间设置失败");
  } finally {
    loading.value = false;
  }
}
async function saveProfile() {
  if (!profile.timezone.trim()) {
    ElMessage.warning("请输入 IANA 时区");
    return;
  }
  saving.value = true;
  try {
    profile.quietHoursJson = JSON.stringify(quietHours);
    profile.source = "explicit";
    Object.assign(profile, await updateTemporalProfile(profile));
    ElMessage.success("时间设置已保存");
    await loadSnapshot();
  } catch {
    ElMessage.error("保存失败，请检查时区格式");
  } finally {
    saving.value = false;
  }
}
async function detectDeviceTimezone(silent = false) {
  const zone = Intl.DateTimeFormat().resolvedOptions().timeZone;
  if (!zone) return;
  if (zone === profile.timezone) {
    if (!silent) ElMessage.success(`设备时区为 ${zone}`);
    return;
  }
  if (!profile.autoDetectTimezone || profile.timezoneMode !== "follow_device") {
    if (!silent) ElMessage.info(`检测到设备时区 ${zone}`);
    return;
  }
  if (sessionStorage.getItem(`temporal-timezone-rejected:${zone}`)) return;
  try {
    Object.assign(profile, await suggestTemporalTimezone(zone));
    if (!silent) ElMessage.info("已生成待确认时区建议");
  } catch {
    if (!silent) ElMessage.error("时区检测失败");
  }
}
async function resolveSuggestion(accept: boolean) {
  const zone = profile.pendingTimezoneSuggestion || "";
  try {
    Object.assign(
      profile,
      accept
        ? await acceptTemporalTimezoneSuggestion()
        : await rejectTemporalTimezoneSuggestion(),
    );
    if (!accept && zone)
      sessionStorage.setItem(`temporal-timezone-rejected:${zone}`, "1");
    ElMessage.success(accept ? "已更新时区" : "已保留当前时区");
    await loadSnapshot();
  } catch {
    ElMessage.error("处理时区建议失败");
  }
}
async function loadSnapshot() {
  snapshotLoading.value = true;
  try {
    snapshot.value = await getTemporalSnapshot();
  } catch {
    snapshot.value = null;
  } finally {
    snapshotLoading.value = false;
  }
}
async function loadAnchors() {
  anchorsLoading.value = true;
  try {
    anchors.value = await listTemporalAnchors();
  } catch {
    ElMessage.error("加载时间锚点失败");
  } finally {
    anchorsLoading.value = false;
  }
}
function openAnchorDialog(anchor?: TemporalAnchor) {
  if (anchor?.timeKind === "derived") {
    ElMessage.info("派生时间锚点由系统维护，不能在此直接编辑");
    return;
  }
  Object.assign(anchorForm, emptyAnchor(), anchor || {});
  anchorDialogVisible.value = true;
}
async function saveAnchor() {
  if (!anchorFormRef.value) return;
  await anchorFormRef.value.validate();
  anchorSaving.value = true;
  try {
    if (anchorForm.id) await updateTemporalAnchor(anchorForm.id, anchorForm);
    else await createTemporalAnchor(anchorForm);
    ElMessage.success("时间锚点已保存");
    anchorDialogVisible.value = false;
    await loadAnchors();
  } catch {
    ElMessage.error("保存时间锚点失败");
  } finally {
    anchorSaving.value = false;
  }
}
async function removeAnchor(anchor: TemporalAnchor) {
  try {
    await ElMessageBox.confirm(
      `确定删除“${anchor.title}”及其未来调度吗？`,
      "删除时间锚点",
      { type: "warning" },
    );
    await deleteTemporalAnchor(anchor.id, anchor.characterId);
    ElMessage.success("已删除");
    await loadAnchors();
  } catch (error) {
    if (error !== "cancel" && error !== "close") ElMessage.error("删除失败");
  }
}
async function confirmAnchor(anchor: TemporalAnchor) {
  try {
    await confirmTemporalAnchor(anchor.id, anchor.characterId);
    ElMessage.success("候选锚点已确认");
    await loadAnchors();
  } catch {
    ElMessage.error("确认失败");
  }
}
function formatCivil(value: TemporalSnapshot["userTime"]) {
  return `${value.localTime.replace("T", " ").slice(0, 16)} · ${value.timezone}`;
}
function anchorTimeLabel(anchor: TemporalAnchor) {
  if (anchor.timeKind === "instant") return anchor.instantAtUtc || "—";
  if (anchor.timeKind === "range")
    return `${anchor.instantAtUtc || "—"} → ${anchor.endAtUtc || "—"}`;
  const base = [anchor.localDate, anchor.localTime].filter(Boolean).join(" ");
  return anchor.timeKind === "recurring"
    ? `${base} · ${anchor.rrule}`
    : base || "—";
}
watch(activeTab, (value) => {
  if (value === "anchors") loadAnchors();
});
onMounted(async () => {
  await loadProfile();
  await loadSnapshot();
});
</script>

<style scoped>
.temporal-settings {
  max-width: 980px;
  padding-bottom: 32px;
}
.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}
.page-header h2 {
  margin: 0 0 4px;
  font-size: 20px;
  color: var(--ac-color-text);
}
.page-header p {
  margin: 0;
  color: var(--ac-color-text-muted);
  line-height: 1.5;
  font-size: 13px;
}
.suggestion-alert {
  margin-bottom: 16px;
}
.suggestion-actions {
  display: flex;
  gap: 8px;
  margin-top: 10px;
}
.settings-card {
  margin-bottom: 16px;
  border-color: var(--ac-color-border-light);
}
.form-grid,
.dialog-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 4px 20px;
}
.wide-item {
  grid-column: 1/-1;
}
.full-width {
  width: 100%;
}
.field-help {
  width: 100%;
  margin-top: 6px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--ac-color-text-muted);
}
.quiet-hours {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
.detect-button {
  margin-left: 12px;
}
.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.el-slider {
  width: 100%;
}
@media (max-width: 700px) {
  .page-header {
    align-items: stretch;
    flex-direction: column;
  }
  .form-grid,
  .dialog-grid {
    grid-template-columns: 1fr;
  }
  .wide-item {
    grid-column: auto;
  }
  .quiet-hours {
    align-items: stretch;
    flex-direction: column;
  }
  .detect-button {
    margin: 8px 0 0;
  }
  .suggestion-actions {
    flex-wrap: wrap;
  }
}
</style>
