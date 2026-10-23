<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { useRouter } from 'vue-router'

import navigationShowModelIcon from './icons/navigationShowModelIcon.vue'
import NowRouterName from './NowRouterName.vue'
import remewIcon from './icons/remewIcon.vue'
import fullScreenIcon from './icons/fullScreenIcon.vue'
import exitFullScreenIcon from './icons/exitFullScreenIcon.vue'
import Setting from './icons/SettingIcon.vue'
import userInfo from './userInfo.vue'
import changeThemeBtn from './icons/changeThemeBtn/changeThemeBtn.vue'
const router = useRouter()
const openSideSetting = () => {
  router.push('/settings')
}

const FullScreenModel = ref(false)
const toggleFullScreen = () => {
  FullScreenModel.value = true
  console.log(document.fullscreenElement)
  if (!document.fullscreenElement) {
    document.documentElement
      .requestFullscreen()
      .then(() => (FullScreenModel.value = true))
      .catch((err) => {
        ElMessage.error(`全屏请求失败${err}`)
        FullScreenModel.value = false
      })
  } else {
    document.exitFullscreen().then(() => (FullScreenModel.value = false))
  }
}
</script>
<template>
  <div id="OverviewMenu">
    <div class="left">
      <el-tooltip content="导航显示/隐藏" placement="bottom" effect="light">
        <navigationShowModelIcon />
      </el-tooltip>
      <NowRouterName />
      <el-tooltip content="刷新页面" placement="bottom" effect="light">
        <remewIcon />
      </el-tooltip>
    </div>
    <div class="right">
      <el-tooltip v-if="!FullScreenModel" content="全屏模式" placement="bottom" effect="light">
        <fullScreenIcon @toggleFullScreen="toggleFullScreen" />
      </el-tooltip>
      <el-tooltip v-if="FullScreenModel" content="退出全屏模式" placement="bottom" effect="light">
        <exitFullScreenIcon @toggleFullScreen="toggleFullScreen" />
      </el-tooltip>
      <el-tooltip content="切换主题" placement="bottom" effect="light">
        <div class="icon">
          <changeThemeBtn />
        </div>
      </el-tooltip>
      <el-tooltip content="设置" placement="bottom" effect="light">
        <Setting @openSideSetting="openSideSetting" />
      </el-tooltip>
      <userInfo />
    </div>
  </div>
</template>
<style scoped>
#OverviewMenu {
  user-select: none;
  height: var(--OverviewMenuHeight);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 0 16px;
  border-bottom: 1px solid var(--Main_BoxBorderColor);
  background: var(--Main_Bg_BoxColor);
  .left {
    height: 100%;
    display: flex;
    align-items: center;
  }
  .right {
    gap: 2px;
    height: 100%;
    display: flex;
    align-items: center;
  }
}

#OverviewMenu :deep(svg) {
  box-sizing: content-box;
  width: var(--OverviewMenuIconwh);
  height: var(--OverviewMenuIconwh);
  margin: 0;
  padding: 7px;
  border-radius: 9px;
  fill: var(--Main_font_SecondColor_Low);
  transition:
    background-color var(--transitionTime) ease,
    fill var(--transitionTime) ease;
}

#OverviewMenu :deep(svg *) {
  fill: var(--Main_font_SecondColor_Low);
  transition: fill var(--transitionTime) ease;
}

#OverviewMenu :deep(svg:hover) {
  background-color: color-mix(in srgb, var(--Main_font_SecondColor) 9%, transparent);
  fill: var(--Main_font_SecondColor);
}

#OverviewMenu :deep(svg:hover *) {
  fill: var(--Main_font_SecondColor);
}

#OverviewMenu :deep(.now-router-name) {
  margin: 0 10px 0 6px;
  font-size: 16px;
  font-weight: 600;
  color: var(--Main_font_SecondColor);
}

#OverviewMenu :deep(.userInfo) {
  margin-left: 10px;
  padding-left: 14px;
  border-left: 1px solid var(--Main_BoxBorderColor);
}
</style>
