<!-- eslint-disable vue/multi-word-component-names -->
<script setup lang="ts">
import tagsMune from './tagsMune/tagsMune.vue';
import tagsInfo from './tagsInfo/tagsInfo.vue';
import { ref } from 'vue';
interface MenuDataModel {
  name: string
  orderWay: string
}
const menuData = ref<MenuDataModel>({
  name: '',
  orderWay: '',
})
const updateMenuData = (data: MenuDataModel) => {
  menuData.value = data
}

const tagsInfoRef = ref<InstanceType<typeof tagsInfo>>()
const renewInfo = () => {
  tagsInfoRef.value!.useGetData()
}
</script>
<template>
   <div id="tags">
      <tagsMune @updateMenuData="updateMenuData" @renewInfo="renewInfo"/>
      <tagsInfo ref="tagsInfoRef" :menuPropsData="menuData"/>
   </div>
</template>
<style scoped>
#tags{
  width: 100%;
}
</style>

