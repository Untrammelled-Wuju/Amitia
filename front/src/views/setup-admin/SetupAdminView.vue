<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="setup-page">
    <el-card class="setup-card" shadow="never">
      <div class="setup-header">
        <div class="setup-icon">\u{1F510}</div>
        <h1>首次设置管理员</h1>
        <p>这是你第一次启动私有云部署。<br/>请设置管理员账号和密码以保护你的数据。</p>
      </div>

      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-position="top"
        @submit.prevent="handleSetup"
      >
        <el-form-item label="管理员用户名" prop="username">
          <el-input
            v-model="form.username"
            placeholder="设置管理员用户名"
            maxlength="32"
            autocomplete="off"
          />
        </el-form-item>

        <el-form-item label="管理员密码" prop="password">
          <el-input
            v-model="form.password"
            type="password"
            placeholder="至少 6 位密码"
            show-password
            maxlength="64"
            autocomplete="new-password"
          />
        </el-form-item>

        <el-form-item label="确认密码" prop="confirmPassword">
          <el-input
            v-model="form.confirmPassword"
            type="password"
            placeholder="再次输入密码"
            show-password
            maxlength="64"
            autocomplete="new-password"
          />
        </el-form-item>

        <el-alert type="warning" :closable="false" show-icon style="margin-bottom:16px">
          <template #title>请牢记你的管理员密码，丢失后无法找回</template>
        </el-alert>

        <el-form-item>
          <el-button
            type="primary"
            native-type="submit"
            :loading="loading"
            style="width:100%"
            size="large"
          >
            完成设置并登录
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from "vue"
import { useRouter } from "vue-router"
import { ElMessage } from "element-plus"
import { apiClient, setToken } from "../../ui-index"

const router = useRouter()
const formRef = ref()
const loading = ref(false)

const form = reactive({
  username: "",
  password: "",
  confirmPassword: "",
})

const validateConfirm = (_rule: any, value: string, callback: any) => {
  if (value !== form.password) {
    callback(new Error("两次输入的密码不一致"))
  } else {
    callback()
  }
}

const rules = {
  username: [
    { required: true, message: "请输入管理员用户名", trigger: "blur" },
    { min: 2, message: "用户名至少 2 个字符", trigger: "blur" },
  ],
  password: [
    { required: true, message: "请输入管理员密码", trigger: "blur" },
    { min: 6, message: "密码至少 6 位", trigger: "blur" },
  ],
  confirmPassword: [
    { required: true, message: "请确认密码", trigger: "blur" },
    { validator: validateConfirm, trigger: "blur" },
  ],
}

// Check if setup is allowed before showing page
onMounted(async () => {
  try {
    const res = await apiClient.get("/api/auth/status")
    const data = res.data?.data || res.data
    // If already set up and user is logged in, redirect
    if (data?.hasAdmin) {
      const token = localStorage.getItem("ai-companion-token")
      if (token) {
        router.replace("/chat")
      } else {
        router.replace("/login")
      }
    }
  } catch {
    // If status endpoint fails (not yet implemented), just show setup page
  }
})

async function handleSetup() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  loading.value = true
  try {
    const res = await apiClient.post("/api/auth/setup", {
      username: form.username,
      password: form.password,
    })
    const data = res.data?.data || res.data
    if (data?.token) {
      setToken(data.token)
      ElMessage.success("管理员设置成功，已自动登录")
      router.push("/chat")
    }
  } catch (err: any) {
    // If 409 - admin already exists, go to login
    if (err?.response?.status === 409 || err?.response?.data?.code === 20006) {
      ElMessage.warning("管理员已存在，请直接登录")
      router.push("/login")
    }
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.setup-page {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100%;
  padding: 20px;
}

.setup-card {
  width: 400px;
  max-width: 100%;
}

.setup-header {
  text-align: center;
  margin-bottom: 24px;
}

.setup-icon {
  font-size: 40px;
  margin-bottom: 8px;
}

.setup-header h1 {
  font-size: var(--ac-font-size-xl);
  font-weight: 600;
  color: var(--ac-color-text);
  margin-bottom: 8px;
}

.setup-header p {
  font-size: var(--ac-font-size-sm);
  color: var(--ac-color-text-muted);
  line-height: 1.6;
}

@media (max-width: 768px) {
  .setup-page {
    padding: 10px;
    align-items: flex-start;
    padding-top: 60px;
  }

  .setup-card {
    border: none;
    box-shadow: none;
    background: transparent;
  }

  .setup-card :deep(.el-card__body) {
    padding: 16px;
  }
}
</style>
