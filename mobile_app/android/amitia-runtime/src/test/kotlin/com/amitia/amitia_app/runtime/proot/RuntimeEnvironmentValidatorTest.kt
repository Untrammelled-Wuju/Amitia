package com.amitia.amitia_app.runtime.proot

import com.amitia.amitia_app.runtime.proot.internal.RuntimeEnvironmentValidationResult
import com.amitia.amitia_app.runtime.proot.internal.RuntimeEnvironmentValidator
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class RuntimeEnvironmentValidatorTest {

    private fun validEnvironment(): RuntimeEnvironment {
        val guest = mapOf(
            "AMITIA_RUNTIME_ROOT" to "/opt/amitia",
            "AMITIA_CONFIG_ROOT" to "/etc/amitia",
            "AMITIA_DATA_ROOT" to "/var/lib/amitia",
            "AMITIA_CACHE_ROOT" to "/var/cache/amitia",
            "AMITIA_LOG_ROOT" to "/var/log/amitia",
            "AMITIA_RUN_ROOT" to "/run/amitia",
            "AMITIA_TEMP_ROOT" to "/tmp",
            "AMITIA_WORKSPACE_ROOT" to "/var/lib/amitia/workspaces",
            "AMITIA_HOME" to "/home/amitia",
            "HOME" to "/home/amitia",
            "LANG" to "C.UTF-8",
            "LC_ALL" to "C.UTF-8",
            "TZ" to "Etc/UTC",
            "PATH" to "/opt/amitia/node/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
            "AMITIA_SERVER_HOST" to "127.0.0.1",
            "AMITIA_SERVER_PORT" to "18899",
        )
        val host = mapOf(
            "TMPDIR" to "/data/local/tmp",
            "LANG" to "C.UTF-8",
            "LC_ALL" to "C.UTF-8",
            "TZ" to "Etc/UTC",
        )
        return RuntimeEnvironment(hostProcess = host, guestRuntime = guest)
    }

    @Test
    fun validEnvironmentReturnsValid() {
        val env = validEnvironment()
        val result = RuntimeEnvironmentValidator.validate(env)
        assertTrue(result is RuntimeEnvironmentValidationResult.Valid)
    }

    @Test
    fun invalidRuntimeRootReturnsInvalid() {
        val env = validEnvironment().copy(
            guestRuntime = validEnvironment().guestRuntime.toMutableMap().apply {
                this["AMITIA_RUNTIME_ROOT"] = "/wrong/path"
            }
        )
        val result = RuntimeEnvironmentValidator.validate(env)
        assertTrue(result is RuntimeEnvironmentValidationResult.Invalid)
    }

    @Test
    fun invalidTempRootReturnsInvalid() {
        val env = validEnvironment().copy(
            guestRuntime = validEnvironment().guestRuntime.toMutableMap().apply {
                this["AMITIA_TEMP_ROOT"] = "/run/amitia/tmp"
            }
        )
        val result = RuntimeEnvironmentValidator.validate(env)
        assertTrue(result is RuntimeEnvironmentValidationResult.Invalid)
    }

    @Test
    fun invalidWorkspaceRootReturnsInvalid() {
        val env = validEnvironment().copy(
            guestRuntime = validEnvironment().guestRuntime.toMutableMap().apply {
                this["AMITIA_WORKSPACE_ROOT"] = "/tmp/workspaces"
            }
        )
        val result = RuntimeEnvironmentValidator.validate(env)
        assertTrue(result is RuntimeEnvironmentValidationResult.Invalid)
    }

    @Test
    fun invalidHomeReturnsInvalid() {
        val env = validEnvironment().copy(
            guestRuntime = validEnvironment().guestRuntime.toMutableMap().apply {
                this["HOME"] = "/root"
            }
        )
        val result = RuntimeEnvironmentValidator.validate(env)
        assertTrue(result is RuntimeEnvironmentValidationResult.Invalid)
    }

    @Test
    fun invalidLangReturnsInvalid() {
        val env = validEnvironment().copy(
            guestRuntime = validEnvironment().guestRuntime.toMutableMap().apply {
                this["LANG"] = "en_US.UTF-8"
            }
        )
        val result = RuntimeEnvironmentValidator.validate(env)
        assertTrue(result is RuntimeEnvironmentValidationResult.Invalid)
    }

    @Test
    fun invalidTzReturnsInvalid() {
        val env = validEnvironment().copy(
            guestRuntime = validEnvironment().guestRuntime.toMutableMap().apply {
                this["TZ"] = "Asia/Shanghai"
            }
        )
        val result = RuntimeEnvironmentValidator.validate(env)
        assertTrue(result is RuntimeEnvironmentValidationResult.Invalid)
    }

    @Test
    fun invalidPathReturnsInvalid() {
        val env = validEnvironment().copy(
            guestRuntime = validEnvironment().guestRuntime.toMutableMap().apply {
                this["PATH"] = "/usr/bin:/bin"
            }
        )
        val result = RuntimeEnvironmentValidator.validate(env)
        assertTrue(result is RuntimeEnvironmentValidationResult.Invalid)
    }

    @Test
    fun invalidServerHostReturnsInvalid() {
        val env = validEnvironment().copy(
            guestRuntime = validEnvironment().guestRuntime.toMutableMap().apply {
                this["AMITIA_SERVER_HOST"] = "0.0.0.0"
            }
        )
        val result = RuntimeEnvironmentValidator.validate(env)
        assertTrue(result is RuntimeEnvironmentValidationResult.Invalid)
    }

    @Test
    fun nodeOptionsInGuestReturnsInvalid() {
        val env = validEnvironment().copy(
            guestRuntime = validEnvironment().guestRuntime.toMutableMap().apply {
                this["NODE_OPTIONS"] = "--inspect"
            }
        )
        val result = RuntimeEnvironmentValidator.validate(env)
        assertTrue(result is RuntimeEnvironmentValidationResult.Invalid)
    }

    @Test
    fun nodePathInGuestReturnsInvalid() {
        val env = validEnvironment().copy(
            guestRuntime = validEnvironment().guestRuntime.toMutableMap().apply {
                this["NODE_PATH"] = "/evil"
            }
        )
        val result = RuntimeEnvironmentValidator.validate(env)
        assertTrue(result is RuntimeEnvironmentValidationResult.Invalid)
    }

    @Test
    fun ldPreloadInGuestReturnsInvalid() {
        val env = validEnvironment().copy(
            guestRuntime = validEnvironment().guestRuntime.toMutableMap().apply {
                this["LD_PRELOAD"] = "/evil.so"
            }
        )
        val result = RuntimeEnvironmentValidator.validate(env)
        assertTrue(result is RuntimeEnvironmentValidationResult.Invalid)
    }

    @Test
    fun hostPathInGuestValueReturnsInvalid() {
        val env = validEnvironment().copy(
            guestRuntime = validEnvironment().guestRuntime.toMutableMap().apply {
                this["SOME_VAR"] = "/data/user/0/app"
            }
        )
        val result = RuntimeEnvironmentValidator.validate(env)
        assertTrue(result is RuntimeEnvironmentValidationResult.Invalid)
    }
}
