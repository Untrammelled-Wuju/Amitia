export class HarnessError extends Error {
  constructor(message, code, options) {
    super(message, options);
    this.name = "HarnessError";
    this.code = code;
  }
}

export function ReasoningEffortId(value) {
  return value;
}
