function envStr(key: string, fallback: string): string {
    return process.env[key] || fallback;
}
export const sidecarConfig = {
    mergeWindowMs: parseInt(process.env.MERGE_WINDOW_MS || "6000", 10),
    host: envStr("SIDECAR_HOST", "127.0.0.1"),
    port: parseInt(envStr("SIDECAR_PORT", "19876"), 10),
    coreUrl: envStr("AMITIA_CORE_URL", envStr("CORE_URL", "http://127.0.0.1:18899")),
    extensionId: envStr("AMITIA_EXTENSION_ID", ""),
    moduleId: envStr("AMITIA_MODULE_ID", ""),
    serviceAuthToken: envStr("AMITIA_SERVICE_AUTH_TOKEN", ""),
};
