export * from "./types";
export * from "./manifest";
export * from "./catalog";
export * from "./errors";
export * from "./runtime";
export * from "./tools";
export * from "./events";
export * from "./hooks";
export * from "./tasks";
export * from "./host";
export * from "./scheduled-tasks";
export * from "./storage";
export * from "./secrets";
export * from "./ui";
export * from "./slot-contract";
export * from "./context";
export * from "./control";
export * from "./fiber";

export {
  defineExtension,
  bootstrapExtension,
  assertContractVersion,
} from "./context";

export {
  createMockHost,
  createExtensionTestRuntime,
  invokeTool,
  emitEvent,
  executeHook,
  mockPermission,
  mockScope,
  mockStorage,
  mockSecretReference,
  advanceTime,
  simulateCancellation,
  simulateRuntimeCrash,
  InMemoryStorageBackend,
} from "./testing";

export const SDK_VERSION = "1.0.0";
export const SUPPORTED_MANIFEST_VERSION = 1;
export const HOST_API_VERSION = "1.0.0";
export const RUNTIME_RPC_VERSION = "1.0.0";

export * from "./client-plugin-runtime";

export * from "./client-package-runtime";
