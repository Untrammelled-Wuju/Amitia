<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="login-world">
    <main class="login-card" aria-labelledby="web-access-title">
      <div class="login-mark">A</div>
      <h1 id="web-access-title">访问云端</h1>
      <p>请输入此 Cloud Core 的 Web 访问密码。</p>

      <form class="login-form" @submit.prevent="submit">
        <input
          v-model="password"
          class="login-input"
          type="password"
          autocomplete="current-password"
          maxlength="128"
          placeholder="访问密码"
          autofocus
          :disabled="busy"
        />
        <div v-if="error" class="login-error" role="alert">{{ error }}</div>
        <button class="login-button" type="submit" :disabled="busy || !password">
          {{ busy ? "正在验证" : "登录" }}
        </button>
      </form>
    </main>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { getApiBaseURL } from "@/runtime/runtime-adapter";
import { getWebAccessStatus, loginWebAccess } from "@/runtime/web-access";

const route = useRoute();
const router = useRouter();
const password = ref("");
const busy = ref(false);
const error = ref("");

function safeRedirect(): string {
  const raw = typeof route.query.redirect === "string" ? route.query.redirect : "";
  return raw.startsWith("/") && !raw.startsWith("//") && raw !== "/web-access" ? raw : "/chat";
}

async function submit() {
  if (busy.value || !password.value) return;
  busy.value = true;
  error.value = "";
  try {
    const baseURL = await getApiBaseURL();
    await loginWebAccess(baseURL, password.value);
    await router.replace(safeRedirect());
  } catch (e: any) {
    error.value = e?.message || "访问密码错误";
    password.value = "";
  } finally {
    busy.value = false;
  }
}

onMounted(async () => {
  if (window.amitiaDesktop) {
    await router.replace("/chat");
    return;
  }
  try {
    const baseURL = await getApiBaseURL();
    const status = await getWebAccessStatus(baseURL);
    if (!status.configured) {
      await router.replace("/onboarding");
    } else if (status.authenticated) {
      await router.replace(safeRedirect());
    }
  } catch {
    // Keep the login form visible; submit will surface a concrete error.
  }
});
</script>

<style scoped>
.login-world {
  min-height: 100vh;
  display: grid;
  place-items: center;
  box-sizing: border-box;
  padding: 28px;
  background: radial-gradient(circle at 50% 20%, rgba(111, 126, 255, 0.12), transparent 36%), var(--el-bg-color, #fff);
  color: var(--el-text-color-primary, #1f2329);
}
.login-card {
  width: min(390px, 100%);
  display: grid;
  gap: 14px;
  padding: 34px;
  box-sizing: border-box;
  border: 1px solid var(--el-border-color-lighter, #e5e7eb);
  border-radius: 20px;
  background: color-mix(in srgb, var(--el-bg-color, #fff) 94%, transparent);
  box-shadow: 0 18px 60px rgba(0, 0, 0, 0.08);
}
.login-mark {
  width: 42px;
  height: 42px;
  display: grid;
  place-items: center;
  border-radius: 12px;
  background: var(--el-color-primary, #8a5728);
  color: white;
  font-size: 20px;
  font-weight: 700;
}
h1 {
  margin: 4px 0 0;
  font-size: 24px;
}
p {
  margin: 0;
  color: var(--el-text-color-secondary, #606266);
  font-size: 14px;
}
.login-form {
  display: grid;
  gap: 12px;
  margin-top: 8px;
}
.login-input {
  width: 100%;
  height: 44px;
  box-sizing: border-box;
  border: 1px solid var(--el-border-color, #dcdfe6);
  border-radius: 10px;
  padding: 0 13px;
  background: var(--el-fill-color-blank, #fff);
  color: inherit;
  outline: none;
}
.login-input:focus {
  border-color: var(--el-color-primary, #8a5728);
}
.login-button {
  height: 44px;
  border: 0;
  border-radius: 10px;
  background: var(--el-color-primary, #8a5728);
  color: #fff;
  cursor: pointer;
  font-weight: 600;
}
.login-button:disabled {
  cursor: default;
  opacity: 0.55;
}
.login-error {
  font-size: 13px;
  color: var(--el-color-danger, #f56c6c);
}
</style>
