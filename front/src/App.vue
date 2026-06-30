<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <PrivacyConsent v-if="!isPublicPage" />
  <AppLayout v-if="!isPublicPage">
    <router-view />
  </AppLayout>
  <router-view v-else />
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue"
import { useRouter, useRoute } from "vue-router"
import { AppLayout } from "./ui-index"
import { apiClient } from "./ui-index"
import PrivacyConsent from "./components/PrivacyConsent.vue"
import { useTheme } from "./ui-index"

const router = useRouter()
const route = useRoute()

const TOKEN_KEY = "ai-companion-token"

const publicPaths = ["/onboarding", "/login", "/setup", "/privacy", "/usage-boundary"]
const isPublicPage = computed(() => publicPaths.some(p => route.path === p || route.path.startsWith(p + "/")))

function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

onMounted(async () => {
  // Initialize theme
  try {
    const { loadFromServer } = useTheme()
    await loadFromServer()
  } catch {}

  // Already on a public page, let the router guard handle it
  if (publicPaths.some(p => route.path === p)) {
    return
  }

  const token = getToken()

  try {
    // Step 1: Check onboarding
    const onboardingRes = await apiClient.get("/api/onboarding/status")
    const onboardingData = onboardingRes.data?.data || onboardingRes.data
    if (!onboardingData?.completed) {
      router.replace("/onboarding")
      return
    }

    // Step 2: Check auth status
    try {
      const authRes = await apiClient.get("/api/auth/status")
      const authData = authRes.data?.data || authRes.data

      if (!authData?.hasAdmin) {
        router.replace("/setup")
        return
      }

      // Step 3: If no token, redirect to login
      if (!token) {
        router.replace("/login")
        return
      }

      // Step 4: Validate existing token
      try {
        const meRes = await apiClient.get("/api/auth/me")
        const userData = meRes.data?.data || meRes.data
        if (!userData?.id) {
          localStorage.removeItem(TOKEN_KEY)
          router.replace("/login")
        }
      } catch {
        localStorage.removeItem(TOKEN_KEY)
        router.replace("/login")
      }
    } catch {
      // auth/status not available, try token validation directly
      if (token) {
        try {
          const meRes = await apiClient.get("/api/auth/me")
          const userData = meRes.data?.data || meRes.data
          if (!userData?.id) {
            localStorage.removeItem(TOKEN_KEY)
            router.replace("/login")
          }
        } catch {
          localStorage.removeItem(TOKEN_KEY)
          router.replace("/login")
        }
      }
    }
  } catch {
    // Core may not be running yet
  }
})
</script>
