function hexadecimal(bytes: Uint8Array): string {
  return Array.from(bytes, (value) => value.toString(16).padStart(2, "0")).join("");
}

export function randomUUID(): string {
  const cryptoObject = globalThis.crypto;
  if (typeof cryptoObject?.randomUUID === "function" && cryptoObject.randomUUID !== randomUUID) {
    return cryptoObject.randomUUID();
  }
  const bytes = new Uint8Array(16);
  if (typeof cryptoObject?.getRandomValues === "function") {
    cryptoObject.getRandomValues(bytes);
  } else {
    for (let index = 0; index < bytes.length; index += 1) {
      bytes[index] = Math.floor(Math.random() * 256);
    }
  }
  bytes[6] = (bytes[6] & 0x0f) | 0x40;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;
  const value = hexadecimal(bytes);
  return `${value.slice(0, 8)}-${value.slice(8, 12)}-${value.slice(12, 16)}-${value.slice(16, 20)}-${value.slice(20)}`;
}

export function installSecureRandomUUID(): void {
  const target = globalThis.crypto;
  if (!target || typeof target.randomUUID === "function") return;
  Object.defineProperty(target, "randomUUID", {
    configurable: true,
    value: randomUUID,
  });
}
