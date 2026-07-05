<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="safety-page">
    <h2 class="page-title">安全设置</h2>

    <!-- Toggle switches -->
    <el-card shadow="never" class="section-card">
      <template #header><span class="section-title">安全功能开关</span></template>
      <div class="toggle-list">
        <div class="toggle-item">
          <div class="ti-info">
            <div class="ti-label">启用安全卫士</div>
            <div class="ti-desc">自动检测用户输入和 AI 输出，拦截或重写不安全内容</div>
          </div>
          <el-switch v-model="safetyGuard" @change="saveAll" />
        </div>

        <div class="toggle-item">
          <div class="ti-info">
            <div class="ti-label">导入敏感内容检测</div>
            <div class="ti-desc">导入聊天记录时自动检测身份证、银行卡、密码等敏感数据，给出警告</div>
          </div>
          <el-switch v-model="importDetection" @change="saveAll" />
        </div>

        <div class="toggle-item">
          <div class="ti-info">
            <div class="ti-label">允许云端模型处理导入摘要</div>
            <div class="ti-desc">开启后，导入的聊天记录文本将发送到模型服务商进行摘要生成。关闭可保护隐私</div>
          </div>
          <el-switch v-model="allowCloudSummary" @change="saveAll" />
        </div>

        <div class="toggle-item">
          <div class="ti-info">
            <div class="ti-label">AI 身份边界提示</div>
            <div class="ti-desc">在聊天页面显示 AI 身份边界提示，提醒用户 AI 不是真人</div>
          </div>
          <el-switch v-model="showIdentityHint" @change="saveAll" />
        </div>

        <div class="toggle-item">
          <div class="ti-info">
            <div class="ti-label">Web 公网访问提醒</div>
            <div class="ti-desc">在私有云模式下，提醒用户配置访问控制（如 VPN、白名单、防火墙规则）</div>
          </div>
          <el-switch v-model="showWebWarning" @change="saveAll" />
        </div>
      </div>
    </el-card>

    <!-- Security rules summary -->
    <el-card shadow="never" class="section-card">
      <template #header><span class="section-title">安全规则概览</span></template>
      <el-alert type="info" :closable="false" show-icon style="margin-bottom:10px">
        以下规则在 Safety Guard 启用时自动生效
      </el-alert>
      <div class="rules-grid">
        <div v-for="r in rules" :key="r.label" class="rule-card">
          <div class="rule-label">{{ r.label }}</div>
          <div class="rule-action">{{ r.action }}</div>
        </div>
      </div>
    </el-card>

    <el-card shadow="never" class="section-card">
      <template #header><span class="section-title">BDI 硬约束过滤规则</span></template>
      <el-alert type="info" :closable="false" show-icon style="margin-bottom:10px">
        安全卫士启用时，以下硬约束将阻止或覆盖不符合安全边界的候选行为
      </el-alert>
      <el-table :data="hardConstraints" stripe size="small" empty-text="暂无约束规则">
        <el-table-column prop="ruleId" label="规则 ID" width="140" show-overflow-tooltip />
        <el-table-column prop="candidateKey" label="候选行为" width="140" show-overflow-tooltip />
        <el-table-column prop="reason" label="拦截原因" show-overflow-tooltip />
        <el-table-column label="严重程度" width="100">
          <template #default="{row}">
            <el-tag :type="row.severity==='block'?'danger':'warning'" size="small">
              {{ row.severity==='block'?'阻止':'覆盖' }}
            </el-tag>
          </template>
        </el-table-column>
      </el-table>
      <div style="margin-top:10px;display:flex;gap:8px">
        <el-button size="small" @click="addHardConstraint">添加规则</el-button>
        <el-button size="small" @click="fetchBdiConfig">刷新</el-button>
      </div>
    </el-card>

    <el-card shadow="never" class="section-card">
      <template #header><span class="section-title">BDI 软偏好加权</span></template>
      <el-alert type="info" :closable="false" show-icon style="margin-bottom:10px">
        软偏好维度加权影响候选行为的最终效用评分
      </el-alert>
      <el-table :data="softPreferences" stripe size="small" empty-text="暂无偏好维度">
        <el-table-column prop="dimension" label="维度" width="140" show-overflow-tooltip />
        <el-table-column label="原始分" width="100">
          <template #default="{row}">
            <el-input-number v-model="row.rawScore" :min="0" :max="10" size="small" controls-position="right" style="width:100px" />
          </template>
        </el-table-column>
        <el-table-column label="归一化权重" width="110">
          <template #default="{row}">
            <span style="font-size:13px">{{ row.normalizedWeight?.toFixed(2) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="贡献值" width="110">
          <template #default="{row}">
            <span style="font-size:13px">{{ row.contribution?.toFixed(2) }}</span>
          </template>
        </el-table-column>
      </el-table>
      <div style="margin-top:10px;display:flex;gap:8px">
        <el-button size="small" @click="addSoftPreference">添加维度</el-button>
        <el-button size="small" type="primary" @click="saveBdiConfig">保存偏好设置</el-button>
      </div>
    </el-card>

    <el-card shadow="never" class="section-card">
      <template #header><span class="section-title">应对策略选择</span></template>
      <el-alert type="info" :closable="false" show-icon style="margin-bottom:10px">
        当 AI 面对冲突或负面事件时，选择默认应对策略
      </el-alert>
      <div class="coping-section">
        <div class="coping-row">
          <span class="coping-label">当前策略</span>
          <el-select v-model="copingSelected" style="width:200px" size="small">
            <el-option label="主动解决问题" value="active" />
            <el-option label="寻求支持" value="support" />
            <el-option label="重新评估" value="reframe" />
            <el-option label="回避转移" value="avoid" />
            <el-option label="抑制情绪" value="suppress" />
            <el-option label="接受现状" value="accept" />
          </el-select>
        </div>
        <div class="coping-row">
          <span class="coping-label">备选策略</span>
          <el-select v-model="copingAlternatives" multiple style="width:300px" size="small" placeholder="选择备选策略">
            <el-option label="主动解决问题" value="active" />
            <el-option label="寻求支持" value="support" />
            <el-option label="重新评估" value="reframe" />
            <el-option label="回避转移" value="avoid" />
            <el-option label="抑制情绪" value="suppress" />
            <el-option label="接受现状" value="accept" />
          </el-select>
        </div>
        <div class="coping-row">
          <span class="coping-label">选择理由</span>
          <el-input v-model="copingReason" type="textarea" :rows="2" placeholder="描述当前策略的选择理由" maxlength="200" show-word-limit style="flex:1" />
        </div>
      </div>
      <div style="margin-top:10px">
        <el-button size="small" type="primary" @click="saveBdiConfig">保存应对策略</el-button>
      </div>
    </el-card>

    <el-card shadow="never" class="section-card">
      <template #header><span class="section-title">情绪表达控制</span></template>
      <el-alert type="info" :closable="false" show-icon style="margin-bottom:10px">
        控制 AI 在输出中如何显露、抑制或重构情绪表达
      </el-alert>
      <div class="emotion-section">
        <div class="emotion-row">
          <span class="emotion-label">表达模式</span>
          <el-radio-group v-model="emotionDisplayMode" size="small">
            <el-radio value="show">正常显露</el-radio>
            <el-radio value="suppress">抑制表达</el-radio>
            <el-radio value="reframe">重构表达</el-radio>
          </el-radio-group>
        </div>
        <div class="emotion-row">
          <span class="emotion-label">内部强度</span>
          <div class="emotion-slider-group">
            <el-slider v-model="emotionInternalIntensity" :min="0" :max="10" :step="0.5" style="width:200px" />
            <span class="slider-value">{{ emotionInternalIntensity }}</span>
          </div>
        </div>
        <div class="emotion-row">
          <span class="emotion-label">显示强度</span>
          <div class="emotion-slider-group">
            <el-slider v-model="emotionDisplayIntensity" :min="0" :max="10" :step="0.5" style="width:200px" />
            <span class="slider-value">{{ emotionDisplayIntensity }}</span>
          </div>
        </div>
        <div class="emotion-row">
          <span class="emotion-label">覆盖理由</span>
          <el-input v-model="emotionOverrideReason" type="textarea" :rows="2" placeholder="当表达模式非正常显露时，说明原因" maxlength="200" show-word-limit style="flex:1" />
        </div>
      </div>
      <div style="margin-top:10px">
        <el-button size="small" type="primary" @click="saveBdiConfig">保存情绪配置</el-button>
      </div>
    </el-card>

    <!-- Safety event log -->
    <el-card shadow="never" class="section-card">
      <template #header>
        <div class="card-header-row">
          <span class="section-title">安全事件日志</span>
          <el-button text size="small" @click="clearEvents" :disabled="evTotal===0">清除</el-button>
        </div>
      </template>
      <el-table :data="events" stripe size="small" max-height="360">
        <el-table-column prop="eventType" label="事件类型" width="140" show-overflow-tooltip />
        <el-table-column prop="description" label="描述" show-overflow-tooltip />
        <el-table-column prop="direction" label="方向" width="80" />
        <el-table-column label="处理" width="70">
          <template #default="{row}">
            <el-tag :type="row.handled?'success':'danger'" size="small">{{ row.handled?"已处理":"未处理" }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="时间" width="150">
          <template #default="{row}">{{ fmtDate(row.createdAt) }}</template>
        </el-table-column>
      </el-table>
      <el-pagination
        v-if="evTotal>20"
        v-model:current-page="evPage"
        :page-size="20"
        :total="evTotal"
        layout="prev,next"
        size="small"
        @current-change="fetchEvents"
        style="margin-top:10px;justify-content:center"
      />
    </el-card>

    <!-- Privacy note -->
    <el-card shadow="never" class="section-card">
      <template #header><span class="section-title">隐私保护</span></template>
      <ul class="privacy-list">
        <li>日志不记录完整 API Key</li>
        <li>日志不记录微信登录凭据</li>
        <li>备份默认不包含 API Key 明文</li>
        <li>聊天记录保存在你自己的设备或服务器</li>
      </ul>
    </el-card>

    <el-card shadow="never" class="section-card">
      <template #header><span class="section-title">隐私入口</span></template>
      <div class="privacy-grid">
        <div class="privacy-card" @click="goPrivacy">
          <el-icon size="22" color="var(--ac-color-primary)"><Lock /></el-icon>
          <div class="pc-body">
            <div class="pc-title">隐私说明</div>
            <div class="pc-desc">查看数据存储位置、模型 API 数据处理方式以及你的控制权说明</div>
          </div>
          <el-icon><ArrowRight /></el-icon>
        </div>
        <div class="privacy-card" @click="goBoundary">
          <el-icon size="22" color="var(--ac-color-primary)"><WarningFilled /></el-icon>
          <div class="pc-body">
            <div class="pc-title">使用边界</div>
            <div class="pc-desc">了解 AI 角色的能力限制、安全边界和理性使用建议</div>
          </div>
          <el-icon><ArrowRight /></el-icon>
        </div>
        <div class="privacy-card" @click="goPrivacyScan">
          <el-icon size="22" color="var(--ac-color-primary)"><Search /></el-icon>
          <div class="pc-body">
            <div class="pc-title">隐私扫描</div>
            <div class="pc-desc">扫描消息记录中的敏感信息，识别潜在隐私泄露风险</div>
          </div>
          <el-icon><ArrowRight /></el-icon>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from "vue"
import { useRouter } from "vue-router"
import { ElMessage, ElMessageBox } from "element-plus"
import { Lock, WarningFilled, Search, ArrowRight } from "@element-plus/icons-vue"
import { useApi } from "../../composables/useApi"

const { get, put, del } = useApi()
const router = useRouter()

const safetyGuard = ref(true)
const importDetection = ref(true)
const allowCloudSummary = ref(false)
const showIdentityHint = ref(true)
const showWebWarning = ref(true)

const rules = [
  { label:"冒充真人", action:"温和说明 AI 身份" },
  { label:"真实恋人", action:"说明不是真实恋人" },
  { label:"依赖风险", action:"软性重定向" },
  { label:"私隐索取", action:"拒绝并拦截" },
  { label:"危险内容", action:"拦截或重写" },
  { label:"代替回复好友", action:"拒绝并说明" },
]

const events = ref<any[]>([])
const evPage = ref(1)
const evTotal = ref(0)
const isBdiEnabled = computed(() => safetyGuard.value)

const hardConstraints = ref<Array<{ruleId:string;candidateKey:string;reason:string;severity:'block'|'override'}>>([])
const softPreferences = ref<Array<{dimension:string;rawScore:number;normalizedWeight:number;contribution:number}>>([])
const copingSelected = ref("active")
const copingAlternatives = ref<string[]>([])
const copingReason = ref("")
const emotionDisplayMode = ref<"show"|"suppress"|"reframe">("show")
const emotionInternalIntensity = ref(5)
const emotionDisplayIntensity = ref(5)
const emotionOverrideReason = ref("")

function addHardConstraint() {
  hardConstraints.value.push({ ruleId:"", candidateKey:"", reason:"", severity:"block" })
}

function addSoftPreference() {
  softPreferences.value.push({ dimension:"", rawScore:5, normalizedWeight:0, contribution:0 })
}

async function fetchBdiConfig() {
  try {
    const r = await get<any>("/api/safety/bdi-config")
    if (r?.hardConstraints) hardConstraints.value = r.hardConstraints
    if (r?.softPreferences) softPreferences.value = r.softPreferences
    if (r?.copingStrategy) {
      copingSelected.value = r.copingStrategy.selected || "active"
      copingAlternatives.value = r.copingStrategy.alternatives || []
      copingReason.value = r.copingStrategy.selectionReason || ""
    }
    if (r?.emotionExpression) {
      emotionDisplayMode.value = r.emotionExpression.displayMode || "show"
      emotionInternalIntensity.value = r.emotionExpression.internalIntensity ?? 5
      emotionDisplayIntensity.value = r.emotionExpression.displayIntensity ?? 5
      emotionOverrideReason.value = r.emotionExpression.overrideReason || ""
    }
  } catch {}
}

async function saveBdiConfig() {
  try {
    await put("/api/safety/bdi-config", {
      hardConstraints: hardConstraints.value,
      softPreferences: softPreferences.value.map(sp => ({ ...sp, rawScore: Number(sp.rawScore) })),
      copingStrategy: {
        selected: copingSelected.value,
        alternatives: copingAlternatives.value,
        selectionReason: copingReason.value,
      },
      emotionExpression: {
        displayMode: emotionDisplayMode.value,
        internalIntensity: Number(emotionInternalIntensity.value),
        displayIntensity: Number(emotionDisplayIntensity.value),
        overrideReason: emotionOverrideReason.value,
      },
    })
    ElMessage.success("BDI 配置已保存")
  } catch {}
}


async function loadSettings() {
  try {
    const s = await get<any>("/api/config/settings")
    if (s?.enable_safety_guard) safetyGuard.value = s.enable_safety_guard === "true"
    if (s?.enable_import_detection) importDetection.value = s.enable_import_detection !== "false"
    if (s?.allow_cloud_summary) allowCloudSummary.value = s.allow_cloud_summary === "true"
    if (s?.show_identity_hint) showIdentityHint.value = s.show_identity_hint !== "false"
    if (s?.show_web_warning) showWebWarning.value = s.show_web_warning !== "false"
  } catch {}
}

async function saveAll() {
  try {
    await put("/api/config", { settings: {
      enable_safety_guard: String(safetyGuard.value),
      enable_import_detection: String(importDetection.value),
      allow_cloud_summary: String(allowCloudSummary.value),
      show_identity_hint: String(showIdentityHint.value),
      show_web_warning: String(showWebWarning.value),
    }})
    ElMessage.success("保存成功")
  } catch {}
}

async function fetchEvents() {
  try {
    const r = await get<any>("/api/safety/events", { page: evPage.value, pageSize: 20 })
    events.value = r?.items || []
    evTotal.value = r?.total || 0
  } catch {}
}

async function clearEvents() {
  await ElMessageBox.confirm("确定清除所有安全事件日志？","提示",{type:"warning"})
  try { await del("/api/safety/events"); ElMessage.success("已清除"); fetchEvents() } catch {}
}

function fmtDate(d: string) { if(!d)return""; try{return new Date(d).toLocaleString("zh-CN")}catch{return d} }

function goPrivacy() { router.push('/privacy') }
function goBoundary() { router.push('/usage-boundary') }
function goPrivacyScan() { router.push('/privacy-scan') }

onMounted(() => { loadSettings(); fetchBdiConfig(); fetchEvents() })
</script>

<style scoped>
.safety-page { }
.page-title { font-size:var(--ac-font-size-lg); font-weight:600; margin-bottom:14px; }
.section-card { margin-bottom:12px; }
.section-title { font-weight:600; font-size:var(--ac-font-size-sm); }
.card-header-row { display:flex; align-items:center; justify-content:space-between; }

.toggle-list { display:flex; flex-direction:column; gap:10px; }
.toggle-item { display:flex; align-items:flex-start; justify-content:space-between; gap:16px; padding:10px 0; border-bottom:1px solid var(--ac-color-border-light); }
.toggle-item:last-child { border-bottom:none; }
.ti-info { flex:1; }
.ti-label { font-size:var(--ac-font-size-sm); font-weight:500; margin-bottom:2px; }
.ti-desc { font-size:var(--ac-font-size-xs); color:var(--ac-color-text-muted); line-height:1.4; }

.rules-grid { display:grid; grid-template-columns:1fr 1fr; gap:8px; }
.rule-card { padding:10px; border-radius:var(--ac-radius-sm); background:var(--ac-color-bg-secondary); }
.rule-label { font-size:var(--ac-font-size-sm); font-weight:500; }
.rule-action { font-size:var(--ac-font-size-xs); color:var(--ac-color-text-muted); margin-top:2px; }

.privacy-list { font-size:var(--ac-font-size-sm); color:var(--ac-color-text-secondary); padding-left:18px; line-height:1.8; }
.privacy-grid { display: flex; flex-direction: column; gap: 8px; }
.privacy-card { display: flex; align-items: center; gap: 12px; padding: 12px 14px; border: 1px solid var(--ac-color-border-light); border-radius: var(--ac-radius-md); cursor: pointer; transition: all 0.15s; background: var(--ac-color-surface); }
.privacy-card:hover { border-color: var(--ac-color-primary); background: var(--ac-color-surface-hover); }
.privacy-card .pc-body { flex: 1; min-width: 0; }
.privacy-card .pc-title { font-size: 14px; font-weight: 600; color: var(--ac-color-text); margin-bottom: 2px; }
.privacy-card .pc-desc { font-size: 12px; color: var(--ac-color-text-muted); line-height: 1.4; }

.coping-section { display:flex; flex-direction:column; gap:12px; }
.coping-row { display:flex; align-items:flex-start; gap:12px; }
.coping-label { font-size:var(--ac-font-size-sm); font-weight:500; min-width:80px; padding-top:4px; }

.emotion-section { display:flex; flex-direction:column; gap:12px; }
.emotion-row { display:flex; align-items:center; gap:12px; }
.emotion-label { font-size:var(--ac-font-size-sm); font-weight:500; min-width:80px; }
.emotion-slider-group { display:flex; align-items:center; gap:8px; }
.slider-value { font-size:var(--ac-font-size-sm); color:var(--ac-color-text-secondary); min-width:24px; text-align:right; }

@media (max-width:640px) { .rules-grid { grid-template-columns:1fr; } }
</style>
