<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="step-panel">
    <template v-if="hasAdmin">
      <h2>管理员账号已设置</h2>
      <p class="step-desc">管理员账号已在之前的设置中创建，请输入密码继续。</p>
      <el-alert type="success" :closable="false" show-icon style="margin-bottom:16px">
        <template #title>账号已就绪，无需重新创建</template>
      </el-alert>
      <el-form label-position="top" size="default">
        <el-form-item label="用户名">
          <el-input v-model="usernameModel" placeholder="输入已设置的用户名" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="passwordModel" type="password" placeholder="输入密码" show-password />
        </el-form-item>
      </el-form>
    </template>
    <template v-else>
      <h2>设置管理员账号</h2>
      <p class="step-desc">创建管理员账号以保护你的数据安全。</p>
      <el-form label-position="top" size="default">
        <el-form-item label="用户名">
          <el-input v-model="usernameModel" placeholder="设置管理员用户名" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="passwordModel" type="password" placeholder="至少 6 位" show-password />
        </el-form-item>
        <el-form-item label="确认密码">
          <el-input v-model="password2Model" type="password" placeholder="再次输入密码" show-password />
        </el-form-item>
      </el-form>
    </template>
  </div>
</template>
<script setup lang="ts">
import { computed } from "vue"
const props = defineProps<{ username: string; password: string; password2: string; hasAdmin: boolean }>()
const emit = defineEmits<{
  (e: "update:username", v: string): void
  (e: "update:password", v: string): void
  (e: "update:password2", v: string): void
}>()
const usernameModel = computed({ get: () => props.username, set: (v) => emit("update:username", v) })
const passwordModel = computed({ get: () => props.password, set: (v) => emit("update:password", v) })
const password2Model = computed({ get: () => props.password2, set: (v) => emit("update:password2", v) })
</script>
