import { describe, expect, it, vi } from "vitest";
import { requestSchemaActionConfirmation } from "../components/extension/schema-action-confirmation";

describe("Schema action confirmation", () => {
  it("keeps the dialog open and switches the confirm button to loading", async () => {
    const done = vi.fn();
    const state = {
      confirmButtonText: "确定",
      confirmButtonLoading: false,
      confirmButtonDisabled: false,
      cancelButtonText: "取消",
      showCancelButton: true,
    };
    const open = vi.fn((_message: string, _title: string, options: any) => {
      options.beforeClose("confirm", state, done);
      return new Promise(() => {});
    });

    const pending = requestSchemaActionConfirmation("确定卸载？", "卸载中", open);
    await Promise.resolve();
    const handle = await pending;

    expect(handle).not.toBeNull();
    expect(done).not.toHaveBeenCalled();
    expect(state.confirmButtonText).toBe("卸载中");
    expect(state.confirmButtonLoading).toBe(true);
    expect(state.confirmButtonDisabled).toBe(true);
    expect(state.cancelButtonText).toBe("");
    expect(state.showCancelButton).toBe(false);

    handle?.done();
    expect(done).toHaveBeenCalledOnce();
  });

  it("closes without loading when canceled", async () => {
    const done = vi.fn();
    const state = {
      confirmButtonText: "确定",
      confirmButtonLoading: false,
      confirmButtonDisabled: false,
      cancelButtonText: "取消",
      showCancelButton: true,
    };
    const open = vi.fn((_message: string, _title: string, options: any) => {
      options.beforeClose("cancel", state, done);
      return Promise.resolve("cancel");
    });

    await expect(requestSchemaActionConfirmation("确定卸载？", "卸载中", open)).resolves.toBeNull();
    expect(done).toHaveBeenCalledOnce();
    expect(state.confirmButtonText).toBe("确定");
    expect(state.confirmButtonLoading).toBe(false);
  });
});
