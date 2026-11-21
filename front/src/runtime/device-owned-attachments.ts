import { createAuthenticatedFetchInit } from "./request-auth";
import { resolveApiUrl } from "./runtime-adapter";

export interface OwnedAttachment {
  artifactId: string;
  ownerDeviceId: string;
  storageDeviceId: string;
  kind: "image" | "audio" | "file" | "video";
  name: string;
  mimeType: string;
  sha256: string;
  sizeBytes: number;
}

export interface OwnedAttachmentInput {
  file: Blob;
  kind: OwnedAttachment["kind"];
  name?: string;
}

export interface OwnedAttachmentMessageContext {
  messageId: string;
  conversationId: string;
  characterId?: string;
  conversationOwnerId?: string;
  historicalRoleId?: string;
}

interface ArtifactEnvelope {
  artifactId: string;
  ownerDeviceId: string;
  storageDeviceId: string;
  kind: OwnedAttachment["kind"];
  filename: string;
  mimeType: string;
  blobDigest: string;
  sizeBytes: number;
}

function originalError(payload: unknown, fallback: string): Error {
  const row = payload && typeof payload === "object" ? payload as Record<string, unknown> : {};
  const values = ["message", "msg", "error", "internalMessage", "detail", "cause", "rawError", "reason"]
    .map((key) => typeof row[key] === "string" ? String(row[key]).trim() : "")
    .filter(Boolean);
  return new Error(values.length ? values.join("\n") : fallback);
}

function validAttachmentName(name: string): boolean {
  return !!name && new TextEncoder().encode(name).length <= 256 && !/[\x00\r\n]/.test(name);
}

export function ownedAttachmentKindOf(file: { type?: string; name?: string }): OwnedAttachment["kind"] {
  const type = String(file?.type || "").toLowerCase();
  const name = String(file?.name || "");
  if (type.startsWith("image/") || /\.(png|jpe?g|gif|webp|bmp|svg)$/i.test(name)) return "image";
  if (type.startsWith("video/") || /\.(mp4|m4v|mov|webm|avi|mkv)$/i.test(name)) return "video";
  return "file";
}

export async function uploadOwnedAttachment(input: OwnedAttachmentInput): Promise<OwnedAttachment> {
  const name = String(input.name || (input.file as File).name || "attachment").trim();
  if (!(input.file.size > 0) || !validAttachmentName(name)) {
    throw new Error("附件为空或名称无效");
  }
  if (!["image", "audio", "video", "file"].includes(input.kind)) {
    throw new Error("附件类型无效");
  }
  const mimeType = String(input.file.type || "").split(";", 1)[0].trim().toLowerCase() || "application/octet-stream";
  const path = `/api/artifacts/v1/chat-uploads?${new URLSearchParams({ filename: name, kind: input.kind })}`;
  const init = await createAuthenticatedFetchInit(path, {
    method: "POST",
    headers: { "Content-Type": mimeType },
    body: input.file,
  });
  const response = await fetch(await resolveApiUrl(path), init);
  const payload = await response.json().catch((error) => {
    throw new Error(`附件上传响应无法解析: ${response.status} ${response.statusText}\n${String(error)}`);
  });
  if (!response.ok) {
    throw originalError(payload, `附件上传失败: ${response.status} ${response.statusText}`);
  }
  const artifact = payload?.artifact as ArtifactEnvelope | undefined;
  if (!artifact || !artifact.artifactId || !artifact.ownerDeviceId || !artifact.storageDeviceId || !artifact.filename || !artifact.mimeType || !/^sha256:[a-f0-9]{64}$/.test(String(artifact.blobDigest || "")) || !Number.isSafeInteger(artifact.sizeBytes) || artifact.sizeBytes <= 0) {
    throw new Error(`附件上传响应缺少完整元数据:\n${JSON.stringify(payload)}`);
  }
  return {
    artifactId: artifact.artifactId,
    ownerDeviceId: artifact.ownerDeviceId,
    storageDeviceId: artifact.storageDeviceId,
    kind: artifact.kind,
    name: artifact.filename,
    mimeType: artifact.mimeType,
    sha256: artifact.blobDigest.slice("sha256:".length),
    sizeBytes: artifact.sizeBytes,
  };
}

export function ownedAttachmentContentURL(attachment: unknown, context: OwnedAttachmentMessageContext): string | undefined {
  const row = attachment as Partial<OwnedAttachment>;
  if (!row?.artifactId || !row.ownerDeviceId || !row.storageDeviceId || !row.kind || !row.name || !row.mimeType || !/^[a-f0-9]{64}$/.test(String(row.sha256 || "")) || !Number.isSafeInteger(row.sizeBytes)) {
    return undefined;
  }
  const query = new URLSearchParams({
    conversationId: context.conversationId,
    messageId: context.messageId,
  });
  if (context.conversationOwnerId) query.set("conversationOwnerId", context.conversationOwnerId);
  if (context.characterId) query.set("characterId", context.characterId);
  if (context.historicalRoleId) query.set("historicalRoleId", context.historicalRoleId);
  return `/api/device-mesh/v1/business/attachments/${encodeURIComponent(row.artifactId)}/content?${query}`;
}

export function ownedAttachmentBlock(attachment: unknown, context: OwnedAttachmentMessageContext): Record<string, unknown> | undefined {
  const row = attachment as Partial<OwnedAttachment>;
  const url = ownedAttachmentContentURL(row, context);
  if (!url) return undefined;
  return { ...row, id: row.artifactId, size: row.sizeBytes, url, downloadUrl: url };
}

export function ownedAttachmentURL(attachment: unknown, context?: OwnedAttachmentMessageContext): string | undefined {
  return context ? ownedAttachmentContentURL(attachment, context) : undefined;
}

export function ownedAudioURL(attachments: unknown, context?: OwnedAttachmentMessageContext): string | undefined {
  if (!Array.isArray(attachments)) return undefined;
  return ownedAttachmentURL(attachments.find((item) => item?.kind === "audio"), context);
}

export function ownedImageURL(attachments: unknown, context?: OwnedAttachmentMessageContext): string | undefined {
  if (!Array.isArray(attachments)) return undefined;
  return ownedAttachmentURL(attachments.find((item) => item?.kind === "image"), context);
}

export function ownedVideoURL(attachments: unknown, context?: OwnedAttachmentMessageContext): string | undefined {
  if (!Array.isArray(attachments)) return undefined;
  return ownedAttachmentURL(attachments.find((item) => item?.kind === "video"), context);
}

export function ownedMessageText(row: Record<string, unknown>): string {
  return typeof row.transcription === "string" && row.transcription && row.content === row.transcriptionSourceContent ? row.transcription : String(row.content || "");
}
