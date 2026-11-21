import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { CORE_OWNER_PREFIX, DEVICE_OWNER_PREFIX } from "@/runtime/ownership-label";

const categories = ["角色", "对话", "群聊", "团队", "skill", "mcp", "任务"];

describe("归属命名在桌面与移动保持一致", () => {
  const mobile = readFileSync(resolve(process.cwd(), "../mobile_app/lib/core/device_mesh/ownership_label.dart"), "utf8");

  it("两端使用同一组前缀", () => {
    expect(CORE_OWNER_PREFIX).toBe("统筹中心");
    expect(DEVICE_OWNER_PREFIX).toBe("设备");
    expect(mobile).toContain(`const String coreOwnerPrefix = '${CORE_OWNER_PREFIX}'`);
    expect(mobile).toContain(`const String deviceOwnerPrefix = '${DEVICE_OWNER_PREFIX}'`);
  });

  it("两端覆盖同一组七类资源", () => {
    for (const category of categories) {
      expect(mobile).toContain(`'${category}'`);
    }
    const listed = [...mobile.matchAll(/^ {2}'([^']+)',$/gm)].map((match) => match[1]);
    expect(new Set(listed)).toEqual(new Set(categories));
  });
});
