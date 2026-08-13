package com.amitia.amitia_app.runtime.proot.internal

import com.amitia.amitia_app.runtime.proot.GuestLayoutContract
import com.amitia.amitia_app.runtime.proot.RuntimeEnvironment

internal object RuntimeEnvironmentValidator {

    fun validate(env: RuntimeEnvironment): RuntimeEnvironmentValidationResult {
        val guest = env.guestRuntime

        if (guest["AMITIA_RUNTIME_ROOT"] != GuestLayoutContract.RUNTIME_ROOT) {
            return RuntimeEnvironmentValidationResult.Invalid("AMITIA_RUNTIME_ROOT must be ${GuestLayoutContract.RUNTIME_ROOT}")
        }
        if (guest["AMITIA_CONFIG_ROOT"] != GuestLayoutContract.CONFIG_ROOT) {
            return RuntimeEnvironmentValidationResult.Invalid("AMITIA_CONFIG_ROOT must be ${GuestLayoutContract.CONFIG_ROOT}")
        }
        if (guest["AMITIA_DATA_ROOT"] != GuestLayoutContract.DATA_ROOT) {
            return RuntimeEnvironmentValidationResult.Invalid("AMITIA_DATA_ROOT must be ${GuestLayoutContract.DATA_ROOT}")
        }
        if (guest["AMITIA_CACHE_ROOT"] != GuestLayoutContract.CACHE_ROOT) {
            return RuntimeEnvironmentValidationResult.Invalid("AMITIA_CACHE_ROOT must be ${GuestLayoutContract.CACHE_ROOT}")
        }
        if (guest["AMITIA_LOG_ROOT"] != GuestLayoutContract.LOG_ROOT) {
            return RuntimeEnvironmentValidationResult.Invalid("AMITIA_LOG_ROOT must be ${GuestLayoutContract.LOG_ROOT}")
        }
        if (guest["AMITIA_RUN_ROOT"] != GuestLayoutContract.RUN_ROOT) {
            return RuntimeEnvironmentValidationResult.Invalid("AMITIA_RUN_ROOT must be ${GuestLayoutContract.RUN_ROOT}")
        }

        val tempRoot = guest["AMITIA_TEMP_ROOT"]
        if (tempRoot == null || tempRoot != GuestLayoutContract.TEMP_ROOT) {
            return RuntimeEnvironmentValidationResult.Invalid("AMITIA_TEMP_ROOT must be ${GuestLayoutContract.TEMP_ROOT}")
        }

        val workspaceRoot = guest["AMITIA_WORKSPACE_ROOT"]
        if (workspaceRoot == null || !workspaceRoot.startsWith(GuestLayoutContract.DATA_ROOT)) {
            return RuntimeEnvironmentValidationResult.Invalid("AMITIA_WORKSPACE_ROOT must be within AMITIA_DATA_ROOT")
        }

        if (guest["HOME"] != GuestLayoutContract.HOME) {
            return RuntimeEnvironmentValidationResult.Invalid("HOME must be ${GuestLayoutContract.HOME}")
        }

        if (guest["LANG"] != GuestLayoutContract.LANG) {
            return RuntimeEnvironmentValidationResult.Invalid("LANG must be ${GuestLayoutContract.LANG}")
        }
        if (guest["LC_ALL"] != GuestLayoutContract.LC_ALL) {
            return RuntimeEnvironmentValidationResult.Invalid("LC_ALL must be ${GuestLayoutContract.LC_ALL}")
        }
        if (guest["TZ"] != GuestLayoutContract.TZ) {
            return RuntimeEnvironmentValidationResult.Invalid("TZ must be ${GuestLayoutContract.TZ}")
        }

        val path = guest["PATH"]
        if (path == null || !path.startsWith(GuestLayoutContract.NODE_BIN)) {
            return RuntimeEnvironmentValidationResult.Invalid("PATH must start with ${GuestLayoutContract.NODE_BIN}")
        }

        for ((key, value) in guest) {
            if (value.contains("/data/user/") ||
                value.contains("/data/data/") ||
                value.contains("/sdcard/") ||
                value.contains("/storage/emulated/")
            ) {
                return RuntimeEnvironmentValidationResult.Invalid("Guest env value for '$key' must not contain host paths")
            }
        }

        if (guest.containsKey("NODE_OPTIONS")) {
            return RuntimeEnvironmentValidationResult.Invalid("NODE_OPTIONS must not be set in guest runtime")
        }
        if (guest.containsKey("NODE_PATH")) {
            return RuntimeEnvironmentValidationResult.Invalid("NODE_PATH must not be set in guest runtime")
        }
        if (guest.containsKey("LD_PRELOAD")) {
            return RuntimeEnvironmentValidationResult.Invalid("LD_PRELOAD must not be set in guest runtime")
        }

        if (guest["AMITIA_SERVER_HOST"] != "127.0.0.1") {
            return RuntimeEnvironmentValidationResult.Invalid("AMITIA_SERVER_HOST must be 127.0.0.1")
        }

        return RuntimeEnvironmentValidationResult.Valid
    }
}

internal sealed interface RuntimeEnvironmentValidationResult {
    object Valid : RuntimeEnvironmentValidationResult
    data class Invalid(val reason: String) : RuntimeEnvironmentValidationResult
}
