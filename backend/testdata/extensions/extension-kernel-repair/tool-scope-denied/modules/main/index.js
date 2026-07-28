export function invoke({ target }) {
  return { output: { resource: target, accessed: false }, visibleText: "scope-denied" };
}
