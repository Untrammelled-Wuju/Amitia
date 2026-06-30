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
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue"
import { ElMessage, ElMessageBox } from "element-plus"
import { useApi } from "../../composables/useApi"

const { get, put, del } = useApi()

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

onMounted(() => { loadSettings(); fetchEvents() })
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

@media (max-width:640px) { .rules-grid { grid-template-columns:1fr; } }
</style>
