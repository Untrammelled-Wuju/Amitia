<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="call-window-root">
    <div class="call-window-titlebar">
      <div class="call-window-title">
        <span class="call-window-dot" />
        <span>Amitia 通话</span>
      </div>
      <div class="call-window-actions">
        <button class="call-window-action" type="button" aria-label="最小化" title="最小化" @click="minimizeWindow">
          <el-icon><Minus /></el-icon>
        </button>
        <button class="call-window-action close" type="button" aria-label="关闭" title="关闭" @click="closeWindow">
          <el-icon><Close /></el-icon>
        </button>
      </div>
    </div>
    <RealtimeCallDialog
      :mode="mode"
      :voice-type="voiceType"
      :resource-id="resourceId"
      :conversation-id="conversationId"
      :char-name="charName"
      :char-avatar="charAvatar"
      @state-change="handleStateChange"
      @close="closeWindow"
    />
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { Close, Minus } from "@element-plus/icons-vue";
import { useRoute } from "vue-router";
import RealtimeCallDialog from "../../components/RealtimeCallDialog.vue";

const route = useRoute();

function queryText(name: string): string {
  const value = route.query[name];
  if (Array.isArray(value)) return String(value[0] ?? "");
  return String(value ?? "");
}

const mode = computed<"voice" | "video" | "screen">(() => {
  const value = queryText("mode");
  return value === "video" || value === "screen" ? value : "voice";
});
const voiceType = computed(() => queryText("voiceType"));
const resourceId = computed(() => queryText("resourceId"));
const conversationId = computed(() => queryText("conversationId"));
const charName = computed(() => queryText("charName"));
const charAvatar = computed(() => queryText("charAvatar"));

function minimizeWindow() {
  void window.electronWindowApi?.minimize("child");
}

function closeWindow() {
  if (window.amitiaDesktop?.closeRealtimeCallWindow) {
    void window.amitiaDesktop.closeRealtimeCallWindow();
    return;
  }
  void window.electronWindowApi?.close("child");
}

function handleStateChange(state: string) {
  if (state === "idle") closeWindow();
}
</script>

<style scoped>
:global(html),
:global(body),
:global(#app) {
  width: 100%;
  height: 100%;
  margin: 0;
  overflow: hidden;
  background: #121212;
}

:global(html.amitia-desktop-shell body) {
  padding-top: 0 !important;
}

:global(html.amitia-desktop-shell #app) {
  height: 100vh !important;
}

.call-window-root {
  position: relative;
  width: 100vw;
  height: 100vh;
  overflow: hidden;
  background: #121212;
}

.call-window-titlebar {
  position: fixed;
  top: 0;
  right: 0;
  left: 0;
  z-index: 4000;
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 34px;
  padding-left: 12px;
  color: rgba(255, 255, 255, 0.72);
  background: rgba(18, 18, 18, 0.86);
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  backdrop-filter: blur(14px);
  -webkit-app-region: drag;
}

.call-window-title {
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 12px;
  font-weight: 500;
}

.call-window-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #52b788;
}

.call-window-actions {
  display: flex;
  align-self: stretch;
  -webkit-app-region: no-drag;
}

.call-window-action {
  display: grid;
  place-items: center;
  width: 42px;
  height: 100%;
  padding: 0;
  border: 0;
  color: rgba(255, 255, 255, 0.72);
  background: transparent;
  cursor: pointer;
}

.call-window-action:hover {
  color: #fff;
  background: rgba(255, 255, 255, 0.08);
}

.call-window-action.close:hover {
  background: #c42b1c;
}
</style>
