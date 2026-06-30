<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="login-page">
    <el-card class="login-card" shadow="never">
      <div class="login-header">
        <h1>AI-Amitia</h1>
      </div>

      <!-- Status detection -->
      <div v-if="checkingStatus" class="login-status">
        <el-icon class="is-loading" :size="20"><Loading /></el-icon>
        <span>正在检查服务状态...</span>
      </div>

      <template v-else>
        <el-form
          ref="formRef"
          :model="form"
          :rules="rules"
          label-position="top"
          @submit.prevent="handleLogin"
        >
          <el-form-item label="用户名" prop="username">
            <el-input
              v-model="form.username"
              placeholder="请输入用户名"
              autocomplete="username"
            />
          </el-form-item>

          <el-form-item label="密码" prop="password">
            <el-input
              v-model="form.password"
              type="password"
              placeholder="请输入密码"
              show-password
              autocomplete="current-password"
              @keyup.enter="handleLogin"
            />
          </el-form-item>

          <el-form-item>
            <el-button
              type="primary"
              native-type="submit"
              :loading="loading"
              style="width:100%"
              size="large"
            >
              登录
            </el-button>
          </el-form-item>
        </el-form>

      </template>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from "vue"
import { useRouter, useRoute } from "vue-router"
import { ElMessage } from "element-plus"
import { Loading } from "@element-plus/icons-vue"
import { apiClient, setToken } from "../../composables/useApi"

const router = useRouter()
const route = useRoute()
const formRef = ref()
const loading = ref(false)
const checkingStatus = ref(true)

const form = reactive({ username: "", password: "" })
const rules = {
  username: [{ required: true, message: "请输入用户名", trigger: "blur" }],
  password: [{ required: true, message: "请输入密码", trigger: "blur" }],
}

onMounted(() => {
  checkingStatus.value = false
})

async function handleLogin() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  loading.value = true
  try {
    const res = await apiClient.post("/api/auth/login", {
      username: form.username,
      password: form.password,
    })
    const data = res.data?.data || res.data
    if (data?.token) {
      setToken(data.token)
      ElMessage.success(`欢迎回来，${data.username || form.username}`)

      // Redirect to intended page or chat
      const redirect = (route.query.redirect as string) || "/chat"
      router.push(redirect)
    }
  } catch (err: any) {
    // Error handled by interceptor
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-page {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100%;
  padding: 20px;
}

.login-card {
  width: 360px;
  max-width: 100%;
}

.login-header {
  text-align: center;
  margin-bottom: 24px;
}

.login-header h1 {
  font-size: var(--ac-font-size-xl);
  font-weight: 600;
  color: var(--ac-color-primary);
  margin-bottom: 4px;
}

.login-header p {
  font-size: var(--ac-font-size-sm);
  color: var(--ac-color-text-muted);
}

.login-status {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 32px 0;
  color: var(--ac-color-text-muted);
  font-size: var(--ac-font-size-sm);
}


.footer-text {
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-muted);
}

@media (max-width: 768px) {
  .login-page {
    padding: 10px;
    align-items: flex-start;
    padding-top: 60px;
  }

  .login-card {
    border: none;
    box-shadow: none;
    background: transparent;
  }

  .login-card :deep(.el-card__body) {
    padding: 16px;
  }

  .login-header h1 {
    font-size: var(--ac-font-size-xl);
  }
}
</style>
