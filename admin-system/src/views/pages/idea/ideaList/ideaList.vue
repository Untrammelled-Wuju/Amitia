<!-- eslint-disable vue/multi-word-component-names -->
<script setup lang="ts">
import ideaListMenu from './ideaListMenu/ideaListMenu.vue';
import ideaListInfo from './ideaListInfo/ideaListInfo.vue';
import { ref } from 'vue';
interface MenuDataModel {
  isTop: string
  status: string
  isDeleted: string
  category: string
  startTime: string
  endTime: string
  value: string
  orderWay: 'ASC' | 'DESC'
}
const menuData = ref<MenuDataModel>({
  isTop: '',
  status: '',
  isDeleted: '',
  category: '',
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
      <ideaListMenu @updateMenuData="updateMenuData"/>
      <ideaListInfo :menuPropsData="menuData"/>
   </div>
</template>
<style scoped>
#ideaList{
  width: 100%;
}
</style>

