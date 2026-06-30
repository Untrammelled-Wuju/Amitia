<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="sw-info">
    <div v-if="deployMode === 'cloud-web'" class="sw-info-card">
      <h3>Cloud Deployment</h3>
      <ul>
        <li>Your companion runs 24/7 on the cloud server</li>
        <li>No need to keep your personal computer running</li>
        <li>Access via web browser or WeChat from anywhere</li>
        <li>All data is stored on your own server</li>
      </ul>
    </div>
    <div v-else class="sw-info-card">
      <h3>Desktop Local</h3>
      <ul>
        <li>All services run on this machine</li>
        <li>Data stays on this computer only</li>
        <li>This computer needs to be running for the app to work</li>
      </ul>
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{ deployMode: string }>()
</script>
