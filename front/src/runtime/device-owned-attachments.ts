export interface OwnedAttachment {
  kind: "image" | "audio" | "file" | "video";
  name: string;
  mimeType: string;
  data: string;
  sha256: string;
}

const ownedFileMimes = ["text/plain", "text/markdown", "text/csv", "application/json", "application/xml", "text/xml", "application/pdf", "application/vnd.openxmlformats-officedocument.wordprocessingml.document"];
const ownedVideoMimes = ["video/mp4", "video/webm", "video/quicktime"];
export async function ownedFileAttachment(blob: Blob, name: string, kind: "file" | "video" = "file"): Promise<OwnedAttachment> {
  const extensions: Record<string, string> = { txt: "text/plain", md: "text/markdown", csv: "text/csv", json: "application/json", xml: "application/xml", pdf: "application/pdf", docx: ownedFileMimes[7], mp4: "video/mp4", webm: "video/webm", mov: "video/quicktime" };
  const mimeType = blob.type.split(";")[0].trim().toLowerCase() || extensions[name.split(".").pop()?.toLowerCase() || ""] || "";
  if (!(kind === "file" ? ownedFileMimes : ownedVideoMimes).includes(mimeType) || !name || new TextEncoder().encode(name).length > 256 || /[\x00\r\n]/.test(name) || !blob.size || blob.size > 1048576 || !globalThis.crypto?.subtle) throw new Error(kind === "file" ? "请选择 UTF-8 文本、JSON、CSV、XML、PDF 或 DOCX 文件，单件不超过 1 MiB" : "请选择 MP4、WebM 或 MOV 视频，单段不超过 1 MiB");
  const bytes = new Uint8Array(await blob.arrayBuffer());
  if (!bytes.length || bytes.length !== blob.size) throw new Error("附件在读取期间发生变化");
  const digest = await crypto.subtle.digest("SHA-256", bytes);
  let binary = "";
  for (let offset = 0; offset < bytes.length; offset += 8192) binary += String.fromCharCode(...bytes.subarray(offset, offset + 8192));
  return { kind, name, mimeType, data: btoa(binary), sha256: Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, "0")).join("") };
}
export function ownedAttachmentURL(attachment: unknown): string | undefined {
  const row = attachment as Partial<OwnedAttachment>;
  const allowed = row?.kind === "file" ? ownedFileMimes : row?.kind === "video" ? ownedVideoMimes : [];
  return typeof row?.data === "string" && allowed.includes(String(row.mimeType)) && row.data.length <= Math.ceil(1048576 / 3) * 4 && /^[A-Za-z0-9+/]+={0,2}$/.test(row.data) ? `data:${row.mimeType};base64,${row.data}` : undefined;
}

export async function ownedAudioAttachment(blob: Blob, name = "voice.webm"): Promise<OwnedAttachment> {
	const mimeType = blob.type.split(";")[0].trim().toLowerCase();
	if (!["audio/wav", "audio/webm"].includes(mimeType) || !blob.size || blob.size > 1048576 || !name || name.length > 256 || /[\x00\r\n]/.test(name) || !globalThis.crypto?.subtle) throw new Error("请选择 WAV 或 WebM 音频，单段不超过 1 MiB");
	const bytes = new Uint8Array(await blob.arrayBuffer());
	if (!bytes.length || bytes.length > 1048576 || bytes.length !== blob.size) throw new Error("音频大小无效或在读取时发生变化");
	const digest = await crypto.subtle.digest("SHA-256", bytes);
	const sha256 = Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, "0")).join("");
	let binary = "";
	for (let offset = 0; offset < bytes.length; offset += 8192) binary += String.fromCharCode(...bytes.subarray(offset, offset + 8192));
	return { kind: "audio", name, mimeType, data: btoa(binary), sha256 };
}

export function ownedAudioURL(attachments: unknown): string | undefined {
	if (!Array.isArray(attachments)) return undefined;
	const audio = attachments.find((item) => item?.kind === "audio" && ["audio/wav", "audio/webm"].includes(item.mimeType) && typeof item.data === "string" && /^[A-Za-z0-9+/]+={0,2}$/.test(item.data) && item.data.length <= Math.ceil(1048576 / 3) * 4);
	return audio ? `data:${audio.mimeType};base64,${audio.data}` : undefined;
}

export function ownedMessageText(row: Record<string, unknown>): string {
	return typeof row.transcription === "string" && row.transcription && row.content === row.transcriptionSourceContent ? row.transcription : String(row.content || "");
}

export async function ownedImageAttachment(uri: string, name = "image.png"): Promise<OwnedAttachment> {
  const match = /^data:(image\/(?:png|jpeg|gif));base64,([A-Za-z0-9+/]+={0,2})$/.exec(uri);
  if (!match || match[2].length > Math.ceil(1048576 / 3) * 4) throw new Error("请选择 PNG、JPEG 或 GIF 图片，单张不超过 1 MiB");
  const bytes = Uint8Array.from(atob(match[2]), (value) => value.charCodeAt(0));
  if (!bytes.length || bytes.length > 1048576 || !globalThis.crypto?.subtle) throw new Error("图片大小无效或当前连接无法校验附件完整性");
  const digest = await crypto.subtle.digest("SHA-256", bytes);
  const sha256 = Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, "0")).join("");
  return { kind: "image", name: name.slice(0, 256), mimeType: match[1], data: match[2], sha256 };
}

export function ownedImageURL(attachments: unknown): string | undefined {
  if (!Array.isArray(attachments)) return undefined;
  const image = attachments.find((item) => item?.kind === "image" && ["image/png", "image/jpeg", "image/gif"].includes(item.mimeType) && typeof item.data === "string" && /^[A-Za-z0-9+/]+={0,2}$/.test(item.data) && item.data.length <= Math.ceil(1048576 / 3) * 4);
  return image ? `data:${image.mimeType};base64,${image.data}` : undefined;
}
