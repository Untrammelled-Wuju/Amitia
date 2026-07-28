export function invoke({ message }) {
  throw new Error("intentional crash: " + (message || "runtime-crash"));
}
