export function invoke({ text }) {
  return { output: { echoed: text }, visibleText: text };
}
