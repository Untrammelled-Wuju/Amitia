import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { shadowsProtectedRoute } from "@/ui-runtime/providerRoutes";

interface ProviderRouteCase {
  path: string;
  extensionId: string;
  manifest: string;
  registryProviderId: string;
  routeId: string;
  pageProviderId: string;
  entries: string[];
}

interface RouteConsistencyContract {
  schemaVersion: number;
  protectedRoutes: string[];
  providerRoutes: ProviderRouteCase[];
}

interface ManifestContribution {
  id?: string;
  spec?: {
    providerId?: string;
    capability?: string;
    entries?: Record<string, unknown>;
    metadata?: {
      routes?: Array<{
        id?: string;
        path?: string;
        providerId?: string;
        capability?: string;
      }>;
    };
  };
}

interface Manifest {
  plugin?: { id?: string };
  modules?: Array<{ contributions?: ManifestContribution[] }>;
}

const root = resolve(__dirname, "../../..");
const contract = JSON.parse(
  readFileSync(resolve(root, "contracts/ui-runtime/v1/provider-route-consistency.json"), "utf8"),
) as RouteConsistencyContract;

function contributionFor(item: ProviderRouteCase): ManifestContribution {
  const manifest = JSON.parse(readFileSync(resolve(root, item.manifest), "utf8")) as Manifest;
  expect(manifest.plugin?.id).toBe(item.extensionId);
  const contribution = (manifest.modules ?? [])
    .flatMap((module) => module.contributions ?? [])
    .find((candidate) =>
      candidate.spec?.capability === "route.registry" &&
      candidate.spec.providerId === item.registryProviderId
    );
  expect(contribution).toBeDefined();
  return contribution!;
}

describe("provider route cross-platform contract", () => {
  it("protects the same core recovery routes", () => {
    expect(contract.schemaVersion).toBe(1);
    for (const path of contract.protectedRoutes) {
      expect(shadowsProtectedRoute(path), path).toBe(true);
    }
  });

  it("keeps the same plugin route batch provider-owned", () => {
    for (const item of contract.providerRoutes) {
      expect(shadowsProtectedRoute(item.path), item.path).toBe(false);
    }
  });

  it("publishes the same manifest route and platform entries", () => {
    for (const item of contract.providerRoutes) {
      const contribution = contributionFor(item);
      const entries = contribution.spec?.entries ?? {};
      expect(Object.keys(entries).sort(), item.path).toEqual([...item.entries].sort());
      const route = (contribution.spec?.metadata?.routes ?? []).find(
        (candidate) => candidate.id === item.routeId && candidate.path === item.path,
      );
      expect(route, item.path).toMatchObject({
        path: item.path,
        providerId: item.pageProviderId,
        capability: "page.provider",
      });
    }
  });
});
