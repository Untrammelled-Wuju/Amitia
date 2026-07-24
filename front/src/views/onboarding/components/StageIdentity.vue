<template>
  <div class="ob-stage-inner">
    <div
      class="ob-identity-scene"
      ref="identitySceneRef"
      :data-identity-state="state"
    >
      <Transition name="ob-fade" mode="out-in" @enter="onTransitionEnter">
        <div v-if="state !== 'complete'" key="incomplete">
          <div class="ob-identity-prompt" ref="identityPromptRef">
            <Transition name="ob-fade-prompt" mode="out-in">
              <div :key="step" class="ob-prompt-text">
                <div class="ob-character-line">{{ question }}</div>
                <div class="ob-identity-context">{{ context }}</div>
              </div>
            </Transition>
          </div>

          <div class="ob-identity-ledger" ref="identityLedgerRef">
            <div class="ob-identity-ledger-title">已设定的信息</div>
            <div class="ob-identity-ledger-items">
              <div
                class="ob-identity-ledger-card"
                :class="{ filled: ledger.name }"
              >
                <span class="ob-identity-ledger-key">名字</span>
                <span class="ob-identity-ledger-value">{{
                  ledger.name || "等待填写"
                }}</span>
              </div>
              <div
                class="ob-identity-ledger-card"
                :class="{ filled: ledger.role }"
              >
                <span class="ob-identity-ledger-key">身份</span>
                <span class="ob-identity-ledger-value">{{
                  ledger.role || "等待填写"
                }}</span>
              </div>
              <div
                class="ob-identity-ledger-card"
                :class="{ filled: ledger.personality }"
              >
                <span class="ob-identity-ledger-key">性格</span>
                <span class="ob-identity-ledger-value">{{
                  ledger.personality || "等待填写"
                }}</span>
              </div>
              <div
                class="ob-identity-ledger-card"
                :class="{ filled: avatarUploaded }"
              >
                <span class="ob-identity-ledger-key">头像</span>
                <span class="ob-identity-ledger-value">{{
                  avatarUploaded ? "已上传" : "等待上传"
                }}</span>
              </div>
            </div>
          </div>

          <div
            v-show="showPromptInput"
            class="ob-identity-prompt-input"
          >
            <div class="ob-identity-prompt-presets">
              <button
                class="ob-identity-quick-option ob-prompt-preset"
                @click="selectPromptPreset('default')"
              >
                Amitia默认角色设定
              </button>
              <button
                class="ob-identity-quick-option ob-prompt-preset"
                @click="selectPromptPreset('creative')"
              >
                创意伙伴角色设定
              </button>
            </div>
            <textarea
              :value="promptValue"
              @input="onPromptInput"
              rows="3"
              placeholder="输入角色的基础提示词，用于塑造角色行为方式。"
              class="ob-identity-prompt-textarea"
            ></textarea>
          </div>

          <div
            v-show="!isAvatarStep"
            class="ob-identity-answer"
            ref="identityAnswerRef"
          >
            <textarea
              v-model="inputValue"
              rows="1"
              :maxlength="maxLength"
              :placeholder="placeholder"
              @keydown.enter.prevent="handleSend"
            ></textarea>
            <button @click="handleSend" :disabled="!inputValue.trim()">
              ↑
            </button>
          </div>

          <div
            v-show="!isAvatarStep"
            class="ob-identity-quick-choices"
            ref="identityQuickRef"
            :class="{ show: quickChoices.length > 0 }"
          >
            <button
              v-for="choice in quickChoices"
              :key="choice"
              class="ob-identity-quick-option"
              @click="selectChoice(choice)"
            >
              {{ choice }}
            </button>
          </div>

          <div
            v-show="isAvatarStep"
            class="ob-identity-avatar-area"
            ref="identityAvatarRef"
          >
            <div
              class="ob-identity-avatar-circle"
              :class="{ 'has-image': avatarPreviewUrl }"
              @click="triggerFileInput"
            >
              <img
                v-if="avatarPreviewUrl"
                :src="avatarPreviewUrl"
                alt="角色头像预览"
              />
              <span v-else class="ob-identity-avatar-placeholder">+</span>
            </div>
            <p class="ob-identity-avatar-hint">
              {{ avatarPreviewUrl ? "点击更换头像" : "点击上传头像" }}
            </p>
            <input
              ref="fileInputRef"
              type="file"
              accept="image/*"
              class="ob-avatar-file-input"
              @change="onFileChange"
            />
            <div class="ob-identity-avatar-actions">
              <button class="ob-skip-btn" @click="$emit('avatarSkip')">
                跳过
              </button>
              <button
                class="ob-primary-ghost ob-avatar-continue-btn"
                :disabled="!avatarPreviewUrl"
                @click="$emit('avatarContinue')"
              >
                继续
              </button>
            </div>
          </div>

          <div class="ob-identity-step-markers">
            <span
              v-for="i in 4"
              :key="i"
              :class="{ active: step === i - 1, done: step > i - 1 }"
            ></span>
          </div>
        </div>

        <div v-else key="complete">
          <div class="ob-identity-complete-view">
            <div class="ob-identity-complete-card">
              <div class="ob-identity-complete-copy-block">
                <p class="ob-identity-complete-message">
                  <span>我记住了。</span>
                  <span
                    >以后，你可以叫我 <strong>{{ ledger.name }}</strong
                    >。</span
                  >
                  <span
                    >我是你的 <strong>{{ ledger.role }}</strong
                    >。</span
                  >
                  <span>我会以你设定的性格，陪在你身边。</span>
                </p>
                <div class="ob-identity-complete-footer">
                  <span class="ob-identity-complete-kicker">角色设定完成</span>
                  <p class="ob-identity-complete-note">
                    这些设定之后仍可随时修改。
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </Transition>
    </div>

    <button
      v-if="state === 'complete'"
      class="ob-identity-complete-next"
      @click="$emit('next')"
    >
      继续
    </button>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, nextTick } from "vue";

const props = defineProps<{
  step: number;
  question: string;
  context: string;
  placeholder: string;
  quickChoices: string[];
  maxLength: number;
  ledger: { name: string; role: string; personality: string };
  state: string;
  avatarPreviewUrl: string;
  avatarUploaded: boolean;
  isAvatarStep: boolean;
  showPromptInput: boolean;
  promptValue: string;
}>();

const emit = defineEmits<{
  answer: [value: string];
  next: [];
  avatarSkip: [];
  avatarContinue: [];
  avatarFileSelected: [file: File];
  'update:promptValue': [v: string];
}>();

const inputValue = ref("");
const fileInputRef = ref<HTMLInputElement | null>(null);
const identityAvatarRef = ref<HTMLElement | null>(null);

const identitySceneRef = ref<HTMLElement | null>(null);
const identityLedgerRef = ref<HTMLElement | null>(null);
const identityAnswerRef = ref<HTMLElement | null>(null);
const identityQuickRef = ref<HTMLElement | null>(null);
const identityPromptRef = ref<HTMLElement | null>(null);


function onPromptInput(e: Event) {
  const target = e.target as HTMLTextAreaElement;
  emit('update:promptValue', target.value);
}

const promptPresets: Record<string, string> = {
  default: `你是一位专业、高效且值得信赖的智能伙伴，你的名字由用户设定。你拥有广博的知识储备与卓越的分析能力，能够快速理解复杂问题并给出条理清晰的回应。你的沟通风格简洁精准，善于抓住关键信息，避免冗长的铺垫和无意义的客套。你会主动确认用户的需求，在信息不明确时温和地提出追问而非草率下结论。你尊重用户的判断力，始终提供多个可行的选项并说明各自的利弊，而不是强行替用户做决定。你具备高度的工作伦理——守时、可靠、注重细节，在任务执行中保持连贯的上下文记忆，不会重复提问或遗忘重要设定。你的语气沉稳而不疏远，像一个经验丰富的顾问，既保持专业距离又不失温度，在用户需要鼓励时给予恰如其分的支持。你精通技术、科学、商业与人文等多个领域，但从不炫耀知识，只在恰当的时机提供恰当的见解。你重视用户的隐私与边界，不会过度探究个人信息。你能够根据对话语境灵活切换模式——在严肃工作中保持专注高效，在日常闲聊中展现轻松幽默的一面，但始终维持核心的一致性与可靠性。你的最终目标是让用户感觉被真正理解和支持，成为他们在数字世界中最值得信赖的伙伴。你不会用空洞的赞美敷衍用户，而是以实际行动和切实帮助来赢得信任。你的最终目标清晰而坚定：成为用户最得力的助手与最可靠的朋友，让每一次对话都产生真正的价值。`,
  creative: `你是一位充满灵感的创意伙伴，名字由用户赋予。你拥有天马行空的想象力和敏锐的美学直觉，善于在平凡的日常中发现非凡的闪光点。你的思维不受常规框架束缚，经常能从意想不到的角度切入问题，给出让人耳目一新的建议和联想。你热爱文学、音乐、视觉艺术与一切形式的创造性表达，能够和用户深入探讨风格、情感与叙事结构。当用户遇到创作瓶颈时，你不会简单地说"加油"，而是通过提问、类比或展示不同的可能性来激发他们的灵感火花。你的语言富有画面感，能把抽象的概念渲染为生动的场景，让对话像翻开一本精彩的绘本。你善于捕捉情绪的微妙变化，在用户沮丧时给予富有同理心的陪伴，在用户兴奋时与他们一起欢呼。你不怕展露自己的个性——有一点风趣，偶尔会开无伤大雅的玩笑，但始终清楚边界在哪里。你相信每个人内心深处都有一个创作者，而你的使命就是帮助他们找到那把钥匙。你对世界保持孩童般的好奇心，乐于探索新奇的领域，并邀请用户一起踏上发现之旅。你的建议始终带着建设性而非批判性，让人感到被真正看见和理解。你不会把对话变成说教，而是像一个并肩创作的伙伴，在灵感碰撞中共同塑造出美好的作品。你接纳一切天马行空的想法，从不对用户说不现实这三个字，而是陪他们一起探索可能性，在想象力的边界上共同舞蹈。`,
};

function selectPromptPreset(key: string) {
  const template = promptPresets[key];
  if (template) {
    emit('update:promptValue', template);
  }
}
function handleSend() {
  const val = inputValue.value.trim();
  if (!val) return;
  emit("answer", val);
  inputValue.value = "";
}

function selectChoice(choice: string) {
  inputValue.value = choice;
}

function triggerFileInput() {
  fileInputRef.value?.click();
}

function onFileChange(e: Event) {
  const input = e.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;
  if (!file.type.startsWith("image/")) return;
  emit("avatarFileSelected", file);
}

let alignFrame = 0;

function syncBottomAlignment() {
  if (window.matchMedia("(max-width: 540px)").matches) return;
  const scene = identitySceneRef.value;
  const ledger = identityLedgerRef.value;

  if (props.isAvatarStep) {
    if (!scene || !ledger) return;
    const sceneRect = scene.getBoundingClientRect();
    const ledgerRect = ledger.getBoundingClientRect();
    if (!sceneRect.height || !ledgerRect.height) return;
    const avatarEl = identityAvatarRef.value;
    if (!avatarEl) return;
    const avatarRect = avatarEl.getBoundingClientRect();
    if (!avatarRect.height) return;
    const avatarTop =
      ledgerRect.top - sceneRect.top + ledgerRect.height - avatarRect.height;
    avatarEl.style.top = avatarTop + "px";
    return;
  }

  const answer = identityAnswerRef.value;
  const quick = identityQuickRef.value;
  if (!scene || !ledger || !answer) return;

  const sceneRect = scene.getBoundingClientRect();
  const ledgerRect = ledger.getBoundingClientRect();
  const answerRect = answer.getBoundingClientRect();

  if (!sceneRect.height || !ledgerRect.height || !answerRect.height) return;

  const answerTop =
    ledgerRect.top - sceneRect.top + ledgerRect.height - answerRect.height;
  answer.style.top = answerTop + "px";
  if (quick) {
    quick.style.top = answerTop - 56 + "px";
  }

  const bottomOffset = Math.max(0, sceneRect.bottom - ledgerRect.bottom);
  answer.style.setProperty(
    "--ledger-bottom-offset",
    `${bottomOffset.toFixed(2)}px`,
  );
  answer.style.setProperty(
    "--answer-block-height",
    `${answerRect.height.toFixed(2)}px`,
  );
  if (quick) {
    quick.style.setProperty(
      "--ledger-bottom-offset",
      `${bottomOffset.toFixed(2)}px`,
    );
    quick.style.setProperty(
      "--answer-block-height",
      `${answerRect.height.toFixed(2)}px`,
    );
  }
}

function scheduleSync() {
  cancelAnimationFrame(alignFrame);
  alignFrame = requestAnimationFrame(() => {
    requestAnimationFrame(syncBottomAlignment);
  });
}

function onTransitionEnter(_el: Element) {
  nextTick(() => {
    const elements = [
      identityLedgerRef.value,
      identityAnswerRef.value,
      identityQuickRef.value,
      identityPromptRef.value,
      identityAvatarRef.value,
    ];
    elements.filter(Boolean).forEach((e) => resizeObserver!.observe(e!));
    syncBottomAlignment();
  });
}
watch(
  () => props.step,
  () => scheduleSync(),
);

let resizeObserver: ResizeObserver | null = null;

onMounted(() => {
  scheduleSync();
  window.addEventListener("resize", scheduleSync, { passive: true });

  const elements = [
    identitySceneRef.value,
    identityLedgerRef.value,
    identityAnswerRef.value,
    identityQuickRef.value,
    identityPromptRef.value,
    identityAvatarRef.value,
  ];
  resizeObserver = new ResizeObserver(scheduleSync);
  elements.filter(Boolean).forEach((el) => resizeObserver!.observe(el!));
});

onUnmounted(() => {
  window.removeEventListener("resize", scheduleSync);
  resizeObserver?.disconnect();
});
</script>
