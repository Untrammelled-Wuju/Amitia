<template>
  <div class="sw-form">
    <div class="sw-notice">
      Click "Test Connection" to verify your model API is reachable and working.
    </div>
    <div v-if="testResult" class="sw-test-result" :class="testResult.passed ? 'success' : 'failed'">
      <p>{{ testResult.message }}</p>
      <p v-if="testResult.detail" class="sw-test-detail">{{ testResult.detail }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  testResult: { passed: boolean; message: string; detail?: string } | null
}>()
</script>
