package com.amitia.amitia_app.runtime.proot.internal

import com.amitia.amitia_app.runtime.connection.BackendEndpointPolicy
import com.amitia.amitia_app.runtime.install.RuntimeHostLayout
import com.amitia.amitia_app.runtime.proot.GuestLayoutContract
import com.amitia.amitia_app.runtime.proot.RuntimeEnvironment
import com.amitia.amitia_app.runtime.proot.RuntimeEnvironmentBuilder
import com.amitia.amitia_app.runtime.proot.RuntimeEnvironmentErrorCode
import com.amitia.amitia_app.runtime.proot.RuntimeEnvironmentRequest
import com.amitia.amitia_app.runtime.proot.RuntimeEnvironmentResult

internal class DefaultRuntimeEnvironmentBuilder : RuntimeEnvironmentBuilder {

    override fun build(request: RuntimeEnvironmentRequest): RuntimeEnvironmentResult {
        if (!validateHostLayout(request.hostLayout)) {
            return RuntimeEnvironmentResult.Failure(
                RuntimeEnvironmentErrorCode.HOST_LAYOUT_INVALID,
                "host layout path must be safe and absolute"
            )
        }

        val endpointError = validateEndpoint(request.endpoint)
        if (endpointError != null) {
            return RuntimeEnvironmentResult.Failure(endpointError.first, endpointError.second)
        }

        val hostProcess = buildHostProcessEnvironment(request.hostLayout)
        val guestRuntime = buildGuestRuntimeEnvironment(request.endpoint)

        return try {
            RuntimeEnvironmentResult.Success(RuntimeEnvironment(hostProcess, guestRuntime))
        } catch (e: IllegalArgumentException) {
            RuntimeEnvironmentResult.Failure(
                RuntimeEnvironmentErrorCode.VALIDATION_FAILED,
                e.message ?: "environment validation failed"
            )
        }
    }

    private fun validateHostLayout(layout: RuntimeHostLayout): Boolean {
        val paths = listOf(
            layout.runRoot, layout.cacheRoot, layout.dataRoot,
            layout.configRoot, layout.logRoot,
        )
        for (path in paths) {
            val ap = path.absolutePath
            if (ap.isBlank()) return false
            if (!path.isAbsolute && !ap.startsWith("/")) return false
            if (ap.contains("..")) return false
            if (ap.contains("\u0000")) return false
        }
        return true
    }

    private fun validateEndpoint(policy: BackendEndpointPolicy): Pair<RuntimeEnvironmentErrorCode, String>? {
        if (policy.host != "127.0.0.1") {
            return RuntimeEnvironmentErrorCode.ENDPOINT_INVALID to "backend host must be 127.0.0.1"
        }
        if (policy.port != 18899) {
            return RuntimeEnvironmentErrorCode.ENDPOINT_INVALID to "backend port must be 18899"
        }
        return null
    }

    private fun buildHostProcessEnvironment(layout: RuntimeHostLayout): Map<String, String> {
        val env = LinkedHashMap<String, String>()
        env["TMPDIR"] = layout.runRoot.absolutePath + "/tmp"
        env["HOME"] = "/"
        env["LANG"] = GuestLayoutContract.LANG
        env["LC_ALL"] = GuestLayoutContract.LC_ALL
        env["TZ"] = GuestLayoutContract.TZ
        return env
    }

    private fun buildGuestRuntimeEnvironment(policy: BackendEndpointPolicy): Map<String, String> {
        val env = LinkedHashMap<String, String>()

        env["AMITIA_RUNTIME_ROOT"] = GuestLayoutContract.RUNTIME_ROOT
        env["AMITIA_CONFIG_ROOT"] = GuestLayoutContract.CONFIG_ROOT
        env["AMITIA_DATA_ROOT"] = GuestLayoutContract.DATA_ROOT
        env["AMITIA_CACHE_ROOT"] = GuestLayoutContract.CACHE_ROOT
        env["AMITIA_LOG_ROOT"] = GuestLayoutContract.LOG_ROOT
        env["AMITIA_RUN_ROOT"] = GuestLayoutContract.RUN_ROOT
        env["AMITIA_TEMP_ROOT"] = GuestLayoutContract.TEMP_ROOT
        env["AMITIA_WORKSPACE_ROOT"] = GuestLayoutContract.WORKSPACE_ROOT
        env["AMITIA_HOME"] = GuestLayoutContract.HOME

        env["HOME"] = GuestLayoutContract.HOME
        env["LANG"] = GuestLayoutContract.LANG
        env["LC_ALL"] = GuestLayoutContract.LC_ALL
        env["TZ"] = GuestLayoutContract.TZ
        env["PATH"] = GuestLayoutContract.PATH

        env["AMITIA_SERVER_HOST"] = policy.host
        env["AMITIA_SERVER_PORT"] = policy.port.toString()

        env["NO_PROXY"] = "127.0.0.1,localhost"
        env["no_proxy"] = "127.0.0.1,localhost"

        return env
    }
}
