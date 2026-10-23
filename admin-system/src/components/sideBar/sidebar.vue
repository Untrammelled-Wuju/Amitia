<!-- eslint-disable vue/multi-word-component-names -->
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useAuthStore } from '@/stores/authStore'
import logoDark from '@/assets/images/amitia-logo-dark.png'
import logoLight from '@/assets/images/amitia-logo-light.png'
const { logout } = useAuthStore()

const confirmAlert = async (text: string) => {
  try {
    await ElMessageBox.confirm(`您确定要${text}吗？`, '确认操作', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    })
    await logout()
    ElMessage.success(`${text}成功！`)
  } catch {
    ElMessage.info(`已取消${text}`)
  }
}

const route = useRoute()
const nowRouterPath = computed(() => route.path)
const currentTheme = ref(document.documentElement.dataset.theme || 'light')
const logoSrc = computed(() => (currentTheme.value === 'dark' ? logoLight : logoDark))
let themeObserver: MutationObserver | null = null

onMounted(() => {
  themeObserver = new MutationObserver(() => {
    currentTheme.value = document.documentElement.dataset.theme || 'light'
  })
  themeObserver.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ['data-theme'],
  })
})

onBeforeUnmount(() => {
  themeObserver?.disconnect()
})

const foldModel = ref(false)
const H1ShowModel = ref(true)
foldModel.value = localStorage.getItem('NavShowFoldModel') === 'true'
H1ShowModel.value = localStorage.getItem('NavShowFoldModel') === 'false'
setInterval(() => {
  foldModel.value = localStorage.getItem('NavShowFoldModel') === 'true'
  H1ShowModel.value = localStorage.getItem('NavShowFoldModel') === 'false'
}, 100)

</script>
<template>
  <el-aside id="siderBar">
    <el-menu
      :default-active="nowRouterPath"
      class="el-menu-vertical-demo"
      :unique-opened="true"
      :collapse="foldModel"
      router
      id="sideBarMenu"
    >
      <a href="/" class="logo">
        <span class="logoBadge">
          <img :src="logoSrc" alt="Amitia Logo" />
        </span>
        <h1 class="title" v-if="H1ShowModel">更新管理</h1>
      </a>
      <el-menu-item index="/">
        <el-icon><Monitor /></el-icon>
        <span>工作台</span>
      </el-menu-item>
      <el-menu-item index="/desktop">
        <el-icon><Monitor /></el-icon>
        <span>桌面端更新</span>
      </el-menu-item>
      <el-menu-item index="/android">
        <el-icon><Cellphone /></el-icon>
        <span>手机端更新</span>
      </el-menu-item>
      <el-menu-item index="/audit">
        <el-icon><Document /></el-icon>
        <span>发布审计</span>
      </el-menu-item>
      <el-menu-item index="/feedback">
        <el-icon><ChatDotSquare /></el-icon>
        <span>用户反馈</span>
      </el-menu-item>
      <div class="sysFunc">系统功能</div>
      <el-menu-item index="/settings">
        <el-icon><Setting /></el-icon>
        <span>发布设置</span>
      </el-menu-item>
      <el-menu-item index="" class="logoutItem" @click="confirmAlert('退出登录')">
        <el-icon><SwitchButton /></el-icon>
        <span>退出登录</span>
      </el-menu-item>
    </el-menu>
  </el-aside>
</template>

<style scoped>
#siderBar {
  height: 100vh;
  width: auto;
  display: flex;
  flex-direction: column;
  background: var(--Main_Bg_BoxColor);
  border-right: 1px solid var(--Main_BoxBorderColor);
  z-index: 2;
  user-select: none;
  overflow: scroll;
  &::-webkit-scrollbar {
    display: none;
  }
}
.el-menu {
  width: var(--sideBarWidth);
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
  border-right: none;
  padding: 0 10px 10px;
  background-color: var(--Main_Bg_BoxColor);
  .el-menu-item {
    height: 40px;
    margin: 2px 0;
    padding-left: 8px !important;
    font-size: 14px;
    transition:
      background-color var(--transitionTime) ease,
      color var(--transitionTime) ease !important;
    border-radius: var(--BorderRadius);
    color: var(--Main_font_SecondColor);
    .el-icon {
      margin-right: 10px;
      font-size: 18px;
      color: var(--Main_font_SecondColor_Low);
    }
    &:hover {
      background-color: var(--SidebarHoverBgColor) !important;
    }
    &.is-active {
      background-color: var(--SidebarActiveBgColor);
      color: var(--SidebarActiveFontColor);
      font-weight: 600;
      position: relative;
      border-radius: var(--BorderRadius);
      .el-icon {
        color: var(--SidebarActiveFontColor);
      }
    }
  }
  &.el-menu--collapse {
    width: 60px;
    & h1 {
      display: none;
    }
  }
  .sysFunc {
    margin: 18px 0 6px;
    padding-left: 10px !important;
    width: 100%;
    font-size: 11px;
    font-weight: 600;
    letter-spacing: 0.06em;
    color: var(--Main_font_SecondColor_Low);
  }
  .logoutItem {
    margin-top: auto;
    color: var(--Main_font_SecondColor_Low) !important;
    &::before {
      position: absolute;
      top: -8px;
      right: 4px;
      left: 4px;
      height: 1px;
      background: var(--Main_BoxBorderColor);
      content: '';
    }
  }
}
:deep(.el-sub-menu) {
  .el-sub-menu__title {
    padding-left: 8px !important;
    border-radius: var(--BorderRadius) !important;
    height: 45px !important;
    margin: 2px 0;
    color: var(--Main_font_SecondColor);
    &:hover {
      background-color: var(--SidebarHoverBgColor);
    }
  }
  &.is-opened {
    .el-sub-menu__title {
      color: var(--SidebarSubMenuTitleActiveFontColor);
    }
  }
  .el-menu-item {
    padding-left: 40px !important;
  }
}
.el-menu-item-group {
  background-color: var(--Main_Bg_BoxColor);
  padding: 6px;
  .el-menu-item {
    padding-left: 20px !important;
  }
}
:deep(.el-menu-item-group__title) {
  padding-left: 20px !important;
}
.logo {
  height: 60px;
  width: 100%;
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
  padding: 0 6px 0 8px;
  border-bottom: 1px solid var(--Main_BoxBorderColor);
  text-decoration: none;
  .logoBadge {
    display: inline-grid;
    width: 34px;
    height: 34px;
    flex: 0 0 34px;
    place-items: center;
    overflow: hidden;
    border-radius: 9px;
  }
  .logoBadge img {
    --wh: 28px;
    width: var(--wh);
    height: var(--wh);
    object-fit: contain;
  }
  h1 {
    margin: 0;
    font-size: 16px;
    font-weight: 600;
    color: var(--Main_font_SecondColor);
    overflow: hidden;
    white-space: nowrap;
  }
}
</style>
