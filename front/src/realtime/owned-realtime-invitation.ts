import { apiClient } from "../composables/useApi";
import type { OwnedExecutionScope } from "../runtime/device-owned-chat";
import { ownedAuthorityFields, sameOwnedAuthority } from "../runtime/owned-speech-result";

export interface OwnedAcceptedInvitation {
  characterId: string;
  conversationId: string;
  conversationOrigin?: { ownerId: string; id: string };
  historicalRoleId?: string;
  scope: OwnedExecutionScope;
  ticket: Record<string, any>;
}

export async function acceptOwnedRealtimeInvitation(callId: string, characterId: string, signal: AbortSignal): Promise<OwnedAcceptedInvitation> {
  const current = () => { if (signal.aborted) throw new Error("Core 连接已变化，邀请已失效"); };
  current();
  if (!/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(callId) || !characterId) throw new Error("邀请编号或角色无效");
  const path = `/api/device-mesh/v1/business/realtime/invitations/${callId}`;
  const response = await apiClient.get(path, { params: { characterId }, signal });
  current();
  const invitation = (response?.data?.data ?? response?.data)?.invitation;
  const scope = invitation?.executionScope as OwnedExecutionScope;
  if (!scope || ownedAuthorityFields.some((key) => scope[key] === undefined) || invitation.id !== callId || invitation.characterId !== characterId || scope.roleId !== characterId || invitation.recipientDeviceId !== scope.initiatorDeviceId || invitation.status !== "pending" || invitation.revision !== 1 || Date.parse(invitation.expiresAt) <= Date.now() || !Number.isFinite(Date.parse(invitation.expiresAt)) || typeof invitation.conversationId !== "string" || !invitation.conversationId || !/^[A-Za-z0-9_-]{43}$/.test(invitation.nonce)) throw new Error("邀请已过期、已处理或不属于当前设备角色");
  const frozen = JSON.parse(JSON.stringify(scope)) as OwnedExecutionScope;
  const requestId = crypto.randomUUID();
  const acceptedResponse = await apiClient.post(`${path}/accept`, { requestId, characterId, expectedExecutionScope: frozen, expectedRevision: 1, nonce: invitation.nonce }, { signal });
  current();
  const accepted = acceptedResponse?.data?.data ?? acceptedResponse?.data;
  const ack = accepted?.acknowledgement;
  const versions = ack?.versions;
  const ticket = accepted?.ticket;
  if (accepted?.saved !== true || accepted.invitation?.id !== callId || accepted.invitation?.status !== "accepted" || accepted.invitation?.revision !== 2 || !sameOwnedAuthority(frozen, accepted.invitation?.executionScope) || ack?.requestId !== requestId || ack?.ownerId !== frozen.resourceOwnerId || !versions || Object.keys(versions).length !== 1 || versions[`checkpoint/realtime-invitation/${callId}`] !== 2 || !sameOwnedAuthority(frozen, ticket?.executionScope) || ticket?.wsPath !== "/api/device-mesh/v1/business/realtime/session" || !/^[A-Za-z0-9_-]{43}$/.test(ticket?.ticket)) throw new Error("接听未获得原所有者确认，或通话票据无效");
  return { characterId, conversationId: invitation.conversationId, conversationOrigin: invitation.conversationOrigin, historicalRoleId: invitation.historicalRoleId, scope: frozen, ticket };
}
