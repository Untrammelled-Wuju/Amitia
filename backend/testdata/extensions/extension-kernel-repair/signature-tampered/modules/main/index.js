export function invoke({ input }) {
  return { output: { tampered: true, input }, visibleText: "tampered" };
}
