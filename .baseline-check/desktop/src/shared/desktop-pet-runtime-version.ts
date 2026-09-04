import desktopPackage from "../../package.json";

const STRICT_SEMVER_PATTERN = /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$/;

function requireSemVer(field: string, value: unknown): string {
  if (typeof value !== "string" || !STRICT_SEMVER_PATTERN.test(value)) {
    throw new Error(`[desktop-pet] ${field} must be a valid SemVer string`);
  }
  return value;
}

const packageMetadata = desktopPackage as {
  desktopPetRuntimeVersion?: unknown;
  desktopPetRuntimeContractVersion?: unknown;
};

/**
 * Single desktop-side source of truth for package compatibility checks and
 * the runtime hello identity. It is validated as soon as the desktop-pet
 * runtime modules are loaded.
 */
export const DESKTOP_PET_RUNTIME_VERSION = requireSemVer(
  "desktopPetRuntimeVersion",
  packageMetadata.desktopPetRuntimeVersion,
);

/**
 * Desktop counterpart of the backend runtime-v2 schema version. Build
 * verification checks that this value stays aligned with the Go contract.
 */
export const DESKTOP_PET_RUNTIME_CONTRACT_VERSION = requireSemVer(
  "desktopPetRuntimeContractVersion",
  packageMetadata.desktopPetRuntimeContractVersion,
);
