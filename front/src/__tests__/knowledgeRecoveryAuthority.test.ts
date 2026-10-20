import { beforeEach, describe, expect, it, vi } from "vitest";
import { useKnowledge } from "../composables/useKnowledge";

const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), del: vi.fn(), deployment: vi.fn() }));
vi.mock("../composables/useApi", () => ({ useApi: () => mocks }));
vi.mock("../runtime/runtime-adapter", () => ({ getDeploymentConfig: mocks.deployment }));
vi.mock("../composables/useCoreConfigurationAccess", () => ({
  coreConfigurationRequestConfig: (context: string) => {
    const fields = JSON.parse(context);
    return { headers: { "X-Amitia-Expected-Core-ID": fields[1], "X-Amitia-Expected-Configuration-Policy": fields.slice(2).join(":") } };
  },
}));

const auditID = "11111111-1111-1111-1111-111111111111";
const reviewID = "22222222-2222-2222-2222-222222222222";
const recheckID = "33333333-3333-3333-3333-333333333333";

function fixture() {
  const status = { coreId: "core-A", canAdminister: true, policy: { coordinated: true, administrator: true, providerEpoch: 1, modeRevision: 1, permissionRevision: 2 } };
  mocks.deployment.mockResolvedValue({ mode: "cloud", serverURL: "https://core.example" });
  mocks.get.mockImplementation(async (url: string) => {
    if (url.endsWith("/coordination/me")) return structuredClone(status);
    if (url.endsWith("/recovery-reviews")) return { items: [{ id: reviewID, auditId: auditID, status: "pending" }], executable: false };
    if (url.endsWith("/plan")) return { reviewId: reviewID, auditId: auditID, action: "manual_investigation", executable: false, status: "pending", historyStatus: "verified", evidenceStatus: "valid", manualReviewReady: true, canApprove: true };
    if (url.endsWith("/events")) return { reviewId: reviewID, historyStatus: "verified", items: [], truncated: false, executable: false };
    throw new Error(`unexpected request ${url}`);
  });
  mocks.post.mockResolvedValue({ request: { id: reviewID, auditId: auditID }, executable: false });
  return status;
}

beforeEach(() => { Object.values(mocks).forEach((mock) => mock.mockReset()); });

describe("Knowledge 恢复审查 Core 授权", () => {
  it("列表与审批使用相同权限版本，撤销后不暴露缓存", async () => {
    const status = fixture();
    const knowledge = useKnowledge();
    await knowledge.fetchRecoveryReviews(auditID);
    expect(mocks.get).toHaveBeenCalledWith(expect.stringContaining("/recovery-reviews"), { limit: 10 }, { headers: { "X-Amitia-Expected-Core-ID": "core-A", "X-Amitia-Expected-Configuration-Policy": "1:1:2" } });
    await knowledge.transitionRecoveryReview(reviewID, auditID, "approve");
    expect(mocks.post).toHaveBeenCalledWith(expect.stringContaining("/approve"), undefined, { headers: { "X-Amitia-Expected-Core-ID": "core-A", "X-Amitia-Expected-Configuration-Policy": "1:1:2" } });
    status.policy.permissionRevision = 3;
    status.canAdminister = false;
    await expect(knowledge.fetchRecoveryReviews(auditID)).rejects.toThrow();
    expect(knowledge.recoveryReviews.value).toEqual([]);
    expect(knowledge.recoveryReviewPlan.value).toBeNull();
  });

  it("延迟的旧列表不得覆盖新版本列表", async () => {
    const status = fixture();
    const knowledge = useKnowledge();
    let complete!: (data: any) => void;
    mocks.get.mockImplementationOnce(async () => structuredClone(status));
    mocks.get.mockImplementationOnce(() => new Promise((resolve) => { complete = resolve; }));
    const pending = knowledge.fetchRecoveryReviews(auditID);
    await new Promise((resolve) => setTimeout(resolve, 0));
    status.policy.permissionRevision = 4;
    await knowledge.fetchRecoveryReviews(auditID);
    complete({ executable: false, items: [{ id: "old", auditId: auditID }] });
    await expect(pending).rejects.toThrow("状态已变化");
    expect(knowledge.recoveryReviews.value.map((item) => item.id)).toEqual([reviewID]);
  });

  it("网络失败后保持人工核查申请的幂等键以支持安全重试", async () => {
    fixture();
    const knowledge = useKnowledge();
    mocks.post.mockRejectedValueOnce(new Error("network interrupted"));
    await expect(knowledge.requestRecoveryReview(auditID, recheckID)).rejects.toThrow("network interrupted");
    await knowledge.requestRecoveryReview(auditID, recheckID);
    expect(mocks.post).toHaveBeenCalledTimes(2);
    expect(mocks.post.mock.calls[0][1].requestKey).toBe(mocks.post.mock.calls[1][1].requestKey);
  });
});
