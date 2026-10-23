 <!-- eslint-disable vue/multi-word-component-names -->
<script setup lang="ts">
import commentModerationMune from './commentModerationMune/commentModerationMune.vue';
import commentModerationInfo from './commentModerationInfo/commentModerationInfo.vue';
import { ref } from 'vue';
interface MenuDataModel {
  name: string
  orderWay: string
}
const menuData = ref<MenuDataModel>({
  name: '',
  orderWay: '1',
})
const updateMenuData = (data: MenuDataModel) => {
  menuData.value = data
}

const commentModerationInfoRef = ref<InstanceType<typeof commentModerationInfo>>()
const renewInfo = () => {
  commentModerationInfoRef.value!.useGetData()
}
</script>
<template>
   <div id="tags">
      <commentModerationMune @updateMenuData="updateMenuData" @renewInfo="renewInfo"/>
      <commentModerationInfo ref="commentModerationInfoRef" :menuPropsData="menuData"/>
   </div>
</template>
<style scoped>
#tags{
  width: 100%;
}
</style>

