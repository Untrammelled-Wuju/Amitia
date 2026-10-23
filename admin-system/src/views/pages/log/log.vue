<!-- eslint-disable vue/multi-word-component-names -->
<script setup lang="ts">
import logMenu from './logMenu/logMenu.vue';
</script>
<template>
  <div id="Log">
    <logMenu />
    <div id="logContent">
      <RouterView />
    </div>
  </div>
</template>
<style scoped>
#logContent{
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
