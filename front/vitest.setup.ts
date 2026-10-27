// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
const storageValues = new WeakMap<object, Map<string, string>>();

function valuesFor(storage: object): Map<string, string> {
  let values = storageValues.get(storage);
  if (!values) {
    values = new Map();
    storageValues.set(storage, values);
  }
  return values;
}

Object.defineProperties(Storage.prototype, {
  length: {
    configurable: true,
    get(this: Storage) {
      return valuesFor(this).size;
    },
  },
  clear: {
    configurable: true,
    writable: true,
    value(this: Storage) {
      valuesFor(this).clear();
    },
  },
  getItem: {
    configurable: true,
    writable: true,
    value(this: Storage, key: string) {
      return valuesFor(this).get(String(key)) ?? null;
    },
  },
  key: {
    configurable: true,
    writable: true,
    value(this: Storage, index: number) {
      return [...valuesFor(this).keys()][index] ?? null;
    },
  },
  removeItem: {
    configurable: true,
    writable: true,
    value(this: Storage, key: string) {
      valuesFor(this).delete(String(key));
    },
  },
  setItem: {
    configurable: true,
    writable: true,
    value(this: Storage, key: string, value: string) {
      valuesFor(this).set(String(key), String(value));
    },
  },
});

function createMemoryStorage(): Storage {
  return Object.create(Storage.prototype) as Storage;
}

const localStorageMock = createMemoryStorage();
const sessionStorageMock = createMemoryStorage();
Object.defineProperty(window, "localStorage", {
  configurable: true,
  value: localStorageMock,
});
Object.defineProperty(window, "sessionStorage", {
  configurable: true,
  value: sessionStorageMock,
});
Object.defineProperty(globalThis, "localStorage", {
  configurable: true,
  value: localStorageMock,
});
Object.defineProperty(globalThis, "sessionStorage", {
  configurable: true,
  value: sessionStorageMock,
});

// Mock window.matchMedia for jsdom
Object.defineProperty(window, "matchMedia", {
  writable: true,
  value: (query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  }),
});
