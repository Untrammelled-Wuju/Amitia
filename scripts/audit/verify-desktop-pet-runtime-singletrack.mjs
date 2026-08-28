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
  ],
  [
    /runtimev2\.NewRuntimeFacade\s*\(/,
    /runtime\.NewService\s*\(/,
    /wiring\.NewRuntimeActionAdapter\s*\(/,
    /wiring\.NewActivePetAdapter\s*\(/,
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
