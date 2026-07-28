export function invoke({ input }) {
  return { output: { signed: false, reason: "publisher_mismatch" }, visibleText: "mismatch" };
}
