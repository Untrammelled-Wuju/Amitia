<!-- eslint-disable vue/multi-word-component-names -->
<script setup lang="ts">
import recycleBinMune from './recycleBinMune/recycleBinMune.vue';
import recycleBinInfo from './recycleBinInfo/recycleBinInfo.vue';
import { ref } from 'vue';
interface MenuDataModel {
  isTop: string
  status: string
  isDeleted: string
  IsComment: string
  categoryId: string
  tagId: string
  startTime: string
  endTime: string
  title: string
  orderWay: string
  isCarousel: string
  type: string
}
const menuData = ref<MenuDataModel>({
  isTop: '',
  status: '0',
  isDeleted: '1',
  IsComment: '',
  categoryId: '',
  tagId: '',
  startTime: '',
  endTime: '',
  title: '',
  orderWay: '',
  isCarousel: '',
  type: '',
})
const updateMenuData = (data: MenuDataModel) => {
  menuData.value = data
}

</script>
<template>
   <div id="recycleBin">
      <recycleBinMune @updateMenuData="updateMenuData"/>
      <recycleBinInfo :menuPropsData="menuData"/>
   </div>
</template>
<style scoped>
#recycleBin{
  width: 100%;
}
</style>

