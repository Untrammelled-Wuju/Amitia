<!-- eslint-disable vue/multi-word-component-names -->
<script setup lang="ts">
import { RouterView } from 'vue-router'
import NProgress from 'nprogress'
import 'nprogress/nprogress.css'

// NProgress配置
NProgress.configure({
  showSpinner: false,
  parent: 'html',
  easing: 'ease',
  speed: 500,
})
</script>
<template>
   <div id="navigation">
    <RouterView class="mainPage" v-slot="{ Component }">
      <Transition
        name="fade-slide"
        mode="out-in"
        :duration="{ enter: 500, leave: 200 }"
        @before-enter="NProgress.start()"
        @after-enter="NProgress.done()"
        @after-leave="NProgress.done()"
      >
        <component :is="Component"/>
      </Transition>
    </RouterView>
   </div>
</template>
<style scoped>
#navigation{
  width: 100%;
}
/* 页面切换过渡动画 */
.fade-slide-enter-active {transition: all 0.5s;}
.fade-slide-leave-active {transition: all 0.08s ease-in;}
.fade-slide-enter-from {opacity: 0;transform: translateX(-16px);}
.fade-slide-leave-to {opacity: 0;transform: translateX(16px);}
</style>

