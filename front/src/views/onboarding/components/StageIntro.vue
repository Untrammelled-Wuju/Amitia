<template>
  <div class="ob-stage-inner">
    <div class="ob-intro-copy">
      <div class="kicker">初次设置</div>
      <h1 class="ob-title">从这里，<br>开始认识彼此。</h1>
      <p class="ob-subtitle">先确定运行方式并连接模型能力，再设置角色和初始认知。称呼、记忆与相处方式，之后仍可随时调整。</p>
      <button class="ob-primary-ghost" @click="$emit('next')">开始设置</button>
    </div>
  </div>
</template>

<script setup lang="ts">
defineEmits<{ next: [] }>()
</script>
