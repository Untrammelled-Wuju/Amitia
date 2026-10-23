<!-- eslint-disable vue/multi-word-component-names -->
<script setup lang="ts">
import projectListMenu from './projectListMenu/projectListMenu.vue';
import projectListInfo from './projectListInfo/projectListInfo.vue';
import { ref } from 'vue';
interface MenuDataModel {
  status: string
  isTop: number
  isDeleted: string
  startTime: string
  endTime: string
  value: string
  orderWay: 'ASC' | 'DESC'
}
const menuData = ref<MenuDataModel>({
  status: '0',
  isTop: 1,
  isDeleted: '',
  startTime: '',
  endTime: '',
  value: '',
  orderWay: 'DESC',
})
const updateMenuData = (data:MenuDataModel) => {
  menuData.value = data
  console.log(menuData.value)
}
</script>
<template>
   <div id="ideaList">
      <projectListMenu @updateMenuData="updateMenuData"/>
      <projectListInfo :menuPropsData="menuData"/>
   </div>
</template>
<style scoped>
#ideaList{
  width: 100%;
}
</style>

