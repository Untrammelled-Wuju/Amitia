export class OwnedChatStateChangedError extends Error {}

export async function pollOwnedChat(
  owned: { refresh(): Promise<boolean>; markUnavailable(): void; notice: { value: string } },
  onReady: () => Promise<void>,
  onDataError: (error: unknown) => void,
  disposed: () => boolean,
) {
  let available: boolean;
  try {
    available = await owned.refresh();
  } catch (error) {
    if (!disposed() && !(error instanceof OwnedChatStateChangedError) && !owned.notice.value.includes("等待新服务批准")) owned.markUnavailable();
    return;
  }
  if (disposed() || !available) return;
  try {
    await onReady();
  } catch (error) {
    if (!disposed()) onDataError(error);
  }
}
