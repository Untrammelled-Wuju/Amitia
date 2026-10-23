<script setup lang="ts">

</script>
<template>
  <div class="workbench-1LeftTop">

  </div>
</template>
<style scoped>
.workbench-1LeftTop{
  height: 100%;
  width: 32.5%;
  background: var(--Main_Bg_BoxColor);
  border-radius: var(--BorderRadius);
  box-shadow: var(--Main_boxShadow);
  display: flex;
  justify-content: center;
  flex-direction: column;
  flex-shrink: 0;
  padding: 10px;
  .notice{
    margin-top: 20px;
    font-size: 16px;
    display: flex;
    align-items: center;
    span{
      color: var(--admin-danger);
    }
  }
}
.flex-box {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  height: 50%;
}
.el-col {
  text-align: center;
}
:global(h2#card-usage ~ .example .example-showcase) {
  background-color: var(--el-fill-color) !important;
}
.el-statistic {
  --el-statistic-content-font-size: 28px;
  display: flex;
  flex-direction: column;
  align-items: start;
}
.statistic-card {
  display: flex;
  flex-direction: column;
  align-items: start;
}
.statistic-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  font-size: 12px;
  color: var(--el-text-color-regular);
  margin-top: 2px;
}
.statistic-footer .footer-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.statistic-footer .footer-item span:last-child {
  display: inline-flex;
  align-items: center;
  margin-left: 4px;
}
.SexNum {
  color: var(--Main_font_SecondColor);
}

.DataIcon{
  height: 60px;
  width: 60px;
  background: var(--SysSuccess);
  padding: 10px;
  border-radius: var(--BorderRadius);
  margin-left: 16px;
}
</style>
