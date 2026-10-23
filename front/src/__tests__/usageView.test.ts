import { expect, it } from "vitest";
import { usageMetricSummary, usageNumber, usageScopeLabel } from "../runtime/usage-view";

it("保留真实零值并拒绝未知值和错误值", () => {
  expect(usageNumber(0)).toBe(0);
  for (const value of [null, undefined, NaN, "0", {}]) expect(usageNumber(value)).toBeNull();
});

it("Owner 不可用和部分缺失的业务汇总不显示为零", () => {
  expect(usageMetricSummary([], "messages", false)).toEqual({ total: null, average: null });
  expect(usageMetricSummary([{ messages: 2 }, {}], "messages", true).total).toBeNull();
  expect(usageMetricSummary([{ messages: 2 }, { messages: 4 }], "messages", true)).toEqual({ total: 6, average: 3 });
  expect(usageMetricSummary([], "messages", true)).toEqual({ total: 0, average: 0 });
});

it("区分云端设备用量和授权的 Core 汇总", () => {
  expect(usageScopeLabel({ scope: { deviceId: "phone" } })).toBe("本设备在当前 Core");
  expect(usageScopeLabel({ scope: { coreId: "core" } })).toBe("当前 Core");
});
