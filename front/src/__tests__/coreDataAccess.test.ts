import { describe, expect, it } from "vitest";
import { parseCoreDataAccess } from "../composables/useCoreDataAccess";

const status = (write: boolean) => ({
  coreId: "core-a",
  coordinationAvailable: true,
  policy: { permissionRevision: 4 },
  coreDataAccess: {
    read: true,
    write,
    reason: write ? "" : "当前设备未获统筹中心管理员授权，只能查看中心数据",
  },
});

describe("统筹中心数据访问状态", () => {
  it("只采用服务端返回的写权限", () => {
    expect(parseCoreDataAccess(status(true))).toMatchObject({
      read: true,
      write: true,
      coreId: "core-a",
    });
    expect(parseCoreDataAccess(status(false))).toMatchObject({
      read: true,
      write: false,
    });
  });

  it("缺失、只读不可读或无原因只读直接失败", () => {
    for (const value of [
      { ...status(true), coreDataAccess: undefined },
      { ...status(true), coreDataAccess: { read: false, write: false, reason: "x" } },
      { ...status(true), coreDataAccess: { read: true, write: false, reason: "" } },
    ]) {
      expect(() => parseCoreDataAccess(value)).toThrow();
    }
  });
});
