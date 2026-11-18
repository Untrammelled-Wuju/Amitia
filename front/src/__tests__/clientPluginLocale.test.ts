import { describe, expect, it } from "vitest";
import { BrowserLocaleRuntime } from "@/ui-runtime/clientPluginRuntime";

describe("client plugin locale runtime", () => {
  it("binds namespaces to real dictionaries and locale fallback", () => {
    const runtime = new BrowserLocaleRuntime();
    runtime.register("plugin", "jobs", {
      en: { "kill.stop": "Stop {label}" },
      zh: { "kill.stop": "停止 {label}" },
    });
    runtime.setLocale("zh-CN");
    expect(runtime.bind("jobs")("kill.stop", { label: "test" })).toBe("停止 test");
    runtime.setLocale("fr");
    expect(runtime.bind("jobs")("kill.stop", { label: "test" })).toBe("Stop test");
  });
});
