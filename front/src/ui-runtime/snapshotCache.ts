import type { UIContributionSnapshot } from "@/stores/extensionUI";

const databaseName = "amitia.ui.snapshot";
const storeName = "snapshots";
const localSnapshotPrefix = "amitia.ui.snapshot.lkg.v1:";

function removeSupersededLocalSnapshots(platform: string, deviceId: string): void {
  const suffix = `:${platform}:${deviceId}`;
  const keys = Array.from({ length: window.localStorage.length }, (_, index) => window.localStorage.key(index));
  for (const storedKey of keys) {
    if (!storedKey?.startsWith(localSnapshotPrefix) || !storedKey.endsWith(suffix)) continue;
    const namespace = decodeURIComponent(storedKey.slice(localSnapshotPrefix.length, -suffix.length));
    if (/^http:\/\/127\.0\.0\.1:\d+\|device=/.test(namespace) && namespace.endsWith(`|device=${deviceId}`)) {
      window.localStorage.removeItem(storedKey);
    }
  }
}

function key(platform: string, deviceId: string, backendNamespace?: string): string {
  return JSON.stringify([backendNamespace || window.location.origin, platform, deviceId]);
}

function openDatabase(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open(databaseName, 1);
    request.onupgradeneeded = () => request.result.createObjectStore(storeName);
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error);
    request.onblocked = () => reject(new Error("界面快照数据库被占用"));
  });
}

export async function saveLastKnownGoodSnapshot(platform: string, deviceId: string, snapshot: UIContributionSnapshot, backendNamespace?: string): Promise<void> {
  const db = await openDatabase();
  try {
    await new Promise<void>((resolve, reject) => {
      const transaction = db.transaction(storeName, "readwrite");
      transaction.objectStore(storeName).put({ savedAt: Date.now(), snapshot: JSON.parse(JSON.stringify(snapshot)) }, key(platform, deviceId, backendNamespace));
      transaction.oncomplete = () => resolve();
      transaction.onabort = () => reject(transaction.error);
      transaction.onerror = () => reject(transaction.error);
    });
    if (window.amitiaDesktop && backendNamespace === `desktop-local:${deviceId}` && deviceId) {
      removeSupersededLocalSnapshots(platform, deviceId);
    }
  } finally {
    db.close();
  }
}

export async function loadLastKnownGoodSnapshot(platform: string, deviceId: string, backendNamespace?: string): Promise<UIContributionSnapshot | null> {
  const db = await openDatabase();
  try {
    return await new Promise((resolve, reject) => {
      const transaction = db.transaction(storeName, "readonly");
      const request = transaction.objectStore(storeName).get(key(platform, deviceId, backendNamespace));
      transaction.oncomplete = () => resolve(request.result?.snapshot ?? null);
      transaction.onabort = () => reject(transaction.error);
      transaction.onerror = () => reject(transaction.error);
    });
  } finally {
    db.close();
  }
}
