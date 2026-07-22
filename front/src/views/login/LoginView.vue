<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="login-shell">
    <div class="login-world">
      <div class="starfield">
        <i v-for="s in stars" :key="s.id" class="star" :style="s.style" />
      </div>

      <div class="core-zone">
        <div class="core-halo"></div>
        <div class="orbit o1"></div>
        <div class="orbit o2"></div>
        <div class="orbit o3"></div>
        <div class="core-symbol" aria-hidden="true">
          <span class="sigil-loop loop-a"></span>
          <span class="sigil-loop loop-b"></span>
          <span class="sigil-loop loop-c"></span>
          <span class="sigil-center"></span>
          <span class="sigil-node node-a"></span>
          <span class="sigil-node node-b"></span>
          <span class="sigil-node node-c"></span>
        </div>
        <div class="core-caption">Amitia</div>
      </div>

      <div class="login-panel-wrapper">
        <div class="login-panel">
          <div class="login-title">登录 Amitia</div>
          <div class="login-desc">登录后进入管理页面，保护配置、聊天记录和记忆数据。</div>

          <div v-if="checkingStatus" class="login-status">
            <span class="ob-spin-icon"></span>
            <span>正在检查服务状态...</span>
          </div>

          <template v-else>
            <div class="login-form">
              <label class="login-input-label">
                账号
                <input v-model="form.username" autocomplete="username" placeholder="请输入账号" @keyup.enter="handleLogin" />
              </label>
              <label class="login-input-label">
                密码
                <input v-model="form.password" type="password" autocomplete="current-password" placeholder="请输入密码" @keyup.enter="handleLogin" />
              </label>
            </div>
            <div class="login-error">{{ errorMsg }}</div>
            <button class="login-action" :disabled="loading" @click="handleLogin">
              <span v-if="loading" class="ob-spin-icon"></span>
              {{ loading ? '登录中...' : '登录' }}
            </button>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from "vue"
import { useRouter, useRoute } from "vue-router"
import { ElMessage } from "element-plus"
import { apiClient, setToken } from "../../composables/useApi"

const router = useRouter()
const route = useRoute()
const loading = ref(false)
const checkingStatus = ref(true)
const errorMsg = ref("")

const form = reactive({ username: "", password: "" })

const stars = Array.from({ length: 56 }, (_, i) => ({
  id: i,
  style: {
    left: `${Math.random() * 100}%`,
    top: `${Math.random() * 100}%`,
    '--dur': `${12 + Math.random() * 24}s`,
    '--op': (0.08 + Math.random() * 0.25).toFixed(2),
    '--dx': `${(-18 + Math.random() * 36).toFixed(1)}px`,
    animationDelay: `${(-Math.random() * 20).toFixed(1)}s`,
  },
}))

onMounted(() => {
  checkingStatus.value = false
})

async function handleLogin() {
  errorMsg.value = ""
  const name = form.username.trim()
  const pw = form.password

  if (!name || !pw) {
    errorMsg.value = "请输入账号和密码"
    return
  }

  loading.value = true
  try {
    const res = await apiClient.post("/api/auth/login", {
      username: name,
      password: pw,
    })
    const data = res.data?.data || res.data
    if (data?.token) {
      setToken(data.token)
      ElMessage.success(`欢迎回来，${data.username || name}`)
      const redirect = (route.query.redirect as string) || "/chat"
      router.push(redirect)
    }
  } catch {
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>


.login-shell {
  width: 100%;
  height: 100%;
  overflow: hidden;
  background: var(--tp-page);
  display: grid;
  place-items: center;
}

.login-world {
  position: relative;
  width: 100%;
  height: 100%;
  font-family: Inter, ui-sans-serif, -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif;
  background: var(--tp-page);
  color: var(--tp-text);
  overflow: hidden;
}

.login-world button,
.login-world input {
  font: inherit;
  box-sizing: border-box;
}

.login-world button {
  color: inherit;
}

.login-world button:focus-visible,
.login-world input:focus-visible {
  outline: none;
}

.login-world::before {
  content: "";
  position: fixed;
  inset: 0;
  pointer-events: none;
  opacity: 0.24;
  background-image: url("data:image/svg+xml,%3Csvg viewBox='0 0 180 180' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='.8' numOctaves='2' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23n)' opacity='.08'/%3E%3C/svg%3E");
  mix-blend-mode: soft-light;
}

.starfield {
  position: absolute;
  inset: 0;
  overflow: hidden;
  pointer-events: none;
}

.star {
  position: absolute;
  width: 2px;
  height: 2px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.34);
  animation: star-drift var(--dur) linear infinite;
  opacity: var(--op);
}

@keyframes star-drift {
  from { transform: translate3d(0, 14px, 0); }
  50% { opacity: calc(var(--op) * 0.4); }
  to { transform: translate3d(var(--dx), -26px, 0); }
}

.core-zone {
  position: absolute;
  left: 50%;
  top: 47%;
  width: 350px;
  height: 350px;
  transform: translate(calc(-50% - 202.5px), -50%);
  display: grid;
  place-items: center;
}

.core-halo {
  position: absolute;
  inset: 5%;
  border-radius: 50%;
  background: radial-gradient(circle, rgba(200, 121, 91, 0.15), rgba(200, 121, 91, 0.035) 45%, transparent 70%);
  filter: blur(6px);
  opacity: 0.5;
}

.orbit {
  position: absolute;
  border: 1px solid rgba(255, 255, 255, 0.075);
  border-radius: 50%;
}

.orbit.o1 {
  inset: 6%;
  animation: orbit-spin 28s linear infinite;
}

.orbit.o2 {
  inset: 15%;
  border-style: dashed;
  animation: orbit-spin-reverse 34s linear infinite;
}

.orbit.o3 {
  inset: 25%;
  opacity: 0.55;
  animation: orbit-spin 22s linear infinite;
}

@keyframes orbit-spin {
  to { transform: rotate(360deg); }
}

@keyframes orbit-spin-reverse {
  to { transform: rotate(-360deg); }
}

.orbit::after {
  content: "";
  position: absolute;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  left: 50%;
  top: -4px;
  transform: translateX(-50%);
  background: rgba(255, 255, 255, 0.22);
  box-shadow: 0 0 18px rgba(255, 255, 255, 0.12);
}

.core-symbol {
  position: relative;
  width: 54%;
  height: 54%;
  opacity: 0.3;
  transform: scale(0.92) rotate(-2deg);
  animation: core-breathe 5.8s ease-in-out infinite;
  filter: drop-shadow(0 0 14px rgba(255, 255, 255, 0.04));
}

.sigil-loop {
  position: absolute;
  left: 50%;
  top: 50%;
  width: 76%;
  height: 38%;
  border: 3px solid currentColor;
  border-radius: 50%;
  color: rgba(242, 239, 233, 0.78);
  transform-origin: center;
  box-shadow: 0 0 22px rgba(255, 255, 255, 0.025) inset;
}

.loop-a {
  transform: translate(-50%, -50%) rotate(0deg);
}

.loop-b {
  transform: translate(-50%, -50%) rotate(60deg);
}

.loop-c {
  transform: translate(-50%, -50%) rotate(120deg);
}

.sigil-center {
  position: absolute;
  left: 50%;
  top: 50%;
  width: 18%;
  height: 18%;
  transform: translate(-50%, -50%) rotate(45deg);
  border: 1px solid rgba(224, 154, 125, 0.68);
  border-radius: 34%;
  background: radial-gradient(circle at 35% 35%, rgba(224, 154, 125, 0.22), rgba(200, 121, 91, 0.045));
  box-shadow: 0 0 26px rgba(200, 121, 91, 0.16);
}

.sigil-node {
  position: absolute;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--tp-primary-hover);
  box-shadow: 0 0 16px rgba(224, 154, 125, 0.52);
}

.node-a {
  left: 50%;
  top: 8%;
  transform: translateX(-50%);
}

.node-b {
  right: 12%;
  bottom: 24%;
}

.node-c {
  left: 12%;
  bottom: 24%;
}

@keyframes core-breathe {
  0%, 100% { transform: scale(0.92) rotate(-2deg); }
  50% { transform: scale(0.955) rotate(1deg); }
}

.core-caption {
  position: absolute;
  top: 78%;
  left: 50%;
  transform: translateX(-50%);
  width: 340px;
  text-align: center;
  color: #706b64;
  font-size: 11px;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.login-panel-wrapper {
  position: absolute;
  left: 50%;
  top: 47%;
  transform: translate(calc(-50% + 187.5px), -50%);
}

.login-panel {
  width: min(380px, 34vw);
  padding: 28px;
  border: 1px solid var(--tp-border);
  border-radius: 24px;
  background: var(--tp-panel);
  backdrop-filter: blur(18px);
  box-shadow: var(--tp-shadow-float);
  display: flex;
  flex-direction: column;
  align-items: center;
}


.login-title {
  font-size: 28px;
  font-weight: 570;
  letter-spacing: -0.03em;
  margin-bottom: 8px;
}

.login-desc {
  color: var(--tp-text-secondary);
  font-size: 12px;
  line-height: 1.65;
  margin-bottom: 22px;
}

.login-status {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 32px 0;
  color: var(--tp-text-secondary);
  font-size: 12px;
}

.ob-spin-icon {
  display: inline-block;
  width: 11px;
  height: 11px;
  border: 1.5px solid currentColor;
  border-top-color: transparent;
  border-radius: 50%;
  animation: ob-spin 0.6s linear infinite;
  vertical-align: middle;
}

@keyframes ob-spin {
  to { transform: rotate(360deg); }
}

.login-form {
  display: grid;
  gap: 13px;
  width: 100%;
}

.login-input-label {
  display: grid;
  gap: 7px;
  color: var(--tp-text-secondary);
  font-size: 11px;
}

.login-input-label input {
  width: 100%;
  height: 44px;
  padding: 0 12px;
  border: 1px solid var(--tp-border);
  border-radius: 11px;
  background: var(--tp-control);
  color: var(--tp-text);
  outline: none;
}

.login-input-label input:focus {
  border-color: var(--tp-primary-ring);
}

.login-input-label input:hover {
  border-color: var(--tp-primary-border);
}

.login-input-label input::placeholder {
  color: var(--tp-text-muted);
}

.login-error {
  min-height: 18px;
  margin-top: 10px;
  color: var(--tp-danger);
  font-size: 11px;
}

.login-action {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  margin-top: 6px;
  height: 40px;
  padding: 0 56px;
  border: 1px solid var(--tp-primary);
  border-radius: 12px;
  background: var(--tp-primary);
  color: var(--tp-text-on-primary);
  font-size: 13px;
  font-weight: 590;
  cursor: pointer;
  box-shadow: 0 14px 40px var(--tp-primary-soft);
  transition: background 0.2s ease;
}

.login-action:hover {
  background: var(--tp-primary-hover);
}

.login-action:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}

@media (max-width: 860px) {

  .login-panel-wrapper {
    left: 50%;
    top: auto;
    bottom: 10%;
    transform: translate(-50%, 0);
  }

  .login-panel {
    width: min(340px, 88vw);
  }
}

@media (max-width: 480px) {

  .login-panel-wrapper {
    bottom: 6%;
  }
}
</style>

<style>
html[data-theme="light"] .star {
  background: rgba(0, 0, 0, 0.16);
}

html[data-theme="light"] .orbit {
  border-color: rgba(0, 0, 0, 0.05);
}

html[data-theme="light"] .orbit::after {
  background: rgba(0, 0, 0, 0.12);
  box-shadow: 0 0 18px rgba(0, 0, 0, 0.06);
}

html[data-theme="light"] .core-halo {
  opacity: 0.65;
}

html[data-theme="light"] .core-symbol {
  filter: drop-shadow(0 0 14px rgba(0, 0, 0, 0.06));
}

html[data-theme="light"] .sigil-loop {
  color: rgba(60, 52, 44, 0.32);
  box-shadow: 0 0 22px rgba(0, 0, 0, 0.025) inset;
}

html[data-theme="light"] .sigil-center {
  border-color: rgba(160, 110, 60, 0.5);
  background: radial-gradient(circle at 35% 35%, rgba(160, 110, 60, 0.16), rgba(155, 100, 45, 0.04));
  box-shadow: 0 0 26px rgba(160, 110, 60, 0.12);
}

html[data-theme="light"] .login-world::before {
  mix-blend-mode: overlay;
  opacity: 0.12;
}

html[data-theme="light"] .core-caption {
  color: var(--tp-text-muted);
}
</style>

