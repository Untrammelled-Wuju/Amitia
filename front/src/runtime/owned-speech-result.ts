import type { OwnedExecutionScope } from "./device-owned-chat";

export const ownedAuthorityFields: Array<keyof OwnedExecutionScope> = ["authorizationRealm", "spaceId", "initiatorDeviceId", "targetDeviceId", "coreId", "providerEpoch", "coordinated", "modeRevision", "permissionRevision", "targetPermissionRevision", "targetProviderEpoch", "roleId", "roleRevision", "roleOwnerId", "resourceOwnerId"];
export const sameOwnedAuthority = (left: OwnedExecutionScope, right: OwnedExecutionScope) => ownedAuthorityFields.every((key) => left?.[key] === right?.[key]);

export async function validateOwnedSpeechResult(result: any, requestId: string, expected: OwnedExecutionScope): Promise<Uint8Array> {
  const digest = async (bytes: Uint8Array) => Array.from(new Uint8Array(await crypto.subtle.digest("SHA-256", bytes as Uint8Array<ArrayBuffer>)), (value) => value.toString(16).padStart(2, "0")).join("");
  const resourceId = `speech/${(await digest(new TextEncoder().encode(`${expected.coreId}\0${expected.initiatorDeviceId}\0${requestId}`))).slice(0, 32)}`;
  if (ownedAuthorityFields.some((key) => expected[key] === undefined) || result?.requestId !== requestId || result?.executionScope?.requestId !== `speech/${requestId}` || result?.saved !== true || !sameOwnedAuthority(expected, result.executionScope) || result?.acknowledgement?.requestId !== result.executionScope.requestId || result?.acknowledgement?.ownerId !== expected.resourceOwnerId || result?.acknowledgement?.versions?.[`tool-result/${resourceId}`] !== 1 || result?.acknowledgement?.versions?.[`checkpoint/${resourceId}`] !== 2) throw new Error("语音尚未获得数据所有者保存确认，已拦截播放");
  const audio = result.audio;
  if (audio?.mime !== "audio/mpeg" || typeof audio.data !== "string" || audio.data.length > 1398104 || !/^[a-f0-9]{64}$/.test(audio.sha256 || "") || !/^[A-Za-z0-9+/]+={0,2}$/.test(audio.data)) throw new Error("语音文件格式或大小无效");
  const binary = atob(audio.data);
  if (!binary.length || binary.length > 1048576) throw new Error("语音文件超出大小上限");
  const bytes = Uint8Array.from(binary, (value) => value.charCodeAt(0));
  if (await digest(bytes) !== audio.sha256) throw new Error("语音文件完整性校验失败");
  return bytes;
}
