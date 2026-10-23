<!-- eslint-disable vue/multi-word-component-names -->
<script setup lang="ts">
import categoriseMune from './categoriseMune/categoriseMune.vue';
import categoriseInfo from './categoriseInfo/categoriseInfo.vue';
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

const categoriseInfoRef = ref<InstanceType<typeof categoriseInfo>>()
const renewInfo = () => {
  categoriseInfoRef.value!.useGetData()
}
</script>
<template>
   <div id="tags">
      <categoriseMune @updateMenuData="updateMenuData" @renewInfo="renewInfo"/>
      <categoriseInfo ref="categoriseInfoRef" :menuPropsData="menuData"/>
   </div>
</template>
<style scoped>
#tags{
  width: 100%;
}
</style>

