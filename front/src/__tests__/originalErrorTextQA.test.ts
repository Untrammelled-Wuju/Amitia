import { expect, it } from "vitest";
import { originalErrorText } from "@/runtime/original-error";

it("preserves nested error fields, non-enumerable stack and long raw messages", () => {
  const cause = new Error("inner-QA");
  const error = Object.assign(new Error("outer-QA", { cause }), {
    msg: "msg-QA", detail: "detail-QA", internalMessage: "internal-QA",
    reason: "reason-QA", error: { message: "nested-QA" }, raw: "x".repeat(20000),
  });
  const result = JSON.parse(originalErrorText(error));
  expect(result).toMatchObject({
    name: "Error", message: "outer-QA", stack: error.stack,
    cause: { message: "inner-QA", stack: cause.stack },
    msg: error.msg, detail: error.detail, internalMessage: error.internalMessage,
    reason: error.reason, error: error.error, raw: error.raw,
  });
});

it("handles circular error metadata without losing the original error", () => {
  const error = Object.assign(new Error("circular-QA"), { self: null as unknown, count: 12n });
  error.self = error;
  const result = JSON.parse(originalErrorText(error));
  expect(result.message).toBe(error.message);
  expect(result.stack).toBe(error.stack);
  expect(result.self).toBe("[Circular reference]");
  expect(result.count).toBe("12");
});
