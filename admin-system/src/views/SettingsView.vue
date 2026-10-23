<script setup lang="ts">
import { onMounted, reactive, ref } from "vue"
import { ElMessage } from "element-plus"

import { errorMessage } from "@/api/client"
import { changePassword, fetchSettings } from "@/api/releases"
import type { AdminSettings } from "@/api/types"
import { useAuthStore } from "@/stores/authStore"

const auth = useAuthStore()
const loading = ref(true)
const passwordSaving = ref(false)
const settings = ref<AdminSettings | null>(null)
const passwordForm = reactive({
  currentPassword: "",
  newPassword: "",
  confirmPassword: "",
})

async function load() {
  loading.value = true
  try {
    settings.value = await fetchSettings()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    loading.value = false
  }
}

async function copy(value: string) {
  await navigator.clipboard.writeText(value)
  ElMessage.success("已复制")
}

async function submitPassword() {
  if (passwordForm.newPassword.length < 6) {
    ElMessage.error("新密码至少需要 6 个字符")
    return
  }
  if (passwordForm.newPassword !== passwordForm.confirmPassword) {
    ElMessage.error("两次输入的新密码不一致")
    return
  }
  passwordSaving.value = true
  try {
    await changePassword(passwordForm.currentPassword, passwordForm.newPassword)
    ElMessage.success("密码已更新，请重新登录")
    await auth.logout()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    passwordSaving.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="page-stack" v-loading="loading">
    <section class="page-intro">
      <div>
        <h2>发布设置</h2>
        <p>这里显示当前管理服务采用的发布目录和客户端地址，敏感配置只通过服务器环境变量注入。</p>
      </div>
    </section>

    <section class="settings-grid">
      <article class="panel setting-card">
        <div class="setting-card-header">
          <span class="stat-icon blue"><el-icon><Monitor /></el-icon></span>
          <div>
            <h3>桌面端更新</h3>
            <p>electron-updater generic provider</p>
          </div>
        </div>
        <label>客户端请求地址</label>
        <div class="copy-field">
          <code>{{ settings?.desktopClientURL || "-" }}</code>
          <el-button text @click="copy(settings?.desktopClientURL || '')">复制</el-button>
        </div>
        <label>发布目录</label>
        <div class="copy-field">
          <code>{{ settings?.desktopPublishDir || "-" }}</code>
          <el-button text @click="copy(settings?.desktopPublishDir || '')">复制</el-button>
        </div>
      </article>

      <article class="panel setting-card">
        <div class="setting-card-header">
          <span class="stat-icon amber"><el-icon><Cellphone /></el-icon></span>
          <div>
            <h3>手机端更新</h3>
            <p>签名清单与 arm64-v8a APK</p>
          </div>
        </div>
        <label>客户端请求地址</label>
        <div class="copy-field">
          <code>{{ settings?.androidClientBaseURL || "-" }}</code>
          <el-button text @click="copy(settings?.androidClientBaseURL || '')">复制</el-button>
        </div>
        <label>发布目录</label>
        <div class="copy-field">
          <code>{{ settings?.androidPublishDir || "-" }}</code>
          <el-button text @click="copy(settings?.androidPublishDir || '')">复制</el-button>
        </div>
      </article>
    </section>

    <section class="panel">
      <div class="panel-heading">
        <div>
          <h3>运行约束</h3>
          <p>生产环境应通过 Nginx 提供 HTTPS，并把管理 API 限制在可信网络或访问控制之后。</p>
        </div>
      </div>
      <dl class="settings-list horizontal">
        <div>
          <dt>应用包名</dt>
          <dd>{{ settings?.androidPackageName || "-" }}</dd>
        </div>
        <div>
          <dt>手机签名私钥</dt>
          <dd>
            <el-tag :type="settings?.signingKeyConfigured ? 'success' : 'warning'" effect="plain">
              {{ settings?.signingKeyConfigured ? "已配置" : "未配置" }}
            </el-tag>
          </dd>
        </div>
        <div>
          <dt>会话 Cookie</dt>
          <dd>
            <el-tag :type="settings?.cookieSecure ? 'success' : 'warning'" effect="plain">
              {{ settings?.cookieSecure ? "Secure" : "开发模式" }}
            </el-tag>
          </dd>
        </div>
      </dl>
    </section>

    <section class="panel password-panel">
      <div class="panel-heading">
        <div>
          <h3>修改登录密码</h3>
          <p>密码至少 6 个字符。修改成功后所有管理会话会立即失效。</p>
        </div>
      </div>
      <el-form label-position="top" class="password-form">
        <el-form-item label="当前密码">
          <el-input v-model="passwordForm.currentPassword" type="password" show-password autocomplete="current-password" />
        </el-form-item>
        <el-form-item label="新密码">
          <el-input v-model="passwordForm.newPassword" type="password" show-password autocomplete="new-password" />
        </el-form-item>
        <el-form-item label="确认新密码">
          <el-input v-model="passwordForm.confirmPassword" type="password" show-password autocomplete="new-password" />
        </el-form-item>
        <el-button type="primary" :loading="passwordSaving" @click="submitPassword">更新密码</el-button>
      </el-form>
    </section>
  </div>
</template>
