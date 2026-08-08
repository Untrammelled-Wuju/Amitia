package com.amitia.amitia_app.runtime.proot

import com.amitia.amitia_app.runtime.connection.BackendEndpointPolicy
import com.amitia.amitia_app.runtime.install.RuntimeHostLayout

internal data class RuntimeEnvironmentRequest(
    val hostLayout: RuntimeHostLayout,
    val endpoint: BackendEndpointPolicy,
)

internal sealed interface RuntimeEnvironmentResult {
    data class Success(val environment: RuntimeEnvironment) : RuntimeEnvironmentResult
    data class Failure(val code: RuntimeEnvironmentErrorCode, val message: String) : RuntimeEnvironmentResult
}

internal enum class RuntimeEnvironmentErrorCode {
    HOST_LAYOUT_INVALID,
    GUEST_LAYOUT_INVALID,
    ENDPOINT_INVALID,
    KEY_INVALID,
    VALUE_INVALID,
    REQUIRED_VALUE_MISSING,
    BUILD_FAILED,
    VALIDATION_FAILED,
}

internal fun interface RuntimeEnvironmentBuilder {
    fun build(request: RuntimeEnvironmentRequest): RuntimeEnvironmentResult
}

internal object GuestLayoutContract {
    const val RUNTIME_ROOT = "/opt/amitia"
    const val CONFIG_ROOT = "/etc/amitia"
    const val DATA_ROOT = "/var/lib/amitia"
    const val CACHE_ROOT = "/var/cache/amitia"
    const val LOG_ROOT = "/var/log/amitia"
    const val RUN_ROOT = "/run/amitia"
    const val TEMP_ROOT = "/run/amitia/tmp"
    const val WORKSPACE_ROOT = "/var/lib/amitia/workspaces"
    const val HOME = "/home/amitia"

    const val NODE_BIN = "/opt/amitia/node/bin"
    val PATH = "$NODE_BIN:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

    const val LANG = "C.UTF-8"
    const val LC_ALL = "C.UTF-8"
    const val TZ = "Etc/UTC"
}

internal object ForbiddenHostVars {
    val PREFIXES = setOf(
        "QDRANT__", "NODE_OPTIONS", "NPM_CONFIG_", "npm_config_",
        "http_proxy", "https_proxy", "all_proxy", "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY",
        "JAVA_HOME", "ANDROID_HOME", "ANDROID_SDK_ROOT",
        "GOROOT", "GOPATH", "GOMODCACHE", "GOTOOLCHAIN",
        "LD_PRELOAD", "LD_LIBRARY_PATH",
        "OPENAI_API_KEY", "ANTHROPIC_API_KEY", "DEEPSEEK_API_KEY", "OPENROUTER_API_KEY",
        "GITHUB_TOKEN", "NPM_TOKEN",
        "NODE_PATH",
    )

    val EXACT = setOf(
        "AMITIA_LOCAL_TOKEN",
        "AMITIA_RUNTIME_VERSION",
        "AMITIA_ACTIVE_VERSION",
        "PACKAGE_SHA", "RUNTIME_PACKAGE_SHA", "MANIFEST_SHA",
    )

    fun isForbidden(key: String): Boolean {
        if (EXACT.contains(key)) return true
        for (prefix in PREFIXES) {
            if (key.startsWith(prefix)) return true
        }
        return false
    }
}
