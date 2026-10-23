<!-- eslint-disable vue/multi-word-component-names -->
<script setup lang="ts">
import workbench1LeftTop from './workbench-1LeftTop/workbench-1LeftTop1.vue';
import workbench1LeftTop2 from './workbench-1LeftTop/workbench-1LeftTop2.vue';
import workbench1LeftTop3 from './workbench-1LeftTop/workbench-1LeftTop3.vue';
import workbench1Right from './workbench-1-right/workbench-1-right.vue';
import workbench1Center from './workbench-1-center/workbench-1-center.vue';
import workbench1Center2 from './workbench-1-center-2/workbench-1-center-2.vue';
import representative from './workbench-1-right/representative.vue';
</script>
<template>
   <div id="workbench">
    <div class="workbench-1">
      <div class="workbench-1-left">
        <div class="title">· 日程提醒</div>
        <div class="workbench-1-left-top">
          <workbench1LeftTop />
          <workbench1LeftTop2 />
          <workbench1LeftTop3 />
        </div>
        <div class="title">· 数据一览</div>
        <div class="workbench-1-center">
          <workbench1Center />
        </div>
        <div class="workbench-1-center-2">
          <workbench1Center2 />
        </div>
      </div>
      <div class="workbench-1-right">
        <workbench1Right />
        <representative />
      </div>
    </div>
   </div>
</template>
<style scoped>
#workbench{
  width: 100%;
  display: flex;
  flex-direction: column;
}
.title{
  margin-left: 2px;
  margin-bottom: 5px;
  color: var(--Main_font_SecondColor);
  margin-top: 12px;
  &:first-child{
    margin-top: 0;
  }
}
.workbench-1{
  /* height: 700px; */
  width: 100%;
  display: flex;
  justify-content: space-between;
  .workbench-1-left{
    width: 79%;
    height: 100%;
    display: flex;
    flex-direction: column;
    .workbench-1-left-top{
      width: 100%;
      height: 160px;
      display: flex;
      justify-content: space-between;
    }
    .workbench-1-center{
      width: 100%;
      height: 280px;
    }
    .workbench-1-center-2{
      margin-top: 12px;
      width: 100%;
      height: 320px;
    }
  }
  .workbench-1-right{
    width: 20%;
    height: 100%;
    margin-left: 1%;
    display: flex;
    flex-direction: column;
  }
}
</style>
