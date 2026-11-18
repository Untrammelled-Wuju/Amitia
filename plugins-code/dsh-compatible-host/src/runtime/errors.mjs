export const DSH_HOST_ERROR_CODES = {
  SERVICE_MISSING: "DSH_HOST_SERVICE_MISSING",
  SERVICE_CONFLICT: "DSH_HOST_SERVICE_CONFLICT",
  SERVICE_UNINJECTED: "DSH_HOST_SERVICE_UNINJECTED",
};

const conflictPattern = /^service "[^"]+" has been registered at/;
const uninjectedPattern = /^cannot get (?:property "[^"]+" without inject|required service "[^"]+" in inactive context)$/;

export function tagDshHostError(error, code) {
  if (!error || typeof error !== "object") return error;
  if (Object.prototype.hasOwnProperty.call(error, "code") && error.code !== undefined) return error;
  Object.defineProperty(error, "code", {
    value: code,
    enumerable: true,
    configurable: true,
    writable: true,
  });
  return error;
}

export function classifyDshPluginError(error) {
  const message = typeof error?.message === "string" ? error.message : "";
  if (uninjectedPattern.test(message)) return tagDshHostError(error, DSH_HOST_ERROR_CODES.SERVICE_UNINJECTED);
  if (conflictPattern.test(message)) return tagDshHostError(error, DSH_HOST_ERROR_CODES.SERVICE_CONFLICT);
  return error;
}

export function createDshHostError(code, message, options) {
  const error = new Error(message, options);
  return tagDshHostError(error, code);
}
