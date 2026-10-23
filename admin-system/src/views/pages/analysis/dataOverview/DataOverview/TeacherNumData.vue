<script lang="ts" setup>

</script>

<template>
  <div class="Num1">

  </div>
</template>

<style scoped>
.Num1{
  width: 100%;
  height: 100%;
  padding: 10px 24px;
  border-radius: var(--BorderRadius);
  box-shadow: var(--Main_boxShadow);
  background: var(--Main_Bg_BoxColor);
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-shrink: 0;
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
  height: 100%;
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
  color: #333;
}

.DataIcon{
  height: 60px;
  width: 60px;
  background: linear-gradient(45deg, #895bbe, #6648ff);
  padding: 10px;
  border-radius: var(--BorderRadius);
  margin-left: 16px;
}
</style>
