package com.amitia.amitia_app.runtime.proot.internal

import com.amitia.amitia_app.runtime.connection.BackendEndpointPolicy
import com.amitia.amitia_app.runtime.install.RuntimeHostLayout
import com.amitia.amitia_app.runtime.proot.GuestLayout
import com.amitia.amitia_app.runtime.proot.GuestLayoutContract
import com.amitia.amitia_app.runtime.proot.RuntimeEnvironment
import com.amitia.amitia_app.runtime.proot.RuntimeEnvironmentBuilder
import com.amitia.amitia_app.runtime.proot.RuntimeEnvironmentErrorCode
import com.amitia.amitia_app.runtime.proot.RuntimeEnvironmentRequest
import com.amitia.amitia_app.runtime.proot.RuntimeEnvironmentResult
import java.io.File
import java.security.SecureRandom

internal class DefaultRuntimeEnvironmentBuilder(
    private val prootLoaderPath: String = "",
) : RuntimeEnvironmentBuilder {

    override fun build(request: RuntimeEnvironmentRequest): RuntimeEnvironmentResult {
        if (!validateHostLayout(request.hostLayout)) {
            return RuntimeEnvironmentResult.Failure(
                RuntimeEnvironmentErrorCode.HOST_LAYOUT_INVALID,
                "host layout path must be safe and absolute",
            )
        }

        val endpointError = validateEndpoint(request.endpoint)
        if (endpointError != null) {
            return RuntimeEnvironmentResult.Failure(endpointError.first, endpointError.second)
        }

        val securityMaterial = try {
            val material = ensureLocalSecurityMaterial(request.hostLayout)
            ensureRuntimeConfig(request.hostLayout, material)
            material
        } catch (e: Exception) {
            return RuntimeEnvironmentResult.Failure(
                RuntimeEnvironmentErrorCode.BUILD_FAILED,
                "failed to prepare local runtime security material: ${e.message ?: e.javaClass.simpleName}",
            )
        }

        val hostProcess = buildHostProcessEnvironment(request.hostLayout)
        val guestRuntime = buildGuestRuntimeEnvironment(request.endpoint, securityMaterial)

        return try {
            RuntimeEnvironmentResult.Success(RuntimeEnvironment(hostProcess, guestRuntime))
        } catch (e: IllegalArgumentException) {
            RuntimeEnvironmentResult.Failure(
                RuntimeEnvironmentErrorCode.VALIDATION_FAILED,
                e.message ?: "environment validation failed",
            )
        }
    }

    private fun validateHostLayout(layout: RuntimeHostLayout): Boolean {
        val paths = listOf(
            layout.runRoot,
            layout.cacheRoot,
            layout.dataRoot,
            layout.configRoot,
            layout.logRoot,
            layout.homeRoot,
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
        if (prootLoaderPath.isNotBlank()) env["PROOT_LOADER"] = prootLoaderPath
        env["PROOT_TMP_DIR"] = File(layout.runRoot, "proot-tmp").absolutePath

        env["PROOT_NO_SECCOMP"] = "1"
        env["ANDROID_ROOT"] = "/system"
        env["ANDROID_DATA"] = "/data"

        env["TMPDIR"] = File(layout.runRoot, "tmp").absolutePath
        env["HOME"] = "/"
        env["LANG"] = GuestLayoutContract.LANG
        env["LC_ALL"] = GuestLayoutContract.LC_ALL
        env["TZ"] = GuestLayoutContract.TZ
        return env
    }

    private fun buildGuestRuntimeEnvironment(
        policy: BackendEndpointPolicy,
        securityMaterial: LocalSecurityMaterial,
    ): Map<String, String> {
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

        env["AMITIA_CONFIG_DIR"] = GuestLayoutContract.CONFIG_ROOT
        env["AMITIA_DATA_DIR"] = GuestLayoutContract.DATA_ROOT
        env["AMITIA_CACHE_DIR"] = GuestLayoutContract.CACHE_ROOT
        env["AMITIA_LOG_DIR"] = GuestLayoutContract.LOG_ROOT
        env["AMITIA_TEMP_DIR"] = GuestLayoutContract.TEMP_ROOT
        env["AMITIA_WORKSPACE_DIR"] = GuestLayoutContract.WORKSPACE_ROOT

        env["AMITIA_RUNTIME_MODE"] = "android-native-disabled"
        env["AMITIA_SECURITY_MODE"] = "local_single_user"
        env["AMITIA_ALLOW_REMOTE_ACCESS"] = "false"
        env["AMITIA_LOCAL_TOKEN_FILE"] = GuestLayout.LOCAL_TOKEN

        env["AMITIA_JWT_SECRET"] = securityMaterial.jwtSecret

        env["AMITIA_GRAPH_STORE_ENABLED"] = "false"
        env["AMITIA_GRAPH_STORE_REQUIRED"] = "false"
        env["AMITIA_SURREAL_ENABLED"] = "false"

        env["HOME"] = GuestLayoutContract.HOME
        env["TMPDIR"] = GuestLayoutContract.TEMP_ROOT
        env["GODEBUG"] = "disablethp=1"
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

    private fun ensureLocalSecurityMaterial(layout: RuntimeHostLayout): LocalSecurityMaterial {
        val securityDir = File(layout.dataRoot, "security")
        ensureDirectory(securityDir)

        val localTokenFile = File(securityDir, "local-token")
        ensureCredential(localTokenFile, minLength = 32, randomBytes = 32)

        val jwtSecretFile = File(securityDir, "jwt-secret")
        val jwtSecret = ensureCredential(jwtSecretFile, minLength = 32, randomBytes = 48)
        return LocalSecurityMaterial(jwtSecret = jwtSecret)
    }

    private fun ensureRuntimeConfig(layout: RuntimeHostLayout, securityMaterial: LocalSecurityMaterial) {
        ensureDirectory(layout.configRoot)
        val configFile = File(layout.configRoot, "config.yml")
        val value = "jwt:\n  secret: \"${securityMaterial.jwtSecret}\"\ndesktopPetRuntime:\n  enabled: false\n"
        configFile.writeText(value, Charsets.UTF_8)
        configFile.setReadable(false, false)
        configFile.setWritable(false, false)
        configFile.setReadable(true, true)
        configFile.setWritable(true, true)
    }

    private fun ensureDirectory(directory: File) {
        if (directory.exists()) {
            require(directory.isDirectory) { "path is not a directory: ${directory.absolutePath}" }
            return
        }
        require(directory.mkdirs() || directory.isDirectory) {
            "failed to create directory: ${directory.absolutePath}"
        }
    }

    private fun ensureCredential(file: File, minLength: Int, randomBytes: Int): String {
        val existing = if (file.isFile) file.readText(Charsets.UTF_8).trim() else ""
        if (existing.length >= minLength) {
            return existing
        }

        ensureDirectory(file.parentFile ?: error("credential parent missing"))
        val bytes = ByteArray(randomBytes)
        secureRandom.nextBytes(bytes)
        val value = bytes.joinToString(separator = "") { byte ->
            val v = byte.toInt() and 0xFF
            HEX[v ushr 4].toString() + HEX[v and 0x0F]
        }

        val temp = File(file.parentFile, ".${file.name}.${System.nanoTime()}.tmp")
        try {
            temp.writeText(value + "\n", Charsets.UTF_8)
            temp.setReadable(false, false)
            temp.setWritable(false, false)
            temp.setExecutable(false, false)
            temp.setReadable(true, true)
            temp.setWritable(true, true)
            if (file.exists() && !file.delete()) {
                throw IllegalStateException("failed to replace credential: ${file.absolutePath}")
            }
            if (!temp.renameTo(file)) {
                file.writeText(value + "\n", Charsets.UTF_8)
                if (!temp.delete() && temp.exists()) {
                    throw IllegalStateException("failed to remove credential temp file: ${temp.absolutePath}")
                }
            }
            file.setReadable(false, false)
            file.setWritable(false, false)
            file.setExecutable(false, false)
            file.setReadable(true, true)
            file.setWritable(true, true)
            return value
        } finally {
            if (temp.exists()) temp.delete()
        }
    }

    private data class LocalSecurityMaterial(
        val jwtSecret: String,
    )

    private companion object {
        val secureRandom = SecureRandom()
        val HEX = "0123456789abcdef".toCharArray()
    }
}
