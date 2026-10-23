<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Lock, User } from '@element-plus/icons-vue'
import { useRouter } from 'vue-router'
import { errorMessage } from '@/api/client'
import logoDark from '@/assets/images/amitia-logo-dark.png'
import logoLight from '@/assets/images/amitia-logo-light.png'
import { useAuthStore } from '@/stores/authStore'

const { login } = useAuthStore()
const router = useRouter()

const userAccount = ref('')
const userPassword = ref('')
const passwordVisible = ref(false)
const accountError = ref('')
const passwordError = ref('')
const formError = ref('')
const loading = ref(false)

const submitClick = async () => {
  accountError.value = userAccount.value.trim() ? '' : '请输入账号'
  passwordError.value = userPassword.value ? '' : '请输入密码'
  formError.value = ''
  if (accountError.value || passwordError.value) {
    return
  }
  loading.value = true
  try {
    await login(userAccount.value.trim(), userPassword.value)
    ElMessage.success('登录成功')
    await router.replace('/')
  } catch (err) {
    formError.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div id="login">
    <main class="login-panel">
      <h1 class="brand">
        <img class="brand-mark brand-mark-for-light" :src="logoDark" alt="" aria-hidden="true" />
        <img class="brand-mark brand-mark-for-dark" :src="logoLight" alt="" aria-hidden="true" />
        <span class="brand-name">Amitia Release</span>
      </h1>

      <form class="login-form" novalidate @submit.prevent="submitClick">
        <div class="field">
          <label class="field-label" for="login-account">账号</label>
          <el-input
            id="login-account"
            v-model="userAccount"
            :prefix-icon="User"
            name="username"
            size="large"
            placeholder="请输入账号"
            autocomplete="username"
            :aria-invalid="Boolean(accountError)"
            :aria-describedby="accountError ? 'login-account-error' : undefined"
            @input="accountError = ''"
          />
          <p v-if="accountError" id="login-account-error" class="field-error" role="alert">
            {{ accountError }}
          </p>
        </div>

        <div class="field">
          <label class="field-label" for="login-password">密码</label>
          <el-input
            id="login-password"
            v-model="userPassword"
            :type="passwordVisible ? 'text' : 'password'"
            :prefix-icon="Lock"
            name="password"
            size="large"
            placeholder="请输入密码"
            autocomplete="current-password"
            :aria-invalid="Boolean(passwordError)"
            :aria-describedby="passwordError ? 'login-password-error' : undefined"
            @input="passwordError = ''"
          >
            <template #suffix>
              <button
                type="button"
                class="reveal"
                :aria-label="passwordVisible ? '隐藏密码' : '显示密码'"
                :aria-pressed="passwordVisible"
                @click="passwordVisible = !passwordVisible"
              >
                <el-icon>
                  <Hide v-if="passwordVisible" />
                  <View v-else />
                </el-icon>
              </button>
            </template>
          </el-input>
          <p v-if="passwordError" id="login-password-error" class="field-error" role="alert">
            {{ passwordError }}
          </p>
        </div>

        <p v-if="formError" class="form-error" role="alert">{{ formError }}</p>

        <el-button class="submit" type="primary" size="large" native-type="submit" :loading="loading">
          {{ loading ? '登录中' : '登录' }}
        </el-button>
      </form>
    </main>
  </div>
</template>
<style scoped>
#login {
  --el-color-primary: var(--BrandPrimary);

  position: relative;
  display: flex;
  flex: 1;
  min-height: 100dvh;
  padding: 24px;
  overflow: hidden;
  background-color: var(--SysBg);
  font-family:
    -apple-system, BlinkMacSystemFont, 'SF Pro Text', 'Segoe UI', 'PingFang SC', 'Microsoft YaHei',
    'Helvetica Neue', sans-serif;
}

#login::before {
  position: absolute;
  inset: -80px;
  background: var(--LoginBgImage) center center / cover no-repeat;
  filter: blur(72px) saturate(95%);
  transform: scale(1.05);
  content: '';
  pointer-events: none;
}

#login::after {
  position: absolute;
  inset: 0;
  background: color-mix(in srgb, var(--SysBg) 56%, transparent);
  content: '';
  pointer-events: none;
}

.login-panel {
  position: relative;
  z-index: 1;
  width: min(380px, 100%);
  margin: auto;
  padding: 36px 34px 32px;
  border: 1px solid color-mix(in srgb, var(--Main_BoxBorderColor) 36%, transparent);
  border-radius: 22px;
  background: color-mix(in srgb, var(--Main_Bg_BoxColor) 84%, transparent);
  box-shadow:
    0 1px 2px rgba(15, 23, 42, 0.06),
    0 22px 52px rgba(15, 23, 42, 0.16);
  -webkit-backdrop-filter: blur(24px) saturate(160%);
  backdrop-filter: blur(24px) saturate(160%);
  animation: login-panel-in 320ms cubic-bezier(0.22, 0.61, 0.36, 1) both;
}

.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 0;
}

.brand-mark {
  width: 36px;
  height: 36px;
  object-fit: contain;
}

.brand-mark-for-dark {
  display: none;
}

html[data-theme='dark'] .brand-mark-for-light {
  display: none;
}

html[data-theme='dark'] .brand-mark-for-dark {
  display: block;
}

.brand-name {
  font-size: 17px;
  font-weight: 600;
  color: var(--Main_font_SecondColor);
}

.login-form {
  display: grid;
  gap: 18px;
  margin-top: 30px;
}

.field {
  display: grid;
  gap: 8px;
}

.field-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--Main_font_SecondColor_Low);
}

.field-error,
.form-error {
  margin: 0;
  font-size: 13px;
  line-height: 1.5;
  color: var(--SysDangerText);
}

#login .login-form :deep(.el-input__wrapper) {
  min-height: 46px;
  padding: 0 14px;
  border: 0;
  border-radius: 12px;
  background-color: color-mix(in srgb, var(--Main_Bg_BoxColor_Secont) 92%, transparent);
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--Main_BoxBorderColor) 50%, transparent);
  transition:
    box-shadow 200ms ease,
    background-color 200ms ease;
}

#login .login-form :deep(.el-input__wrapper:hover) {
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--Main_BoxBorderColor) 78%, transparent);
}

#login .login-form :deep(.el-input__wrapper.is-focus) {
  box-shadow:
    inset 0 0 0 1px var(--BrandPrimary),
    0 0 0 3px var(--FocusRing);
}

#login .login-form :deep(.el-input__inner) {
  font-size: 16px;
  color: var(--Main_font_SecondColor);
}

#login .login-form :deep(.el-input__inner::placeholder) {
  color: var(--Main_font_SecondColor_Low);
}

#login .login-form :deep(.el-input__prefix) {
  color: var(--Main_font_SecondColor_Low);
}

.reveal {
  display: grid;
  width: 40px;
  height: 40px;
  margin-right: -10px;
  place-items: center;
  border: 0;
  border-radius: 10px;
  background: transparent;
  color: var(--Main_font_SecondColor_Low);
  cursor: pointer;
  transition:
    color 200ms ease,
    background-color 200ms ease;
}

.reveal:hover {
  background: color-mix(in srgb, var(--Main_BoxBorderColor) 20%, transparent);
  color: var(--Main_font_SecondColor);
}

.reveal:focus-visible {
  outline: 2px solid var(--BrandPrimary);
  outline-offset: 2px;
}

#login .submit {
  --el-button-bg-color: var(--BrandPrimary);
  --el-button-border-color: var(--BrandPrimary);
  --el-button-text-color: var(--Main_font_FirstColor);
  --el-button-hover-bg-color: var(--BrandPrimaryStrong);
  --el-button-hover-border-color: var(--BrandPrimaryStrong);
  --el-button-hover-text-color: var(--Main_font_FirstColor);
  --el-button-active-bg-color: var(--BrandPrimaryStrong);
  --el-button-active-border-color: var(--BrandPrimaryStrong);
  --el-button-active-text-color: var(--Main_font_FirstColor);
  --el-button-disabled-bg-color: color-mix(in srgb, var(--BrandPrimary) 42%, transparent);
  --el-button-disabled-border-color: transparent;
  --el-button-disabled-text-color: var(--Main_font_FirstColor);

  width: 100%;
  min-height: 50px;
  margin-top: 6px;
  border-radius: 14px;
  font-size: 17px;
  font-weight: 600;
  transition:
    transform 180ms cubic-bezier(0.22, 0.61, 0.36, 1),
    background-color 200ms ease;
}

#login .submit:active {
  transform: scale(0.985);
}

@keyframes login-panel-in {
  from {
    opacity: 0;
    transform: translateY(10px);
  }

  to {
    opacity: 1;
    transform: none;
  }
}

@media (max-width: 480px) {
  .login-panel {
    padding: 32px 24px 28px;
    border-radius: 18px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .login-panel {
    animation: none;
  }

  #login .submit {
    transition: none;
  }
}
</style>
