<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="sw-info">
    <div class="sw-info-card">
      <h3>Privacy &amp; Security Boundaries</h3>
      <ul>
        <li><strong>Single User:</strong> This is a single-user system. No multi-user, tenant, or billing features.</li>
        <li><strong>Data Ownership:</strong> All data is stored locally in your database. No data is sent to third parties.</li>
        <li><strong>Model Calls:</strong> Messages are sent to your configured AI model provider for processing.</li>
        <li><strong>No Registration:</strong> There is no public registration, invitation, or platform admin panel.</li>
      </ul>
    </div>
    <div class="sw-confirm">
      <label>
        <input v-model="confirmedModel" type="checkbox" />
        I understand and accept these boundaries
      </label>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"

const props = defineProps<{ confirmed: boolean }>()
const emit = defineEmits<{ (e: "update:confirmed", v: boolean): void }>()

const confirmedModel = computed({
  get: () => props.confirmed,
  set: (v) => emit("update:confirmed", v)
})
</script>
