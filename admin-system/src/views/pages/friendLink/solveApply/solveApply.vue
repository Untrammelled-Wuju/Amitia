 <!-- eslint-disable vue/multi-word-component-names -->
<script setup lang="ts">
import solveApplyMune from './solveApplyMune/solveApplyMune.vue';
import solveApplyInfo from './solveApplyInfo/solveApplyInfo.vue';
import { ref } from 'vue';
interface MenuDataModel {
  value: string
  orderWay: string
  isDeleted: string
  startTime: string
  endTime: string
  status: string
}
const menuData = ref<MenuDataModel>({
  value: '',
  orderWay: '',
  isDeleted: '',
  startTime: '',
  endTime: '',
  status: '0',
})
const updateMenuData = (data: MenuDataModel) => {
  menuData.value = data
}

const solveApplyInfoRef = ref<InstanceType<typeof solveApplyInfo>>()
const renewInfo = () => {
  solveApplyInfoRef.value!.useGetData()
}
</script>
<template>
   <div id="tags">
      <solveApplyMune @updateMenuData="updateMenuData" @renewInfo="renewInfo"/>
      <solveApplyInfo ref="solveApplyInfoRef" :menuPropsData="menuData"/>
   </div>
</template>
<style scoped>
#tags{
  width: 100%;
}
</style>

