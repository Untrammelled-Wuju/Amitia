<template>
  <div class="ob-shell ob-stage-9">
    <div
      class="onboarding-world"
      data-level="7"
      data-stage-current="9"
      :data-entry-preparing="entryPreparing ? 'true' : null"
      :data-entering="enteringState"
    >
      <StarfieldBg />
      <CoreZone ref="coreZoneRef" caption="设置权限与渠道" />

      <section class="ob-stage active">
        <StageBoundary
          :permissions="permissions"
          :entering="entering"
          @enter="startEntryTransition"
        />
      </section>

      <AmitiaEntryTransition />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from "vue"
import { useRouter } from "vue-router"
import StarfieldBg from "./components/StarfieldBg.vue"
import CoreZone from "./components/CoreZone.vue"
import StageBoundary from "./components/StageBoundary.vue"
import AmitiaEntryTransition from "./components/AmitiaEntryTransition.vue"

import "./styles/onboarding.css"
import "./styles/stages.css"
import "./styles/typography.css"
import "./styles/stage-specific.css"
import "./styles/identity-memory.css"
import "./styles/boundary.css"
import "./styles/core-layout.css"
import "./styles/entry-transition.css"
import "./styles/bottom-alignment.css"
import "./styles/completion-buttons.css"

const router = useRouter()

const coreZoneRef = ref<InstanceType<typeof CoreZone> | null>(null)

const permissions = reactive({
  autostart: false,
  web: true,
  wechat: false,
  qq: false,
})

const entering = ref(false)
const entryPreparing = ref(false)
const enteringState = ref<string | null>(null)

import("@/views/web-chat/WebChatView.vue")

function waitForCoreMoveComplete(): Promise<void> {
  return new Promise((resolve) => {
    const el = coreZoneRef.value?.getEl()
    if (!el) { resolve(); return }

    const onEnd = (e: TransitionEvent) => {
      if (e.propertyName === 'left' || e.propertyName === 'top') {
        el.removeEventListener('transitionend', onEnd)
        resolve()
      }
    }

    el.addEventListener('transitionend', onEnd)

    const timeout = setTimeout(() => {
      el.removeEventListener('transitionend', onEnd)
      resolve()
    }, 2000)
  })
}

async function startEntryTransition() {
  if (entering.value || entryPreparing.value) return
  import("@/views/web-chat/WebChatView.vue")

  entryPreparing.value = true

  await waitForCoreMoveComplete()
  entryPreparing.value = false
  enteringState.value = "true"

  setTimeout(() => {
    enteringState.value = "complete"
  }, 2200)

  setTimeout(() => {
    router.push("/chat")
  }, 2700)
}
</script>

<style scoped>
.ob-shell {
  width: 100%;
  height: 100%;
  overflow: hidden;
  background: #070708;
  display: grid;
  place-items: center;
}
</style>
