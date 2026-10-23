<template>
  <div class="Num1">
    <div class="Num1-left">
      <Num1LeftChart />
    </div>
    <div class="Num1-right">
      <Num1RightChart />
    </div>
  </div>
</template>

<script setup lang="ts">
import Num1LeftChart from './echart/Num1LeftChart.vue';
import Num1RightChart from './echart/Num1RightChart.vue';
</script>

<style scoped>
.Num1 {
  width: 100%;
  height: 100%;
  padding: 10px;
  border-radius: var(--BorderRadius);
  box-shadow: var(--Main_boxShadow);
  background: var(--Main_Bg_BoxColor);
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-shrink: 0;
  .Num1-left {
    width: 50%;
    height: 100%;
  }
  .Num1-right {
    width: 50%;
    height: 100%;
  }
}


</style>
