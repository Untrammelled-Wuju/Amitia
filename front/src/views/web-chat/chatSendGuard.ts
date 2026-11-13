export function chatSendDisabledReason(modelMissing: boolean, characterCount: number): string {
  if (modelMissing) return "请先配置模型，才能发送消息。";
  if (!(characterCount > 0)) return "请先创建角色，才能发送消息。";
  return "";
}
