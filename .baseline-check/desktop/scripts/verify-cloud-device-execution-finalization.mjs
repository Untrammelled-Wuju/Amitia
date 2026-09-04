import { promises as fs } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(scriptDir, "../..");
const read = (relativePath) => fs.readFile(path.join(repoRoot, relativePath), "utf8");

function assert(condition, message) {
  if (!condition) throw new Error(`[verify-cloud-device-execution-finalization] ${message}`);
}

const [
  cloudGeneration,
  cloudProvider,
  cloudProtocol,
  petService,
  petWorkerResolver,
  petWorker,
  runtimeProfile,
  runtimeProfileResolve,
  serverRouter,
  serverServices,
  gameBridge,
  meshRuntime,
  connectionHub,
  runtimeDispatcher,
  deviceAgentRouter,
  devicePackageRoutes,
  frontendCapabilities,
  frontendApi,
  frontendRequestAuth,
  extensionApi,
  gameCenterView,
  runtimeCapabilityTests,
] = await Promise.all([
  read("backend/cmd/server/desktop_pet_cloud_generation.go"),
  read("backend/internal/imageprovider/cloudbridge/provider.go"),
  read("backend/internal/imageprovider/cloudbridge/protocol.go"),
  read("backend/internal/desktoppet/service.go"),
  read("backend/internal/desktoppet/worker/provider_resolution.go"),
  read("backend/internal/desktoppet/worker/worker.go"),
  read("backend/internal/runtimeprofile/current.go"),
  read("backend/internal/runtimeprofile/resolve.go"),
  read("backend/cmd/server/router.go"),
  read("backend/cmd/server/services.go"),
  read("backend/cmd/server/game_center_cloud_bridge.go"),
  read("backend/internal/devicemesh/runtime.go"),
  read("backend/internal/devicemesh/server/connection_hub.go"),
  read("backend/internal/devicemesh/agent/runtime_dispatcher.go"),
  read("backend/cmd/server/device_agent_router.go"),
  read("backend/internal/extension/device_execution_routes.go"),
  read("front/src/runtime/runtime-capabilities.ts"),
  read("front/src/composables/useApi.ts"),
  read("front/src/runtime/request-auth.ts"),
  read("front/src/views/extensions/api.ts"),
  read("front/src/views/game-center/GameCenterView.vue"),
  read("backend/cmd/server/router_runtime_capabilities_test.go"),
]);

// Desktop-pet generation: Cloud Core owns provider configuration and secrets.
assert(
  cloudProtocol.includes('ProviderName = "cloud_core"') &&
    cloudProtocol.includes('EndpointPath = "/api/device-mesh/v1/desktop-pet/image-generation"') &&
    cloudProtocol.includes("HasAPIKey bool") &&
    !/\bApiKey\s+(string|\[\]byte)\b/.test(cloudProtocol) &&
    !/json:\"apiKey/.test(cloudProtocol),
  "cloud image-generation protocol must expose metadata only and never define an API-key field",
);
assert(
  cloudGeneration.includes("runtimeprofile.ProfileCloudCore") &&
    cloudGeneration.includes("credential.DeviceAuthMiddleware") &&
    cloudGeneration.includes("repo.GetByID(req.ConfigID)") &&
    cloudGeneration.includes("ApiKey:    cfg.ApiKey") &&
    cloudGeneration.includes("provider.Submit") &&
    cloudGeneration.includes("provider.Query") &&
    cloudGeneration.includes("provider.Cancel") &&
    cloudGeneration.includes("Never return provider secrets to the device"),
  "Cloud Core must authenticate the device, resolve cloud model config, and invoke the real image provider without exporting its secret",
);
assert(
  cloudProvider.includes("agent.NewCredentialStore(dataDir)") &&
    cloudProvider.includes('Authorization", "AmitiaDevice "+cred.Credential') &&
    cloudProvider.includes("cred.CloudBaseUrl") &&
    cloudProvider.includes("materializeReferenceImages") &&
    !cloudProvider.includes("config.ApiKey"),
  "Device Agent cloud provider must use its Device Mesh credential and must never consume a provider API key locally",
);
assert(
  runtimeProfile.includes("func CurrentProcessProfile() Profile") &&
    runtimeProfile.includes("cliFlag") &&
    runtimeProfile.includes("envKey") &&
    runtimeProfileResolve.includes('const cliFlag = "--runtime-profile"') &&
    runtimeProfileResolve.includes('const envKey = "AMITIA_RUNTIME_PROFILE"'),
  "lower-level device services must resolve the actual runtime profile from the canonical CLI/environment contract instead of assuming local mode",
);
assert(
  petService.includes("runtimeprofile.CurrentProcessProfile().IsDeviceAgent()") &&
    petService.includes("cloudProvider.DescribeConfig") &&
    petService.includes("validateModelConfigForExecution") &&
    petWorkerResolver.includes("runtimeprofile.CurrentProcessProfile().IsDeviceAgent()") &&
    petWorkerResolver.includes("cloudProvider.DescribeConfig") &&
    petWorkerResolver.includes("ProviderName:   metadata.Provider") &&
    petWorkerResolver.includes("ApiType:   cloudbridge.ProviderName") &&
    petWorker.includes("resolveGenerationProvider(ctx, task)"),
  "desktop-pet task creation, retry validation, and Worker execution must all use the cloud provider bridge in Device Agent mode",
);

// Cloud Game Center: Cloud is the control plane, the selected device is the execution plane.
assert(
  serverRouter.includes("remoteGameModeAvailable") &&
    serverRouter.includes("runtimeprofile.ProfileCloudCore") &&
    serverRouter.includes("registerCloudGameCenterGateway(apiGroup, services)") &&
    runtimeCapabilityTests.includes("CloudCoreExposesRemoteGameModeWhenMeshGatewayReady") &&
    runtimeCapabilityTests.includes("CloudCoreFailsClosedWithoutMeshGateway"),
  "Cloud Core must advertise Game Mode only when the Device Mesh gateway infrastructure is composed",
);
assert(
  !frontendCapabilities.includes('runtimeProfile === "local"') &&
    frontendCapabilities.includes("gameMode: profileKnown && asBoolean(rawCapabilities.gameMode)"),
  "frontend Game Mode capability must not be hard-coded to local profile",
);
assert(
  gameBridge.includes('X-Amitia-Target-Device-ID') &&
    gameBridge.includes("RequireDeviceOwnedBy") &&
    gameBridge.includes("InvokeDeviceHandler") &&
    gameBridge.includes('gameHostManagementInvokeHandler = "gamehost.management.http"') &&
    gameBridge.includes("maxGameCenterProxyBody = 256 << 10") &&
    gameBridge.includes("buildDeviceGameCenterHTTPHandler") &&
    gameBridge.includes("registerDeviceGameCenterManagementRoutes"),
  "Cloud Game Center requests must require an owned target device and execute against that device's in-memory GameHost management router",
);
assert(
  meshRuntime.includes("func (rt *Runtime) InvokeDeviceHandler(") &&
    meshRuntime.includes("rt.Hub.GetByDevice(userID, targetDeviceID)") &&
    meshRuntime.includes("RuntimeTypeGameHost") &&
    meshRuntime.includes("ProviderPlacementDevice"),
  "Device Mesh must provide an explicit target-device GameHost invocation primitive",
);
assert(
  connectionHub.includes("func (h *ConnectionHub) GetByDevice") &&
    connectionHub.includes("func (h *ConnectionHub) GetByRuntime") &&
    connectionHub.includes("c.Generation > selected.Generation"),
  "Device Mesh routing must select the newest connection generation instead of relying on map iteration",
);
assert(
  serverServices.includes("localRuntimeDispatcher.RegisterCancellable(gameHostManagementInvokeHandler") &&
    serverServices.includes("NewChainedRuntimeDispatcher(localRuntimeDispatcher, dispatcher)") &&
    runtimeDispatcher.includes("type chainedRuntimeDispatcher struct") &&
    runtimeDispatcher.includes("func (d *defaultRuntimeDispatcher) CancelInvocation") &&
    runtimeDispatcher.includes("func (d *chainedRuntimeDispatcher) CancelInvocation") &&
    runtimeDispatcher.includes("RegisterCancellable") &&
    runtimeDispatcher.includes("RuntimeCancelDispatcher"),
  "Device Agent must compose the GameHost handler with existing runtime adapters without losing cancellation semantics",
);
assert(
  frontendApi.includes('config.headers["X-Amitia-Target-Device-ID"] = deviceId') &&
    frontendRequestAuth.includes('headers.set("X-Amitia-Target-Device-ID", deviceId)') &&
    frontendApi.includes("resolveUIHostDeviceId"),
  "desktop cloud Game Center calls must explicitly target the current Mesh device identity",
);

// Large .amitiax artifacts remain local; they do not ride the 1 MiB Mesh control channel.
assert(
  devicePackageRoutes.includes("RegisterDeviceExecutionPackageRoutes") &&
    devicePackageRoutes.includes("actor.IsLocalTrusted") &&
    devicePackageRoutes.includes("registerExtensionPackageRoutes(group, runtime)") &&
    devicePackageRoutes.includes('kernel.POST("/extensions/uninstall"') &&
    deviceAgentRouter.includes('r.Group("/api/extensions")') &&
    deviceAgentRouter.includes("RequireAuthMethod(security.AuthMethodDesktopSession)") &&
    deviceAgentRouter.includes("RegisterDeviceExecutionPackageRoutes(deviceExtensions, services.Extension)") &&
    !deviceAgentRouter.includes('r.Group("/api/game-center")'),
  "Device Agent must expose only authenticated device-local package lifecycle HTTP, not a public/full Game Center API",
);
assert(
  extensionApi.includes('managementTarget?: "game-center"') &&
    extensionApi.includes('"X-Amitia-Management-Target": managementTarget') &&
    frontendApi.includes('managementTarget === "game-center"') &&
    frontendApi.includes("gamePackageLocal") &&
    gameCenterView.includes('"X-Amitia-Management-Target": "game-center"') &&
    gameCenterView.includes('"game-center",') &&
    gameCenterView.includes("waitForPackageOperation"),
  "Game Center package preview/install/update/uninstall/status traffic must be routed to the loopback Device Agent while runtime control stays cloud-mediated",
);

console.log(
  "[verify-cloud-device-execution-finalization] PASSED: cloud pet generation secrets, local pet asset authority, Cloud Game Center gateway, explicit target device routing, and local game package authority are frozen",
);
