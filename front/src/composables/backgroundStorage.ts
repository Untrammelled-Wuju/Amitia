export type BackgroundKind = "image" | "video";
export type BackgroundMode = "default" | "color" | "gradient" | "image" | "video";
export interface BackgroundSettings {
  enabled: boolean;
  kind: BackgroundKind;
  mode: BackgroundMode;
  name: string;
  opacity: number;
  blurEnabled: boolean;
  blurRadius: number;
  color: string;
  gradientStart: string;
  gradientEnd: string;
  gradientAngle: number;
  sourceUri: string;
  fit: "cover" | "contain" | "stretch" | "repeat";
  overlayColor: string;
  overlayOpacity: number;
}
export const defaultBackground: BackgroundSettings = {
  enabled: false,
  kind: "image",
  mode: "default",
  name: "",
  opacity: 0.35,
  blurEnabled: false,
  blurRadius: 10,
  color: "#F7F8FA",
  gradientStart: "#F7F8FA",
  gradientEnd: "#E8EDF5",
  gradientAngle: 180,
  sourceUri: "",
  fit: "cover",
  overlayColor: "#000000",
  overlayOpacity: 0,
};
export function normalizeBackground(value: unknown): BackgroundSettings {
  const raw = value && typeof value === "object" ? value as Partial<BackgroundSettings> : {};
  const number = (value: unknown, fallback: number, max: number) => typeof value === "number" && Number.isFinite(value) ? Math.max(0, Math.min(max, value)) : fallback;
  const color = (value: unknown, fallback: string) => typeof value === "string" && /^#[\da-f]{6}$/i.test(value) ? value.toUpperCase() : fallback;
  const mode = ["default", "color", "gradient", "image", "video"].includes(raw.mode ?? "") ? raw.mode as BackgroundMode : raw.kind === "video" ? "video" : "image";
  return {
    enabled: raw.enabled === true,
    kind: raw.kind === "video" ? "video" : "image",
    mode,
    name: typeof raw.name === "string" ? raw.name : "",
    opacity: number(raw.opacity, 0.35, 1),
    blurEnabled: raw.blurEnabled === true,
    blurRadius: number(raw.blurRadius, 10, 30),
    color: color(raw.color, defaultBackground.color),
    gradientStart: color(raw.gradientStart, defaultBackground.gradientStart),
    gradientEnd: color(raw.gradientEnd, defaultBackground.gradientEnd),
    gradientAngle: number(raw.gradientAngle, 180, 360),
    sourceUri: typeof raw.sourceUri === "string" ? raw.sourceUri : "",
    fit: ["cover", "contain", "stretch", "repeat"].includes(raw.fit ?? "") ? raw.fit as BackgroundSettings["fit"] : "cover",
    overlayColor: color(raw.overlayColor, defaultBackground.overlayColor),
    overlayOpacity: number(raw.overlayOpacity, 0, 1),
  };
}
export function backgroundFileKind(file: Pick<File, "name" | "size">): BackgroundKind {
  const extension = file.name.split(".").pop()?.toLowerCase();
  const kind = ["mp4", "webm"].includes(extension ?? "") ? "video" : ["png", "jpg", "jpeg", "webp", "gif"].includes(extension ?? "") ? "image" : null;
  if (!kind) throw new Error("请选择 PNG、JPG、WebP、GIF 图片或 MP4、WebM 视频");
  if (file.size <= 0) throw new Error("背景文件为空");
  if (file.size > (kind === "video" ? 150 : 20) * 1024 * 1024) throw new Error(kind === "video" ? "视频不能超过 150 MB" : "图片不能超过 20 MB");
  return kind;
}
async function database(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open("amitia-appearance-background", 1);
    request.onupgradeneeded = () => request.result.createObjectStore("background");
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error);
    request.onblocked = () => reject(new Error("背景存储被占用，请关闭其他窗口重试"));
  });
}
export async function readBackground(): Promise<{ settings: BackgroundSettings; media: Blob | null }> {
  const db = await database();
  try {
    return await new Promise((resolve, reject) => {
      const tx = db.transaction("background", "readonly");
      const store = tx.objectStore("background");
      const settings = store.get("settings"), media = store.get("media");
      tx.oncomplete = () => resolve({ settings: normalizeBackground(settings.result), media: media.result instanceof Blob ? media.result : null });
      tx.onabort = () => reject(tx.error);
      tx.onerror = () => reject(tx.error);
    });
  } finally { db.close(); }
}
export async function writeBackground(settings: BackgroundSettings, media?: Blob | null): Promise<void> {
  const db = await database();
  try {
    await new Promise<void>((resolve, reject) => {
      const tx = db.transaction("background", "readwrite");
      const store = tx.objectStore("background");
      store.put(settings, "settings");
      if (media === null) store.delete("media");
      else if (media !== undefined) store.put(media, "media");
      tx.oncomplete = () => resolve();
      tx.onabort = () => reject(tx.error);
      tx.onerror = () => reject(tx.error);
    });
  } finally { db.close(); }
}

export async function writeBackgroundHistoryMedia(key: string, media: Blob): Promise<void> {
  const db = await database();
  try {
    await new Promise<void>((resolve, reject) => {
      const tx = db.transaction("background", "readwrite");
      tx.objectStore("background").put(media, key);
      tx.oncomplete = () => resolve();
      tx.onabort = () => reject(tx.error);
      tx.onerror = () => reject(tx.error);
    });
  } finally {
    db.close();
  }
}

export async function readBackgroundHistoryMedia(key: string): Promise<Blob | null> {
  if (!key) return null;
  const db = await database();
  try {
    return await new Promise<Blob | null>((resolve, reject) => {
      const tx = db.transaction("background", "readonly");
      const request = tx.objectStore("background").get(key);
      tx.oncomplete = () => resolve(request.result instanceof Blob ? request.result : null);
      tx.onabort = () => reject(tx.error);
      tx.onerror = () => reject(tx.error);
    });
  } finally {
    db.close();
  }
}

export async function deleteBackgroundHistoryMedia(key: string): Promise<void> {
  if (!key) return;
  const db = await database();
  try {
    await new Promise<void>((resolve, reject) => {
      const tx = db.transaction("background", "readwrite");
      tx.objectStore("background").delete(key);
      tx.oncomplete = () => resolve();
      tx.onabort = () => reject(tx.error);
      tx.onerror = () => reject(tx.error);
    });
  } finally {
    db.close();
  }
}
