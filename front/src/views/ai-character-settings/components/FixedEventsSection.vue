<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="section-card">
    <div class="section-title">
      课程/固定日程
      <el-button size="small" type="primary" @click="addCourse" style="margin-left:12px">+ 添加</el-button>
    </div>
    <div v-if="courses.length === 0" class="gender-hint" style="padding:8px 0">暂无固定日程，点击上方按钮添加课程或会议</div>
    <div v-for="c in courses" :key="c.id" class="course-item">
      <div class="course-info">
        <span class="course-title">{{ c.title }}</span>
        <el-tag size="small" type="info">{{ EVENT_TYPE_LABELS[c.eventType] || c.eventType }}</el-tag>
        <span class="course-time">{{ c.startTime }}-{{ c.endTime }}</span>
        <span class="course-days" v-if="c.repeatDays">{{ dayLabels(c.repeatDays) }}</span>
        <span class="course-reply">回复：{{ REPLY_MODE_LABELS[c.replyMode] || c.replyMode }}</span>
      </div>
      <div class="course-actions">
        <el-switch v-model="c.enabled" size="small" @change="(v: any) => toggleCourse(c.id, v)" />
        <el-button size="small" @click="editCourse(c)">编辑</el-button>
        <el-popconfirm title="确定删除？" @confirm="deleteCourse(c.id)">
          <template #reference>
            <el-button size="small" type="danger">删除</el-button>
          </template>
        </el-popconfirm>
      </div>
    </div>

    <el-dialog v-model="showCourseDialog" :title="editingCourse ? '编辑日程' : '添加日程'" width="480px" destroy-on-close>
      <div class="course-form">
        <div class="form-item">
          <label>标题</label>
          <el-input v-model="courseForm.title" placeholder="例如：高等数学" size="default" />
        </div>
        <div class="form-item">
          <label>类型</label>
          <el-select v-model="courseForm.eventType" size="default" style="width:100%">
            <el-option v-for="opt in EVENT_TYPE_OPTIONS" :key="opt.value" :label="opt.label" :value="opt.value" />
          </el-select>
        </div>
        <div class="form-row">
          <div class="form-item" style="flex:1">
            <label>开始时间</label>
            <el-time-picker v-model="courseForm.startTime" format="HH:mm" value-format="HH:mm" placeholder="开始" size="default" style="width:100%" />
          </div>
          <div class="form-item" style="flex:1">
            <label>结束时间</label>
            <el-time-picker v-model="courseForm.endTime" format="HH:mm" value-format="HH:mm" placeholder="结束" size="default" style="width:100%" />
          </div>
        </div>
        <div class="form-item">
          <label>重复星期</label>
          <el-checkbox-group v-model="courseForm.repeatDays">
            <el-checkbox v-for="d in WEEKDAY_OPTIONS" :key="d.value" :label="d.value" :value="d.value">{{ d.label }}</el-checkbox>
          </el-checkbox-group>
        </div>
        <div class="form-row">
          <div class="form-item" style="flex:1">
            <label>准备时间(最少分)</label>
            <el-input-number v-model="courseForm.prepareMinMinutes" :min="5" :max="60" size="default" style="width:100%" />
          </div>
          <div class="form-item" style="flex:1">
            <label>准备时间(最多分)</label>
            <el-input-number v-model="courseForm.prepareMaxMinutes" :min="5" :max="90" size="default" style="width:100%" />
          </div>
        </div>
        <div class="form-item">
          <label>回复模式</label>
          <el-select v-model="courseForm.replyMode" size="default" style="width:100%">
            <el-option v-for="opt in REPLY_MODE_OPTIONS" :key="opt.value" :label="opt.label" :value="opt.value" />
          </el-select>
          <span class="gender-hint">决定该日程期间的回复方式</span>
        </div>
      </div>
      <template #footer>
        <el-button @click="showCourseDialog = false">取消</el-button>
        <el-button type="primary" @click="saveCourse" :loading="courseSaving">
          {{ editingCourse ? '更新' : '添加' }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from "vue"
import { ElMessage } from "element-plus"
import { useFixedEvents, WEEKDAY_OPTIONS, EVENT_TYPE_OPTIONS, REPLY_MODE_OPTIONS, type FixedEvent } from "../../../composables/useFixedEvents"

const props = defineProps<{
  characterId: string
}>()

const { getFixedEvents, createFixedEvent, updateFixedEvent, deleteFixedEvent } = useFixedEvents()

const courses = ref<FixedEvent[]>([])
const showCourseDialog = ref(false)
const editingCourse = ref<FixedEvent | null>(null)
const courseSaving = ref(false)

const EVENT_TYPE_LABELS: Record<string, string> = {
  CLASS: "上课", STUDY: "自习", FULL_TIME_WORK: "全职上班",
  PART_TIME_WORK: "兼职", MEETING: "会议", CUSTOM_BUSY: "自定义"
}

const REPLY_MODE_LABELS: Record<string, string> = {
  NO_REPLY: "不回复", SHORT_REPLY: "简短回复",
  NORMAL_REPLY: "正常回复", DELAY_REPLY: "延迟回复",
}

const WEEKDAY_LABELS: Record<string, string> = {
  MON: "周一", TUE: "周二", WED: "周三", THU: "周四",
  FRI: "周五", SAT: "周六", SUN: "周日",
}

function dayLabels(days: string): string {
  if (!days) return ""
  return days.split(",").map(d => WEEKDAY_LABELS[d] || d).join(" ")
}

const courseForm = reactive({
  title: "",
  eventType: "CLASS",
  startTime: "",
  endTime: "",
  repeatDays: [] as string[],
  prepareMinMinutes: 10,
  prepareMaxMinutes: 40,
  replyMode: "SHORT_REPLY",
})

onMounted(async () => {
  await loadCourses()
})

async function loadCourses() {
  try {
    courses.value = await getFixedEvents(props.characterId || undefined)
  } catch { }
}

function addCourse() {
  editingCourse.value = null
  courseForm.title = ""
  courseForm.eventType = "CLASS"
  courseForm.startTime = ""
  courseForm.endTime = ""
  courseForm.repeatDays = []
  courseForm.prepareMinMinutes = 10
  courseForm.prepareMaxMinutes = 40
  courseForm.replyMode = "SHORT_REPLY"
  showCourseDialog.value = true
}

function editCourse(c: FixedEvent) {
  editingCourse.value = c
  courseForm.title = c.title
  courseForm.eventType = c.eventType
  courseForm.startTime = c.startTime
  courseForm.endTime = c.endTime
  courseForm.repeatDays = c.repeatDays ? c.repeatDays.split(",") : []
  courseForm.prepareMinMinutes = c.prepareMinMinutes
  courseForm.prepareMaxMinutes = c.prepareMaxMinutes
  courseForm.replyMode = c.replyMode
  showCourseDialog.value = true
}

async function saveCourse() {
  if (!courseForm.title.trim() || !courseForm.startTime || !courseForm.endTime) {
    ElMessage.warning("请填写标题和时间")
    return
  }
  courseSaving.value = true
  try {
    const input = {
      title: courseForm.title.trim(),
      eventType: courseForm.eventType,
      startTime: courseForm.startTime,
      endTime: courseForm.endTime,
      repeatDays: courseForm.repeatDays.length > 0 ? courseForm.repeatDays.join(",") : null,
      prepareMinMinutes: courseForm.prepareMinMinutes,
      prepareMaxMinutes: courseForm.prepareMaxMinutes,
      replyMode: courseForm.replyMode,
    }
    if (editingCourse.value) {
      await updateFixedEvent(editingCourse.value.id, input, props.characterId || undefined)
    } else {
      await createFixedEvent(input, props.characterId || undefined)
    }
    showCourseDialog.value = false
    const wasEditing = !!editingCourse.value
    editingCourse.value = null
    await loadCourses()
    ElMessage.success(wasEditing ? "已更新" : "已添加")
  } catch (e: any) {
    ElMessage.error(e?.message || "保存失败")
  } finally {
    courseSaving.value = false
  }
}

async function deleteCourse(id: number) {
  try {
    await deleteFixedEvent(id, props.characterId || undefined)
    await loadCourses()
    ElMessage.success("已删除")
  } catch {
    ElMessage.error("删除失败")
  }
}

async function toggleCourse(id: number, enabled: boolean) {
  try {
    await updateFixedEvent(id, { enabled }, props.characterId || undefined)
  } catch { }
}
</script>
