<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <main class="action-editor" v-loading="loading">
    <ExtensionPageHeader
      title="动作编辑器"
      :description="actionKey ? `动作Key: ${actionKey}` : '动作Key: —'"
      grandparent-title="创意工坊"
      grandparent-path="/creative-workshop"
      parent-title="桌宠"
      parent-path="/creative-workshop/pet"
    >
      <template #actions>
        <el-button :icon="Back" @click="goBack">返回</el-button>
        <el-button
          v-if="activeRevisionId && !hasSession"
          type="primary"
          :loading="creatingSession"
          @click="createSession"
          >创建编辑会话</el-button
        >
        <template v-if="hasSession">
          <el-button
            :icon="RefreshLeft"
            :disabled="!canUndo"
            @click="undo"
            >撤销</el-button
          >
          <el-button
            :icon="RefreshRight"
            :disabled="!canRedo"
            @click="redo"
            >重做</el-button
          >
          <el-button
            type="success"
            :icon="Check"
            :loading="committing"
            @click="commitSession"
            >提交会话</el-button
          >
          <el-button
            type="danger"
            plain
            :icon="Close"
            :loading="abandoning"
            @click="abandonSession"
            >放弃会话</el-button
          >
        </template>
      </template>
    </ExtensionPageHeader>

    <div v-if="!activeRevisionId && !loading" class="empty-state">
      <el-empty description="暂无活跃 Revision；处理完成后会自动生成可编辑 Revision" />
    </div>

    <div v-else-if="activeRevisionId" class="editor-content">
      <div class="editor-main">
        <div class="canvas-area">
          <div class="canvas-header">
            <span class="frame-indicator">
              第 {{ currentFrameIndex + 1 }} /
              {{ editSummary?.timeline.length || 0 }} 帧
            </span>
            <span v-if="hasSession" class="session-badge">
              会话 #{{ sessionVersion }}
            </span>
          </div>
          <div class="canvas-stage">
            <img
              v-if="currentFrame && currentImageUrl"
              :src="currentImageUrl"
              :alt="`帧 ${currentFrameIndex + 1}`"
              class="frame-image"
            />
            <div v-else class="canvas-placeholder">
              <el-icon :size="48"><Picture /></el-icon>
              <span>暂无帧图片</span>
            </div>
          </div>
          <div class="canvas-controls">
            <el-button
              :icon="isPlaying ? VideoPause : VideoPlay"
              :disabled="!editSummary?.timeline.length"
              @click="togglePlay"
            >
              {{ isPlaying ? "暂停" : "播放" }}
            </el-button>
          </div>
        </div>

        <div class="props-panel">
          <el-card shadow="never" class="prop-card">
            <template #header>
              <div class="card-header">
                <span>帧属性</span>
                <span v-if="currentFrame" class="card-subtitle">
                  帧 #{{ currentFrameIndex + 1 }}
                </span>
              </div>
            </template>
            <el-form
              label-width="80px"
              size="small"
              :disabled="!hasSession || !currentFrame"
            >
              <el-form-item label="时长(ms)">
                <el-input-number
                  v-model="frameDuration"
                  :min="16"
                  :step="16"
                  controls-position="right"
                  style="width: 100%"
                  @change="onFrameDurationChange"
                />
              </el-form-item>
              <el-form-item label="锚点X">
                <el-input-number
                  v-model="frameAnchorX"
                  :step="1"
                  controls-position="right"
                  style="width: 100%"
                  @change="onFrameAnchorChange"
                />
              </el-form-item>
              <el-form-item label="锚点Y">
                <el-input-number
                  v-model="frameAnchorY"
                  :step="1"
                  controls-position="right"
                  style="width: 100%"
                  @change="onFrameAnchorChange"
                />
              </el-form-item>
            </el-form>
          </el-card>

          <el-card shadow="never" class="prop-card">
            <template #header>
              <div class="card-header">
                <span>动作属性</span>
              </div>
            </template>
            <el-form label-width="80px" size="small" :disabled="!hasSession">
              <el-form-item label="FPS">
                <el-input-number
                  v-model="actionProps.fps"
                  :min="1"
                  :max="60"
                  controls-position="right"
                  style="width: 100%"
                />
              </el-form-item>
              <el-form-item label="循环类型">
                <el-select
                  v-model="actionProps.loopType"
                  style="width: 100%"
                >
                  <el-option label="循环 (loop)" value="loop" />
                  <el-option label="来回 (pingpong)" value="pingpong" />
                  <el-option label="单次 (once)" value="once" />
                </el-select>
              </el-form-item>
              <el-form-item label="返回动作">
                <el-input
                  v-model="actionProps.returnAction"
                  placeholder="动作Key"
                />
              </el-form-item>
              <el-form-item label="可打断">
                <el-switch v-model="actionProps.interruptible" />
              </el-form-item>
              <el-form-item label="优先级">
                <el-input-number
                  v-model="actionProps.priority"
                  :step="1"
                  controls-position="right"
                  style="width: 100%"
                />
              </el-form-item>
              <el-form-item label="冷却(ms)">
                <el-input-number
                  v-model="actionProps.cooldownMs"
                  :min="0"
                  :step="100"
                  controls-position="right"
                  style="width: 100%"
                />
              </el-form-item>
              <el-form-item>
                <el-button
                  type="primary"
                  size="small"
                  :loading="applying"
                  @click="updateActionProps"
                  >应用动作属性</el-button
                >
              </el-form-item>
            </el-form>
          </el-card>

          <el-card shadow="never" class="prop-card">
            <template #header>
              <div class="card-header">
                <span>质量信息</span>
              </div>
            </template>
            <div class="quality-info">
              <div class="quality-row">
                <span class="quality-label">质量评估</span>
                <el-tag :type="qualityTagType" size="small">
                  {{ editSummary?.qualityVerdict || "—" }}
                </el-tag>
              </div>
              <div class="quality-row">
                <span class="quality-label">总帧数</span>
                <span class="quality-value">
                  {{ editSummary?.frameCount || 0 }}
                </span>
              </div>
              <div class="quality-row">
                <span class="quality-label">问题帧数</span>
                <span
                  class="quality-value"
                  :class="{ 'has-issue-text': issueFrameCount > 0 }"
                >
                  {{ issueFrameCount }}
                </span>
              </div>
              <div class="quality-row">
                <span class="quality-label">总时长</span>
                <span class="quality-value">
                  {{ editSummary?.durationMs || 0 }} ms
                </span>
              </div>
              <div class="quality-row">
                <span class="quality-label">Revision</span>
                <span class="quality-value">
                  #{{ editSummary?.activeRevisionNum || "—" }}
                </span>
              </div>
            </div>
          </el-card>
        </div>
      </div>

      <div class="timeline">
        <div class="timeline-header">
          <span class="timeline-title">时间轴</span>
          <span class="timeline-hint">点击帧选中</span>
        </div>
        <div class="timeline-track">
          <div
            v-for="(frame, index) in editSummary?.timeline || []"
            :key="frame.frameId"
            class="timeline-item"
            :class="{
              active: index === currentFrameIndex,
              'has-issue': frame.hasQualityIssue,
            }"
            @click="selectFrame(index)"
          >
            <div class="timeline-thumb">
              <img
                v-if="thumbUrls[frame.frameId]"
                :src="thumbUrls[frame.frameId]"
                :alt="`帧 ${index + 1}`"
              />
              <div v-else class="thumb-placeholder">
                <el-icon><Picture /></el-icon>
              </div>
              <div
                v-if="frame.hasQualityIssue"
                class="issue-badge"
              >
                !
              </div>
            </div>
            <div class="timeline-meta">
              <span class="timeline-index">{{ index + 1 }}</span>
              <span class="timeline-duration">{{ frame.durationMs }}ms</span>
            </div>
            <div v-if="hasSession" class="timeline-actions">
              <el-button
                size="small"
                link
                @click.stop="deleteFrame(frame.frameId)"
                >删除</el-button
              >
              <el-button
                size="small"
                link
                @click.stop="duplicateFrame(frame.frameId)"
                >复制</el-button
              >
              <el-button
                size="small"
                link
                :disabled="index === 0"
                @click.stop="moveFrame(frame.frameId, 'left')"
                >前移</el-button
              >
              <el-button
                size="small"
                link
                :disabled="index === (editSummary?.timeline.length || 0) - 1"
                @click.stop="moveFrame(frame.frameId, 'right')"
                >后移</el-button
              >
            </div>
          </div>
        </div>
      </div>
    </div>
  </main>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  Back,
  Check,
  Close,
  RefreshLeft,
  RefreshRight,
  VideoPause,
  VideoPlay,
  Picture,
} from "@element-plus/icons-vue";
import * as editorApi from "@/composables/useActionEditor";
import type {
  ActionEditSummary,
  RevisionSummary,
  EditSession,
  FrameTimelineItem,
  RevisionDetail,
} from "@/composables/useActionEditor";
import { apiClient } from "@/composables/useApi";
import ExtensionPageHeader from "@/views/extensions/components/ExtensionPageHeader.vue";

const route = useRoute();
const router = useRouter();
const processingTaskId = computed(() => route.params.processingTaskId as string);
const actionKey = computed(() => route.params.actionKey as string);

const loading = ref(false);
const editSummary = ref<ActionEditSummary | null>(null);
const revisions = ref<RevisionSummary[]>([]);
const revisionDetail = ref<RevisionDetail | null>(null);
const session = ref<EditSession | null>(null);
const currentFrameIndex = ref(0);
const isPlaying = ref(false);
const playTimer = ref<number | null>(null);
const sessionVersion = ref(0);

const creatingSession = ref(false);
const committing = ref(false);
const abandoning = ref(false);
const applying = ref(false);

const frameImageUrls = reactive<Record<string, string>>({});
const thumbUrls = reactive<Record<string, string>>({});
const createdObjectUrls = new Set<string>();

const actionProps = reactive({
  fps: 12,
  loopType: "loop",
  returnAction: "",
  interruptible: true,
  priority: 0,
  cooldownMs: 0,
});

const frameDuration = ref(0);
const frameAnchorX = ref(0);
const frameAnchorY = ref(0);

const currentFrame = computed<FrameTimelineItem | null>(() => {
  if (!editSummary.value || !editSummary.value.timeline.length) return null;
  return editSummary.value.timeline[currentFrameIndex.value] || null;
});

const activeRevisionId = computed(
  () => editSummary.value?.activeRevisionId || "",
);

const hasSession = computed(
  () => !!session.value && session.value.status === "open",
);

const canUndo = computed(
  () => hasSession.value && (session.value?.cursor ?? 0) > 0,
);

const canRedo = computed(
  () =>
    hasSession.value &&
    (session.value?.cursor ?? 0) < (session.value?.lastOperationSeq ?? 0),
);

const currentImageUrl = computed(() => {
  if (!currentFrame.value) return "";
  return frameImageUrls[currentFrame.value.frameId] || "";
});

const issueFrameCount = computed(
  () =>
    editSummary.value?.timeline.filter((f) => f.hasQualityIssue).length || 0,
);

const qualityTagType = computed<"" | "success" | "warning" | "danger" | "info">(
  () => {
    const verdict = (editSummary.value?.qualityVerdict || "").toLowerCase();
    if (
      verdict.includes("fail") ||
      verdict.includes("bad") ||
      verdict.includes("error")
    )
      return "danger";
    if (verdict.includes("warn")) return "warning";
    if (
      verdict.includes("pass") ||
      verdict.includes("good") ||
      verdict === "normal"
    )
      return "success";
    return "info";
  },
);

async function loadFrameImage(revisionId: string, frameId: string) {
  if (!revisionId || !frameId) return;
  if (frameImageUrls[frameId]) return;
  try {
    const path = editorApi.getFrameImageUrl(revisionId, frameId);
    const res = await apiClient.get(path, { responseType: "blob" });
    const blob = res.data as Blob;
    if (blob && blob.size > 0) {
      const url = URL.createObjectURL(blob);
      createdObjectUrls.add(url);
      frameImageUrls[frameId] = url;
    }
  } catch {
    // ignore
  }
}

async function loadThumb(revisionId: string, frameId: string) {
  if (!revisionId || !frameId) return;
  if (thumbUrls[frameId]) return;
  try {
    const path = editorApi.getFrameThumbnailUrl(revisionId, frameId);
    const res = await apiClient.get(path, { responseType: "blob" });
    const blob = res.data as Blob;
    if (blob && blob.size > 0) {
      const url = URL.createObjectURL(blob);
      createdObjectUrls.add(url);
      thumbUrls[frameId] = url;
    }
  } catch {
    // ignore
  }
}

function clearImageCache() {
  for (const url of createdObjectUrls) {
    URL.revokeObjectURL(url);
  }
  createdObjectUrls.clear();
  Object.keys(frameImageUrls).forEach((k) => delete frameImageUrls[k]);
  Object.keys(thumbUrls).forEach((k) => delete thumbUrls[k]);
}

async function loadData() {
  if (!processingTaskId.value || !actionKey.value) {
    ElMessage.error("缺少处理任务 ID 或动作 Key");
    return;
  }
  loading.value = true;
  try {
    const [summary, revList] = await Promise.all([
      editorApi.getEditSummary(processingTaskId.value, actionKey.value),
      editorApi.listRevisions(processingTaskId.value, actionKey.value),
    ]);
    editSummary.value = summary;
    revisions.value = revList || [];
    if (currentFrameIndex.value >= summary.timeline.length) {
      currentFrameIndex.value = Math.max(0, summary.timeline.length - 1);
    }
    const anySummary = summary as any;
    if (anySummary.sessionId && !session.value) {
      try {
        const s = await editorApi.getSession(anySummary.sessionId);
        if (s && s.status === "open") {
          session.value = s;
          sessionVersion.value = s.sessionVersion;
        }
      } catch {
        // ignore
      }
    }
    if (summary.activeRevisionId) {
      await loadRevisionDetail(summary.activeRevisionId);
    }
  } catch (err: any) {
    ElMessage.error(err?.message || "加载数据失败");
  } finally {
    loading.value = false;
  }
}

async function loadRevisionDetail(revisionId: string) {
  if (!revisionId) return;
  try {
    const detail = await editorApi.getRevision(revisionId);
    revisionDetail.value = detail;
    const rev = detail.revision;
    if (rev) {
      actionProps.fps = rev.defaultFps ?? rev.fps ?? 12;
      actionProps.loopType = rev.loopType ?? "loop";
      actionProps.returnAction = rev.returnAction ?? "";
      actionProps.interruptible =
        rev.interruptible === undefined || rev.interruptible === null
          ? true
          : Boolean(rev.interruptible);
      actionProps.priority = rev.priorityOverride ?? 0;
      actionProps.cooldownMs = rev.cooldownMsOverride ?? 0;
    }
  } catch {
    // ignore
  }
}

async function refreshSummary() {
  if (!processingTaskId.value || !actionKey.value) return;
  try {
    const summary = await editorApi.getEditSummary(
      processingTaskId.value,
      actionKey.value,
    );
    editSummary.value = summary;
    if (currentFrameIndex.value >= summary.timeline.length) {
      currentFrameIndex.value = Math.max(0, summary.timeline.length - 1);
    }
  } catch (err: any) {
    ElMessage.error(err?.message || "刷新编辑摘要失败");
  }
}

async function refreshSession() {
  if (!session.value) return;
  try {
    const s = await editorApi.getSession(session.value.id);
    session.value = s;
    sessionVersion.value = s.sessionVersion;
  } catch {
    // ignore
  }
}

async function createSession() {
  if (!processingTaskId.value || !actionKey.value) return;
  creatingSession.value = true;
  try {
    const s = await editorApi.createSession(
      processingTaskId.value,
      actionKey.value,
      {
        baseRevisionId: activeRevisionId.value,
      },
    );
    session.value = s;
    sessionVersion.value = s.sessionVersion;
    ElMessage.success("编辑会话已创建");
  } catch (err: any) {
    ElMessage.error(err?.message || "创建编辑会话失败");
  } finally {
    creatingSession.value = false;
  }
}

async function applyOpRequest(type: string, payload: unknown) {
  if (!session.value) {
    throw new Error("请先创建编辑会话");
  }
  const res = await editorApi.applyOperation(session.value.id, {
    baseSessionVersion: sessionVersion.value,
    idempotencyKey: `op:${crypto.randomUUID()}`,
    operation: {
      type,
      schemaVersion: 1,
      payload,
    },
  });
  sessionVersion.value = res.sessionVersion;
  session.value.sessionVersion = res.sessionVersion;
  return res;
}

async function applyOp(type: string, payload: unknown) {
  if (!session.value) {
    ElMessage.warning("请先创建编辑会话");
    return;
  }
  applying.value = true;
  try {
    await applyOpRequest(type, payload);
    await refreshSession();
    await refreshSummary();
    ElMessage.success("操作已应用");
  } catch (err: any) {
    ElMessage.error(err?.message || "操作失败");
  } finally {
    applying.value = false;
  }
}

async function undo() {
  if (!session.value) return;
  try {
    const res = await editorApi.undo(session.value.id, sessionVersion.value);
    sessionVersion.value = res.sessionVersion;
    await refreshSession();
    await refreshSummary();
  } catch (err: any) {
    ElMessage.error(err?.message || "撤销失败");
  }
}

async function redo() {
  if (!session.value) return;
  try {
    const res = await editorApi.redo(session.value.id, sessionVersion.value);
    sessionVersion.value = res.sessionVersion;
    await refreshSession();
    await refreshSummary();
  } catch (err: any) {
    ElMessage.error(err?.message || "重做失败");
  }
}

async function commitSession() {
  if (!session.value) return;
  try {
    await ElMessageBox.confirm(
      "提交会话将生成新的 Revision 并关闭当前会话，确认提交？",
      "确认提交",
      {
        confirmButtonText: "确认提交",
        cancelButtonText: "取消",
        type: "warning",
      },
    );
  } catch {
    return;
  }
  committing.value = true;
  try {
    await editorApi.commitSession(session.value.id, {
      expectedSessionVersion: sessionVersion.value,
      activationPolicy: "immediate",
      idempotencyKey: `commit:${session.value.id}:${sessionVersion.value}`,
    });
    ElMessage.success("会话已提交");
    session.value = null;
    sessionVersion.value = 0;
    stopPlay();
    await loadData();
  } catch (err: any) {
    ElMessage.error(err?.message || "提交会话失败");
  } finally {
    committing.value = false;
  }
}

async function abandonSession() {
  if (!session.value) return;
  try {
    await ElMessageBox.confirm(
      "放弃会话将丢弃所有未提交的修改，确认放弃？",
      "确认放弃",
      {
        confirmButtonText: "确认放弃",
        cancelButtonText: "取消",
        type: "warning",
      },
    );
  } catch {
    return;
  }
  abandoning.value = true;
  try {
    await editorApi.abandonSession(session.value.id);
    ElMessage.success("会话已放弃");
    session.value = null;
    sessionVersion.value = 0;
    stopPlay();
    await loadData();
  } catch (err: any) {
    ElMessage.error(err?.message || "放弃会话失败");
  } finally {
    abandoning.value = false;
  }
}

function selectFrame(index: number) {
  currentFrameIndex.value = index;
}

function togglePlay() {
  if (isPlaying.value) {
    stopPlay();
  } else {
    startPlay();
  }
}

function stopPlay() {
  isPlaying.value = false;
  if (playTimer.value !== null) {
    clearTimeout(playTimer.value);
    playTimer.value = null;
  }
}

function startPlay() {
  if (!editSummary.value?.timeline.length) return;
  isPlaying.value = true;
  playNextFrame();
}

function playNextFrame() {
  if (!isPlaying.value || !editSummary.value?.timeline.length) {
    stopPlay();
    return;
  }
  const timeline = editSummary.value.timeline;
  const current = timeline[currentFrameIndex.value];
  const duration = current?.durationMs || 100;
  playTimer.value = window.setTimeout(() => {
    if (!isPlaying.value) return;
    currentFrameIndex.value = (currentFrameIndex.value + 1) % timeline.length;
    playNextFrame();
  }, duration);
}

async function deleteFrame(frameId: string) {
  await applyOp("frame.delete", { frameId });
}

async function duplicateFrame(frameId: string) {
  await applyOp("frame.duplicate", { frameId });
}

async function moveFrame(frameId: string, direction: "left" | "right") {
  const timeline = editSummary.value?.timeline ?? [];
  const index = timeline.findIndex((frame) => frame.frameId === frameId);
  if (index < 0) return;

  if (direction === "left") {
    if (index === 0) return;
    await applyOp("frame.reorder", {
      frameId,
      beforeFrameId: timeline[index - 1].frameId,
    });
    return;
  }

  if (index >= timeline.length - 1) return;
  await applyOp("frame.reorder", {
    frameId,
    afterFrameId: timeline[index + 1].frameId,
  });
}

async function updateFrameDuration(frameId: string, duration: number) {
  await applyOp("frame.set_duration", { frameId, durationMs: duration });
}

function onFrameDurationChange() {
  if (!currentFrame.value || !hasSession.value) return;
  void updateFrameDuration(currentFrame.value.frameId, frameDuration.value);
}

function onFrameAnchorChange() {
  if (!currentFrame.value || !session.value) return;
  void applyOp("anchor.set_frame", {
    frameId: currentFrame.value.frameId,
    anchorX: frameAnchorX.value,
    anchorY: frameAnchorY.value,
    space: "normalized_canvas",
  });
}

async function updateActionProps() {
  if (!session.value) {
    ElMessage.warning("请先创建编辑会话");
    return;
  }
  applying.value = true;
  try {
    await applyOpRequest("action.set_default_fps", {
      defaultFps: actionProps.fps,
      recalculate: false,
    });
    await applyOpRequest("action.set_loop_type", {
      loopType: actionProps.loopType,
    });
    await applyOpRequest("action.set_return_action", {
      returnAction: actionProps.returnAction,
    });
    await applyOpRequest("action.set_interruptible", {
      interruptible: actionProps.interruptible,
    });
    await applyOpRequest("action.set_priority_override", {
      priority: actionProps.priority,
    });
    await applyOpRequest("action.set_cooldown_override", {
      cooldownMs: actionProps.cooldownMs,
    });
    await refreshSession();
    await refreshSummary();
    ElMessage.success("动作属性已更新");
  } catch (err: any) {
    ElMessage.error(err?.message || "更新动作属性失败");
  } finally {
    applying.value = false;
  }
}

function goBack() {
  router.push(`/creative-workshop/pet/processing/${processingTaskId.value}`);
}

watch(currentFrame, (frame) => {
  if (frame) {
    frameDuration.value = frame.durationMs;
    frameAnchorX.value = frame.anchorX;
    frameAnchorY.value = frame.anchorY;
    if (activeRevisionId.value) {
      void loadFrameImage(activeRevisionId.value, frame.frameId);
    }
  }
});

watch(activeRevisionId, (newId, oldId) => {
  if (newId !== oldId) {
    clearImageCache();
  }
});

watch(
  () => editSummary.value?.timeline,
  (timeline) => {
    if (!timeline || !activeRevisionId.value) return;
    for (const frame of timeline) {
      if (!thumbUrls[frame.frameId]) {
        void loadThumb(activeRevisionId.value, frame.frameId);
      }
    }
  },
);

watch([processingTaskId, actionKey], () => {
  stopPlay();
  session.value = null;
  sessionVersion.value = 0;
  currentFrameIndex.value = 0;
  clearImageCache();
  void loadData();
});

onMounted(() => {
  void loadData();
});

onUnmounted(() => {
  stopPlay();
  clearImageCache();
});
</script>

<style scoped>
.action-editor {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}

.empty-state {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}

.editor-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-height: 0;
  padding-top: 12px;
}

.editor-main {
  flex: 1;
  display: flex;
  gap: 12px;
  min-height: 0;
}

.canvas-area {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
  border: 1px solid var(--el-border-color-light);
  border-radius: 8px;
  background: var(--ac-color-surface, var(--el-bg-color));
  padding: 12px;
}

.canvas-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.frame-indicator {
  font-size: 14px;
  font-weight: 500;
  color: var(--el-text-color-primary);
}

.session-badge {
  font-size: 12px;
  color: var(--el-color-success);
  padding: 2px 8px;
  border-radius: 4px;
  background: var(--el-color-success-light-9);
}

.canvas-stage {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 0;
  border-radius: 6px;
  background: var(--el-fill-color-light, #f5f7fa);
  overflow: hidden;
}

.frame-image {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
  image-rendering: pixelated;
}

.canvas-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  color: var(--el-text-color-secondary);
  font-size: 14px;
}

.canvas-controls {
  display: flex;
  justify-content: center;
  gap: 8px;
}

.props-panel {
  width: 320px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
  overflow-y: auto;
}

.prop-card {
  border: 1px solid var(--el-border-color-light);
  background: var(--ac-color-surface, var(--el-bg-color));
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.card-subtitle {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.quality-info {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.quality-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.quality-label {
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.quality-value {
  color: var(--el-text-color-primary);
  font-size: 13px;
  font-weight: 500;
}

.has-issue-text {
  color: var(--el-color-warning);
}

.timeline {
  height: 120px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  border: 1px solid var(--el-border-color-light);
  border-radius: 8px;
  background: var(--ac-color-surface, var(--el-bg-color));
  padding: 8px 12px;
  overflow: hidden;
}

.timeline-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
}

.timeline-title {
  font-size: 13px;
  font-weight: 500;
  color: var(--el-text-color-primary);
}

.timeline-hint {
  font-size: 12px;
  color: var(--el-text-color-placeholder);
}

.timeline-track {
  flex: 1;
  display: flex;
  gap: 8px;
  overflow-x: auto;
  overflow-y: hidden;
  padding-bottom: 4px;
}

.timeline-item {
  flex-shrink: 0;
  width: 96px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  padding: 4px;
  border: 2px solid transparent;
  border-radius: 6px;
  cursor: pointer;
  transition: border-color 180ms ease, background 180ms ease;
}

.timeline-item:hover {
  background: var(--el-fill-color-light, #f5f7fa);
}

.timeline-item.active {
  border-color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
}

.timeline-item.has-issue {
  border-color: var(--el-color-warning-light-5);
}

.timeline-item.active.has-issue {
  border-color: var(--el-color-warning);
}

.timeline-thumb {
  position: relative;
  width: 80px;
  height: 48px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 4px;
  overflow: hidden;
  background: var(--el-fill-color-light, #f5f7fa);
  display: flex;
  align-items: center;
  justify-content: center;
}

.timeline-thumb img {
  width: 100%;
  height: 100%;
  object-fit: contain;
  image-rendering: pixelated;
}

.thumb-placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--el-text-color-placeholder);
}

.issue-badge {
  position: absolute;
  top: 2px;
  right: 2px;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: var(--el-color-warning);
  color: #fff;
  font-size: 11px;
  font-weight: 700;
  line-height: 16px;
  text-align: center;
}

.timeline-meta {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
}

.timeline-index {
  font-size: 12px;
  font-weight: 500;
  color: var(--el-text-color-primary);
}

.timeline-duration {
  font-size: 11px;
  color: var(--el-text-color-secondary);
}

.timeline-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 2px;
}

.timeline-actions :deep(.el-button) {
  margin-left: 0;
  padding: 2px 4px;
  font-size: 11px;
}

@media (max-width: 900px) {
  .editor-main {
    flex-direction: column;
  }

  .props-panel {
    width: 100%;
    flex-direction: row;
    flex-wrap: wrap;
  }

  .prop-card {
    flex: 1;
    min-width: 280px;
  }
}
</style>
