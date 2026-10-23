<script setup>
import centerLeft from './center-left.vue';
import centerRight from './center-right.vue';
</script>
<template>
  <div class="workbench-1-center">
    <centerLeft />
    <centerRight />
  </div>
</template>
<style scoped>
.workbench-1-center {
  width: 100%;
  height: 100%;
  display: flex;
  justify-content: space-between;
}
</style>
