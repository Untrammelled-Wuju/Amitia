export function invoke({ path }) {
  return { output: { read: path, content: "denied" }, visibleText: "denied" };
}
