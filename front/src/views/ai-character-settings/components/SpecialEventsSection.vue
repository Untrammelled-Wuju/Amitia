<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="section-card">
    <div class="section-title">
      特殊状态
      <el-button size="small" type="primary" @click="addSpecial" style="margin-left:12px">+ 添加</el-button>
    </div>
    <div v-if="specialEvents.length === 0" class="gender-hint" style="padding:8px 0">暂无特殊状态，可添加考试周、兼职、健身、图书馆、生病等</div>
    <div v-for="se in specialEvents" :key="se.id" class="course-item">
      <div class="course-info">
        <span class="course-title">{{ se.title }}</span>
        <el-tag size="small" type="info">{{ SPECIAL_TYPE_LABELS[se.eventType] || se.eventType }}</el-tag>
        <span class="course-time" v-if="se.startDate">{{ se.startDate }} ~ {{ se.endDate }}</span>
        <span class="course-time" v-if="se.startTime">{{ se.startTime }}-{{ se.endTime }}</span>
        <span class="course-reply">回复：{{ se.replyMode === 'NO_REPLY' ? '不回复' : se.replyMode === 'SHORT_REPLY' ? '简短' : '正常' }}</span>
      </div>
      <div class="course-actions">
        <el-switch v-model="se.enabled" size="small" @change="(v: any) => toggleSpecial(se.id, v)" />
        <el-button size="small" @click="editSpecial(se)">编辑</el-button>
        <el-button size="small" type="danger" @click="deleteSpecial(se.id)">删除</el-button>
      </div>
    </div>

    <el-dialog v-model="showSpecialDialog" :title="editingSpecial ? '编辑特殊状态' : '添加特殊状态'" width="500px" destroy-on-close>
      <div class="course-form">
        <div class="form-item">
          <label>标题</label>
          <el-input v-model="specialForm.title" placeholder="例如：期末考、周末家教" size="default" />
        </div>
        <div class="form-item">
          <label>类型</label>
          <el-select v-model="specialForm.eventType" size="default" style="width:100%">
            <el-option v-for="opt in SPECIAL_EVENT_TYPE_OPTIONS" :key="opt.value" :label="opt.label" :value="opt.value" />
          </el-select>
        </div>
        <div class="form-row">
          <div class="form-item" style="flex:1">
            <label>开始日期</label>
            <el-date-picker v-model="specialForm.startDate" type="date" value-format="YYYY-MM-DD" placeholder="开始" size="default" style="width:100%" />
          </div>
          <div class="form-item" style="flex:1">
            <label>结束日期</label>
            <el-date-picker v-model="specialForm.endDate" type="date" value-format="YYYY-MM-DD" placeholder="结束" size="default" style="width:100%" />
          </div>
        </div>
        <div class="form-row">
          <div class="form-item" style="flex:1">
            <label>开始时间</label>
            <el-time-picker v-model="specialForm.startTime" format="HH:mm" value-format="HH:mm" size="default" style="width:100%" />
          </div>
          <div class="form-item" style="flex:1">
            <label>结束时间</label>
            <el-time-picker v-model="specialForm.endTime" format="HH:mm" value-format="HH:mm" size="default" style="width:100%" />
          </div>
        </div>
        <div class="form-item">
          <label>回复模式</label>
          <el-select v-model="specialForm.replyMode" size="default" style="width:100%">
            <el-option v-for="opt in SPECIAL_REPLY_OPTIONS" :key="opt.value" :label="opt.label" :value="opt.value" />
          </el-select>
        </div>
        <div class="form-item" v-if="specialForm.eventType === 'SICK_REST'">
          <el-checkbox v-model="specialForm.affectSleep" label="影响睡眠（提前睡觉）" />
        </div>
      </div>
      <template #footer>
        <el-button @click="showSpecialDialog = false">取消</el-button>
        <el-button type="primary" @click="saveSpecial" :loading="specialSaving">
          {{ editingSpecial ? '更新' : '添加' }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from "vue"
import { ElMessage } from "element-plus"
import { useSpecialEvents, SPECIAL_EVENT_TYPE_OPTIONS, SPECIAL_REPLY_OPTIONS } from "../../../composables/useSpecialEvents"

const props = defineProps<{
  characterId: string
}>()

const { getSpecialEvents, createSpecialEvent, updateSpecialEvent, deleteSpecialEvent } = useSpecialEvents()

const SPECIAL_TYPE_LABELS: Record<string, string> = {
  EXAM_WEEK: "考试周", EXAM: "具体考试", PART_TIME_WORK: "周末兼职",
  EVENING_WORKOUT: "晚上健身", LIBRARY_STUDY: "图书馆学习",
  SICK_REST: "生病休息", CUSTOM: "自定义",
}

const specialEvents = ref<any[]>([])
const showSpecialDialog = ref(false)
const editingSpecial = ref<any>(null)
const specialSaving = ref(false)
const specialForm = reactive({
  title: "", eventType: "EXAM", startDate: null as string | null, endDate: null as string | null,
  startTime: null as string | null, endTime: null as string | null,
  replyMode: "SHORT_REPLY", affectSleep: false,
})

onMounted(async () => {
  await loadSpecialEvents()
})

async function loadSpecialEvents() {
  try {
    specialEvents.value = await getSpecialEvents(props.characterId || undefined)
  } catch { }
}

function addSpecial() {
  editingSpecial.value = null
  specialForm.title = ""
  specialForm.eventType = "EXAM"
  specialForm.startDate = null
  specialForm.endDate = null
  specialForm.startTime = null
  specialForm.endTime = null
  specialForm.replyMode = "SHORT_REPLY"
  specialForm.affectSleep = false
  showSpecialDialog.value = true
}

function editSpecial(se: any) {
  editingSpecial.value = se
  specialForm.title = se.title
  specialForm.eventType = se.eventType
  specialForm.startDate = se.startDate
  specialForm.endDate = se.endDate
  specialForm.startTime = se.startTime
  specialForm.endTime = se.endTime
  specialForm.replyMode = se.replyMode || "SHORT_REPLY"
  specialForm.affectSleep = !!se.affectSleep
  showSpecialDialog.value = true
}

async function saveSpecial() {
  if (!specialForm.title.trim()) { ElMessage.warning("请填写标题"); return }
  specialSaving.value = true
  try {
    const input = { ...specialForm, eventType: specialForm.eventType }
    if (editingSpecial.value) {
      await updateSpecialEvent(editingSpecial.value.id, input, props.characterId || undefined)
    } else {
      await createSpecialEvent(input, props.characterId || undefined)
    }
    showSpecialDialog.value = false
    editingSpecial.value = null
    specialForm.title = ""
    specialForm.startDate = null
    specialForm.endDate = null
    specialForm.startTime = null
    specialForm.endTime = null
    specialForm.replyMode = "SHORT_REPLY"
    specialForm.affectSleep = false
    await loadSpecialEvents()
    ElMessage.success("已保存")
  } catch (e: any) {
    ElMessage.error(e?.message || "保存失败")
  } finally {
    specialSaving.value = false
  }
}

async function deleteSpecial(id: number) {
  try {
    await deleteSpecialEvent(id, props.characterId || undefined)
    await loadSpecialEvents()
    ElMessage.success("已删除")
  } catch {
    ElMessage.error("删除失败")
  }
}

async function toggleSpecial(id: number, enabled: boolean) {
  try {
    await updateSpecialEvent(id, { enabled }, props.characterId || undefined)
  } catch { }
}
</script>
