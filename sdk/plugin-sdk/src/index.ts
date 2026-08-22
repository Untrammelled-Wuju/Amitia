export * from "./types";
export * from "./manifest";
export * from "./errors";
export * from "./runtime";
export * from "./tools";
export * from "./events";
export * from "./hooks";
export * from "./tasks";
export * from "./host";
export * from "./storage";
export * from "./secrets";
export * from "./ui";
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
export const SUPPORTED_MANIFEST_VERSION = 2;
export const SUPPORTED_HOST_API_RANGE = ">=1.0.0 <2.0.0";
export const SUPPORTED_RUNTIME_RPC_RANGE = ">=1.0.0 <2.0.0";
export const MIN_AMITIA_VERSION = "0.1.0";

export interface CompatibilityMatrix {
  readonly sdkVersion: string;
  readonly supportedManifestVersion: number;
  readonly supportedHostApiRange: string;
  readonly supportedRuntimeRpcRange: string;
  readonly minAmitiaVersion: string;
}

export function getCompatibilityMatrix(): CompatibilityMatrix {
  return {
    sdkVersion: SDK_VERSION,
    supportedManifestVersion: SUPPORTED_MANIFEST_VERSION,
    supportedHostApiRange: SUPPORTED_HOST_API_RANGE,
    supportedRuntimeRpcRange: SUPPORTED_RUNTIME_RPC_RANGE,
    minAmitiaVersion: MIN_AMITIA_VERSION,
  };
}

export * from "./client-plugin-runtime";

export * from "./client-package-runtime";
