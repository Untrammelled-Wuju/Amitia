<template>
  <div class="runtime-mode-page">
    <h2 class="page-title">
      <el-icon><Monitor /></el-icon>
      运行模式
    </h2>

    <div class="mode-hero" :class="modeClass">
      <div class="mode-hero-icon">
        <el-icon v-if="mode.deployMode === 'desktop-local'" :size="32"><Monitor /></el-icon>
        <el-icon v-else :size="32"><Cloudy /></el-icon>
      </div>
      <div class="mode-hero-body">
        <div class="mode-hero-label">{{ modeLabel }}</div>
        <div class="mode-hero-desc">{{ modeDescription }}</div>
        <div class="mode-hero-addr">
          <el-tag size="small" effect="plain" type="info">{{ mode.host }}:{{ mode.port }}</el-tag>
          <el-tag
            v-if="mode.web.enabled"
            size="small"
            :type="mode.web.requireAuth ? 'warning' : 'success'"
            effect="plain"
            style="margin-left:6px"
          >
            {{ mode.web.requireAuth ? '需要登录' : '可选登录' }}
          </el-tag>
        </div>
      </div>
    </div>

    <!-- Switch Mode -->
    <el-card shadow="never" class="section-card">
      <template #header>
        <span class="section-title">切换模式</span>
      </template>

      <div class="mode-switch-row">
        <div
          class="mode-option"
          :class="{ active: mode.deployMode === 'desktop-local' }"
          @click="selectMode('desktop-local')"
        >
          <div class="mo-icon"><el-icon :size="24"><Monitor /></el-icon></div>
          <div class="mo-label">桌面本地模式</div>
          <div class="mo-desc">Core 在本机运行，可免登录</div>
          <div class="mo-tag-row">
            <el-tag size="small" effect="plain" type="success">本机 127.0.0.1</el-tag>
          </div>
        </div>

        <div class="mode-arrow">
          <el-icon v-if="mode.deployMode === 'desktop-local'" :size="20"><Right /></el-icon>
          <div v-else class="mode-arrow-label">当前</div>
        </div>

        <div
          class="mode-option"
          :class="{ active: mode.deployMode === 'cloud-web' }"
          @click="selectMode('cloud-web')"
        >
          <div class="mo-icon"><el-icon :size="24"><Cloudy /></el-icon></div>
          <div class="mo-label">私有云模式</div>
          <div class="mo-desc">Core 部署在云服务器，需登录</div>
          <div class="mo-tag-row">
            <el-tag size="small" effect="plain" type="warning">HTTPS 访问</el-tag>
          </div>
        </div>
      </div>

      <!-- Impact description -->
      <div v-if="selectedMode && selectedMode !== mode.deployMode" class="impact-box">
        <div class="impact-box-header">
          <el-icon><Warning /></el-icon>
          <span>模式切换的影响</span>
        </div>
        <ul class="impact-list">
          <template v-if="selectedMode === 'cloud-web'">
            <li>Core 将在你的云服务器上运行</li>
            <li>Web UI 通过 HTTPS 访问</li>
            <li><strong>登录变为必需</strong>（系统自动开启）</li>
            <li>微信桥 Bridge 在同一云服务器或内网运行</li>
            <li>你的个人电脑<strong>不需要常开</strong></li>
            <li>需要配置 publicBaseUrl 指向你的域名</li>
          </template>
          <template v-else>
            <li>Core 将在本机运行（127.0.0.1）</li>
            <li>登录可选择关闭（免登录模式）</li>
            <li>微信桥 Bridge 在本机启动</li>
            <li>你的电脑需要<strong>保持开机</strong></li>
            <li>仅限本机访问，不暴露到网络</li>
          </template>
        </ul>
        <div class="impact-actions">
          <el-button type="primary" :loading="switching" @click="confirmSwitch">
            确认切换到{{ selectedMode === 'cloud-web' ? '私有云模式' : '桌面本地模式' }}
          </el-button>
          <el-button @click="selectedMode = null">取消</el-button>
        </div>
      </div>
    </el-card>

    <!-- Current Config Details -->
    <el-card shadow="never" class="section-card">
      <template #header>
        <div class="section-header-row">
          <span class="section-title">当前配置</span>
          <el-button size="small" :loading="validating" @click="runValidate">
            <el-icon><Checked /></el-icon>
            校验配置
          </el-button>
        </div>
      </template>

      <el-descriptions :column="2" border size="small">
        <el-descriptions-item label="运行模式">
          <el-tag :type="mode.deployMode === 'cloud-web' ? 'warning' : 'success'" size="small">
            {{ mode.deployMode === 'cloud-web' ? '私有云模式' : '桌面本地模式' }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="监听地址">{{ mode.host }}:{{ mode.port }}</el-descriptions-item>
        <el-descriptions-item label="Web 界面">
          <el-tag :type="mode.web.enabled ? 'success' : 'info'" size="small">
            {{ mode.web.enabled ? '已启用' : '已禁用' }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="登录要求">
          <el-tag :type="mode.web.requireAuth ? 'warning' : 'success'" size="small">
            {{ mode.web.requireAuth ? '必需' : '可选' }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="Bridge 模式">
          <el-tag :type="mode.bridge.mode === 'cloud' ? 'warning' : 'success'" size="small">
            {{ mode.bridge.mode === 'cloud' ? '云端' : '本地' }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="Bridge 地址">
          {{ mode.bridge.enabled ? mode.bridge.host + ':' + mode.bridge.port : '已禁用' }}
        </el-descriptions-item>
        <el-descriptions-item v-if="mode.deployMode === 'cloud-web'" label="公开地址">
          {{ mode.web.publicBaseUrl || '未配置' }}
        </el-descriptions-item>
        <el-descriptions-item label="数据目录">{{ mode.storage.dataDir }}</el-descriptions-item>
      </el-descriptions>

      <!-- Validation Results -->
      <div v-if="validationResult" class="validation-result" :class="validationResult.valid ? 'vr-ok' : 'vr-error'">
        <div class="vr-header">
          <el-icon v-if="validationResult.valid"><CircleCheck /></el-icon>
          <el-icon v-else><CircleClose /></el-icon>
          <span>{{ validationResult.valid ? '配置校验通过' : '配置存在问题' }}</span>
        </div>
        <div class="vr-checks">
          <div
            v-for="check in validationResult.checks"
            :key="check.name"
            class="vr-check-item"
            :class="'vr-' + check.level"
          >
            <span class="vr-check-icon">
              <el-icon v-if="check.passed"><CircleCheck /></el-icon>
              <el-icon v-else-if="check.level === 'error'"><CircleClose /></el-icon>
              <el-icon v-else><Warning /></el-icon>
            </span>
            <div class="vr-check-body">
              <div class="vr-check-name">{{ check.name }}</div>
              <div class="vr-check-msg">{{ check.message }}</div>
              <div v-if="check.suggestion" class="vr-check-suggestion">{{ check.suggestion }}</div>
            </div>
          </div>
        </div>
      </div>
    </el-card>

    <!-- Cloud-web deployment checklist -->
    <el-card v-if="mode.deployMode === 'cloud-web'" shadow="never" class="section-card">
      <template #header>
        <span class="section-title">
          <el-icon><List /></el-icon>
          云端部署检查项
        </span>
      </template>
      <el-checkbox-group v-model="cloudChecklist" class="checklist-group">
        <el-checkbox label="1" disabled>Core 绑定 0.0.0.0 允许外部访问</el-checkbox>
        <el-checkbox label="2" disabled>配置 HTTPS 反向代理（nginx/Caddy）</el-checkbox>
        <el-checkbox label="3" :value="!!mode.web.publicBaseUrl">
          设置 publicBaseUrl 指向 HTTPS 域名
        </el-checkbox>
        <el-checkbox label="4" :value="mode.bridge.mode === 'cloud'">
          Bridge 在同一云服务器运行（cloud 模式）
        </el-checkbox>
        <el-checkbox label="5" disabled>防火墙开放配置端口</el-checkbox>
        <el-checkbox label="6" :value="mode.web.requireAuth">
          登录验证已启用
        </el-checkbox>
        <el-checkbox label="7" disabled>定期备份数据库</el-checkbox>
      </el-checkbox-group>
    </el-card>

    <!-- Desktop-local details -->
    <el-card v-if="mode.deployMode === 'desktop-local'" shadow="never" class="section-card">
      <template #header>
        <span class="section-title">
          <el-icon><InfoFilled /></el-icon>
          本机运行详情
        </span>
      </template>
      <el-descriptions :column="1" border size="small">
        <el-descriptions-item label="Core 端口">
          <code>{{ mode.host }}:{{ mode.port }}</code>
          <span class="form-tip">仅本机可访问（127.0.0.1）</span>
        </el-descriptions-item>
        <el-descriptions-item label="Bridge 端口">
          <code>{{ mode.bridge.host }}:{{ mode.bridge.port }}</code>
          <span class="form-tip">微信桥在同一台机器上运行</span>
        </el-descriptions-item>
        <el-descriptions-item label="数据位置">
          <code>{{ mode.storage.dataDir }}</code>
          <span class="form-tip">所有数据存储在本地</span>
        </el-descriptions-item>
        <el-descriptions-item label="外部访问">
          <el-tag size="small" type="info">未暴露</el-tag>
          <span class="form-tip">不监听外部网络接口</span>
        </el-descriptions-item>
        <el-descriptions-item label="开机要求">
          <el-tag size="small" type="warning">需要常开</el-tag>
          <span class="form-tip">电脑需要保持开机才能运行</span>
        </el-descriptions-item>
      </el-descriptions>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from "vue"
import { ElMessage, ElMessageBox } from "element-plus"
import {
  Monitor, Cloudy, Right, Warning, Checked,
  CircleCheck, CircleClose, InfoFilled, List,
} from "@element-plus/icons-vue"
import { apiClient } from "../../composables/useApi"
import type {
  RuntimeModeResponse,
  RuntimeModeValidationResult,
  DeployMode,
} from "@/types"

const mode = reactive<RuntimeModeResponse>({
  deployMode: "desktop-local",
  host: "127.0.0.1",
  port: 8899,
  web: { enabled: true, publicBaseUrl: "", requireAuth: true },
  bridge: { enabled: true, mode: "cloud", host: "127.0.0.1", port: 8898 },
  storage: { dataDir: "./data" },
})

const selectedMode = ref<DeployMode | null>(null)
const switching = ref(false)
const validating = ref(false)
const validationResult = ref<RuntimeModeValidationResult | null>(null)
const cloudChecklist = ref<string[]>([])

const modeClass = computed(() => ({
  "mode-desktop": mode.deployMode === "desktop-local",
  "mode-cloud": mode.deployMode === "cloud-web",
}))

const modeLabel = computed(() =>
  mode.deployMode === "desktop-local" ? "桌面本地模式" : "私有云模式"
)

const modeDescription = computed(() =>
  mode.deployMode === "desktop-local"
    ? "Core 运行在你的电脑上，通过 127.0.0.1 访问，支持免登录使用"
    : "Core 部署在你的云服务器上，通过 HTTPS 远程访问，需要登录。你的电脑不需要常开"
)

onMounted(async () => {
  await fetchMode()
})

async function fetchMode() {
  try {
    const res = await apiClient.get("/api/runtime/mode")
    const data = res.data as any
    if (data) {
      Object.assign(mode, {
        deployMode: data.deployMode,
        host: data.host,
        port: data.port,
        web: data.web,
        bridge: data.bridge,
        storage: data.storage,
      })
    }
  } catch {
    // Silently fallback to defaults
  }
}

function selectMode(m: DeployMode) {
  if (m === mode.deployMode) {
    selectedMode.value = null
    return
  }
  selectedMode.value = m
}

async function confirmSwitch() {
  if (!selectedMode.value) return

  switching.value = true
  try {
    await apiClient.put("/api/runtime/mode", {
      deployMode: selectedMode.value,
    })

    await fetchMode()
    selectedMode.value = null
    ElMessage.success(`已切换到${mode.deployMode === 'cloud-web' ? '私有云模式' : '桌面本地模式'}。建议重启 Core 使配置生效。`)
  } catch (err: any) {
    ElMessage.error("切换失败: " + (err.response?.data?.message || err.message))
  } finally {
    switching.value = false
  }
}

async function runValidate() {
  validating.value = true
  try {
    const res = await apiClient.post("/api/runtime/mode/validate")
    const data = res.data as any
    validationResult.value = data

    if (data.valid) {
      ElMessage.success("配置校验通过")
    } else if (data.errors?.length) {
      ElMessage.warning(`发现 ${data.errors.length} 个错误`)
    } else if (data.warnings?.length) {
      ElMessage.info(`发现 ${data.warnings.length} 个警告`)
    }
  } catch (err: any) {
    ElMessage.error("校验失败: " + (err.response?.data?.message || err.message))
  } finally {
    validating.value = false
  }
}
</script>

<style scoped>
.runtime-mode-page {
  padding: 20px;
  max-width: 780px;
}
.page-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--ac-color-text);
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;
}

/* Hero banner */
.mode-hero {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 20px 24px;
  border-radius: 10px;
  margin-bottom: 16px;
  border: 1px solid var(--ac-color-border-light);
}
.mode-desktop {
  background: linear-gradient(135deg, #f0f9f2 0%, #e8f5e9 100%);
  border-color: #b7e4c7;
}
.mode-cloud {
  background: linear-gradient(135deg, #e8f4fd 0%, #dbeafe 100%);
  border-color: #93c5fd;
}
.mode-hero-icon {
  width: 56px;
  height: 56px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 12px;
  background: rgba(255,255,255,0.7);
  flex-shrink: 0;
}
.mode-desktop .mode-hero-icon { color: #5a9e6f; }
.mode-cloud .mode-hero-icon { color: #3b82f6; }
.mode-hero-body { flex: 1; min-width: 0; }
.mode-hero-label { font-size: 16px; font-weight: 700; color: var(--ac-color-text); }
.mode-hero-desc { font-size: 13px; color: var(--ac-color-text-secondary); margin-top: 4px; line-height: 1.5; }
.mode-hero-addr { margin-top: 8px; display: flex; align-items: center; gap: 6px; }

/* Section cards */
.section-card {
  margin-bottom: 16px;
}
.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--ac-color-text);
  display: flex;
  align-items: center;
  gap: 6px;
}
.section-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

/* Mode switch row */
.mode-switch-row {
  display: flex;
  align-items: center;
  gap: 20px;
  margin-bottom: 12px;
}
.mode-option {
  flex: 1;
  padding: 16px;
  border: 2px solid var(--ac-color-border);
  border-radius: 10px;
  text-align: center;
  cursor: pointer;
  transition: all 0.2s;
  background: var(--ac-color-surface);
}
.mode-option:hover {
  border-color: var(--ac-color-primary);
}
.mode-option.active {
  border-color: var(--ac-color-primary);
  background: var(--ac-color-primary-bg, #f0f8f2);
}
.mo-icon { margin-bottom: 8px; color: var(--ac-color-text-secondary); }
.mode-option.active .mo-icon { color: var(--ac-color-primary); }
.mo-label { font-size: 14px; font-weight: 600; color: var(--ac-color-text); }
.mo-desc { font-size: 12px; color: var(--ac-color-text-secondary); margin-top: 4px; }
.mo-tag-row { margin-top: 8px; }
.mode-arrow {
  display: flex;
  align-items: center;
  color: var(--ac-color-text-muted);
  flex-shrink: 0;
}
.mode-arrow-label {
  font-size: 12px;
  color: var(--ac-color-text-muted);
  background: var(--ac-color-bg-secondary);
  padding: 2px 8px;
  border-radius: 4px;
}

/* Impact box */
.impact-box {
  padding: 16px;
  border: 1px solid #fde68a;
  border-radius: 8px;
  background: #fffbeb;
  margin-top: 12px;
}
.impact-box-header {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  font-weight: 600;
  color: #92400e;
  margin-bottom: 10px;
}
.impact-list {
  margin: 0;
  padding-left: 20px;
  font-size: 13px;
  color: #78350f;
  line-height: 1.8;
}
.impact-list li { margin-bottom: 2px; }
.impact-actions {
  margin-top: 12px;
  display: flex;
  gap: 8px;
}

/* Validation results */
.validation-result {
  margin-top: 16px;
  padding: 12px 16px;
  border-radius: 8px;
}
.vr-ok { background: #f0fdf4; border: 1px solid #bbf7d0; }
.vr-error { background: #fef2f2; border: 1px solid #fecaca; }
.vr-header {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  font-weight: 600;
  margin-bottom: 10px;
}
.vr-ok .vr-header { color: #166534; }
.vr-error .vr-header { color: #991b1b; }
.vr-checks {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.vr-check-item {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 8px 10px;
  border-radius: 6px;
}
.vr-ok { background: #f0fdf4; }
.vr-warn { background: #fefce8; }
.vr-error { background: #fef2f2; }
.vr-check-icon { font-size: 16px; flex-shrink: 0; margin-top: 2px; }
.vr-ok .vr-check-icon { color: #16a34a; }
.vr-warn .vr-check-icon { color: #ca8a04; }
.vr-error .vr-check-icon { color: #dc2626; }
.vr-check-body { flex: 1; min-width: 0; }
.vr-check-name { font-size: 12px; font-weight: 600; color: var(--ac-color-text); }
.vr-check-msg { font-size: 12px; color: var(--ac-color-text-secondary); margin-top: 2px; }
.vr-check-suggestion {
  font-size: 11px;
  color: #92400e;
  margin-top: 4px;
  padding: 4px 8px;
  border-radius: 4px;
  background: #fef3c7;
  line-height: 1.4;
}

/* Checklist */
.checklist-group {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

/* Form tip */
.form-tip {
  display: block;
  font-size: 11px;
  color: var(--ac-color-text-muted);
  margin-top: 2px;
}
code {
  font-family: 'Consolas', 'Courier New', monospace;
  font-size: 12px;
  background: var(--ac-color-bg-secondary);
  padding: 1px 6px;
  border-radius: 3px;
}
</style>
