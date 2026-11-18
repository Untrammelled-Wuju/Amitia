export const DSH_ENV_PREFIX = "DSH_";
export const SENSITIVE_ENV_PATTERN = /KEY|PASSWORD|SECRET|TOKEN/i;

export function scrubbedParentEnv() {
  const environment = {};
  for (const [key, value] of Object.entries(process.env)) {
    if (value === undefined) continue;
    if (SENSITIVE_ENV_PATTERN.test(key)) continue;
    if (key.toUpperCase().startsWith(DSH_ENV_PREFIX)) continue;
    environment[key] = value;
  }
  return environment;
}
