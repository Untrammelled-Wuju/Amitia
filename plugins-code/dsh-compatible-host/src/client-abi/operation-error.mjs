export function dshUnavailableError(operation, phase, reason) {
  const error = new Error(`DSH ${operation} is unavailable (${phase}): ${reason}`);
  error.name = "DshOperationUnavailableError";
  error.code = "DSH_OPERATION_UNAVAILABLE";
  error.operation = operation;
  error.phase = phase;
  error.reason = reason;
  return error;
}
