<script setup lang="ts">
import studentNumData from './studentNumData.vue'
import TeacherNumData from './TeacherNumData.vue'
import CourseNumData from './CourseNumData.vue'
</script>
<template>
  <div id="DataOverview">
    <div class="title">·&nbsp;数据统计</div>
    <div class="content">
      <div class="content-left">
        <studentNumData />
      </div>
      <div class="content-right">
        <TeacherNumData />
        <CourseNumData />
      </div>
    </div>
  </div>
</template>
<style scoped>
#DataOverview {
  --DataOverviewNavHeight: 320px;
  width: 100%;
  height: var(--DataOverviewNavHeight);
}
.title {
  height: 32px;
    color: #666;
    padding-left: 10px;
    display: flex;
    align-items: center;
}
.content {
  width: 100%;
  height: calc(100% - 30px);
  display: flex;
  justify-content: space-between;
  align-items: center;
  .content-left {
    height: 100%;
    width: 69%;
    min-width: 600px;
  }
  .content-right{
    height: 100%;
    width: 30%;
    margin-left: 1%;
    display: grid;
    grid-template-columns: repeat(1, 1fr);
    grid-template-rows: repeat(2, 1fr);
    grid-gap: 16px;
  }
}
</style>
