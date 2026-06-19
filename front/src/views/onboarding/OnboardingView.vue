<template>
  <div class="onboarding-shell">
    <div class="onb-container">
      <header class="onb-header">
        <h1 class="onb-brand">AI Companion</h1>
        <p class="onb-sub">本地部署的 AI 虚拟陪伴智能体，守护你的每一次对话</p>
      </header>

      <div class="onb-steps">
        <div
          v-for="(s, idx) in steps"
          :key="idx"
          class="step-dot"
          :class="{ active: idx === current, done: idx < current }"
          @click="idx < current ? current = idx : null"
        >
          <span class="dot-icon">{{ idx < current ? '\u2713' : (idx + 1) }}</span>
          <span class="dot-label">{{ s.label }}</span>
        </div>
      </div>

      <div class="onb-content">
        <!-- Step 0: Welcome -->
        <div v-if="current === 0" class="step-panel">
          <h2>欢迎使用 AI Companion</h2>
          <p class="step-desc">让我们花几分钟完成基础设置，让 AI 陪伴更好地为你服务。</p>
          <div class="step-illustration">
            <span>AI 陪伴角色可以帮助你：</span>
            <span>日常聊天、情绪支持、学习陪伴、工作整理</span>
          </div>
        </div>

        <!-- Step 1: Deploy Mode -->
        <div v-if="current === 1" class="step-panel">
          <h2>选择部署方式</h2>
          <p class="step-desc">决定 AI Companion 的运行环境和访问方式。</p>
          <div class="deploy-cards">
            <div class="deploy-card" :class="{ selected: form.deployMode === 'desktop' }" @click="form.deployMode = 'desktop'">
              <div class="dc-radio"><span class="dc-dot" :class="{ on: form.deployMode === 'desktop' }"></span></div>
              <div class="dc-body">
                <div class="dc-title">桌面本地模式</div>
                <p class="dc-desc">Core 和 Web 都运行在本机，仅本机访问。数据完全在本地，微信桥接直接用</p>
              </div>
            </div>
            <div class="deploy-card" :class="{ selected: form.deployMode === 'cloud' }" @click="form.deployMode = 'cloud'">
              <div class="dc-radio"><span class="dc-dot" :class="{ on: form.deployMode === 'cloud' }"></span></div>
              <div class="dc-body">
                <div class="dc-title">私有云模式</div>
                <p class="dc-desc">Core 部署在服务器上，手机/多设备可访问。微信桥接仍需一台本地电脑运行</p>
              </div>
            </div>
          </div>
        </div>

        <!-- Step 2: Admin Setup -->
        <div v-if="current === 2" class="step-panel">
          <template v-if="hasAdmin">
            <h2>管理员账号已设置</h2>
            <p class="step-desc">管理员账号已在之前的设置中创建，请输入密码继续。</p>
            <el-alert type="success" :closable="false" show-icon style="margin-bottom:16px">
              <template #title>账号已就绪，无需重新创建</template>
            </el-alert>
            <el-form :model="form" label-position="top" size="default">
              <el-form-item label="用户名">
                <el-input v-model="form.username" placeholder="输入已设置的用户名" />
              </el-form-item>
              <el-form-item label="密码">
                <el-input v-model="form.password" type="password" placeholder="输入密码" show-password />
              </el-form-item>
            </el-form>
          </template>
          <template v-else>
            <h2>设置管理员账号</h2>
            <p class="step-desc">创建管理员账号以保护你的数据安全。</p>
            <el-form :model="form" label-position="top" size="default">
              <el-form-item label="用户名">
                <el-input v-model="form.username" placeholder="设置管理员用户名" />
              </el-form-item>
              <el-form-item label="密码">
                <el-input v-model="form.password" type="password" placeholder="至少 6 位" show-password />
              </el-form-item>
              <el-form-item label="确认密码">
                <el-input v-model="form.password2" type="password" placeholder="再次输入密码" show-password />
              </el-form-item>
            </el-form>
          </template>
        </div>

        <!-- Step 3: Model Config -->
        <div v-if="current === 3" class="step-panel">
          <h2>配置 AI 模型</h2>
          <p class="step-desc">AI Companion 需要连接大语言模型才能对话。</p>
          <el-form :model="form" label-position="top" size="default">
            <el-form-item label="API 类型">
              <el-select v-model="form.apiType" style="width:100%">
                <el-option label="OpenAI 兼容 (DeepSeek等)" value="openai-compatible" />
                <el-option label="Ollama (本地)" value="ollama" />
                <el-option label="自定义 HTTP" value="custom-http" />
              </el-select>
            </el-form-item>
            <el-form-item label="API Base URL">
              <el-input v-model="form.baseUrl" placeholder="https://api.deepseek.com/v1" />
            </el-form-item>
            <el-form-item label="API Key">
              <el-input v-model="form.apiKey" type="password" placeholder="sk-..." show-password />
            </el-form-item>
            <el-form-item label="模型名称">
            <div class="model-detect-wrap">
              <div class="model-detect-row">
                <el-input v-model="form.modelName" placeholder="deepseek-chat" class="model-input" />
                <el-button type="success" :loading="detectingModels" @click="detectModels" :disabled="!form.baseUrl || !form.apiKey">
                  {{ detectingModels ? '检测中...' : '检测模型' }}
                </el-button>
              </div>
              <div v-if="detectError" class="detect-error">{{ detectError }}</div>
              <Transition name="dropdown">
                <div v-if="detectedModels.length > 0" class="detect-dropdown">
                  <div class="detect-hint">检测到 {{ detectedModels.length }} 个模型，点击：</div>
                  <div v-for="m in detectedModels" :key="m.id" class="detect-option" :class="{ active: form.modelName === m.id }" @click="pickModel(m.id)">
                    {{ m.id }}
                  </div>
                </div>
              </Transition>
            </div>
            </el-form-item>
          </el-form>
        </div>

        <!-- Step 4: Character -->
        <div v-if="current === 4" class="step-panel">
          <h2>创建 AI 陪伴角色</h2>
          <p class="step-desc">给你的 AI 陪伴角色一个名字和性格，让它更有人情味。</p>
          <el-form :model="form" label-position="top" size="default">
            <el-form-item label="角色名称">
              <el-input v-model="form.charName" placeholder="例如：小暖" />
            </el-form-item>
            <el-form-item label="角色身份">
              <el-input v-model="form.charIdentity" placeholder="AI 虚拟陪伴角色" />
            </el-form-item>
            <el-form-item label="性格描述">
              <el-input v-model="form.charPersonality" type="textarea" :rows="2" placeholder="温和、体贴、有耐心" />
            </el-form-item>
          </el-form>
        </div>

        <!-- Step 5: Profile -->
        <div v-if="current === 5" class="step-panel">
          <h2>用户画像（可选）</h2>
          <p class="step-desc">填写你的个人信息，让 AI 更好地了解你。</p>
          <div class="profile-list">
            <div v-for="(item, idx) in profileList" :key="idx" class="profile-row">
              <el-select v-model="item.category" style="width:120px" size="default">
                <el-option label="基本信息" value="basic" />
                <el-option label="兴趣爱好" value="hobby" />
                <el-option label="工作学习" value="work" />
                <el-option label="生活习惯" value="lifestyle" />
                <el-option label="社交关系" value="social" />
                <el-option label="其他" value="other" />
              </el-select>
              <el-input v-model="item.attributeName" placeholder="属性名" style="width:140px" />
              <el-input v-model="item.attributeValue" placeholder="属性值" style="flex:1" />
              <el-button size="small" type="danger" text @click="profileList.splice(idx, 1)">删除</el-button>
            </div>
          </div>
          <el-button size="small" @click="profileList.push({ category: 'basic', attributeName: '', attributeValue: '' })" style="margin-top:8px">
            + 添加画像
          </el-button>
        </div>

        <!-- Step 6: Web Chat -->
        <div v-if="current === 6" class="step-panel">
          <h2>启用 Web 聊天</h2>
          <p class="step-desc">通过浏览器直接与 AI 角色对话。</p>
          <div class="toggle-card">
            <div class="tc-title">Web 聊天功能</div>
            <div class="tc-desc">开启后可通过浏览器访问聊天界面，与 AI 陪伴角色对话。</div>
            <el-switch v-model="form.webChatEnabled" />
          </div>
        </div>

        <!-- Step 7: WeChat (optional) -->
        <div v-if="current === 7" class="step-panel">
          <h2>微信接入（可选）</h2>
          <p class="step-desc">通过微信桥接，在微信中与 AI 角色对话。</p>
          <div class="toggle-card">
            <div class="tc-desc">需要桌面端运行微信桥接服务。可稍后在设置中配置。</div>
            <el-switch v-model="form.wechatEnabled" />
          </div>
          <template v-if="form.wechatEnabled">
            <template v-if="wxConnected">
              <el-alert type="success" :closable="false" show-icon style="margin-top:16px;margin-bottom:12px">
                <template #title>微信已成功连接</template>
              </el-alert>
              <el-card shadow="never" class="section-card">
                <template #header>
                  <div class="card-header-row">
                    <span class="card-header-title">连接状态</span>
                    <el-button size="small" @click="startWxLogin" :loading="wxQrLoading">重新扫码</el-button>
                  </div>
                </template>
                <div class="status-main">
                  <div class="status-row">
                    <div class="status-indicator ok"></div>
                    <span class="status-label">已连接</span>
                  </div>
                  <div class="status-detail-grid">
                    <div class="sd-item">
                      <span class="sd-label">消息数</span>
                      <span class="sd-value">{{ wxMessageCount }}</span>
                    </div>
                    <div class="sd-item" v-if="wxAccountId">
                      <span class="sd-label">账号</span>
                      <span class="sd-value">{{ wxAccountId.slice(0, 12) }}...</span>
                    </div>
                    <div class="sd-item">
                      <span class="sd-label">模式</span>
                      <span class="sd-value">OpenClaw</span>
                    </div>
                  </div>
                </div>
              </el-card>
              <el-alert type="warning" :closable="false" show-icon style="margin-top:12px">
                <template #title>主动推送须知</template>
                添加微信好友后，必须主动给机器人发一条消息，系统才能记录你的用户ID用于主动推送。用户ID每7天自动刷新，届时需重新发送一条消息。
              </el-alert>
            </template>

            <template v-if="!wxConnected">
              <el-card shadow="never" class="section-card" style="margin-top:16px">
                <template #header><span class="card-header-title">扫码连接</span></template>
                <div class="login-layout">
                  <div class="login-steps">
                    <div class="qr-step-row">
                      <div class="qr-step-num" :class="{ active: wxQrStep >= 0 }">1</div>
                      <div class="qr-step-body">
                        <span class="qr-step-title">生成二维码</span>
                        <el-button size="small" type="primary" :loading="wxQrLoading" @click="startWxLogin" :disabled="wxQrLoading">获取二维码</el-button>
                      </div>
                    </div>
                    <div class="qr-step-row">
                      <div class="qr-step-num" :class="{ active: wxQrStep >= 1 }">2</div>
                      <div class="qr-step-body">
                        <span class="qr-step-title">用微信扫码</span>
                        <span v-if="wxQrStep >= 1" class="qr-status">
                          <el-icon class="is-loading" v-if="wxScanning"><Loading /></el-icon>
                          {{ wxScanning ? '等待扫码中...' : '请扫描二维码' }}
                        </span>
                      </div>
                    </div>
                    <div class="qr-step-row">
                      <div class="qr-step-num" :class="{ active: wxConnected }">3</div>
                      <div class="qr-step-body">
                        <span class="qr-step-title">确认连接</span>
                        <span v-if="wxConnected" class="qr-done">
                          <el-icon><CircleCheckFilled /></el-icon> 已连接
                        </span>
                      </div>
                    </div>
                  </div>
                  <div class="login-qr">
                    <div class="qr-frame" v-if="wxQrCodeUrl">
                      <img :src="wxQrCodeUrl" alt="二维码" />
                    </div>
                    <div class="qr-frame qr-empty" v-else>
                      <el-icon :size="36"><Picture /></el-icon>
                      <span>点击按钮获取二维码</span>
                    </div>
                  </div>
                </div>
                <div class="qr-tip" v-if="wxQrCodeUrl">打开微信，扫描二维码确认连接。</div>
              </el-card>
              <div v-if="wxError" style="color:#f56c6c;font-size:12px;margin-top:8px">{{ wxError }}</div>
              <el-alert type="warning" :closable="false" show-icon style="margin-top:12px">
                <template #title>主动推送须知</template>
                添加微信好友后，必须主动给机器人发一条消息，系统才能记录你的用户ID用于主动推送。用户ID每7天自动刷新，届时需重新发送一条消息。
              </el-alert>
            </template>
          </template>
        </div>

        <!-- Step 8: QQ (optional) -->
        <div v-if="current === 8" class="step-panel">
          <h2>QQ 接入（可选）</h2>
          <p class="step-desc">通过 QQ 机器人，在 QQ 中与 AI 角色对话。</p>
          <div class="toggle-card">
            <div class="tc-desc">需要后端运行 QQ 桥接服务。可稍后在设置中配置。</div>
            <el-switch v-model="form.qqEnabled" />
          </div>
          <template v-if="form.qqEnabled">
            <template v-if="qqConnected">
              <el-alert type="success" :closable="false" show-icon style="margin-top:16px;margin-bottom:12px">
                <template #title>QQ Bot 已成功连接</template>
              </el-alert>
              <el-card shadow="never" class="section-card">
                <template #header>
                  <div class="card-header-row">
                    <span class="card-header-title">连接状态</span>
                    <el-button size="small" @click="connectQQ" :loading="qqConnecting">重新连接</el-button>
                  </div>
                </template>
                <div class="status-main">
                  <div class="status-row">
                    <div class="status-indicator ok"></div>
                    <span class="status-label">已连接</span>
                  </div>
                  <div class="status-detail-grid">
                    <div class="sd-item">
                      <span class="sd-label">Bot ID</span>
                      <span class="sd-value">{{ qqAccountId }}</span>
                    </div>
                    <div class="sd-item">
                      <span class="sd-label">协议</span>
                      <span class="sd-value">QQBot (WebSocket)</span>
                    </div>
                    <div class="sd-item">
                      <span class="sd-label">消息数</span>
                      <span class="sd-value">{{ qqMessageCount }}</span>
                    </div>
                  </div>
                </div>
              </el-card>
              <el-alert type="warning" :closable="false" show-icon style="margin-top:12px">
                <template #title>主动推送须知</template>
                添加QQ好友后，必须主动给机器人发一条消息，系统才能记录你的用户ID用于主动推送。用户ID每7天自动刷新，届时需重新发送一条消息。
              </el-alert>
            </template>

            <template v-if="!qqConnected">
              <el-form :model="form" label-position="top" size="default" style="margin-top:16px">
                <el-form-item label="AppID">
                  <el-input v-model="form.qqAppId" placeholder="QQ 机器人 AppID" />
                </el-form-item>
                <el-form-item label="Token">
                  <el-input v-model="form.qqToken" placeholder="QQ 机器人 Token" type="password" show-password />
                </el-form-item>
                <el-form-item>
                  <el-button type="primary" @click="connectQQ" :loading="qqConnecting" :disabled="!form.qqAppId || !form.qqToken">
                    {{ qqConnecting ? '连接中...' : '连接' }}
                  </el-button>
                  <span v-if="qqError" style="color:#f56c6c;margin-left:8px">{{ qqError }}</span>
                </el-form-item>
              </el-form>
              <div class="step-illustration" style="margin-top:12px">
                <span>使用步骤：</span>
                <span>1. 前往 <a href="https://q.qq.com/" target="_blank" style="color:#409eff">QQ开放平台</a> 创建机器人</span>
                <span>2. 获取 AppID 和 Token</span>
                <span>3. 填入上方表单，点击"连接"</span>
                <span>4. 连接成功后，在QQ中 @机器人 即可对话</span>
              </div>
              <el-alert type="warning" :closable="false" show-icon style="margin-top:12px">
                <template #title>主动推送须知</template>
                连接成功后，添加QQ好友必须主动给机器人发一条消息，系统才能记录你的用户ID用于主动推送。用户ID每7天自动刷新，届时需重新发送一条消息。
              </el-alert>
            </template>
          </template>
        </div>

        <!-- Step 9: Privacy -->
        <div v-if="current === 9" class="step-panel">
          <h2>隐私与安全</h2>
          <p class="step-desc">了解 AI Companion 如何处理你的数据。</p>
          <div class="privacy-cards">
            <div class="pc-card">
              <div class="pc-title">数据存储位置</div>
              <div class="pc-desc">所有对话记录、记忆和配置均存储在本地 data 目录中，不上传至任何云端服务。</div>
            </div>
            <div class="pc-card">
              <div class="pc-title">API Key 保护</div>
              <div class="pc-desc">你的 API Key 仅用于调用 AI 模型，日志中会自动脱敏，不会泄露。</div>
            </div>
            <div class="pc-card">
              <div class="pc-title">AI 安全边界</div>
              <div class="pc-desc">AI 角色不会声称自己是真人，不会索要隐私信息，不会替代真实人际关系。</div>
            </div>
            <div class="pc-card">
              <div class="pc-title">你可以完全掌控</div>
              <div class="pc-desc">随时可以查看、导出或删除所有数据。随时可以停止使用。</div>
            </div>
          </div>
        </div>
      </div>

      <!-- Actions -->
      <div class="onb-actions">
        <el-button v-if="current > 0" @click="current--">上一步</el-button>
        <el-button v-if="current < steps.length - 1" type="primary" @click="handleNext">下一步</el-button>
        <el-button v-if="current === steps.length - 1" type="primary" @click="handleFinish">完成设置</el-button>
      </div>

      <div v-if="stepError" class="step-error">{{ stepError }}</div>

      <div class="onb-summary" v-if="current === steps.length - 1">
        <el-descriptions title="设置预览" :column="2" border size="small">
          <el-descriptions-item label="部署方式">{{ form.deployMode === 'desktop' ? '桌面本地' : '私有云' }}</el-descriptions-item>
          <el-descriptions-item label="Web 聊天">{{ form.webChatEnabled ? '已开启' : '未开启' }}</el-descriptions-item>
          <el-descriptions-item label="用户画像">{{ profileList.filter(p=>p.attributeName).length || '未填写' }}</el-descriptions-item>
          <el-descriptions-item label="微信接入">{{ form.wechatEnabled ? '已开启' : '未开启' }}</el-descriptions-item>
          <el-descriptions-item label="QQ 接入">{{ form.qqEnabled ? '已开启' : '未开启' }}</el-descriptions-item>
          <el-descriptions-item label="管理员">{{ form.username || '-' }}</el-descriptions-item>
          <el-descriptions-item label="API 类型">{{ form.apiType || '-' }}</el-descriptions-item>
          <el-descriptions-item label="模型">{{ form.modelName || '-' }}</el-descriptions-item>
          <el-descriptions-item label="角色名称">{{ form.charName || '-' }}</el-descriptions-item>
          <el-descriptions-item label="角色性格">{{ form.charPersonality || '-' }}</el-descriptions-item>
        </el-descriptions>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted, watch } from "vue"
import { Loading, Picture, CircleCheckFilled } from "@element-plus/icons-vue"
import { useRouter } from "vue-router"
import { ElMessage } from "element-plus"
import { useApi, setToken } from "../../ui-index"
import axios from "axios"

const router = useRouter()
const { get, post } = useApi()

const current = ref(0)
const stepError = ref("")
const detectingModels = ref(false)
const detectedModels = ref<{ id: string; owned_by?: string }[]>([])
const detectError = ref("")

const hasAdmin = ref(false)

onMounted(async () => {
  try {
    const res = await get<any>("/api/auth/status")
    hasAdmin.value = !!res?.hasAdmin
  } catch { /* ignore */ }
})

const steps = [
  { label: "欢迎", key: "welcome" },
  { label: "部署", key: "deploy" },
  { label: "账号", key: "admin" },
  { label: "模型", key: "model" },
  { label: "角色", key: "character" },
  { label: "画像", key: "profile" },
  { label: "Web", key: "web" },
  { label: "微信", key: "wechat" },
  { label: "QQ", key: "qq" },
  { label: "隐私", key: "privacy" },
]

const form = reactive({
  deployMode: "desktop",
  username: "",
  password: "",
  password2: "",
  apiType: "openai-compatible",
  baseUrl: "https://api.deepseek.com/v1",
  apiKey: "",
  modelName: "deepseek-chat",
  charName: "小暖",
  charIdentity: "AI 虚拟陪伴角色",
  charPersonality: "温和、体贴、有耐心",
  webChatEnabled: true,
  wechatEnabled: false,
  qqEnabled: false,
  qqAppId: "",
  qqToken: "",
})

watch(current, async (val) => {
  if (val === 7 && form.wechatEnabled) { await refreshWxStatus() }
  if (val === 8 && form.qqEnabled) { await refreshQQStatus() }
})

watch(() => form.wechatEnabled, async (enabled) => {
  if (enabled && current.value === 7) { await refreshWxStatus() }
})

watch(() => form.qqEnabled, async (enabled) => {
  if (enabled && current.value === 8) { await refreshQQStatus() }
})


const profileList = reactive<{ category: string; attributeName: string; attributeValue: string }[]>([])

const wxQrLoading = ref(false)
const wxQrCodeUrl = ref("")
const wxQrStep = ref(0)
const wxScanning = ref(false)
const wxConnected = ref(false)
const wxAccountId = ref("")
const wxMessageCount = ref(0)
const wxError = ref("")
let wxPollTimer: ReturnType<typeof setInterval> | null = null

const QQ_API = "http://127.0.0.1:8899/api/qq"
const qqConnected = ref(false)
const qqConnecting = ref(false)
const qqAccountId = ref("")
const qqMessageCount = ref(0)
const qqError = ref("")

async function detectModels() {
  detectError.value = ""
  detectedModels.value = []
  detectingModels.value = true
  try {
    const res = await post<any>("/api/model/detect-models", {
      baseUrl: form.baseUrl,
      apiKey: form.apiKey,
      apiType: form.apiType,
    })
    detectedModels.value = res?.models || []
    if (detectedModels.value.length === 0) {
      detectError.value = "未检测到可用模型"
    }
  } catch (err: any) {
    detectError.value = err?.message || "检测失败，请检查 Base URL 和 API Key"
  } finally {
    detectingModels.value = false
  }
}

function pickModel(id: string) {
  form.modelName = id
  detectError.value = ""
  detectedModels.value = []
}


async function startWxLogin() {
  wxError.value = ""
  wxQrLoading.value = true
  stopWxPolling()
  try {
    const res = await get<any>("/api/wechat/login/start")
    const imgUrl = res?.data?.qrImageUrl || res?.qrImageUrl || res?.data?.qrCodeUrl || res?.qrCodeUrl
    if (imgUrl) {
      wxQrCodeUrl.value = imgUrl
      wxQrStep.value = 1
      wxScanning.value = true
      startWxPolling()
    } else {
      wxError.value = "获取二维码失败"
    }
  } catch (err: any) {
    wxError.value = err?.message || "获取二维码失败"
  } finally {
    wxQrLoading.value = false
  }
}

async function refreshWxStatus() {
  try {
    const res = await get<any>("/api/wechat/status")
    const data = res?.data || res
    if (data?.status === "connected") {
      wxConnected.value = true
      wxScanning.value = false
      wxQrStep.value = 3
      wxAccountId.value = data?.accountId || ""
      wxMessageCount.value = data?.messageCount || 0
      stopWxPolling()
    }
  } catch { /* ignore */ }
}

function startWxPolling() {
  stopWxPolling()
  const startTime = Date.now()
  wxPollTimer = setInterval(async () => {
    if (Date.now() - startTime > 130000) {
      stopWxPolling()
      wxScanning.value = false
      wxQrStep.value = 0
      ElMessage.warning("扫码超时，请重新获取二维码")
      return
    }
    await refreshWxStatus()
  }, 2000)
}

function stopWxPolling() {
  if (wxPollTimer) { clearInterval(wxPollTimer); wxPollTimer = null }
}

let qqPollTimer: ReturnType<typeof setInterval> | null = null

function stopQQPoll() {
  if (qqPollTimer) { clearInterval(qqPollTimer); qqPollTimer = null }
}

async function connectQQ() {
  if (!form.qqAppId || !form.qqToken) return
  qqError.value = ""
  qqConnecting.value = true
  try {
    await axios.post(QQ_API + "/connect", {
      appId: form.qqAppId,
      token: form.qqToken,
      sandbox: false,
    })
    stopQQPoll()
    const startTime = Date.now()
    qqPollTimer = setInterval(async () => {
      await refreshQQStatus()
      if (qqConnected.value) {
        stopQQPoll()
        qqConnecting.value = false
        return
      }
      if (Date.now() - startTime > 30000) {
        stopQQPoll()
        qqConnecting.value = false
        qqError.value = "连接超时，请检查AppID和Token是否有效"
      }
    }, 2000)
  } catch (e: any) {
    qqError.value = e?.response?.data?.error || "连接失败，请检查AppID和Token"
    qqConnecting.value = false
  }
}

async function refreshQQStatus() {
  try {
    const res = await axios.get(QQ_API + "/status")
    const data = res.data?.data || res.data
    qqConnected.value = !!data?.qqOnline
    qqAccountId.value = data?.accountId || ""
    qqMessageCount.value = data?.messageCount || 0
  } catch { /* ignore */ }
}

async function handleNext() {
  stepError.value = ""

  // Step 2: Admin setup -> create admin + login to get token
  if (current.value === 2) {
    if (!form.username || !form.password) {
      stepError.value = "请填写用户名和密码"
      return
    }
    if (!hasAdmin.value && form.password !== form.password2) {
      stepError.value = "两次密码不一致"
      return
    }
    if (form.password.length < 6) {
      stepError.value = "密码至少 6 位"
      return
    }
    try {
      if (!hasAdmin.value) {
        // Create admin for ALL modes
        try {
          await post("/api/auth/setup", { username: form.username, password: form.password })
        } catch (setupErr: any) {
          // 409 = admin already exists, thats fine, just login
          if (setupErr?.response?.status !== 409 && setupErr?.response?.data?.code !== 20006) {
            throw setupErr
          }
        }
      }
      // Login to get token for subsequent authenticated calls
      const loginRes = await post<any>("/api/auth/login", { username: form.username, password: form.password })
      if (loginRes?.token) {
        setToken(loginRes.token)
      }
      current.value++
    } catch (e: any) {
      stepError.value = e?.response?.data?.message || e?.message || (hasAdmin.value ? "登录失败，请检查密码" : "创建账号失败，请重试")
    }
    return
  }

  if (current.value === 4) {
    if (!form.charName) {
      stepError.value = "请输入角色名称"
      return
    }
  }

  current.value++
}

async function handleFinish() {
  stepError.value = ""
  try {
    // 1. Configure model (authenticated)
    if (form.apiKey && form.baseUrl && form.modelName) {
      await post("/api/model/configs", {
        apiType: form.apiType,
        baseUrl: form.baseUrl,
        apiKey: form.apiKey,
        modelName: form.modelName,
        isActive: 1,
      })
    }

    // 2. Create character (authenticated)
    if (form.charName) {
      await post("/api/characters", {
        name: form.charName,
        identity: form.charIdentity,
        personality: form.charPersonality,
        isActive: 1,
        isDefault: true,
      })
    }

    // 3. Mark onboarding as completed
    const validProfiles = profileList.filter(p => p.attributeName && p.attributeValue)
    for (const p of validProfiles) {
      await post("/api/profiles", {
        category: p.category,
        attributeName: p.attributeName,
        attributeValue: p.attributeValue,
      }).catch(() => {})
    }

    await post("/api/onboarding/complete", {
      deployMode: form.deployMode === "cloud" ? "cloud-web" : "desktop-local",
      webChatEnabled: form.webChatEnabled,
      wechatEnabled: form.wechatEnabled,
      qqEnabled: form.qqEnabled,
      modelConfig: form.apiKey ? { name: "default", apiType: form.apiType, baseUrl: form.baseUrl, apiKey: form.apiKey, modelName: form.modelName } : undefined,
      username: form.username,
      password: form.password || undefined,
    })

    ElMessage.success("设置完成！即将跳转到聊天页面")
    setTimeout(() => router.push("/chat"), 1500)
  } catch (err: any) {
    stepError.value = err?.message || err?.response?.data?.message || "设置过程中出现错误，请重试"
  }
}
onUnmounted(() => {
  stopWxPolling()
  stopQQPoll()
})
</script>

<style scoped>
.onboarding-shell {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--ac-color-bg);
  padding: 20px;
}

.onb-container {
  width: 100%;
  max-width: 600px;
}

.onb-header {
  text-align: center;
  margin-bottom: 24px;
}

.onb-brand {
  font-size: 28px;
  font-weight: 700;
  color: var(--ac-color-primary);
  margin: 0 0 4px;
}

.onb-sub {
  font-size: 14px;
  color: var(--ac-color-text-muted);
}

.onb-steps {
  display: flex;
  justify-content: center;
  gap: 12px;
  margin-bottom: 28px;
  flex-wrap: wrap;
}

.step-dot {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  cursor: pointer;
  opacity: 0.4;
  transition: opacity 0.2s;
}

.step-dot.active,
.step-dot.done {
  opacity: 1;
}

.dot-icon {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  background: var(--ac-color-bg-secondary);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  font-weight: 600;
}

.step-dot.active .dot-icon {
  background: var(--ac-color-primary);
  color: #fff;
}

.step-dot.done .dot-icon {
  background: var(--ac-color-success);
  color: #fff;
}

.dot-label {
  font-size: 11px;
  color: var(--ac-color-text-muted);
}

.onb-content {
  min-height: 200px;
  margin-bottom: 20px;
}

.step-panel h2 {
  font-size: 20px;
  font-weight: 600;
  margin: 0 0 8px;
}

.step-desc {
  font-size: 14px;
  color: var(--ac-color-text-secondary);
  margin: 0 0 16px;
}

.step-illustration {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 16px;
  background: var(--ac-color-primary-bg);
  border-radius: 8px;
  font-size: 13px;
}

.deploy-cards {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.deploy-card {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 14px;
  border: 1px solid var(--ac-color-border-light);
  border-radius: 8px;
  cursor: pointer;
  transition: border-color 0.2s, background 0.2s;
}

.deploy-card:hover {
  background: var(--ac-color-bg-secondary);
}

.deploy-card.selected {
  border-color: var(--ac-color-primary);
  background: var(--ac-color-primary-bg);
}

.dc-radio {
  flex-shrink: 0;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  border: 2px solid var(--ac-color-border);
  display: flex;
  align-items: center;
  justify-content: center;
  margin-top: 2px;
}

.deploy-card.selected .dc-radio {
  border-color: var(--ac-color-primary);
}

.dc-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: transparent;
  transition: background 0.2s;
}

.dc-dot.on {
  background: var(--ac-color-primary);
}

.dc-body {
  flex: 1;
}

.dc-title {
  font-weight: 600;
  font-size: 14px;
}

.dc-desc {
  font-size: 12px;
  color: var(--ac-color-text-muted);
  margin: 4px 0 0;
}

.toggle-card {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 12px;
  background: var(--ac-color-bg-secondary);
  border-radius: 8px;
}

.tc-title {
  font-weight: 600;
  font-size: 14px;
}

.tc-desc {
  flex: 1;
  font-size: 12px;
  color: var(--ac-color-text-muted);
}

.profile-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.profile-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.section-card { margin-bottom: 12px; }
.card-header-row { display: flex; align-items: center; justify-content: space-between; }
.card-header-title { font-weight: 600; font-size: 13px; }

.login-layout {
  display: flex;
  gap: 28px;
  align-items: flex-start;
}
@media (max-width: 560px) {
  .login-layout { flex-direction: column-reverse; align-items: center; }
}

.login-steps { flex: 1; min-width: 0; }
.login-qr { flex-shrink: 0; }

.qr-frame {
  width: 200px;
  height: 200px;
  border: 1px solid #e4e7ed;
  border-radius: 6px;
  overflow: hidden;
  background: #fff;
}
.qr-frame img {
  width: 100%;
  height: 100%;
  object-fit: contain;
  display: block;
}
.qr-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: #909399;
  font-size: 12px;
}

.qr-tip {
  margin-top: 14px;
  font-size: 12px;
  color: #909399;
  text-align: center;
}

.qr-step-row {
  display: flex;
  gap: 10px;
  align-items: center;
  padding: 12px 0;
  border-bottom: 1px solid #ebeef5;
}
.qr-step-row:last-child { border-bottom: none; }
.qr-step-num {
  width: 24px; height: 24px; border-radius: 50%; flex-shrink: 0;
  display: flex; align-items: center; justify-content: center;
  font-size: 12px; font-weight: 600;
  background: #f5f7fa; color: #909399;
  border: 2px solid #dcdfe6;
}
.qr-step-num.active { background: #5a9e6f; color: #fff; border-color: #5a9e6f; }
.qr-step-body { flex: 1; display: flex; align-items: center; gap: 10px; }
.qr-step-title { font-size: 13px; font-weight: 600; color: var(--ac-color-text); }
.qr-status { font-size: 12px; color: #4e5969; display: flex; align-items: center; gap: 4px; }
.qr-done { font-size: 12px; color: #5a9e6f; font-weight: 600; display: flex; align-items: center; gap: 4px; }

.status-main { display: flex; flex-direction: column; gap: 12px; }
.status-row { display: flex; align-items: center; gap: 10px; }
.status-indicator { width: 12px; height: 12px; border-radius: 50%; flex-shrink: 0; }
.status-indicator.ok { background: #5a9e6f; }
.status-label { font-size: 16px; font-weight: 600; color: var(--ac-color-text); }
.status-detail-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(140px, 1fr)); gap: 8px; }
.sd-item { padding: 8px 12px; background: var(--ac-color-bg-secondary); border-radius: 4px; }
.sd-label { font-size: 11px; color: var(--ac-color-text-muted); display: block; }
.sd-value { font-size: 14px; font-weight: 600; color: var(--ac-color-text); }

.privacy-cards {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}

.pc-card {
  padding: 12px;
  background: var(--ac-color-bg-secondary);
  border-radius: 8px;
}

.pc-title {
  font-weight: 600;
  font-size: 13px;
  margin-bottom: 4px;
}

.pc-desc {
  font-size: 12px;
  color: var(--ac-color-text-secondary);
  line-height: 1.5;
}

.onb-actions {
  display: flex;
  justify-content: center;
  gap: 12px;
  margin-bottom: 16px;
}

.step-error {
  text-align: center;
  color: var(--ac-color-error, #f56c6c);
  font-size: 13px;
  margin-bottom: 12px;
}

/* Model detect */
.model-detect-wrap { position: relative; }
.model-detect-row { display: flex; gap: 8px; }
.model-detect-row .model-input { flex: 1; }
.detect-error { color: var(--ac-color-danger, #f56c6c); font-size: 12px; margin-top: 4px; }
.detect-dropdown { background: var(--ac-color-surface); border: 1px solid var(--ac-color-primary-border); border-radius: var(--ac-radius-sm); box-shadow: var(--ac-shadow-md); max-height: 180px; overflow-y: auto; margin-top: 6px; }
.detect-hint { padding: 6px 10px; font-size: 11px; color: var(--ac-color-text-muted); border-bottom: 1px solid var(--ac-color-border-light); }
.detect-option { padding: 8px 10px; cursor: pointer; font-size: 13px; color: var(--ac-color-text); transition: background .15s; }
.detect-option:hover { background: var(--ac-color-primary-bg); }
.dropdown-enter-active, .dropdown-leave-active { transition: all .2s ease; }
.dropdown-enter-from, .dropdown-leave-to { opacity: 0; transform: translateY(-4px); max-height: 0; }

.detect-option.active { background: var(--ac-color-primary-bg); color: var(--ac-color-primary); font-weight: 600; }

.onb-summary {
  margin-top: 16px;
  color: var(--ac-color-text);
}

.onb-summary .el-descriptions__title {
  color: var(--ac-color-text);
  font-size: 15px;
  font-weight: 600;
}

.onb-summary .el-descriptions__label {
  color: var(--ac-color-text-secondary);
}

.onb-summary .el-descriptions__content {
  color: var(--ac-color-text);
}

@media (max-width: 768px) {
  .onb-steps {
    gap: 6px;
  }

  .dot-label {
    display: none;
  }

  .privacy-cards {
    grid-template-columns: 1fr;
  }

  .profile-row {
    flex-wrap: wrap;
  }
}
</style>
