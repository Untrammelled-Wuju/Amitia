<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <main class="task-workspace" aria-label="主动任务">
    <header class="workspace-toolbar">
      <div class="task-kind-switch" role="group" aria-label="主动任务类型">
        <el-radio-group v-model="taskKind" @change="switchTaskKind">
          <el-radio-button value="user_reminder">定时任务</el-radio-button>
          <el-radio-button value="proactive_task">主动消息</el-radio-button>
        </el-radio-group>
      </div>
      <div class="workspace-summary" aria-label="任务概览">
        <span class="summary-item"
          >全部 <strong>{{ statusLoaded ? totalCount : "—" }}</strong></span
        >
        <span class="summary-item"
          >启用 <strong>{{ statusLoaded ? enabledCount : "—" }}</strong></span
        >
        <span class="summary-item"
          >已到期 <strong>{{ statusLoaded ? dueCount : "—" }}</strong></span
        >
        <span
          class="scheduler-state"
          :class="{
            'is-running': statusLoaded && schedulerRunning,
            'is-stopped': statusLoaded && !schedulerRunning,
          }"
        >
          <i aria-hidden="true"></i>
          {{
            statusLoaded ? (schedulerRunning ? "调度正常" : "调度已停止") : "状态未确认"
          }}
        </span>
        <el-button
          text
          circle
          :icon="Refresh"
          :loading="refreshing"
          aria-label="刷新任务"
          title="刷新任务"
          @click="refreshWorkspace"
        />
      </div>
    </header>
    <section class="workspace-content">
      <el-tabs v-model="activeView" class="workspace-tabs">
        <el-tab-pane label="任务" name="tasks">
          <div class="list-toolbar">
            <el-input
              v-model="searchText"
              :prefix-icon="Search"
              clearable
              placeholder="搜索任务、内容或角色"
              aria-label="搜索任务"
              class="task-search"
            />
            <el-select
              v-model="filterCharacterId"
              placeholder="全部角色"
              clearable
              :loading="loadingChars"
              aria-label="筛选角色"
              class="role-filter"
              ><el-option
                v-for="role in characterOptions"
                :key="role.id"
                :label="role.name"
                :value="role.id"
            /></el-select>
            <el-select v-model="activeGroup" aria-label="筛选任务状态" class="state-filter"
              ><el-option label="全部状态" value="" /><el-option
                label="即将执行"
                value="upcoming" /><el-option label="已到期" value="overdue" /><el-option
                label="已结束"
                value="completed" /><el-option label="暂停 / 停用" value="disabled"
            /></el-select>
          </div>
          <el-alert
            v-if="listError && reminders.length"
            :title="listError"
            type="error"
            :closable="false"
            show-icon
            class="load-error"
            ><el-button link type="primary" @click="fetchReminders">重新加载</el-button></el-alert
          >
          <el-table :data="filteredReminders" v-loading="loading" class="task-table">
            <el-table-column label="任务" min-width="240"
              ><template #default="{ row }"
                ><div class="task-heading">
                  <button class="task-title" @click="showDetail(row)">{{ row.title }}</button>
                  <span class="task-role">{{ roleLabel(row) }}</span>
                </div>
                <p class="task-description">
                  {{ row.content || "无附加消息内容" }}
                </p></template
              ></el-table-column
            >
            <el-table-column label="下次执行" min-width="185"
              ><template #default="{ row }"
                ><span>{{
                  row.enabled && !row.paused && row.state === "enabled"
                    ? row.localRemindAt || row.remindAt || "待调度"
                    : "—"
                }}</span>
                <p class="cell-secondary">
                  {{ repeatLabel(row.repeatRule) }}
                </p></template
              ></el-table-column
            >
            <el-table-column label="状态" width="105"
              ><template #default="{ row }"
                ><el-tag :type="taskTone(row)" effect="light" round>{{
                  taskLabel(row)
                }}</el-tag></template
              ></el-table-column
            >
            <el-table-column label="操作" width="132" align="right"
              ><template #default="{ row }"
                ><div class="row-actions">
                  <el-button
                    link
                    type="primary"
                    :disabled="openingEditor || pendingActions.has(row.id)"
                    @click="openEdit(row)"
                    >编辑</el-button
                  ><el-dropdown
                    trigger="click"
                    @command="(command: string) => taskCommand(command, row)"
                    ><el-button
                      text
                      :icon="MoreFilled"
                      :disabled="pendingActions.has(row.id)"
                      :aria-label="'更多操作：' + row.title"
                    /><template #dropdown
                      ><el-dropdown-menu
                        ><el-dropdown-item command="preview">预览与检查</el-dropdown-item
                        ><el-dropdown-item
                          command="trigger"
                          :disabled="!row.enabled || row.paused || row.state !== 'enabled'"
                          >立即执行</el-dropdown-item
                        ><el-dropdown-item
                          command="pause"
                          :disabled="!row.enabled || row.state === 'expired'"
                          >{{ row.paused ? "恢复任务" : "暂停任务" }}</el-dropdown-item
                        ><el-dropdown-item command="delete" divided
                          >取消任务</el-dropdown-item
                        ></el-dropdown-menu
                      ></template
                    ></el-dropdown
                  >
                </div></template
              ></el-table-column
            >
            <template #empty
              ><div v-if="listError" class="task-empty">
                <h3>暂时无法读取任务</h3>
                <p>请检查连接后重新加载，已有任务不会被修改。</p>
                <el-button @click="fetchReminders">重新加载</el-button>
              </div>
              <div v-else class="task-empty">
                <span class="empty-icon"
                  ><el-icon><Calendar /></el-icon
                ></span>
                <h3>
                  {{
                    searchText || filterCharacterId || activeGroup
                      ? "没有匹配的任务"
                      : "还没有定时任务"
                  }}
                </h3>
                <p>
                  {{
                    searchText || filterCharacterId || activeGroup
                      ? "尝试调整搜索内容或筛选条件。"
                      : "从左侧新建任务，安排角色在指定时间发送消息。"
                  }}
                </p>
                <el-button
                  v-if="searchText || filterCharacterId || activeGroup"
                  @click="
                    searchText = '';
                    filterCharacterId = '';
                    activeGroup = '';
                  "
                  >清除筛选</el-button>
                <el-button v-else type="primary" @click="openCreate">新建任务</el-button>
              </div></template
            >
          </el-table>
        </el-tab-pane>
        <el-tab-pane label="待执行" name="upcoming"
          ><ProspectivePanel
            ref="prospectivePanel"
            :kind="taskKind"
            @clear-backpressure="clearBackpressure"
        /></el-tab-pane>
        <el-tab-pane label="执行历史" name="history"
          ><TriggerHistoryPanel ref="historyPanel" :kind="taskKind">
            <template #toolbar>
              <span class="history-retention">历史保留</span>
              <el-select
                v-model="cleanupDays"
                size="small"
                aria-label="历史保留时间"
                class="cleanup-select"
                @change="setCleanupConfig"
              >
                <el-option label="不自动清理" value="0" />
                <el-option
                  v-for="days in [1, 3, 7, 14, 30]"
                  :key="days"
                  :label="days + ' 天后'"
                  :value="String(days)"
                />
              </el-select>
            </template>
          </TriggerHistoryPanel></el-tab-pane
        >
      </el-tabs>
    </section>
    <el-drawer
      v-model="detailVisible"
      title="任务详情"
      size="min(520px, 100vw)"
      class="task-detail-drawer"
    >
      <template v-if="selectedTask"
        ><div class="detail-heading">
          <el-tag :type="taskTone(selectedTask)" round>{{ taskLabel(selectedTask) }}</el-tag>
          <h2>{{ selectedTask.title }}</h2>
          <p>{{ selectedTask.content || "到期后由角色发送消息" }}</p>
        </div>
        <el-descriptions :column="1" class="task-details"
          ><el-descriptions-item label="所属角色">{{
            roleLabel(selectedTask)
          }}</el-descriptions-item
          ><el-descriptions-item label="关联对话">{{
            selectedTask.conversationTitle || selectedTask.conversationId || "任务专属对话"
          }}</el-descriptions-item
          ><el-descriptions-item label="执行时间">{{
            selectedTask.localRemindAt || selectedTask.remindAt || "—"
          }}</el-descriptions-item
          ><el-descriptions-item label="时区">{{ selectedTask.timezone }}</el-descriptions-item
          ><el-descriptions-item label="重复规则"
            >{{ repeatLabel(selectedTask.repeatRule)
            }}<span v-if="selectedTask.recurrence?.intervalSeconds">
              · {{ selectedTask.recurrence.intervalSeconds }} 秒</span
            ><span v-if="selectedTask.recurrence?.weekdays?.length">
              · 星期 {{ selectedTask.recurrence.weekdays.join("、") }}</span
            ></el-descriptions-item
          ><el-descriptions-item label="结束条件"
            >{{ selectedTask.endAt || "不限结束时间" }} ·
            {{
              selectedTask.maxOccurrences
                ? "最多 " + selectedTask.maxOccurrences + " 次"
                : "不限次数"
            }}</el-descriptions-item
          ><el-descriptions-item label="发送目标"
            >所属角色对话
            <div v-for="(target, index) in selectedTask.channelTargets" :key="index">
              {{ target.channelId }} · {{ target.peerId }}
            </div></el-descriptions-item
          ><el-descriptions-item label="系统通知">{{
            selectedTask.systemNotification ? "已开启" : "已关闭"
          }}</el-descriptions-item
          ><el-descriptions-item label="运行位置">{{
            modeLabel(selectedTask.executionMode)
          }}</el-descriptions-item
          ><el-descriptions-item label="任务来源">{{
            sourceLabel(selectedTask.source)
          }}</el-descriptions-item
          ><el-descriptions-item label="上次执行">{{
            selectedTask.lastTriggeredAt || "尚未执行"
          }}</el-descriptions-item
          ><el-descriptions-item label="执行归属">{{
            selectedTask.ownerId
          }}</el-descriptions-item></el-descriptions
        >
        <div class="detail-controls">
          <span>启用任务</span
          ><el-switch
            :model-value="selectedTask.enabled"
            :disabled="pendingActions.has(selectedTask.id)"
            aria-label="启用任务"
            @change="(value: boolean) => toggleReminder(selectedTask!, value)"
          />
        </div>
        <div class="drawer-actions">
          <el-button @click="openEdit(selectedTask)" :loading="openingEditor">编辑任务</el-button
          ><el-button
            :icon="ArrowRight"
            @click="
              detailVisible = false;
              activeView = 'history';
            "
            >查看执行历史</el-button
          >
        </div></template
      >
      <el-empty v-else description="任务已变更，请刷新列表" />
    </el-drawer>
    <el-dialog
      v-model="dialogVisible"
      :title="isEditing ? (isProactiveTask ? '编辑主动消息规则' : '编辑任务') : (isProactiveTask ? '新建主动消息规则' : '新建主动任务')"
      width="min(720px, calc(100vw - 40px))"
      destroy-on-close
      :close-on-click-modal="false"
      class="task-editor"
    >
      <el-form :model="form" label-position="top" @submit.prevent="saveReminder">
        <section class="editor-section">
          <h3><span>01</span> 任务内容</h3>
          <el-form-item label="任务名称" required
            ><el-input
              v-model="form.title"
              placeholder="例如：每晚睡前的一句晚安"
              maxlength="50" /></el-form-item
          ><el-form-item label="消息内容"
            ><el-input
              v-model="form.content"
              type="textarea"
              :rows="3"
              placeholder="填写希望角色发送的内容，也可以留空"
              maxlength="300"
              show-word-limit
          /></el-form-item>
        </section>
        <section class="editor-section">
          <h3><span>02</span> 执行时间</h3>
          <div class="form-grid">
            <el-form-item label="首次执行时间" required
              ><el-date-picker
                v-model="form.remindAt"
                type="datetime"
                format="YYYY-MM-DD HH:mm:ss"
                value-format="YYYY-MM-DD HH:mm:ss"
                placeholder="选择日期和时间" /></el-form-item
            ><el-form-item label="重复规则"
              ><el-select v-model="form.repeatRule"
                ><el-option
                  v-for="(label, key) in repeatLabels"
                  :key="key"
                  :label="label"
                  :value="key" /></el-select
            ></el-form-item>
          </div>
          <el-form-item label="执行时区" required
            ><el-input v-model="form.timezone" placeholder="例如：Asia/Shanghai" />
            <p class="field-hint">按此时区执行，更换设备时区不会改变已保存的任务。</p></el-form-item
          ><template v-if="isProactiveTask">
            <div class="form-grid">
              <el-form-item label="规则类型"
                ><el-select v-model="form.ruleType"
                  ><el-option label="自定义" value="custom" /><el-option
                    label="早安问候"
                    value="daily_greeting" /><el-option
                    label="工作间歇"
                    value="work_break" /><el-option
                    label="睡前提醒"
                    value="sleep_reminder" /></el-select
                ></el-form-item
              ><el-form-item label="Cron 表达式" required
                ><el-input v-model="form.cron" placeholder="例如：0 9 * * *" />
                <p class="field-hint">主动消息按 Cron 计划执行。</p></el-form-item
              >
            </div>
            <div class="form-grid">
              <el-form-item label="主动频率"
                ><el-slider
                  v-model="form.policy.activeLevel"
                  :min="0"
                  :max="100"
                  show-input /><p class="field-hint">数值越高，角色发起主动消息的概率越高。</p></el-form-item
              ><el-form-item label="规则每日上限"
                ><el-input-number
                  v-model="form.policy.ruleMaxPerDay"
                  :min="1"
                  :max="100"
                /></el-form-item
              >
            </div>
            <div class="form-grid">
              <el-form-item label="安静时段开始"
                ><el-time-picker
                  v-model="form.policy.quietStart"
                  format="HH:mm"
                  value-format="HH:mm"
                  placeholder="23:00" /></el-form-item
              ><el-form-item label="安静时段结束"
                ><el-time-picker
                  v-model="form.policy.quietEnd"
                  format="HH:mm"
                  value-format="HH:mm"
                  placeholder="07:00" /></el-form-item
              >
            </div>
            <div class="form-grid">
              <el-form-item label="最小间隔（分钟）"
                ><el-input-number
                  v-model="form.policy.minInterval"
                  :min="0"
                  :max="525600"
                /></el-form-item
              ><el-form-item label="每日总上限"
                ><el-input-number
                  v-model="form.policy.maxPerDay"
                  :min="1"
                  :max="100"
                /></el-form-item
              >
            </div>
            <div class="form-grid">
              <el-form-item label="每日模型调用上限"
                ><el-input-number
                  v-model="form.policy.maxDailyCalls"
                  :min="1"
                  :max="100"
                /></el-form-item
              ><el-form-item label="随机延迟（分钟）"
                ><el-input-number
                  v-model="form.policy.randomMinutes"
                  :min="0"
                  :max="120"
                /></el-form-item
              >
            </div>
            <div class="toggle-setting">
              <div>
                <strong>未回复时自动降频</strong>
                <p>连续未回复后逐步扩大间隔，用户回复后自动恢复</p>
              </div>
              <el-switch
                v-model="form.policy.unrepliedSlowdownEnabled"
                aria-label="未回复时自动降频"
              />
            </div>
          </template>
          ><el-form-item v-if="form.repeatRule === 'weekdays'" label="指定星期"
            ><el-checkbox-group v-model="form.weekdays"
              ><el-checkbox-button
                v-for="(label, i) in ['一', '二', '三', '四', '五', '六', '日']"
                :key="i"
                :value="i + 1"
                >周{{ label }}</el-checkbox-button
              ></el-checkbox-group
            ></el-form-item
          ><el-form-item v-if="form.repeatRule === 'interval'" label="执行间隔（秒）"
            ><el-input-number v-model="form.intervalSeconds" :min="60" :step="60" /></el-form-item
          ><el-collapse v-model="advancedSections" class="advanced-options"
            ><el-collapse-item name="limits" title="结束条件（可选）"
              ><div class="form-grid">
                <el-form-item label="结束时间"
                  ><el-date-picker
                    v-model="form.endAt"
                    type="datetime"
                    value-format="YYYY-MM-DD HH:mm:ss"
                    placeholder="不限制" /></el-form-item
                ><el-form-item label="最多执行次数"
                  ><el-input-number v-model="form.maxOccurrences" :min="0" />
                  <p class="field-hint">0 表示不限制次数</p></el-form-item
                >
              </div></el-collapse-item
            ></el-collapse
          >
        </section>
        <section class="editor-section">
          <h3><span>03</span> 发送目标</h3>
          <div class="form-grid">
            <el-form-item label="所属角色" required
              ><el-select
                v-model="form.characterId"
                placeholder="选择角色"
                filterable
                :loading="loadingChars"
                @change="
                  () => {
                    form.conversationId = '';
                    fetchConversationOptions();
                  }
                "
                ><el-option
                  v-for="role in characterOptions"
                  :key="role.id"
                  :label="role.name"
                  :value="role.id" /></el-select></el-form-item
            ><el-form-item label="关联对话"
              ><el-select
                v-model="form.conversationId"
                placeholder="自动创建任务专属对话"
                clearable
                filterable
                :loading="loadingConvs"
                ><el-option
                  v-for="conversation in conversationOptions"
                  :key="conversation.id"
                  :label="conversation.title"
                  :value="conversation.id" /></el-select
            ></el-form-item>
          </div>
          <div class="channel-heading">
            <span>附加发送渠道</span
            ><el-button
              :icon="Plus"
              text
              type="primary"
              :disabled="!channelOptions.length"
              @click="form.channelTargets.push({ channelId: '', peerId: '' })"
              >添加渠道</el-button
            >
          </div>
          <p v-if="!channelOptions.length" class="field-hint">
            暂无已接入渠道，任务仍会发送到角色对话。
          </p>
          <div v-for="(target, i) in form.channelTargets" :key="i" class="channel-row">
            <el-select
              v-model="target.channelId"
              placeholder="选择渠道"
              :aria-label="'附加渠道 ' + (i + 1)"
              ><el-option
                v-for="id in channelOptions"
                :key="id"
                :label="id"
                :value="id" /></el-select
            ><el-input
              v-model="target.peerId"
              placeholder="接收方标识（必填）"
              :aria-label="'接收方标识 ' + (i + 1)"
            /><el-button text type="danger" @click="form.channelTargets.splice(i, 1)"
              >移除</el-button
            >
          </div>
          <div class="toggle-setting">
            <div>
              <strong>系统通知</strong>
              <p>受设备通知权限与偏好设置控制</p>
            </div>
            <el-switch v-model="form.systemNotification" aria-label="系统通知" />
          </div>
          <div class="toggle-setting">
            <div>
              <strong>保存后启用</strong>
              <p>关闭后保留任务，暂不自动执行</p>
            </div>
            <el-switch v-model="form.enabled" aria-label="保存后启用" />
          </div>
        </section>
      </el-form>
      <template #footer
        ><el-button :disabled="saving" @click="dialogVisible = false">取消</el-button
        ><el-button type="primary" :loading="saving" @click="saveReminder">{{
          isEditing ? "保存修改" : "创建任务"
        }}</el-button></template
      >
    </el-dialog>
    <el-dialog v-model="testVisible" title="预览与检查" width="min(520px, calc(100vw - 40px))"
      ><template v-if="testResult"
        ><el-alert
          title="仅检查配置与权限，不会生成或发送消息"
          type="info"
          :closable="false"
          show-icon
        />
        <h3>{{ testResult.title }}</h3>
        <div class="msg-preview">{{ testResult.messageContent || "未设置自定义内容" }}</div>
        <p>
          <el-tag :type="testResult.authorizationCheck?.allowed ? 'success' : 'danger'">{{
            testResult.authorizationCheck?.allowed ? "检查通过" : "检查未通过"
          }}</el-tag>
        </p>
        <p>{{ testResult.authorizationCheck?.reason }}</p></template
      ></el-dialog
    >
  </main>
</template>
<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  Plus,
  Refresh,
  Search,
  MoreFilled,
  Calendar,
  ArrowRight,
} from "@element-plus/icons-vue";
import { request } from "../../composables/request";
import { useDeviceOwnedConversation } from "../../composables/useDeviceOwnedConversation";
import type { ReminderGroup } from "../../types";
import { randomUUID } from "../../runtime/secure-random";
import ProspectivePanel from "./components/ProspectivePanel.vue";
import TriggerHistoryPanel from "./components/TriggerHistoryPanel.vue";
import { requireSavedTask, currentOwnerConversations, validateChannelTargets } from "./contract";

const route = useRoute();
const router = useRouter();

const props = withDefaults(defineProps<{ initialCharacterId?: string }>(), {
  initialCharacterId: "",
});

interface Reminder {
  id: string;
  generation: number;
  ownerId: string;
  executionMode: string;
  source: string;
  state: string;
  localRemindAt: string;
  timezone: string;
  localAnchorAt: string;
  endAt: string;
  maxOccurrences: number;
  systemNotification: boolean;
  paused: boolean;
  recurrence: { type: string; weekdays?: number[]; intervalSeconds?: number; cron?: string };
  channelTargets: Array<{ channelId: string; peerId: string }>;
  title: string;
  content: string;
  channel: string;
  conversationId: string | null;
  characterId: string | null;
  characterName?: string;
  conversationTitle?: string;
  remindAt: string;
  repeatRule: string;
  enabled: boolean;
  lastTriggeredAt: string | null;
  createdAt: string;
  updatedAt: string;
  policy?: {
    randomMinutes: number;
    ruleQuietStart: string;
    ruleQuietEnd: string;
    enabled: boolean;
    activeLevel: number;
    quietStart: string;
    quietEnd: string;
    minInterval: number;
    maxPerDay: number;
    maxDailyCalls: number;
    unrepliedSlowdownEnabled: boolean;
    unrepliedSlowdownAfter: number;
    unrepliedCooldownMultiplier: number;
    unrepliedRecoveryOnReply: boolean;
    ruleMaxPerDay: number;
    ruleType: string;
  };
}
type TaskKind = "user_reminder" | "proactive_task";
const owned = useDeviceOwnedConversation();
const channelOptions = ref<string[]>([]);
const prospectivePanel = ref<InstanceType<typeof ProspectivePanel>>();
const historyPanel = ref<InstanceType<typeof TriggerHistoryPanel>>();
const editorCore = ref("");
function saved(value: any): Reminder {
  return requireSavedTask(value);
}
const reminders = ref<Reminder[]>([]);
const taskKind = ref<TaskKind>("user_reminder");
const isProactiveTask = computed(() => taskKind.value === "proactive_task");
const loading = ref(false);
const activeView = ref("tasks");
const searchText = ref("");
const listError = ref("");
const statusLoaded = ref(false);
const refreshing = ref(false);
const openingEditor = ref(false);
const detailVisible = ref(false);
const selectedId = ref("");
const advancedSections = ref<string[]>([]);
const selectedTask = computed(() => reminders.value.find((item) => item.id === selectedId.value));
const enabledCount = ref(0);
const pendingActions = ref(new Set<string>());
function taskLabel(row: Reminder): string {
  if (row.paused) return "已暂停";
  return (
    (
      {
        enabled: "待执行",
        disabled: "已停用",
        expired: "已结束",
        uninstalled: "已取消",
        cancelled: "已取消",
      } as Record<string, string>
    )[row.state] || row.state
  );
}
function taskTone(row: Reminder): "success" | "info" | "warning" {
  return row.paused ? "warning" : row.enabled && row.state === "enabled" ? "success" : "info";
}
function showDetail(row: Reminder) {
  selectedId.value = row.id;
  detailVisible.value = true;
}
function roleLabel(row: Reminder) {
  return (
    row.characterName ||
    characterOptions.value.find((role) => role.id === row.characterId)?.name ||
    "未命名角色"
  );
}
function modeLabel(value: string) {
  return (
    (
      { local: "本地", cloud: "云端", delegated: "设备协同", device: "设备" } as Record<
        string,
        string
      >
    )[value] || value
  );
}
function sourceLabel(value: string) {
  return (
    ({ manual: "手动创建", chat: "聊天创建", plugin: "插件创建" } as Record<string, string>)[
      value
    ] || value
  );
}
async function taskCommand(command: string, row: Reminder) {
  if (pendingActions.value.has(row.id)) return;
  if (command === "delete") {
    try {
      await ElMessageBox.confirm("取消后不再执行，已产生的执行历史会保留。", "取消这项任务？", {
        confirmButtonText: "取消任务",
        cancelButtonText: "保留任务",
        type: "warning",
      });
    } catch (error) {
  console.error("[original-error]", error);
return;
    }
  }
  pendingActions.value.add(row.id);
  try {
    if (command === "preview") await testReminder(row);
    if (command === "trigger") await triggerNow(row);
    if (command === "pause") await pauseReminder(row);
    if (command === "delete") {
      await deleteReminder(row);
      detailVisible.value = false;
    }
  } finally {
    pendingActions.value.delete(row.id);
  }
}
async function refreshWorkspace() {
  refreshing.value = true;
  try {
    await fetchAll();
  } finally {
    refreshing.value = false;
  }
}
const saving = ref(false);
const dialogVisible = ref(false);
const isEditing = ref(false);
const editingId = ref<string | null>(null);
const testVisible = ref(false);
const testResult = ref<any>(null);
const schedulerRunning = ref(false);
const totalCount = ref(0);
const dueCount = ref(0);
const conversationOptions = ref<{ id: string; title: string }[]>([]);
const characterOptions = ref<{ id: string; name: string }[]>([]);
const loadingConvs = ref(false);
const loadingChars = ref(false);
const cleanupDays = ref("0");
const activeGroup = ref<ReminderGroup | "">("");
const filterCharacterId = ref(props.initialCharacterId);

const repeatLabels: Record<string, string> = {
  none: "不重复",
  daily: "每天",
  weekly: "每周",
  monthly: "每月",
  weekdays: "指定星期",
  interval: "自定义间隔",
  cron: "Cron 计划",
};
function repeatLabel(r: string): string {
  return repeatLabels[r] || r;
}

const filteredReminders = computed(() => {
  const query = searchText.value.trim().toLocaleLowerCase();
  let list = reminders.value.filter(
    (row) =>
      !query ||
      [row.title, row.content, roleLabel(row)].some((value) =>
        value?.toLocaleLowerCase().includes(query),
      ),
  );
  if (filterCharacterId.value) {
    list = list.filter((r) => r.characterId === filterCharacterId.value);
  }
  const now = new Date();
  switch (activeGroup.value) {
    case "overdue":
      return list.filter(
        (r) =>
          r.enabled &&
          !r.paused &&
          r.state === "enabled" &&
          r.remindAt &&
          new Date(r.remindAt) < now,
      );
    case "upcoming":
      return list.filter(
        (r) =>
          r.enabled &&
          !r.paused &&
          r.state === "enabled" &&
          r.remindAt &&
          new Date(r.remindAt) >= now,
      );
    case "completed":
      return list.filter((r) => r.state === "expired");
    case "disabled":
      return list.filter((r) => r.state === "disabled" || r.paused);
    default:
      return list;
  }
});

const form = ref({
  title: "",
  content: "",
  channel: "web",
  conversationId: "",
  characterId: "",
  remindAt: null as any,
  repeatRule: "none",
  enabled: true,
  generation: 0,
  requestId: randomUUID(),
  timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
  weekdays: [] as number[],
  intervalSeconds: 3600,
  endAt: "",
  maxOccurrences: 0,
  systemNotification: true,
  channelTargets: [] as Array<{ channelId: string; peerId: string }>,
  cron: "0 9 * * *",
  ruleType: "custom",
  policy: defaultProactivePolicy(),
});
function resetForm() {
  form.value = {
    title: "",
    content: "",
    channel: "web",
    conversationId: "",
    characterId: "",
    remindAt: null as any,
    repeatRule: "none",
    enabled: true,
    generation: 0,
    requestId: randomUUID(),
    timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    weekdays: [] as number[],
    intervalSeconds: 3600,
    endAt: "",
    maxOccurrences: 0,
    systemNotification: true,
    channelTargets: [] as Array<{ channelId: string; peerId: string }>,
    cron: "0 9 * * *",
    ruleType: "custom",
    policy: defaultProactivePolicy(),
  };
}

function defaultProactivePolicy() {
  return {
    randomMinutes: 0,
    ruleQuietStart: "",
    ruleQuietEnd: "",
    enabled: true,
    activeLevel: 40,
    quietStart: "23:00",
    quietEnd: "07:00",
    minInterval: 60,
    maxPerDay: 6,
    maxDailyCalls: 10,
    unrepliedSlowdownEnabled: true,
    unrepliedSlowdownAfter: 2,
    unrepliedCooldownMultiplier: 2,
    unrepliedRecoveryOnReply: true,
    ruleMaxPerDay: 1,
    ruleType: "custom",
  };
}

async function switchTaskKind(value: string | number | boolean) {
  taskKind.value = value === "proactive_task" ? "proactive_task" : "user_reminder";
  activeView.value = "tasks";
  detailVisible.value = false;
  selectedId.value = "";
  resetForm();
  await fetchAll();
}

async function fetchAll() {
  const refreshes: Promise<unknown>[] = [fetchReminders(), fetchStatus()];
  if (typeof prospectivePanel.value?.refresh === "function") {
    refreshes.push(prospectivePanel.value.refresh());
  }
  if (typeof historyPanel.value?.refresh === "function") {
    refreshes.push(historyPanel.value.refresh());
  }
  await Promise.all(refreshes);
}

async function fetchReminders() {
  loading.value = reminders.value.length === 0;
  listError.value = "";
  try {
    const res: any = await request.get("/api/reminders", { params: { kind: taskKind.value } });
    if (!Array.isArray(res)) throw new Error("服务未返回任务列表");
    const raw = res;
    reminders.value = raw.map(saved);
  } catch (error: any) {
    listError.value = error?.message || "任务列表加载失败";
  } finally {
    loading.value = false;
  }
}

async function fetchStatus() {
  try {
    const res: any = await request.get("/api/reminders/status", {
      params: { kind: taskKind.value },
    });
    if (!res || typeof res.schedulerRunning !== "boolean") throw new Error("调度状态响应无效");
    const d = res;
    schedulerRunning.value = d?.schedulerRunning ?? false;
    totalCount.value = d?.total ?? 0;
    dueCount.value = d?.dueNow ?? 0;
    enabledCount.value = d?.enabled ?? 0;
    statusLoaded.value = true;
  } catch (error: any) {
    statusLoaded.value = false;
    schedulerRunning.value = false;
    ElMessage.error(error?.message || "无法读取调度状态");
  }
}

async function fetchConversationOptions() {
  loadingConvs.value = true;
  try {
    const role = form.value.characterId;
    conversationOptions.value = role
      ? currentOwnerConversations(await owned.conversations(role), owned.coreId.value)
      : [];
  } catch (error: any) {
    conversationOptions.value = [];
    ElMessage.error(error?.message || "会话加载失败");
  } finally {
    loadingConvs.value = false;
  }
}

async function fetchCharacterOptions() {
  loadingChars.value = true;
  try {
    await owned.refresh();
    characterOptions.value = owned.roles.value;
  } catch (error: any) {
    characterOptions.value = [];
    ElMessage.error(error?.message || "角色加载失败");
  } finally {
    loadingChars.value = false;
  }
}

async function loadEditorContext() {
  await fetchCharacterOptions();
  editorCore.value = owned.coreId.value;
  const channels: any = await request.get("/api/reminders/channels");
  if (!Array.isArray(channels) || channels.some((id: any) => typeof id !== "string"))
    throw new Error("渠道响应无效");
  channelOptions.value = channels.filter((id: string) => id !== "web");
  await fetchConversationOptions();
}
async function openCreate() {
  if (openingEditor.value) return;
  isEditing.value = false;
  editingId.value = null;
  resetForm();
  if (isProactiveTask.value) form.value.repeatRule = "cron";
  advancedSections.value = [];
  openingEditor.value = true;
  try {
    await loadEditorContext();
    dialogVisible.value = true;
  } catch (error: any) {
    ElMessage.error(error.message);
  } finally {
    openingEditor.value = false;
  }
}
async function openEdit(row: Reminder) {
  if (openingEditor.value) return;
  isEditing.value = true;
  editingId.value = row.id;
  form.value = {
    title: row.title,
    content: row.content,
    channel: row.channel,
    conversationId: row.conversationId || "",
    characterId: row.characterId || "",
    remindAt: row.localAnchorAt,
    repeatRule: row.repeatRule,
    enabled: !!row.enabled,
    generation: row.generation,
    requestId: randomUUID(),
    timezone: row.timezone,
    weekdays: row.recurrence.weekdays || [],
    intervalSeconds: row.recurrence.intervalSeconds || 3600,
    endAt: row.endAt || "",
    maxOccurrences: row.maxOccurrences || 0,
    systemNotification: row.systemNotification,
    channelTargets: (row.channelTargets || []).map((target) => ({ ...target })),
    cron: row.recurrence.cron || "0 9 * * *",
    ruleType: row.policy?.ruleType || "custom",
    policy: { ...defaultProactivePolicy(), ...(row.policy || {}) },
  };
  advancedSections.value = row.endAt || row.maxOccurrences ? ["limits"] : [];
  openingEditor.value = true;
  try {
    await loadEditorContext();
    dialogVisible.value = true;
  } catch (error: any) {
    ElMessage.error(error.message);
  } finally {
    openingEditor.value = false;
  }
}
async function fetchCleanupConfig() {
  try {
    const r = await request.get("/api/reminders/cleanup-config");
    cleanupDays.value = String((r as any).cleanupDays);
  } catch (error) {
  console.error("[original-error]", error);
cleanupDays.value = "0";
  }
}

async function setCleanupConfig(val: string) {
  try {
    const result: any = await request.put("/api/reminders/cleanup-config", { cleanupDays: val });
    if (String(result?.cleanupDays) !== val) throw new Error("服务未确认清理策略保存");
    ElMessage.success("已更新");
  } catch (err: any) {
    ElMessage.error(err?.message || "设置失败");
  }
}

async function saveReminder() {
  if (saving.value) return;
  if (!form.value.title.trim()) {
    ElMessage.warning("请输入标题");
    return;
  }
  if (!form.value.remindAt) {
    ElMessage.warning("请选择执行时间");
    return;
  }
  if (!form.value.characterId) {
    ElMessage.warning("请选择所属角色");
    return;
  }
  if (isProactiveTask.value && !form.value.cron.trim()) {
    ElMessage.warning("请输入 Cron 表达式");
    return;
  }
  saving.value = true;
  try {
    await owned.refresh();
    if (owned.coreId.value !== editorCore.value)
      throw new Error("执行核心已变化，请重新打开编辑页面");
    validateChannelTargets(form.value.channelTargets, channelOptions.value);
    const payload = {
      kind: taskKind.value,
      title: form.value.title.trim(),
      content: form.value.content,
      channel: form.value.channel,
      conversationId: form.value.conversationId || null,
      characterId: form.value.characterId || null,
      remindAt: form.value.remindAt,
      repeatRule: form.value.repeatRule,
      enabled: form.value.enabled,
      generation: form.value.generation,
      requestId: form.value.requestId,
      timezone: form.value.timezone,
      recurrence: {
        type: form.value.repeatRule,
        weekdays: form.value.weekdays,
        intervalSeconds: form.value.intervalSeconds,
        cron: form.value.cron.trim(),
      },
      endAt: form.value.endAt || "",
      maxOccurrences: form.value.maxOccurrences,
      systemNotification: form.value.systemNotification,
      channelTargets: form.value.channelTargets,
      policy: isProactiveTask.value
        ? {
            ...form.value.policy,
            ruleType: form.value.ruleType,
            ruleMaxPerDay: form.value.policy.ruleMaxPerDay,
          }
        : undefined,
    };
    if (isEditing.value && editingId.value) {
      saved(await request.put(`/api/reminders/${editingId.value}`, payload));
      ElMessage.success("任务已更新");
    } else {
      saved(await request.post("/api/reminders", payload));
      ElMessage.success("任务已创建");
    }
    dialogVisible.value = false;
    await fetchReminders();
    await fetchStatus();
  } catch (err: any) {
    ElMessage.error(err?.message || "操作失败");
  } finally {
    saving.value = false;
  }
}

async function toggleReminder(row: Reminder, nextValue?: boolean) {
  if (pendingActions.value.has(row.id)) return;
  pendingActions.value.add(row.id);
  const prev = row.enabled;
  const id = row.id;
  const updateLocal = (v: boolean) => {
    const idx = reminders.value.findIndex((r) => r.id === id);
    if (idx >= 0) reminders.value[idx].enabled = v;
  };
  updateLocal(typeof nextValue === "boolean" ? nextValue : !row.enabled);
  try {
    const res = saved(
      await request.post(`/api/reminders/${id}/toggle`, { generation: row.generation }),
    );
    Object.assign(row, res);
    const newVal = res?.enabled != null ? !!res.enabled : !prev;
    updateLocal(newVal);
    ElMessage.success(newVal ? "已启用" : "已停用");
    await fetchStatus();
  } catch (err: any) {
    updateLocal(prev);
    ElMessage.error(err?.message || "操作失败");
  } finally {
    pendingActions.value.delete(row.id);
  }
}

async function pauseReminder(row: Reminder) {
  try {
    saved(
      await request.post("/api/reminders/" + row.id + "/action", {
        generation: row.generation,
        action: row.paused ? "resume" : "pause",
      }),
    );
    await fetchAll();
  } catch (error: any) {
    ElMessage.error(error?.message || "操作失败");
  }
}

async function deleteReminder(row: Reminder) {
  try {
    saved(
      await request.delete(`/api/reminders/${row.id}`, { data: { generation: row.generation } }),
    );
    ElMessage.success("已删除");
    await fetchReminders();
    await fetchStatus();
  } catch (err: any) {
    ElMessage.error(err?.message || "删除失败");
  }
}

async function testReminder(row: Reminder) {
  try {
    const res: any = await request.post(`/api/reminders/${row.id}/test`);
    if (res?.tested !== true || res.id !== row.id) throw new Error("服务未返回有效预览结果");
    testResult.value = res;
    testVisible.value = true;
  } catch (err: any) {
    ElMessage.error(err?.message || "测试失败");
  }
}

async function triggerNow(row: Reminder) {
  try {
    const result: any = await request.post("/api/reminders/" + row.id + "/trigger", {
      generation: row.generation,
    });
    if (result?.accepted !== true || !result.triggerId) throw new Error("任务未受理");
    ElMessage.success("任务已受理，请在执行历史查看发送结果");
    await fetchReminders();
    await fetchStatus();
  } catch (err: any) {
    ElMessage.error(err?.message || "发送失败");
  }
}

async function clearBackpressure() {
  try {
    const result: any = await request.post(`/api/reminders/clear-backpressure?kind=${taskKind.value}`);
    if (!Number.isInteger(result?.clearedCount)) throw new Error("服务未确认熔断清理结果");
    ElMessage.success("背压标记已清除");
  } catch (err: any) {
    ElMessage.error(err?.message || "操作失败");
  }
}

let reminderRefreshTimer: ReturnType<typeof setInterval> | null = null;
let reminderRefreshBusy = false;

async function refreshReminderRuntime() {
  if (reminderRefreshBusy) return;
  reminderRefreshBusy = true;
  try {
    await Promise.all([fetchReminders(), fetchStatus()]);
  } finally {
    reminderRefreshBusy = false;
  }
}

onMounted(async () => {
  await refreshReminderRuntime();
  reminderRefreshTimer = setInterval(() => void refreshReminderRuntime(), 5000);
  await fetchCleanupConfig();
  await fetchCharacterOptions();
  await maybeOpenCreateFromQuery();
});

async function maybeOpenCreateFromQuery() {
  if (String(route.query.create || "") !== "1" || openingEditor.value) return;
  const query = { ...route.query };
  delete query.create;
  await router.replace({ path: route.path, query, hash: route.hash });
  await openCreate();
}

watch(() => route.query.create, () => { void maybeOpenCreateFromQuery(); });

watch(
  () => props.initialCharacterId,
  (characterId) => {
    if ((filterCharacterId.value || "") === (characterId || "")) return;
    filterCharacterId.value = characterId || "";
    void fetchReminders();
  },
);

onUnmounted(() => {
  if (reminderRefreshTimer) clearInterval(reminderRefreshTimer);
  reminderRefreshTimer = null;
});
</script>
<style scoped>
.task-workspace {
  width: 100%;
  max-width: none;
  margin: 0;
  padding: 0;
  color: var(--tp-text);
}
.workspace-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  min-height: 36px;
  margin-bottom: 12px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--tp-border-light);
}
.task-kind-switch {
  display: flex;
  align-items: center;
  gap: 0;
}
.row-actions,
.drawer-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}
.workspace-summary {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 16px;
  min-width: 0;
  color: var(--tp-text-secondary);
  font-size: 12px;
  white-space: nowrap;
}
.summary-item strong {
  margin-left: 3px;
  color: var(--tp-text);
  font-size: 13px;
  font-variant-numeric: tabular-nums;
}
.scheduler-state {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.scheduler-state i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--tp-text-muted);
}
.scheduler-state.is-running i {
  background: var(--tp-success);
}
.scheduler-state.is-stopped {
  color: var(--tp-danger);
}
.scheduler-state.is-stopped i {
  background: var(--tp-danger);
}
.workspace-summary :deep(.el-button) {
  width: 28px;
  height: 28px;
  min-height: 28px;
  flex-shrink: 0;
}
.empty-icon {
  display: grid;
  place-items: center;
  width: 48px;
  height: 48px;
  border-radius: 14px;
  background: var(--tp-primary-soft);
  color: var(--tp-primary);
  font-size: 23px;
  flex-shrink: 0;
}
.workspace-content {
  min-width: 0;
}
.workspace-tabs :deep(.el-tabs__header) {
  margin: 0;
  padding: 0;
}
.workspace-tabs :deep(.el-tabs__item) {
  height: 44px;
  font-size: 14px;
}
.workspace-tabs :deep(.el-tabs__nav-wrap::after) {
  height: 1px;
  background: var(--tp-border-light);
}
.workspace-tabs :deep(.el-tabs__content) {
  padding: 12px 0 0;
}
.list-toolbar {
  display: flex;
  gap: 12px;
  margin-bottom: 12px;
}
.task-search {
  width: auto;
  max-width: none;
  min-width: 220px;
  flex: 1;
  margin-right: auto;
}
.role-filter {
  width: 160px;
}
.state-filter {
  width: 150px;
}
.task-table {
  --el-table-header-bg-color: var(--tp-panel-soft);
  --el-table-border-color: var(--tp-border-light);
  --el-table-row-hover-bg-color: var(--tp-control);
  font-size: 14px;
}
.task-table :deep(th.el-table__cell) {
  height: 40px;
  color: var(--tp-text-secondary);
  font-weight: 500;
}
.task-table :deep(td.el-table__cell) {
  padding: 11px 0;
}
.task-heading {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.task-title {
  min-width: 0;
  border: 0;
  padding: 0;
  background: transparent;
  color: var(--tp-text);
  text-align: left;
  font: inherit;
  font-weight: 600;
  line-height: 1.5;
  cursor: pointer;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.task-title:hover {
  color: var(--tp-primary);
}
.task-role {
  flex-shrink: 0;
  padding: 1px 7px;
  border-radius: 999px;
  background: var(--tp-panel-soft);
  color: var(--tp-text-secondary);
  font-size: 11px;
  line-height: 1.6;
}
.task-description {
  margin: 3px 0 0;
  color: var(--tp-text-secondary);
  font-size: 13px;
  white-space: nowrap;
  text-overflow: ellipsis;
  overflow: hidden;
}
.cell-secondary {
  margin: 5px 0 0;
  color: var(--tp-text-secondary);
  font-size: 12px;
  line-height: 1.5;
}
.row-actions {
  justify-content: flex-end;
}
.row-actions :deep(.el-button) {
  min-height: 30px;
}
.task-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 36px 16px;
  line-height: 1.5;
}
.empty-icon {
  margin-bottom: 8px;
}
.task-empty h3 {
  color: var(--tp-text);
  font-size: 17px;
  margin: 6px 0;
}
.task-empty p {
  color: var(--tp-text-secondary);
  font-size: 14px;
  margin: 0 0 16px;
}
.history-retention {
  color: var(--tp-text-secondary);
  font-size: 12px;
}
.cleanup-select {
  width: 128px;
}
.load-error {
  margin-bottom: 12px;
}
.editor-section {
  margin-bottom: 24px;
}
.editor-section h3 {
  display: flex;
  gap: 10px;
  align-items: center;
  margin: 0 0 20px;
  font-size: 15px;
}
.editor-section h3 > span {
  color: var(--tp-text-secondary);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}
.editor-section + .editor-section {
  border-top: 1px solid var(--tp-border-light);
  padding-top: 24px;
}
.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0 20px;
}
.form-grid > * {
  min-width: 0;
}
.editor-section :deep(.el-select),
.editor-section :deep(.el-date-editor.el-input),
.editor-section :deep(.el-input-number) {
  width: 100%;
}
.field-hint {
  margin: 6px 0 0;
  color: var(--tp-text-secondary);
  font-size: 12px;
  line-height: 1.6;
}
.advanced-options {
  border: 0;
}
.advanced-options :deep(.el-collapse-item__header) {
  border: 0;
  color: var(--tp-text-secondary);
}
.advanced-options :deep(.el-collapse-item__wrap) {
  border: 0;
}
.channel-heading,
.toggle-setting,
.detail-controls {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.channel-heading {
  font-size: 14px;
}
.channel-row {
  display: grid;
  grid-template-columns: 1fr 1.3fr auto;
  gap: 8px;
  margin-top: 12px;
}
.toggle-setting {
  margin-top: 18px;
  background: var(--tp-panel-soft);
  padding: 14px 16px;
  border-radius: 10px;
}
.toggle-setting strong {
  font-size: 14px;
  font-weight: 500;
}
.toggle-setting p {
  margin: 4px 0 0;
  font-size: 12px;
  line-height: 1.5;
  color: var(--tp-text-secondary);
}
.detail-heading {
  margin-bottom: 28px;
}
.detail-heading h2 {
  margin: 16px 0 10px;
  line-height: 1.4;
  font-size: 22px;
  overflow-wrap: anywhere;
}
.detail-heading p {
  white-space: pre-wrap;
  line-height: 1.7;
  color: var(--tp-text-secondary);
}
.task-details {
  overflow-wrap: anywhere;
}
.task-details :deep(.el-descriptions__label) {
  display: inline-block;
  min-width: 82px;
  color: var(--tp-text-secondary);
}
.detail-controls {
  margin-top: 20px;
  padding: 16px 0;
  border-top: 1px solid var(--tp-border-light);
  font-size: 14px;
}
.drawer-actions {
  margin-top: 16px;
  flex-wrap: wrap;
}
.msg-preview {
  white-space: pre-wrap;
  background: var(--tp-panel-soft);
  padding: 16px;
  border-radius: 10px;
  line-height: 1.7;
}
.task-workspace :deep(button:focus-visible),
.task-title:focus-visible {
  outline: 2px solid var(--tp-primary);
  outline-offset: 3px;
  border-radius: 4px;
}
.task-workspace :deep(.el-button),
.task-title {
  transition:
    color 160ms ease,
    background-color 160ms ease;
}
@media (max-width: 1100px) {
  .workspace-summary {
    gap: 12px;
  }
}
@media (max-width: 760px) {
  .workspace-toolbar {
    align-items: flex-start;
    flex-direction: column;
    gap: 10px;
  }
  .workspace-summary {
    width: 100%;
    justify-content: flex-start;
    overflow-x: auto;
    padding-bottom: 2px;
  }
  .workspace-summary :deep(.el-button) {
    margin-left: auto;
  }
  .list-toolbar {
    flex-wrap: wrap;
  }
  .task-search {
    max-width: none;
    flex-basis: 100%;
  }
  .role-filter,
  .state-filter {
    flex: 1;
    min-width: 130px;
  }
  .workspace-tabs :deep(.el-tabs__content) {
    padding: 10px 0 0;
  }
  .form-grid {
    grid-template-columns: 1fr;
  }
  .channel-row {
    grid-template-columns: 1fr;
  }
}
@media (prefers-reduced-motion: reduce) {
  .task-workspace :deep(*),
  .task-title {
    transition: none !important;
    animation: none !important;
  }
}
</style>
