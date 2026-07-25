import type { BrowserWindow } from "electron";
import type { LoadedInstallation } from "./resource-loader";
import {
  ActionPriorities,
  EventSources,
  type DesktopPetActionRequest,
  type DesktopPetActionScheduler,
  type EventSource,
} from "./action-scheduler";

export type AssistantState =
  | "assistant_listening"
  | "assistant_thinking"
  | "assistant_speaking"
  | "assistant_finished"
  | "assistant_error";

export interface ChatStateBridgeCallbacks {
  onStateChange?: (state: AssistantState, actionKey: string) => void;
}

export interface ChatStateIpcPayload {
  state: AssistantState;
  roundId?: string;
  error?: string;
}

export interface ChatStatePetPayload {
  state: AssistantState;
  actionKey: string;
  roundId: string | null;
}

export const CHAT_STATE_CHANGE_CHANNEL = "chat:state-change";
export const PET_CHAT_STATE_CHANNEL = "pet:chat-state";

const ACTION_KEY_LISTENING = "listening";
const ACTION_KEY_THINKING = "thinking";
const ACTION_KEY_SPEAKING = "speaking";
const ACTION_KEY_CONFUSED = "confused";
const ACTION_KEY_IDLE = "idle";

const SUSTAINED_LISTENING = "listening";
const SUSTAINED_THINKING = "thinking";
const SUSTAINED_SPEAKING = "speaking";

const SPEAKING_FALLBACK_CHAIN = [
  "listening",
  "idle",
  "idle_normal",
  "idle_breathing",
];
const THINKING_FALLBACK_CHAIN = [
  "waiting",
  "idle",
  "idle_normal",
  "idle_breathing",
];
const LISTENING_FALLBACK_CHAIN = ["idle", "idle_normal", "idle_breathing"];
const CONFUSED_FALLBACK_CHAIN = [
  "confused",
  "thinking",
  "listening",
  "idle",
  "idle_normal",
  "idle_breathing",
];

export class ChatStateBridge {
  private readonly scheduler: DesktopPetActionScheduler;
  private readonly callbacks: ChatStateBridgeCallbacks;
  private loaded: LoadedInstallation | null = null;
  private petWindow: BrowserWindow | null = null;

  private currentRoundId: string | null = null;
  private hasTriggeredSpeakingThisRound = false;
  private hasTriggeredThinkingThisRound = false;
  private isSpeaking = false;
  private isTTSPlaying = false;

  constructor(
    scheduler: DesktopPetActionScheduler,
    callbacks?: ChatStateBridgeCallbacks,
  ) {
    this.scheduler = scheduler;
    this.callbacks = callbacks ?? {};
  }

  attachLoaded(loaded: LoadedInstallation | null): void {
    this.loaded = loaded;
  }

  detachLoaded(): void {
    this.loaded = null;
  }

  attachPetWindow(window: BrowserWindow | null): void {
    this.petWindow = window;
  }

  handleListening(roundId: string): void {
    this.startRound(roundId);
    this.submitChatState(
      ACTION_KEY_LISTENING,
      EventSources.CHAT_LISTENING,
      ActionPriorities.THINKING,
      SUSTAINED_LISTENING,
    );
    this.emitStateChange("assistant_listening", ACTION_KEY_LISTENING);
  }

  handleThinking(roundId: string): void {
    this.startRound(roundId);
    if (this.hasTriggeredThinkingThisRound) return;
    this.hasTriggeredThinkingThisRound = true;
    this.submitChatState(
      ACTION_KEY_THINKING,
      EventSources.CHAT_THINKING,
      ActionPriorities.THINKING,
      SUSTAINED_THINKING,
    );
    this.emitStateChange("assistant_thinking", ACTION_KEY_THINKING);
  }

  handleSpeakingStart(roundId: string): void {
    this.startRound(roundId);
    this.triggerSpeakingOnce();
  }

  handleSpeakingChunk(roundId: string): void {
    this.startRound(roundId);
    this.triggerSpeakingOnce();
  }

  handleSpeakingEnd(_roundId: string): void {
    this.isSpeaking = false;
    if (this.isTTSPlaying) {
      this.scheduler.setSustainedState(SUSTAINED_SPEAKING);
      return;
    }
    this.scheduler.setSustainedState(null);
  }

  handleTTSStart(): void {
    this.isTTSPlaying = true;
    if (this.hasTriggeredSpeakingThisRound) {
      this.scheduler.setSustainedState(SUSTAINED_SPEAKING);
    }
  }

  handleTTSEnd(): void {
    this.isTTSPlaying = false;
    if (this.isSpeaking) {
      this.scheduler.setSustainedState(SUSTAINED_SPEAKING);
      return;
    }
    this.scheduler.setSustainedState(null);
  }

  handleFinished(_roundId: string): void {
    this.isSpeaking = false;
    this.isTTSPlaying = false;
    this.scheduler.setSustainedState(null);
    this.emitStateChange("assistant_finished", ACTION_KEY_IDLE);
    this.resetRound();
  }

  handleError(roundId: string, _error?: Error): void {
    this.isSpeaking = false;
    this.isTTSPlaying = false;
    this.scheduler.setSustainedState(null);
    const actionKey = this.resolveErrorActionKey();
    this.submitChatState(
      actionKey,
      EventSources.MANUAL,
      ActionPriorities.EMOTION,
      null,
    );
    this.emitStateChange("assistant_error", actionKey);
    this.resetRound();
  }

  reset(): void {
    this.currentRoundId = null;
    this.hasTriggeredSpeakingThisRound = false;
    this.hasTriggeredThinkingThisRound = false;
    this.isSpeaking = false;
    this.isTTSPlaying = false;
    this.scheduler.setSustainedState(null);
  }

  handleIpcPayload(payload: ChatStateIpcPayload): void {
    if (!payload || typeof payload.state !== "string") return;
    const state = payload.state as AssistantState;
    const roundId = typeof payload.roundId === "string" ? payload.roundId : "";
    switch (state) {
      case "assistant_listening":
        this.handleListening(roundId);
        break;
      case "assistant_thinking":
        this.handleThinking(roundId);
        break;
      case "assistant_speaking":
        this.handleSpeakingStart(roundId);
        break;
      case "assistant_finished":
        this.handleFinished(roundId);
        break;
      case "assistant_error":
        this.handleError(
          roundId,
          payload.error ? new Error(payload.error) : undefined,
        );
        break;
      default:
        break;
    }
  }

  private startRound(roundId: string): void {
    if (!roundId) return;
    if (this.currentRoundId !== roundId) {
      this.currentRoundId = roundId;
      this.hasTriggeredSpeakingThisRound = false;
      this.hasTriggeredThinkingThisRound = false;
      this.isSpeaking = false;
      this.isTTSPlaying = false;
      this.scheduler.setSustainedState(null);
    }
  }

  private resetRound(): void {
    this.currentRoundId = null;
    this.hasTriggeredSpeakingThisRound = false;
    this.hasTriggeredThinkingThisRound = false;
  }

  private triggerSpeakingOnce(): void {
    if (this.hasTriggeredSpeakingThisRound) return;
    this.hasTriggeredSpeakingThisRound = true;
    this.isSpeaking = true;
    this.submitChatState(
      ACTION_KEY_SPEAKING,
      EventSources.CHAT_SPEAKING,
      ActionPriorities.SPEAKING,
      SUSTAINED_SPEAKING,
    );
    this.emitStateChange("assistant_speaking", ACTION_KEY_SPEAKING);
  }

  private submitChatState(
    actionKey: string,
    source: EventSource,
    priority: number,
    sustainedState: string | null,
  ): void {
    const resolvedKey = this.resolveActionKey(actionKey);
    if (!resolvedKey) {
      this.scheduler.setSustainedState(sustainedState);
      return;
    }
    const request: DesktopPetActionRequest = {
      actionKey: resolvedKey,
      source,
      priority,
      interrupt: true,
      dedupeKey: `chat_${resolvedKey}_${this.currentRoundId ?? "default"}`,
    };
    this.scheduler.submit(request);
    this.scheduler.setSustainedState(sustainedState);
  }

  private resolveActionKey(actionKey: string): string | null {
    if (this.isActionAvailable(actionKey)) {
      return actionKey;
    }
    return this.findFallbackActionKey(actionKey);
  }

  private isActionAvailable(actionKey: string): boolean {
    if (!this.loaded) return false;
    const action = this.loaded.actions.get(actionKey);
    return !!action && action.available;
  }

  private findFallbackActionKey(actionKey: string): string | null {
    if (!this.loaded) return null;
    const chain = this.getFallbackChain(actionKey);
    for (const key of chain) {
      const action = this.loaded.actions.get(key);
      if (action && action.available) {
        return key;
      }
    }
    if (this.loaded.defaultAction && this.loaded.defaultAction.available) {
      return this.loaded.defaultAction.key;
    }
    return null;
  }

  private getFallbackChain(actionKey: string): string[] {
    switch (actionKey) {
      case ACTION_KEY_SPEAKING:
        return SPEAKING_FALLBACK_CHAIN;
      case ACTION_KEY_THINKING:
        return THINKING_FALLBACK_CHAIN;
      case ACTION_KEY_LISTENING:
        return LISTENING_FALLBACK_CHAIN;
      case ACTION_KEY_CONFUSED:
        return CONFUSED_FALLBACK_CHAIN;
      default:
        return [];
    }
  }

  private resolveErrorActionKey(): string {
    if (this.isActionAvailable(ACTION_KEY_CONFUSED)) {
      return ACTION_KEY_CONFUSED;
    }
    return ACTION_KEY_IDLE;
  }

  private emitStateChange(state: AssistantState, actionKey: string): void {
    if (this.callbacks.onStateChange) {
      try {
        this.callbacks.onStateChange(state, actionKey);
      } catch {
        void 0;
      }
    }
    if (this.petWindow && !this.petWindow.isDestroyed()) {
      const payload: ChatStatePetPayload = {
        state,
        actionKey,
        roundId: this.currentRoundId,
      };
      try {
        this.petWindow.webContents.send(PET_CHAT_STATE_CHANNEL, payload);
      } catch {
        void 0;
      }
    }
  }
}
