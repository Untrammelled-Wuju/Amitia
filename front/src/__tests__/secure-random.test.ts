import { afterEach, expect, it, vi } from "vitest";
import { installSecureRandomUUID, randomUUID } from "@/runtime/secure-random";

afterEach(() => {
  vi.unstubAllGlobals();
});

it("generates a version 4 UUID without crypto.randomUUID", () => {
  vi.stubGlobal("crypto", {
    getRandomValues(bytes: Uint8Array) {
      bytes.fill(7);
      return bytes;
    },
  });
  const value = randomUUID();
  expect(value).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/);
});

it("installs a non-recursive fallback", () => {
  const cryptoObject: { getRandomValues: (bytes: Uint8Array) => Uint8Array; randomUUID?: () => string } = {
    getRandomValues(bytes: Uint8Array) {
      bytes.fill(9);
      return bytes;
    },
  };
  vi.stubGlobal("crypto", cryptoObject);
  installSecureRandomUUID();
  expect(cryptoObject.randomUUID!()).toMatch(/^[0-9a-f-]{36}$/);
});
