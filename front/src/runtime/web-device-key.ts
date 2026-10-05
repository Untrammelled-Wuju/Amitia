type DeviceKey = { privateKey: CryptoKey; publicKey: CryptoKey };
const pendingKeys = new Map<string, Promise<DeviceKey>>();

function openKeyDatabase(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open("amitia.device-identity.v1", 1);
    request.onupgradeneeded = () => request.result.createObjectStore("keys");
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(new Error("浏览器设备安全存储不可用"));
  });
}

async function deviceKey(deviceId: string): Promise<DeviceKey> {
  let pending = pendingKeys.get(deviceId);
  if (!pending) {
    pending = (async () => {
      if (!crypto.subtle || !deviceId) throw new Error("浏览器不支持设备身份签名，请使用 Amitia 客户端");
      const database = await openKeyDatabase();
      try {
        const stored = await new Promise<DeviceKey | undefined>((resolve, reject) => {
          const request = database.transaction("keys", "readonly").objectStore("keys").get(deviceId);
          request.onsuccess = () => resolve(request.result);
          request.onerror = () => reject(new Error("无法读取浏览器设备密钥"));
        });
        if (stored?.privateKey && stored.publicKey) return stored;
        const generated = await crypto.subtle.generateKey({ name: "Ed25519" }, false, ["sign", "verify"]) as CryptoKeyPair;
        const key = { privateKey: generated.privateKey, publicKey: generated.publicKey };
        await new Promise<void>((resolve, reject) => {
          const transaction = database.transaction("keys", "readwrite");
          transaction.objectStore("keys").add(key, deviceId);
          transaction.oncomplete = () => resolve();
          transaction.onerror = () => reject(new Error("无法保存浏览器设备密钥，请重新尝试配对"));
          transaction.onabort = () => reject(new Error("浏览器设备密钥保存未完成"));
        });
        return key;
      } finally { database.close(); }
    })();
    pendingKeys.set(deviceId, pending);
    pending.catch(() => pendingKeys.delete(deviceId));
  }
  return pending;
}

function base64URL(buffer: ArrayBuffer) {
  return btoa(String.fromCharCode(...new Uint8Array(buffer))).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

export async function signWebRequest(deviceId: string, coreId: string, url: string, init: RequestInit): Promise<RequestInit> {
  const key = await deviceKey(deviceId);
  if (init.body instanceof FormData) {
    const headers = new Headers(init.headers);
    headers.delete("Content-Type");
    init = { ...init, headers };
  }
  const request = new Request(url, init);
  const body = await request.clone().arrayBuffer();
  if (body.byteLength > 64 * 1024 * 1024) throw new Error("请求超过设备通道大小限制");
  const encoder = new TextEncoder();
  const digest = async (value: BufferSource) => Array.from(new Uint8Array(await crypto.subtle.digest("SHA-256", value)), (byte) => byte.toString(16).padStart(2, "0")).join("");
  const target = new URL(url);
  const proof = {
    publicKey: base64URL(await crypto.subtle.exportKey("raw", key.publicKey)),
    audience: coreId, method: request.method, path: target.pathname + target.search,
    bodyHash: await digest(body), tokenHash: await digest(encoder.encode((request.headers.get("Authorization") || "").trim())),
    issuedAt: Math.floor(Date.now()/1000), nonce: crypto.randomUUID(), signature: "",
  };
  proof.signature = base64URL(await crypto.subtle.sign("Ed25519", key.privateKey, encoder.encode(JSON.stringify(proof))));
  const headers = new Headers(request.headers);
  headers.set("X-Amitia-Device-Proof", base64URL(encoder.encode(JSON.stringify(proof)).buffer));
  return { ...init, headers, body: ["GET", "HEAD"].includes(request.method) ? undefined : body, redirect: "error" };
}

export async function signWebPairingClaim(coreId: string, body: { deviceId: string; runtimeId: string; platform: string; label: string; offerToken: string; setupCode: string }) {
  const key = await deviceKey(body.deviceId);
  const hash = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(JSON.stringify(body)));
  const proof = {
    publicKey: base64URL(await crypto.subtle.exportKey("raw", key.publicKey)),
    audience: coreId, method: "POST", path: "/api/public/device-mesh/v1/pairing/claim",
    bodyHash: Array.from(new Uint8Array(hash), (value) => value.toString(16).padStart(2, "0")).join(""),
    issuedAt: Math.floor(Date.now()/1000), nonce: crypto.randomUUID(), signature: "",
  };
  proof.signature = base64URL(await crypto.subtle.sign("Ed25519", key.privateKey, new TextEncoder().encode(JSON.stringify(proof))));
  return proof;
}
