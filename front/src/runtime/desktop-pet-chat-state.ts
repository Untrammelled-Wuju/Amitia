export type DesktopPetAssistantState =
  | "assistant_listening"
  | "assistant_thinking"
  | "assistant_speaking"
  | "assistant_finished"
  | "assistant_error";

export function notifyDesktopPetChatState(
  state: DesktopPetAssistantState,
  roundId?: string,
  error?: string,
): void {
  try {
    window.amitiaDesktop?.notifyDesktopPetChatState?.({
      state,
      roundId: roundId || undefined,
      error: error || undefined,
    });
  } catch {
    // Web-only hosts intentionally have no local desktop-pet body.
  }
}
