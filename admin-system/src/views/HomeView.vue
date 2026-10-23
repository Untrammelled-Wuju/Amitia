<script setup lang="ts">
import sidebar from '@/components/sideBar/sidebar.vue'
import OverviewMenu from '@/views/OverviewMenu/OverviewMenu.vue'
import { RouterView } from 'vue-router'
import NProgress from 'nprogress'
import 'nprogress/nprogress.css'

// 初始化NProgress配置
NProgress.configure({
  showSpinner: false,
  parent: 'html',
  easing: 'ease',
  speed: 500,
})
</script>

<template>
  <sidebar />
  <div id="content">
    <OverviewMenu />
    <main>
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
    </main>
  </div>
</template>

<style scoped>
#content {
  height: 100vh;
  width: 0;
  flex: 1;
  min-width: 0;
  background: var(--SysBg);
  overflow: hidden;
}
main {
  --pd: 16px;
  width: 100%;
  height: calc(100% - var(--OverviewMenuHeight));
  overflow: scroll;
  padding: 0 var(--pd);
  padding-top: 12px;
  flex-shrink: 0;
  position: relative;
  &::-webkit-scrollbar {
    height: 0;
    width: 7px;
  }
  &::-webkit-scrollbar-thumb {
    background-color: color-mix(in srgb, var(--BrandPrimary) 45%, transparent);
    border-radius: var(--BorderRadius);
  }
}

/* 页面切换过渡动画 */
.fade-slide-enter-active {transition: all 0.5s;}
.fade-slide-leave-active {transition: all 0.08s ease-in;}
.fade-slide-enter-from {opacity: 0;transform: translateX(-16px);}
.fade-slide-leave-to {opacity: 0;transform: translateX(16px);}
</style>
