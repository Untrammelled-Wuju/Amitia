<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="sw-form">
    <div v-if="deployMode === 'desktop-local'" class="sw-notice">
      <strong>Security Notice:</strong> In desktop local mode, anyone with access to this computer can access the app. Setting a password is recommended but optional.
    </div>
    <div v-if="deployMode === 'cloud-web'" class="sw-notice sw-notice-warn">
      <strong>Required:</strong> Cloud deployment requires an admin password to protect your data from unauthorized access.
    </div>
    <div class="sw-field">
      <label>Username</label>
      <input v-model="usernameModel" type="text" placeholder="Admin username" maxlength="32" />
    </div>
    <div class="sw-field">
      <label>Password</label>
      <input v-model="passwordModel" type="password" placeholder="At least 6 characters" maxlength="64" />
    </div>
    <div class="sw-field">
      <label>Confirm Password</label>
      <input v-model="confirmPasswordModel" type="password" placeholder="Repeat password" maxlength="64" />
    </div>
    <div v-if="deployMode === 'desktop-local'" class="sw-skip-option">
      <label>
        <input v-model="skipAuthModel" type="checkbox" />
        Skip password setup (not recommended)
      </label>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"

const props = defineProps<{
  username: string
  password: string
  confirmPassword: string
  skipAuth: boolean
  deployMode: string
}>()

const emit = defineEmits<{
  (e: "update:username", v: string): void
  (e: "update:password", v: string): void
  (e: "update:confirmPassword", v: string): void
  (e: "update:skipAuth", v: boolean): void
}>()

const usernameModel = computed({ get: () => props.username, set: (v) => emit("update:username", v) })
const passwordModel = computed({ get: () => props.password, set: (v) => emit("update:password", v) })
const confirmPasswordModel = computed({ get: () => props.confirmPassword, set: (v) => emit("update:confirmPassword", v) })
const skipAuthModel = computed({ get: () => props.skipAuth, set: (v) => emit("update:skipAuth", v) })
</script>
