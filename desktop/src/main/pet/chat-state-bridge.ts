import type { BrowserWindow } from "electron";
import type { LoadedInstallation } from "./resource-loader";

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


export class ChatStateBridge {
  private readonly callbacks: ChatStateBridgeCallbacks;
  private loaded: LoadedInstallation | null = null;
  private petWindow: BrowserWindow | null = null;

  private currentRoundId: string | null = null;
  private hasTriggeredSpeakingThisRound = false;
  private hasTriggeredThinkingThisRound = false;
  private isSpeaking = false;
  private isTTSPlaying = false;

  constructor(callbacks?: ChatStateBridgeCallbacks) {
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
    this.emitStateChange("assistant_listening", ACTION_KEY_LISTENING);
  }

  handleThinking(roundId: string): void {
    this.startRound(roundId);
    if (this.hasTriggeredThinkingThisRound) return;
    this.hasTriggeredThinkingThisRound = true;
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
    // Presentation state only. Backend Behavior owns action scheduling.
  }

  handleTTSStart(): void {
    this.isTTSPlaying = true;
  }

  handleTTSEnd(): void {
    this.isTTSPlaying = false;
    // Presentation state only. Backend Behavior owns action scheduling.
  }

  handleFinished(_roundId: string): void {
    this.isSpeaking = false;
    this.isTTSPlaying = false;
    this.emitStateChange("assistant_finished", ACTION_KEY_IDLE);
    this.resetRound();
  }

  handleError(roundId: string, _error?: Error): void {
    this.isSpeaking = false;
    this.isTTSPlaying = false;
    const actionKey = this.resolveErrorActionKey();
    this.emitStateChange("assistant_error", actionKey);
    this.resetRound();
  }

  reset(): void {
    this.currentRoundId = null;
    this.hasTriggeredSpeakingThisRound = false;
    this.hasTriggeredThinkingThisRound = false;
    this.isSpeaking = false;
    this.isTTSPlaying = false;
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
    this.emitStateChange("assistant_speaking", ACTION_KEY_SPEAKING);
  }

  private isActionAvailable(actionKey: string): boolean {
    return Boolean(this.loaded?.actions.get(actionKey)?.available);
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
