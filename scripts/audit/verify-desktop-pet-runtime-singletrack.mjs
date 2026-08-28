#!/usr/bin/env node

import { existsSync, readFileSync, readdirSync } from "node:fs";
import { resolve, relative } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = fileURLToPath(new URL(".", import.meta.url));
const repoRoot = resolve(__dirname, "../..");
const runtimeRoot = resolve(repoRoot, "backend/internal/desktoppet/runtime");
const protocolRoot = resolve(runtimeRoot, "protocol");
const errors = [];

const FORBIDDEN_LEGACY_RUNTIME_FILES = [
  "auth.go",
  "command_store.go",
  "connection.go",
  "dispatcher.go",
  "errors.go",
  "event_dedup.go",
  "handler.go",
  "lifecycle.go",
  "metrics.go",
  "notifier.go",
  "pending.go",
  "reconciler.go",
  "registry.go",
  "router.go",
  "service.go",
  "snapshot.go",
  "state_store.go",
  "version.go",
];

const FORBIDDEN_LEGACY_WIRING_FILES = [
  "backend/internal/desktoppet/behavior/wiring/active_pet_port.go",
  "backend/internal/desktoppet/behavior/wiring/runtime_action_port.go",
];

function requireText(relativePath, requiredPatterns, forbiddenPatterns = []) {
  const absolute = resolve(repoRoot, relativePath);
  if (!existsSync(absolute)) {
    errors.push(`MISSING: ${relativePath}`);
    return;
  }
  const content = readFileSync(absolute, "utf8");
  for (const pattern of requiredPatterns) {
    if (!pattern.test(content)) {
      errors.push(`REQUIRED pattern ${pattern} missing in ${relativePath}`);
    }
  }
  for (const pattern of forbiddenPatterns) {
    if (pattern.test(content)) {
      errors.push(`FORBIDDEN pattern ${pattern} found in ${relativePath}`);
    }
  }
}

for (const name of FORBIDDEN_LEGACY_RUNTIME_FILES) {
  const absolute = resolve(runtimeRoot, name);
  if (existsSync(absolute)) {
    errors.push(`FORBIDDEN legacy runtime file exists: ${relative(repoRoot, absolute)}`);
  }
}

for (const relativePath of FORBIDDEN_LEGACY_WIRING_FILES) {
  if (existsSync(resolve(repoRoot, relativePath))) {
    errors.push(`FORBIDDEN legacy behavior adapter exists: ${relativePath}`);
  }
}

if (existsSync(protocolRoot)) {
  const directGoFiles = readdirSync(protocolRoot, { withFileTypes: true })
    .filter((entry) => entry.isFile() && entry.name.endsWith(".go"))
    .map((entry) => entry.name);
  for (const name of directGoFiles) {
    errors.push(`FORBIDDEN Runtime V1 protocol file exists: backend/internal/desktoppet/runtime/protocol/${name}`);
  }
}

requireText(
  "backend/cmd/server/router.go",
  [/runtimev2\.RegisterInternalRoutes\s*\(/, /runtimev2\.RegisterUserRoutes\s*\(/],
  [/runtime\.RegisterInternalRoutes\s*\(/, /runtime\.RegisterUserRoutes\s*\(/],
);

requireText(
  "backend/cmd/server/services.go",
  [
    /runtimeConfig\.Enabled\s*=\s*config\.AppCfg\.DesktopPetRuntime\.Enabled\s*&&\s*runtimePolicy\.DesktopPet/,
    /runtimev2\.NewRuntimeFacadeWithDeviceRuntime\s*\([\s\S]*?kernelContainer\.DeviceRuntimeSessions\s*\)/,
    /devicemesh\.NewCloudRuntimeWithHubAndSessions\s*\([\s\S]*?kernelContainer\.DeviceRuntimeSessions/,
    /wiring\.NewV2RuntimeActionAdapter\s*\(/,
    /wiring\.NewV2ActivePetAdapter\s*\(/,
    /desktoppet\.LegacyWriteCutoverReady\s*\(ctx\.DB\)/,
    /services\.InstallationCoordinator\s*==\s*nil/,
  ],
  [
    /runtimev2\.NewRuntimeFacade\s*\(/,
    /runtime\.NewService\s*\(/,
    /wiring\.NewRuntimeActionAdapter\s*\(/,
    /wiring\.NewActivePetAdapter\s*\(/,
  ],
);

requireText(
  "backend/internal/desktoppet/runtime/protocol/v2/router.go",
  [
    /runtimeV2WebSocketSubprotocol\s*=\s*"amitia\.runtime\.v2"/,
    /runtimeV2BootstrapProtocolPrefix\s*=\s*"amitia\.runtime\.bootstrap\."/,
    /websocket\.IsWebSocketUpgrade\s*\(r\)/,
    /parseRuntimeBootstrapSubprotocol\s*\(r\)/,
    /upgrader\.Subprotocols\s*=\s*\[\]string\{selectedProtocol\}/,
  ],
  [
    /rawTicket\s*:=\s*strings\.TrimSpace\(r\.URL\.Query\(\)\.Get\("ticket"\)\)/,
  ],
);

requireText(
  "desktop/src/main/pet/manager.ts",
  [
    /bootstrapTicket:\s*issued\.ticket/,
    /buildRuntimeV2URL\(runtimeId,\s*deviceId\)/,
  ],
  [
    /searchParams\.set\("ticket"/,
  ],
);

requireText(
  "desktop/src/desktop-pet/runtime/runtime-handler-v2.ts",
  [
    /RUNTIME_V2_WEBSOCKET_SUBPROTOCOL\s*=\s*"amitia\.runtime\.v2"/,
    /RUNTIME_V2_BOOTSTRAP_SUBPROTOCOL_PREFIX\s*=\s*"amitia\.runtime\.bootstrap\."/,
    /new WebSocket\([\s\S]*?buildRuntimeWebSocketProtocols\(this\.config\.bootstrapTicket\)/,
  ],
);

requireText(
  "backend/internal/desktoppet/legacy_package_flag.go",
  [
    /legacyInstallationWriteDisabled\s+atomic\.Bool/,
    /legacyEditingWriteDisabled\s+atomic\.Bool/,
    /func LegacyWriteCutoverReady\(db \*gorm\.DB\) error/,
  ],
);

requireText(
  "backend/internal/desktoppet/migration/repository.go",
  [
    /JOIN desktop_pet_migration_operations AS o ON o\.id = w\.operation_id/,
    /o\.kind = \? AND o\.status IN \?/,
    /StageLegacyWriteBlocked/,
    /StageCompleted/,
  ],
);

requireText(
  "backend/internal/desktoppet/migration/runner.go",
  [
    /StageWriteCutover, StageLegacyWriteBlocked/,
    /legacyWriteRefresh == nil/,
    /if op\.Stage == StageLegacyWriteBlocked \{[\s\S]*?legacyWriteRefresh\(\)[\s\S]*?StageLegacyWriteBlocked, StageCompleted/,
  ],
);

requireText(
  "backend/internal/desktoppet/migration/plans/desktop_pet_v2_cutover.go",
  [
    /The durable authority for blocking legacy writes is the migration/,
  ],
  [
    /desktop_pet_migration_flags/,
    /legacy_writes_blocked/,
  ],
);

requireText(
  "backend/cmd/server/main.go",
  [/migration\.MarkDesktopPetCanonicalBaselineCutover\(db\)/],
);

requireText(
  "backend/cmd/legacy-package-migrate/main.go",
  [/migration\.MarkDesktopPetCanonicalBaselineCutover\(db\)/],
);

requireText(
  "backend/internal/migration/desktop_pet_canonical_baseline.go",
  [
    /baseline-desktop-pet-v2/,
    /desktop_pet_migration_operations/,
    /desktop_pet_write_cutovers/,
    /"installation", "editing"/,
  ],
);

requireText(
  "backend/internal/desktoppet/runtime/protocol/v2/envelope.go",
  [/CurrentSchemaVersion\s*=\s*contracts\.RuntimeContractVersion/],
);

requireText(
  "backend/internal/desktoppet/runtime/protocol/v2/handler.go",
  [
    /deviceRuntimeSessions\.Acquire\s*\(context\.Background\(\)/,
    /deviceRuntimeSessions\.UpdateCursor\s*\(context\.Background\(\)/,
  ],
  [
    /deviceRuntimeSessions\.(?:Acquire|Close|MarkReady|GetSession|UpdateCursor|Heartbeat)\s*\(\s*nil\b/,
  ],
);

requireText(
  "desktop/src/main/pet/window-adapter.ts",
  [
    /webContents\.off\("render-process-gone",\s*onRenderProcessGone\)/,
    /showWhenRuntimeReady\s*\(\)/,
    /webContents\.on\("will-navigate",\s*\(event\)\s*=>\s*event\.preventDefault\(\)\)/,
    /webContents\.setWindowOpenHandler\(\(\)\s*=>\s*\(\{ action: "deny" \}\)\)/,
  ],
  [
    /removeAllListeners\("render-process-gone"\)/,
    /removeAllListeners\("unresponsive"\)/,
  ],
);

if (errors.length > 0) {
  console.error("[verify-desktop-pet-runtime-singletrack] FAILED:");
  for (const error of errors) console.error(`  ERROR: ${error}`);
  process.exit(1);
}

console.log("[verify-desktop-pet-runtime-singletrack] PASSED: Runtime V2 is the only desktop-pet wire/runtime production path");
