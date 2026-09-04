<template>
  <div class="ob-stage-inner ob-boundary-stage-inner">
    <div class="ob-boundary-copy">
      <div class="kicker">启动与渠道</div>
      <h2 class="ob-boundary-title">选择启动方式和接入渠道</h2>
      <p class="ob-boundary-desc">
        Web 聊天是 Amitia
        的基础入口，会始终保持开启。你可以选择是否开机自启动，并按需连接微信或
        QQ。
      </p>
    </div>
    <div class="ob-boundary">
      <button
        class="ob-permission-node p-autostart"
        :class="{ on: permissions.autostart }"
        @click="permissions.autostart = !permissions.autostart"
      >
        开机自启动<span class="ob-permission-mini">{{
          permissions.autostart ? "已开启" : "未开启"
        }}</span>
      </button>
      <button class="ob-permission-node p-web on locked" disabled>
        Web 聊天<span class="ob-permission-mini">已开启</span>
      </button>
      <button
        class="ob-permission-node p-wechat"
        :class="{ on: permissions.wechat }"
        :disabled="entering"
        aria-label="连接微信"
        @click="openChannelDialog('wechat')"
      >
        微信<span class="ob-permission-mini">{{
          permissions.wechat ? "已连接" : "点击连接"
        }}</span>
      </button>
      <button
        class="ob-permission-node p-qq"
        :class="{
          on: permissions.qq,
          'qq-active': activeChannel === 'qq' && channelDialogVisible,
        }"
        :disabled="entering"
        aria-label="连接 QQ"
        @click="openChannelDialog('qq')"
      >
        QQ<span class="ob-permission-mini">{{
          permissions.qq ? "已连接" : "点击连接"
        }}</span>
      </button>
    </div>
    <button
      class="ob-stage-action ob-enter-amitia-btn"
      @click="$emit('enter')"
      :disabled="entering"
    >
      {{ entering ? "正在进入 Amitia…" : "进入 Amitia" }}
    </button>

    <div
      v-if="activeChannel === 'qq' && channelDialogVisible"
      class="ob-qq-compact-panel"
      @click.stop
    >
      <button
        class="ob-qq-compact-close"
        @click="channelDialogVisible = false"
        aria-label="关闭"
      >
        ✕
      </button>
      <QqConnectView
        embedded
        compact
        @connectionChanged="handleConnectionChanged"
      />
    </div>

    <div
      v-if="activeChannel === 'qq' && channelDialogVisible"
      class="ob-qq-compact-overlay"
      @click="channelDialogVisible = false"
    />

    <div
      v-if="activeChannel === 'wechat' && wechatDialogVisible"
      class="ob-qq-compact-panel"
      @click.stop
    >
      <button
        class="ob-qq-compact-close"
        @click="wechatDialogVisible = false"
        aria-label="关闭"
      >
        ✕
      </button>
      <Suspense>
        <WechatConnectView
          embedded
          compact
          @connectionChanged="
            (v: boolean) => handleConnectionChanged(v, 'wechat')
          "
        />
        <template #fallback>
          <div class="ob-channel-loading" role="status" aria-live="polite">
            正在加载连接功能…
          </div>
        </template>
      </Suspense>
    </div>

    <div
      v-if="activeChannel === 'wechat' && wechatDialogVisible"
      class="ob-qq-compact-overlay"
      @click="wechatDialogVisible = false"
    />
  </div>
</template>

<script setup lang="ts">
import { defineAsyncComponent, ref } from "vue";
import QqConnectView from "../../qq-connect/QqConnectView.vue";

const props = defineProps<{
  permissions: {
    autostart: boolean;
    web: boolean;
    wechat: boolean;
    qq: boolean;
  };
  entering: boolean;
}>();

defineEmits<{
  enter: [];
}>();

const WechatConnectView = defineAsyncComponent(
  () => import("../../wechat-connect/WechatConnectView.vue"),
);

const channelDialogVisible = ref(false);
const wechatDialogVisible = ref(false);
const activeChannel = ref<"wechat" | "qq" | null>(null);

function openChannelDialog(channel: "wechat" | "qq") {
  activeChannel.value = channel;
  if (channel === "wechat") {
    wechatDialogVisible.value = true;
  } else {
    channelDialogVisible.value = !channelDialogVisible.value;
  }
}

function handleConnectionChanged(
  connected: boolean,
  channel?: "wechat" | "qq",
) {
  const ch = channel || activeChannel.value;
  if (!ch) return;
  props.permissions[ch] = connected;
}
</script>
