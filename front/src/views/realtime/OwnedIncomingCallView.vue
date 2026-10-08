<template>
  <main class="incoming-call">
    <template v-if="!accepted">
      <h1>{{ caller }}邀请通话</h1>
      <p>{{ error || (busy ? "正在确认邀请与接听权限…" : "接听后由当前 Core 提供对话服务") }}</p>
      <div>
        <button :disabled="busy || ended" @click="accept">接听</button>
        <button @click="decline">{{ ended ? "返回" : "拒绝" }}</button>
      </div>
    </template>
    <RealtimeCallDialog v-else :mode="mode" voice-type="" resource-id="" :conversation-id="accepted.conversationId" :character-id="accepted.characterId" :expected-execution-scope="accepted.scope" :accepted-ticket="accepted.ticket" :conversation-origin="accepted.conversationOrigin" :historical-role-id="accepted.historicalRoleId" :char-name="caller" @close="decline" />
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import RealtimeCallDialog from "../../components/RealtimeCallDialog.vue";
import { acceptOwnedRealtimeInvitation, type OwnedAcceptedInvitation } from "../../realtime/owned-realtime-invitation";

const route = useRoute();
const router = useRouter();
const accepted = ref<OwnedAcceptedInvitation>();
const busy = ref(false);
const ended = ref(false);
const error = ref("");
const controller = new AbortController();
const caller = computed(() => String(route.query.caller || "Amitia"));
const mode = computed<"voice" | "video" | "screen">(() => route.query.type === "video" ? "video" : route.query.type === "screen" ? "screen" : "voice");
const invalidated = () => { controller.abort(); ended.value = true; error.value = "Core、角色或权限已变化，原邀请已失效"; accepted.value = undefined; };

async function accept() {
  if (busy.value || ended.value) return;
  busy.value = true;
  try {
    if (route.query.owned !== "1") throw new Error("邀请缺少原 Core 授权，请重新发起来电");
    accepted.value = await acceptOwnedRealtimeInvitation(String(route.query.call || ""), String(route.query.characterId || ""), controller.signal);
  } catch (cause) {
    ended.value = true;
    error.value = cause instanceof Error ? cause.message : "接听失败";
  } finally { busy.value = false; }
}
function decline() { controller.abort(); accepted.value = undefined; void router.replace("/chat"); }
onMounted(() => {
  window.addEventListener("amitia:runtime-connection-changed", invalidated);
  window.addEventListener("amitia:execution-scope-changed", invalidated);
  if (route.query.action === "answer") void accept();
});
onUnmounted(() => {
  controller.abort();
  window.removeEventListener("amitia:runtime-connection-changed", invalidated);
  window.removeEventListener("amitia:execution-scope-changed", invalidated);
});
</script>

<style scoped>
.incoming-call { min-height: 100vh; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 20px; padding: 24px; }
.incoming-call h1 { font-size: 24px; }
.incoming-call p { max-width: 480px; text-align: center; }
.incoming-call div { display: flex; gap: 16px; }
.incoming-call button { padding: 12px 24px; border-radius: 12px; cursor: pointer; }
</style>
