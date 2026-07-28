export function invoke({ input }) {
  return { output: { generation: 2, input, reloaded: true }, visibleText: "v2" };
}
