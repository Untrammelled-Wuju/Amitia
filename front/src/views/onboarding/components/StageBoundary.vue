<template>
  <div class="ob-stage-inner ob-boundary-stage-inner">
    <div class="ob-boundary-copy">
      <div class="kicker">启动与渠道</div>
      <h2 class="ob-boundary-title">选择启动方式和接入渠道</h2>
      <p class="ob-boundary-desc">Web 聊天是 Amitia 的基础入口，会始终保持开启。你可以选择是否开机自启动，并按需连接微信或 QQ。</p>
    </div>
    <div class="ob-boundary">
      <button
        class="ob-permission-node p-autostart"
        :class="{ on: permissions.autostart }"
        @click="permissions.autostart = !permissions.autostart"
      >
        开机自启动<span class="ob-permission-mini">{{ permissions.autostart ? '已开启' : '未开启' }}</span>
      </button>
      <button class="ob-permission-node p-web on locked" disabled>
        Web 聊天<span class="ob-permission-mini">已开启</span>
      </button>
      <button
        class="ob-permission-node p-wechat"
        :class="{ on: permissions.wechat }"
        @click="permissions.wechat = !permissions.wechat"
      >
        微信<span class="ob-permission-mini">{{ permissions.wechat ? '已连接' : '可连接' }}</span>
      </button>
      <button
        class="ob-permission-node p-qq"
        :class="{ on: permissions.qq }"
        @click="permissions.qq = !permissions.qq"
      >
        QQ<span class="ob-permission-mini">{{ permissions.qq ? '已连接' : '可连接' }}</span>
      </button>
    </div>
    <button class="ob-stage-action ob-enter-amitia-btn" @click="$emit('enter')" :disabled="entering">
      {{ entering ? '正在进入 Amitia…' : '进入 Amitia' }}
    </button>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  permissions: { autostart: boolean; web: boolean; wechat: boolean; qq: boolean }
  entering: boolean
}>()

defineEmits<{
  enter: []
}>()
</script>
