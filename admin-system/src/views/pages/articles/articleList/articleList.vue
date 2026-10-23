<!-- eslint-disable vue/multi-word-component-names -->
<script setup lang="ts">
import articleListMenu from './articleListMenu/articleListMenu.vue';
import articleListInfo from './articleListInfo/articleListInfo.vue';
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
  isDeleted: '',
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
const updateMenuData = (data:MenuDataModel) => {
  menuData.value = data
  console.log(menuData.value)
}
</script>
<template>
   <div id="articleList">
      <articleListMenu @updateMenuData="updateMenuData"/>
      <articleListInfo :menuPropsData="menuData"/>
   </div>
</template>
<style scoped>
#articleList{
  width: 100%;
}
</style>

