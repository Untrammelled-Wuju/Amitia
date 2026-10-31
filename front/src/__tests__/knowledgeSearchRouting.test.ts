import { beforeEach, describe, expect, it, vi } from "vitest";
import { useKnowledge } from "../composables/useKnowledge";

const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), del: vi.fn(), deployment: vi.fn() }));
vi.mock("../composables/useApi", () => ({ useApi: () => mocks }));
vi.mock("../runtime/runtime-adapter", () => ({ getDeploymentConfig: mocks.deployment }));
vi.mock("../composables/useCoreConfigurationAccess", () => ({ coreConfigurationRequestConfig: () => ({}) }));

function response() {
  return {
    evidence: [],
    sufficient: false,
    answerState: "insufficient",
    sufficiencyReason: "no_evidence",
    query: "核对 PDF",
    originalQuery: "核对 PDF",
    effectiveQuery: "核对 PDF",
    queryComplexity: "simple",
    requestedRetrievalLevel: "auto",
    retrievalLevel: "agentic",
    routingMode: "chat",
    policyMode: "active",
    routeReason: "chat_document_intent",
    routeDegraded: false,
    degraded: false,
    temporalMode: "current",
    ambiguous: false,
  };
}

beforeEach(() => {
  Object.values(mocks).forEach((mock) => mock.mockReset());
  mocks.post.mockResolvedValue(response());
});

describe("Knowledge 聊天有效路由", () => {
  it("自动级别请求聊天路由并保留服务端有效策略", async () => {
    const knowledge = useKnowledge();
    const result = await knowledge.search("核对 PDF", { retrievalLevel: "auto" });
    expect(mocks.post).toHaveBeenCalledWith("/api/knowledge/search", expect.objectContaining({
      retrievalLevel: "auto",
      routingMode: "chat",
    }));
    expect(result.routingMode).toBe("chat");
    expect(result.policyMode).toBe("active");
    expect(result.retrievalLevel).toBe("agentic");
  });

  it("手动指定级别明确使用诊断路由", async () => {
    const knowledge = useKnowledge();
    await knowledge.search("核对 PDF", { retrievalLevel: "deep" });
    expect(mocks.post).toHaveBeenCalledWith("/api/knowledge/search", expect.objectContaining({
      retrievalLevel: "deep",
      routingMode: "diagnostic",
    }));
  });
});
