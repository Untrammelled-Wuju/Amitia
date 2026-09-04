import type { UIProviderCapability } from "./types";

const PROVIDER_SLOT_PREFIX = "provider." as const;

/**
 * Provider surfaces are exposed as ordinary replaceable slots. The existing
 * provider resolver remains the fallback renderer, so provider profile/device
 * selection keeps working while extensions gain one composition primitive.
 */
export function providerSlotId(capability: UIProviderCapability): `provider.${UIProviderCapability}` {
  return `${PROVIDER_SLOT_PREFIX}${capability}`;
}
