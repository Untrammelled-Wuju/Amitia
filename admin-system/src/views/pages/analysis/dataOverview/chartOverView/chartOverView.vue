<script setup lang="ts">
import chartOne from './chartOne/chartOne.vue';
import chartTwe from './chartTwe/chartTwe.vue';
</script>
<template>
  <div id="chartOverView">
    <div class="Title">·&nbsp;数据概览</div>
    <div class="content">
      <div id="leftChart">
        <chartOne />
      </div>
      <div id="rightChart">
        <chartTwe />
      </div>
    </div>
  </div>
</template>
<style scoped>
#chartOverView {
  margin-top: 12px;
  height: 400px;
  width: 100%;
  .Title{
    width: 100%;
    height: 32px;
    color: #666;
    padding-left: 10px;
    display: flex;
    align-items: center;
  }
  .content{
    width: 100%;
    height: calc(100% - 32px);
    display: flex;
    justify-content: space-between;
  }
  #leftChart,
  #rightChart {
    height: 100%;
    background: var(--Main_Bg_BoxColor);
    border-radius: var(--BorderRadius);
    box-shadow: var(--Main_boxShadow);
    padding: 10px;
  }
  #leftChart {
    width: 39%;
    min-width: 540px;
  }
  #rightChart {
    width: 60%;
    margin-left: 20px;
    min-width: 840px;
  }
}
</style>
