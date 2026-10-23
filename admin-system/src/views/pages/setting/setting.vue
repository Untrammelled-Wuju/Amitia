<!-- eslint-disable vue/multi-word-component-names -->
<script setup lang="ts">
import settingMenu from './settingMenu/settingMenu.vue';
</script>
<template>
  <div id="setting">
    <settingMenu />
    <div id="settingContent">
      <RouterView />
    </div>
  </div>
</template>
<style scoped>
#settingContent{
  width: 100%;
  height: 790px;
  background: var(--Main_Bg_BoxColor);
  border-radius: var(--BorderRadius);
  margin-top: 12px;
  padding: 10px;
  box-shadow: 0px 0px 10px rgba(0, 0, 0, 0.1);
  overflow: scroll;
  &::-webkit-scrollbar{
    display: none;
  }
}
</style>
