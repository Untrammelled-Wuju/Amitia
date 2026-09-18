<template>
  <div class="life-rules-page">
    <div class="page-header">
      <div>
        <h2>生活规则</h2>
        <p class="page-desc">管理 AI 角色的作息、状态、日程和互动规则</p>
      </div>
      <el-button @click="regenerateAll" :loading="regenerating" type="warning" plain>重新生成今日作息</el-button>
    </div>

    <div class="cards">

      <!-- 1. 角色基础画像 -->
      <div class="card">
        <div class="card-header" @click="toggle('role')">
          <span class="card-title">角色基础画像</span>
          <el-icon><ArrowDown v-if="openSections.role" /><ArrowRight v-else /></el-icon>
        </div>
        <div v-show="openSections.role" class="card-body">
          <div class="form-grid">
            <div class="form-item"><label>角色名称</label><el-input v-model="roleForm.roleName" size="default" /></div>
            <div class="form-item"><label>角色性别</label>
              <el-select v-model="roleForm.gender" size="default">
                <el-option v-for="o in GENDER_OPTIONS" :key="o.value" :label="o.label" :value="o.value" />
              </el-select>
            </div>
            <div class="form-item"><label>角色代词</label>
              <el-select v-model="roleForm.pronoun" size="default">
                <el-option v-for="o in PRONOUN_OPTIONS" :key="o.value" :label="o.label" :value="o.value" />
              </el-select>
            </div>
            <div class="form-item"><label>称呼风格</label>
              <el-select v-model="roleForm.userAddressingStyle" size="default" clearable>
                <el-option v-for="o in ADDRESSING_OPTIONS" :key="o.value" :label="o.label" :value="o.value" />
              </el-select>
            </div>
            <div class="form-item"><label>性别表达 {{ roleForm.genderExpression }}</label>
              <el-slider v-model="roleForm.genderExpression" :min="0" :max="100" />
            </div>
          </div>
          <el-button type="primary" size="small" @click="saveRole" :loading="savingRole">保存</el-button>
        </div>
      </div>

      <!-- 生活场景 -->
      <div class="card">
        <div class="card-header" @click="toggle('lifeIdentity')">
          <span class="card-title">生活场景</span>
          <el-icon><ArrowDown v-if="openSections.lifeIdentity" /><ArrowRight v-else /></el-icon>
        </div>
        <div v-show="openSections.lifeIdentity" class="card-body">
          <div class="form-grid">
            <div class="form-item">
              <label>选择生活场景</label>
              <el-select v-model="lifeIdentity" placeholder="选择生活场景" size="default" style="width:100%">
                <el-option v-for="opt in LIFE_IDENTITY_OPTIONS" :key="opt.value" :label="opt.label" :value="opt.value" />
              </el-select>
              <span class="gender-hint">选择后将显示对应的生活规则配置项</span>
            </div>
          </div>
          <el-button type="primary" size="small" @click="saveLifeIdentity" :loading="savingLifeIdentity">保存</el-button>
        </div>
      </div>

      <!-- 2. 作息倾向 -->
      <div class="card">
        <div class="card-header" @click="toggle('lifestyle')">
          <span class="card-title">作息倾向</span>
          <el-icon><ArrowDown v-if="openSections.lifestyle" /><ArrowRight v-else /></el-icon>
        </div>
        <div v-show="openSections.lifestyle" class="card-body">
          <div class="slider-grid">
            <div v-for="s in LIFESTYLE_SLIDERS" :key="s.key" class="slider-row">
              <div class="sr-header"><span class="sr-label">{{ s.label }}</span><span class="sr-value">{{ lifestyleForm[s.key] }}</span></div>
              <div class="sr-body">
                <span class="sr-left">{{ s.left }}</span>
                <div class="sr-slider-wrap"><el-slider v-model="lifestyleForm[s.key]" :min="0" :max="100" :step="1" /></div>
                <span class="sr-right">{{ s.right }}</span>
              </div>
              <span class="sr-hint">{{ s.hint }}</span>
            </div>
          </div>
          <div class="btn-row">
            <el-button type="primary" size="small" @click="saveLifestyle" :loading="savingLifestyle">保存</el-button>
            <el-button size="small" @click="resetLifestyle" :loading="resettingLifestyle">恢复默认</el-button>
          </div>
        </div>
      </div>

      <!-- 3. 基础作息 -->
      <div class="card">
        <div class="card-header" @click="toggle('schedule')">
          <span class="card-title">今日作息</span>
          <el-icon><ArrowDown v-if="openSections.schedule" /><ArrowRight v-else /></el-icon>
        </div>
        <div v-show="openSections.schedule" class="card-body">
          <div v-if="todaySchedule" class="schedule-display">
            <div class="sched-row"><span>起床</span><span>{{ todaySchedule.wakeTime?.slice(11,16) }}</span></div>
            <div class="sched-row"><span>午饭</span><span>{{ todaySchedule.lunchTime?.slice(11,16) }}</span></div>
            <div class="sched-row" v-if="todaySchedule.hasNap"><span>午睡</span><span>{{ todaySchedule.napStartTime?.slice(11,16) }} - {{ todaySchedule.napEndTime?.slice(11,16) }}</span></div>
            <div class="sched-row"><span>晚饭</span><span>{{ todaySchedule.dinnerTime?.slice(11,16) }}</span></div>
            <div class="sched-row"><span>睡觉</span><span>{{ todaySchedule.sleepTime?.slice(11,16) }}</span></div>
            <div class="sched-row"><span>休息日</span><span>{{ todaySchedule.isRestDay ? '是' : '否' }}</span></div>
          </div>
          <div v-else class="empty-hint">暂无今日作息，点击上方"重新生成今日作息"</div>
        </div>
      </div>

      <!-- 4. 主动消息 -->
      <div class="card">
        <div class="card-header" @click="toggle('activeMsg')">
          <span class="card-title">主动消息</span>
          <el-icon><ArrowDown v-if="openSections.activeMsg" /><ArrowRight v-else /></el-icon>
        </div>
        <div v-show="openSections.activeMsg" class="card-body">
          <div class="form-grid">
            <div class="form-item"><label>主动消息开关</label><el-switch v-model="activeMsgForm.enabled" /></div>
            <div class="form-item"><label>主动性 {{ activeMsgForm.activeLevel }}</label><el-slider v-model="activeMsgForm.activeLevel" :min="0" :max="100" /></div>
            <div class="form-item"><label>静默分钟数</label><el-input-number v-model="activeMsgForm.quietMinutes" :min="1" :max="120" size="default" /></div>
            <div class="form-item"><label>最小间隔(分)</label><el-input-number v-model="activeMsgForm.minInterval" :min="10" :max="180" size="default" /></div>
            <div class="form-item"><label>每日上限</label><el-input-number v-model="activeMsgForm.maxDaily" :min="1" :max="20" size="default" /></div>
          </div>
          <el-button type="primary" size="small" @click="saveActiveMsg" :loading="savingActiveMsg">保存</el-button>
          <div v-if="activeMsgTasks.length" style="margin-top:12px">
            <div class="sub-title">今日任务</div>
            <div v-for="t in activeMsgTasks" :key="t.id" class="task-row">
              <span class="task-type">{{ t.taskLabel || t.taskType }}</span>
              <span class="task-time">{{ t.dueTime?.slice(11,16) }}</span>
              <el-tag :type="t.status==='SUCCESS'?'success':t.status==='CANCELLED'?'info':t.status==='FAILED'?'danger':'warning'" size="small">{{ t.status }}</el-tag>
            </div>
          </div>
        </div>
      </div>

      <!-- 5. 睡眠设置 -->
      <div class="card">
        <div class="card-header" @click="toggle('sleep')">
          <span class="card-title">睡眠设置</span>
          <el-icon><ArrowDown v-if="openSections.sleep" /><ArrowRight v-else /></el-icon>
        </div>
        <div v-show="openSections.sleep" class="card-body">
          <div class="form-grid">
            <div class="form-item"><label>睡觉后回复</label><el-switch v-model="sleepForm.sleepReplyEnabled" /></div>
            <div class="form-item"><label>回复模式</label>
              <el-select v-model="sleepForm.sleepReplyMode" size="default">
                <el-option v-for="o in SLEEP_REPLY_MODE_OPTIONS" :key="o.value" :label="o.label" :value="o.value" />
              </el-select>
            </div>
          </div>
          <el-button type="primary" size="small" @click="saveSleep" :loading="savingSleep">保存</el-button>
        </div>
      </div>

      <!-- 6. 课程/固定日程 -->
      <div class="card">
        <div class="card-header" @click="toggle('courses')">
          <span class="card-title">课程/固定日程</span>
          <el-button size="small" @click.stop="addCourse" style="margin-left:auto">+ 添加</el-button>
        </div>
        <div v-show="openSections.courses" class="card-body">
          <div v-if="courses.length===0" class="empty-hint">暂无课程</div>
          <div v-for="c in courses" :key="c.id" class="course-row">
            <span>{{ c.title }}</span>
            <span class="c-time">{{ c.startTime }}-{{ c.endTime }}</span>
            <span class="c-days">{{ c.repeatDays || '每天' }}</span>
            <el-switch v-model="c.enabled" size="small" @change="(v:any)=>toggleCourse(c.id,v)" />
            <el-button size="small" @click="editCourseItem(c)">编辑</el-button>
            <el-button size="small" type="danger" @click="delCourse(c.id)">删除</el-button>
          </div>
        </div>
      </div>

      <!-- 7. 上班规则 -->
      <div class="card">
        <div class="card-header" @click="toggle('work')">
          <span class="card-title">上班规则</span>
          <el-icon><ArrowDown v-if="openSections.work" /><ArrowRight v-else /></el-icon>
        </div>
        <div v-show="openSections.work" class="card-body">
          <div class="form-grid">
            <div class="form-item"><label>启用</label><el-switch v-model="workForm.enabled" /></div>
            <div class="form-item"><label>上班</label><el-time-picker v-model="workForm.workStartTime" format="HH:mm" value-format="HH:mm" size="default" /></div>
            <div class="form-item"><label>下班</label><el-time-picker v-model="workForm.workEndTime" format="HH:mm" value-format="HH:mm" size="default" /></div>
            <div class="form-item"><label>午休</label><span>{{ workForm.lunchBreakStartTime }} - {{ workForm.lunchBreakEndTime }}</span></div>
            <div class="form-item"><label>回复模式</label>
              <el-select v-model="workForm.replyMode" size="default">
                <el-option v-for="o in WORK_REPLY_OPTIONS" :key="o.value" :label="o.label" :value="o.value" />
              </el-select>
            </div>
            <div class="form-item"><label>延迟回复</label><el-switch v-model="workForm.delayedReplyEnabled" /></div>
          </div>
          <el-button type="primary" size="small" @click="saveWork" :loading="savingWork">保存</el-button>
        </div>
      </div>

      <!-- 8. 特殊状态 -->
      <div class="card">
        <div class="card-header" @click="toggle('special')">
          <span class="card-title">特殊状态</span>
          <el-button size="small" @click.stop="addSpecial" style="margin-left:auto">+ 添加</el-button>
        </div>
        <div v-show="openSections.special" class="card-body">
          <div v-if="specialEvents.length===0" class="empty-hint">暂无特殊状态</div>
          <div v-for="se in specialEvents" :key="se.id" class="course-row">
            <span>{{ se.title }}</span>
            <el-tag size="small">{{ se.eventType }}</el-tag>
            <span class="c-time" v-if="se.startDate">{{ se.startDate }}~{{ se.endDate }}</span>
            <el-switch v-model="se.enabled" size="small" @change="(v:any)=>toggleSpecial(se.id,v)" />
            <el-button size="small" @click="editSpecialItem(se)">编辑</el-button>
            <el-button size="small" type="danger" @click="delSpecial(se.id)">删除</el-button>
          </div>
        </div>
      </div>

      
  <!-- Course Edit Dialog -->
  <el-dialog v-model="showCourseEdit" :title="editingCourse ? '编辑日程' : '添加日程'" width="480px" destroy-on-close>
    <div class="course-form">
      <div class="form-item"><label>标题</label><el-input v-model="courseEditForm.title" size="default" /></div>
      <div class="form-item"><label>类型</label>
        <el-select v-model="courseEditForm.eventType" size="default" style="width:100%">
          <el-option v-for="opt in EVENT_TYPE_OPTIONS" :key="opt.value" :label="opt.label" :value="opt.value" />
        </el-select>
      </div>
      <div class="form-row">
        <div class="form-item" style="flex:1"><label>开始时间</label><el-time-picker v-model="courseEditForm.startTime" format="HH:mm" value-format="HH:mm" size="default" style="width:100%" /></div>
        <div class="form-item" style="flex:1"><label>结束时间</label><el-time-picker v-model="courseEditForm.endTime" format="HH:mm" value-format="HH:mm" size="default" style="width:100%" /></div>
      </div>
      <div class="form-item"><label>重复星期</label>
        <el-checkbox-group v-model="courseEditForm.repeatDays">
          <el-checkbox v-for="d in WEEKDAY_OPTIONS" :key="d.value" :label="d.value" :value="d.value">{{ d.label }}</el-checkbox>
        </el-checkbox-group>
      </div>
      <div class="form-item"><label>回复模式</label>
        <el-select v-model="courseEditForm.replyMode" size="default" style="width:100%">
          <el-option v-for="opt in REPLY_MODE_OPTIONS" :key="opt.value" :label="opt.label" :value="opt.value" />
        </el-select>
      </div>
    </div>
    <template #footer>
      <el-button @click="showCourseEdit = false">取消</el-button>
      <el-button type="primary" @click="saveCourseEdit" :loading="savingCourseEdit">{{ editingCourse ? '更新' : '添加' }}</el-button>
    </template>
  </el-dialog>

  <!-- Special Event Edit Dialog -->
  <el-dialog v-model="showSpecialEdit" :title="editingSpecial ? '编辑特殊状态' : '添加特殊状态'" width="500px" destroy-on-close>
    <div class="course-form">
      <div class="form-item"><label>标题</label><el-input v-model="specialEditForm.title" size="default" /></div>
      <div class="form-item"><label>类型</label>
        <el-select v-model="specialEditForm.eventType" size="default" style="width:100%">
          <el-option v-for="opt in SPECIAL_EVENT_TYPE_OPTIONS" :key="opt.value" :label="opt.label" :value="opt.value" />
        </el-select>
      </div>
      <div class="form-row">
        <div class="form-item" style="flex:1"><label>开始日期</label><el-date-picker v-model="specialEditForm.startDate" type="date" value-format="YYYY-MM-DD" size="default" style="width:100%" /></div>
        <div class="form-item" style="flex:1"><label>结束日期</label><el-date-picker v-model="specialEditForm.endDate" type="date" value-format="YYYY-MM-DD" size="default" style="width:100%" /></div>
      </div>
      <div class="form-item"><label>回复模式</label>
        <el-select v-model="specialEditForm.replyMode" size="default" style="width:100%">
          <el-option v-for="opt in SPECIAL_REPLY_OPTIONS" :key="opt.value" :label="opt.label" :value="opt.value" />
        </el-select>
      </div>
    </div>
    <template #footer>
      <el-button @click="showSpecialEdit = false">取消</el-button>
      <el-button type="primary" @click="saveSpecialEdit" :loading="savingSpecialEdit">{{ editingSpecial ? '更新' : '添加' }}</el-button>
    </template>
  </el-dialog><!-- 9. 今日时间轴 -->
      <div class="card">
        <div class="card-header" @click="toggle('timeline')">
          <span class="card-title">今日状态时间轴</span>
          <el-icon><ArrowDown v-if="openSections.timeline" /><ArrowRight v-else /></el-icon>
        </div>
        <div v-show="openSections.timeline" class="card-body">
          <div v-if="timeline.length===0" class="empty-hint">暂无时间轴数据</div>
          <div v-for="(t,i) in timeline" :key="i" class="tl-row">
            <span class="tl-time">{{ t.startTime?.slice(11,16) }} - {{ t.endTime?.slice(11,16) }}</span>
            <el-tag size="small" :type="t.state==='SLEEPING'?'info':t.state==='WORKING'?'warning':t.state==='IDLE'?'':'primary'">{{ stateLabel(t.state) }}</el-tag>
            <span class="tl-reason" v-if="t.reason">{{ t.reason }}</span>
          </div>
        </div>
      </div>

    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from "vue"
import { ElMessage } from "element-plus"
import { ArrowDown, ArrowRight } from "@element-plus/icons-vue"
import { useApi } from "../../composables/useApi"
import { useRoleProfile, GENDER_OPTIONS, PRONOUN_OPTIONS, ADDRESSING_OPTIONS } from "../../composables/useRoleProfile"
import { useLifestyleTendency, LIFESTYLE_SLIDERS } from "../../composables/useLifestyleTendency"
import { useSleepSetting, SLEEP_REPLY_MODE_OPTIONS } from "../../composables/useSleepSetting"
import { useFixedEvents, EVENT_TYPE_OPTIONS, REPLY_MODE_OPTIONS, WEEKDAY_OPTIONS, LIFE_IDENTITY_OPTIONS } from "../../composables/useFixedEvents"
import { useWorkProfile, WORK_REPLY_OPTIONS } from "../../composables/useWorkProfile"
import { useSpecialEvents, SPECIAL_EVENT_TYPE_OPTIONS, SPECIAL_REPLY_OPTIONS } from "../../composables/useSpecialEvents"

const { get, post, put, request } = useApi()
const { getRoleProfile, updateRoleProfile } = useRoleProfile()
const { getLifestyleTendency, updateLifestyleTendency, resetLifestyleTendency } = useLifestyleTendency()
const { getSleepSetting, updateSleepSetting } = useSleepSetting()
const { getFixedEvents, createFixedEvent, updateFixedEvent, deleteFixedEvent } = useFixedEvents()
const { getWorkProfile, updateWorkProfile } = useWorkProfile()
const { getSpecialEvents, createSpecialEvent, updateSpecialEvent, deleteSpecialEvent } = useSpecialEvents()

const openSections = reactive<Record<string,boolean>>({
  role: true, lifestyle: false, schedule: true, activeMsg: false,
  sleep: false, courses: false, work: false, special: false, timeline: true,
})
function toggle(k: string) { openSections[k] = !openSections[k] }

const roleForm = reactive({ roleName:"小暖", gender:"UNSPECIFIED", pronoun:"TA", selfReference:"我", userAddressingStyle:"自然称呼", genderExpression:30 })
const lifestyleForm = reactive<Record<string,number>>({ punctualityTendency:50, earlyPrepareTendency:50, selfDisciplineTendency:50, sleepinessTendency:50, randomnessTendency:50, activityEnergy:50, socialEnergy:50, careTendency:50, dailyShareTendency:50 })
const sleepForm = reactive({ sleepReplyEnabled:false, sleepReplyMode:"NO_REPLY" })
const activeMsgForm = reactive({ enabled:true, activeLevel:50, quietMinutes:10, minInterval:40, maxDaily:5 })
const workForm = reactive({ enabled:false, workStartTime:"09:00", workEndTime:"18:00", lunchBreakStartTime:"12:00", lunchBreakEndTime:"13:30", replyMode:"SHORT_REPLY", delayedReplyEnabled:false })
const todaySchedule = ref<any>(null)
const timeline = ref<any[]>([])
const activeMsgTasks = ref<any[]>([])
const courses = ref<any[]>([])
const lifeIdentity = ref("CUSTOM")
const savingLifeIdentity = ref(false)
const specialEvents = ref<any[]>([])
const regenerating = ref(false)
const savingRole = ref(false); const savingLifestyle = ref(false); const resettingLifestyle = ref(false)
const savingSleep = ref(false); const savingActiveMsg = ref(false); const savingWork = ref(false)
// Course editing
const showCourseEdit = ref(false)
const editingCourse = ref<any>(null)
const savingCourseEdit = ref(false)
const courseEditForm = reactive({ title: '', eventType: 'CLASS', startTime: '08:00', endTime: '09:40', repeatDays: ['MON'] as string[], replyMode: 'NO_REPLY' })

// Special event editing
const showSpecialEdit = ref(false)
const editingSpecial = ref<any>(null)
const savingSpecialEdit = ref(false)
const specialEditForm = reactive({ title: '', eventType: 'CUSTOM', startDate: '', endDate: '', replyMode: 'NO_REPLY' })

function stateLabel(s: string) {
  const m: Record<string,string> = { SLEEPING:"睡觉",WAKING_UP:"刚醒",IDLE:"空闲",EATING_LUNCH:"午饭",NAPPING:"午睡",EATING_DINNER:"晚饭",BEFORE_SLEEP:"睡前",WORKING:"工作中",LUNCH_BREAK:"午休",COMMUTING_HOME:"通勤",AFTER_WORK:"下班",IN_CLASS:"上课",PREPARING_CLASS:"准备上课",AFTER_CLASS:"下课",OVERTIME:"加班",SICK_RESTING:"生病",EXAM_PREPARING:"备考",IN_EXAM:"考试中",LIBRARY_STUDYING:"图书馆",WORKING_OUT:"健身",PART_TIME_WORKING:"兼职",LOW_ENERGY:"低精力" }
  return m[s] || s
}

async function loadAll() {
  try { const r = await getRoleProfile(); Object.assign(roleForm, { roleName:r.roleName||"小暖", gender:r.gender||"UNSPECIFIED", pronoun:r.pronoun||"TA", userAddressingStyle:r.userAddressingStyle||"自然称呼", genderExpression:r.genderExpression??30 }) } catch {}
  try { const l = await getLifestyleTendency(); Object.keys(lifestyleForm).forEach(k => { if ((l as any)[k]!==undefined) lifestyleForm[k]=(l as any)[k] }) } catch {}
  try { const s = await getSleepSetting(); Object.assign(sleepForm, { sleepReplyEnabled:s.sleepReplyEnabled, sleepReplyMode:s.sleepReplyMode }) } catch {}
  try { todaySchedule.value = await get<any>("/api/companion/schedule/today") } catch {}
  try { timeline.value = await get<any[]>("/api/companion/timeline/today") } catch {}
  try { activeMsgTasks.value = await get<any[]>("/api/companion/active-message/tasks/today") } catch {}
  try { 
    const ams = await get<any>("/api/companion/active-message/setting")
    Object.assign(activeMsgForm, { enabled:ams.activeMessageEnabled, activeLevel:ams.activeLevel, quietMinutes:ams.quietMinutesAfterUserMessage, minInterval:ams.minIntervalBetweenActiveMessages, maxDaily:ams.maxDailyActiveMessages })
  } catch {}
  try { courses.value = await getFixedEvents() } catch {}
  try {
    const wp = await getWorkProfile()
    Object.assign(workForm, { enabled:wp.enabled, workStartTime:wp.workStartTime, workEndTime:wp.workEndTime, lunchBreakStartTime:wp.lunchBreakStartTime, lunchBreakEndTime:wp.lunchBreakEndTime, replyMode:wp.replyMode, delayedReplyEnabled:wp.delayedReplyEnabled })
  } catch {}
  try { specialEvents.value = await getSpecialEvents() } catch {}
}

async function saveRole() { savingRole.value=true; try { await updateRoleProfile(roleForm); ElMessage.success("已保存") } catch { ElMessage.error("失败") } finally { savingRole.value=false } }
async function saveLifestyle() { savingLifestyle.value=true; try { await updateLifestyleTendency(lifestyleForm as any); ElMessage.success("已保存") } catch { ElMessage.error("失败") } finally { savingLifestyle.value=false } }
async function resetLifestyle() { resettingLifestyle.value=true; try { const d = await resetLifestyleTendency(); Object.keys(lifestyleForm).forEach(k=>{if((d as any)[k]!==undefined)lifestyleForm[k]=(d as any)[k]}); ElMessage.success("已恢复") } catch { ElMessage.error("失败") } finally { resettingLifestyle.value=false } }
async function saveSleep() { savingSleep.value=true; try { await updateSleepSetting(sleepForm); ElMessage.success("已保存") } catch { ElMessage.error("失败") } finally { savingSleep.value=false } }
async function saveActiveMsg() { savingActiveMsg.value=true; try { await put("/api/companion/active-message/setting", activeMsgForm); ElMessage.success("已保存") } catch { ElMessage.error("失败") } finally { savingActiveMsg.value=false } }
async function saveWork() { savingWork.value=true; try { await updateWorkProfile(workForm); ElMessage.success("已保存") } catch { ElMessage.error("失败") } finally { savingWork.value=false } }

async function saveLifeIdentity() {
  savingLifeIdentity.value = true
  try {
    await request.put("/api/characters/" + (charId || ""), { lifeIdentity: lifeIdentity.value })
    ElMessage.success("已保存")
  } catch { ElMessage.error("保存失败") }
  finally { savingLifeIdentity.value = false }
}

async function regenerateAll() {
  regenerating.value=true
  try {
    await post("/api/companion/schedule/regenerate", { date: new Date().toISOString().slice(0,10) })
    ElMessage.success("已重新生成")
    await loadAll()
  } catch { ElMessage.error("重新生成失败") }
  finally { regenerating.value=false }
}

async function addCourse() { try { await createFixedEvent({ title:"新课程", eventType:"CLASS", startTime:"08:00", endTime:"09:40", repeatDays:"MON" }); await loadAll() } catch(e:any) { ElMessage.error(e?.message||"失败") } }
async function editCourseItem(c:any) { editingCourse.value = c; courseEditForm.title = c.title; courseEditForm.eventType = c.eventType || 'CLASS'; courseEditForm.startTime = c.startTime || '08:00'; courseEditForm.endTime = c.endTime || '09:40'; courseEditForm.repeatDays = c.repeatDays ? c.repeatDays.split(',').filter((d:string) => d) : ['MON']; courseEditForm.replyMode = c.replyMode || 'NO_REPLY'; showCourseEdit.value = true }
async function delCourse(id:number) { try { await deleteFixedEvent(id); courses.value=courses.value.filter(c=>c.id!==id) } catch { ElMessage.error("失败") } }
async function toggleCourse(id:number, v:boolean) { try { await updateFixedEvent(id,{enabled:v}) } catch {} }
async function addSpecial() { try { await createSpecialEvent({ title:"新状态", eventType:"CUSTOM" }); await loadAll() } catch(e:any) { ElMessage.error(e?.message||"失败") } }
async function editSpecialItem(se:any) { editingSpecial.value = se; specialEditForm.title = se.title; specialEditForm.eventType = se.eventType || 'CUSTOM'; specialEditForm.startDate = se.startDate || ''; specialEditForm.endDate = se.endDate || ''; specialEditForm.replyMode = se.replyMode || 'NO_REPLY'; showSpecialEdit.value = true }
async function delSpecial(id:number) { try { await deleteSpecialEvent(id); specialEvents.value=specialEvents.value.filter(s=>s.id!==id) } catch { ElMessage.error("失败") } }
async function toggleSpecial(id:number, v:boolean) { try { await updateSpecialEvent(id,{enabled:v}) } catch {} }


async function saveCourseEdit() {
  savingCourseEdit.value = true
  try {
    const data: any = { ...courseEditForm, repeatDays: courseEditForm.repeatDays.join(',') }
    if (editingCourse.value) {
      await updateFixedEvent(editingCourse.value.id, data)
    } else {
      await createFixedEvent(data)
    }
    ElMessage.success(editingCourse.value ? '已更新' : '已添加')
    showCourseEdit.value = false
    editingCourse.value = null
    await loadAll()
  } catch (e: any) { ElMessage.error(e?.message || '保存失败') }
  finally { savingCourseEdit.value = false }
}

async function saveSpecialEdit() {
  savingSpecialEdit.value = true
  try {
    const data: any = { ...specialEditForm }
    if (editingSpecial.value) {
      await updateSpecialEvent(editingSpecial.value.id, data)
    } else {
      await createSpecialEvent(data)
    }
    ElMessage.success(editingSpecial.value ? '已更新' : '已添加')
    showSpecialEdit.value = false
    editingSpecial.value = null
    await loadAll()
  } catch (e: any) { ElMessage.error(e?.message || '保存失败') }
  finally { savingSpecialEdit.value = false }
}onMounted(loadAll)
</script>

<style scoped>
.life-rules-page { padding: 20px 24px; max-width: 860px; margin: 0 auto; }
.page-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 16px; }
.page-header h2 { font-size: 18px; font-weight: 600; margin: 0; }
.page-desc { font-size: 13px; color: var(--ac-color-text-muted); margin: 4px 0 0; }

.cards { display: flex; flex-direction: column; gap: 10px; }
.card { background: var(--ac-color-surface); border: 1px solid var(--ac-color-border-light); border-radius: var(--ac-radius-md); overflow: hidden; }
.card-header { display: flex; align-items: center; gap: 8px; padding: 12px 16px; cursor: pointer; user-select: none; font-size: 14px; font-weight: 600; }
.card-header:hover { background: var(--ac-color-bg-hover); }
.card-title { flex: 1; }
.card-body { padding: 0 16px 14px; }

.form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px 16px; margin-bottom: 10px; }
.form-item { display: flex; flex-direction: column; gap: 3px; }
.form-item label { font-size: 12px; color: var(--ac-color-text-secondary); }

.slider-grid { display: flex; flex-direction: column; gap: 8px; margin-bottom: 10px; }
.slider-row { display: flex; flex-direction: column; gap: 2px; }
.sr-header { display: flex; justify-content: space-between; }
.sr-label { font-size: 12px; font-weight: 500; }
.sr-value { font-size: 11px; font-weight: 700; color: var(--ac-color-primary); }
.sr-body { display: flex; align-items: center; gap: 6px; }
.sr-left, .sr-right { font-size: 10px; color: var(--ac-color-text-placeholder); min-width: 28px; }
.sr-left { text-align: right; }
.sr-slider-wrap { flex: 1; }
.sr-hint { font-size: 10px; color: var(--ac-color-text-muted); }

.schedule-display { display: grid; grid-template-columns: 1fr 1fr; gap: 6px 20px; }
.sched-row { display: flex; justify-content: space-between; font-size: 13px; padding: 4px 0; border-bottom: 1px solid var(--ac-color-border-light); }
.sched-row span:first-child { color: var(--ac-color-text-secondary); }
.sched-row span:last-child { font-weight: 500; }

.task-row, .course-row, .tl-row { display: flex; align-items: center; gap: 10px; padding: 6px 0; border-bottom: 1px solid var(--ac-color-border-light); font-size: 12px; flex-wrap: wrap; }
.task-type { font-weight: 500; min-width: 60px; }
.task-time, .c-time, .tl-time { color: var(--ac-color-text-secondary); min-width: 100px; }
.c-days { color: var(--ac-color-text-muted); font-size: 11px; }
.tl-reason { color: var(--ac-color-text-muted); font-size: 11px; }

.course-row { justify-content: flex-start; }
.course-row > :last-child { margin-left: auto; }

.btn-row { display: flex; gap: 8px; }
.sub-title { font-size: 13px; font-weight: 600; margin: 8px 0 4px; }
.empty-hint { font-size: 12px; color: var(--ac-color-text-muted); padding: 8px 0; }

@media (max-width: 600px) {
  .form-grid { grid-template-columns: 1fr; }
  .schedule-display { grid-template-columns: 1fr; }
}
</style>