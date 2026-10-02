import { describe, expect, it } from "vitest";
import { operationRequestTimeout, setOperationTimeout, readOperationTimeoutHeaders, formatTimeout } from "../runtime/operation-timeout";

describe("operation timeout policy", () => {
  it("uses saved durations, unlimited mode and separate backend policies", () => {
    setOperationTimeout("local", { disabled: false, seconds: 450 });
    expect(operationRequestTimeout("local")).toBe(455000);
    setOperationTimeout("local", { disabled: true, seconds: 450 });
    expect(operationRequestTimeout("local")).toBe(0);
    expect(operationRequestTimeout("cloud")).toBe(185000);
    readOperationTimeoutHeaders("local", { "x-amitia-timeout-disabled": "false", "x-amitia-timeout-seconds": "60" });
    expect(operationRequestTimeout("local")).toBe(65000);
    readOperationTimeoutHeaders("local", { "x-amitia-timeout-disabled": "false", "x-amitia-timeout-seconds": "bad" });
    expect(operationRequestTimeout("local")).toBe(65000);
  });
  it("shows exact minute and second values", () => {
    expect(formatTimeout(30)).toBe("30 秒");
    expect(formatTimeout(180)).toBe("3 分钟");
    expect(formatTimeout(450)).toBe("7 分钟 30 秒");
  });
});
