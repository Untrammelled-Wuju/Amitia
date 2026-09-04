package com.amitia.amitia_app.runtime.proot.internal

import com.amitia.amitia_app.runtime.connection.embeddedAndroidBackendPolicy
import com.amitia.amitia_app.runtime.install.RuntimeHostLayout
import com.amitia.amitia_app.runtime.proot.GuestLayout
import com.amitia.amitia_app.runtime.proot.MountContract
import com.amitia.amitia_app.runtime.proot.ProotBindMount
import com.amitia.amitia_app.runtime.proot.ProotEnvironment
import com.amitia.amitia_app.runtime.proot.ProotLaunchRequest
import com.amitia.amitia_app.runtime.proot.ProotLaunchSpec
import com.amitia.amitia_app.runtime.proot.RuntimeEnvironmentBuilder
import com.amitia.amitia_app.runtime.proot.RuntimeEnvironmentErrorCode
import com.amitia.amitia_app.runtime.proot.RuntimeEnvironmentRequest
import com.amitia.amitia_app.runtime.proot.RuntimeEnvironmentResult
import java.io.File

internal open class ProotEnvironmentAssembler(
    private val layout: RuntimeHostLayout,
    private val environmentBuilder: RuntimeEnvironmentBuilder,
) {

    fun assembleRootfsProbe(activeProgramSource: File): ProotLaunchSpec {
        ensureHostRuntimeDirectories()
        val environment = buildEnvironment()
        val bindMounts = buildBindMounts(activeProgramSource)

        return ProotLaunchSpec(
            binaryPath = "",
            rootfsPath = runtimePath(layout.rootfsRoot),
            workingDirectory = GuestLayout.BACKEND_DIR,
            command = listOf("/usr/bin/true"),
            bindMounts = bindMounts,
            environment = environment,
            fakeRoot = false,
        )
    }

    open fun assembleBackendLaunch(
        activeProgramSource: File,
        runtimeProfile: String = "local",
    ): ProotLaunchSpec {
        ensureHostRuntimeDirectories()
        val environment = buildEnvironment()
        val bindMounts = buildBindMounts(activeProgramSource)

        return ProotLaunchSpec(
            binaryPath = "",
            rootfsPath = runtimePath(layout.rootfsRoot),
            workingDirectory = GuestLayout.BACKEND_DIR,
            command = listOf(GuestLayout.BACKEND_SERVER, "--runtime-profile=$runtimeProfile"),
            bindMounts = bindMounts,
            environment = environment,
            fakeRoot = false,
        )
    }

    fun toProotLaunchRequest(spec: ProotLaunchSpec): ProotLaunchRequest {
        return ProotLaunchRequest.create(
            rootfsPath = spec.rootfsPath,
            workingDirectory = spec.workingDirectory,
            command = spec.command,
            bindMountsSource = spec.bindMounts,
            environmentSource = spec.environment,
            fakeRoot = spec.fakeRoot,
            killOnExit = spec.killOnExit,
        )
    }

    private fun buildEnvironment(): ProotEnvironment {
        val envRequest = RuntimeEnvironmentRequest(
            hostLayout = layout,
            endpoint = embeddedAndroidBackendPolicy(),
        )
        val envResult = environmentBuilder.build(envRequest)
        return when (envResult) {
            is RuntimeEnvironmentResult.Success -> {
                val merged = LinkedHashMap<String, String>()
                merged.putAll(envResult.environment.hostProcess)
                merged.putAll(envResult.environment.guestRuntime)
                ProotEnvironment.of(merged)
            }
            is RuntimeEnvironmentResult.Failure -> throw ProotEnvironmentException(envResult.code, envResult.message)
        }
    }

    private fun buildBindMounts(activeProgramSource: File): List<ProotBindMount> {
        val contract = MountContract.build(layout, activeProgramSource)
        val mounts = contract.mounts.map { mount ->
            ProotBindMount.create(runtimePath(File(mount.hostSource)), mount.guestTarget, readOnly = !mount.writable)
        }
        return mounts + listOf(
            ProotBindMount.create("/system", "/system", readOnly = true),
            ProotBindMount.create("/apex", "/apex", readOnly = true),
            ProotBindMount.create("/dev", "/dev", readOnly = true),
            ProotBindMount.create("/proc", "/proc", readOnly = true),
            ProotBindMount.create("/sys", "/sys", readOnly = true),
        )
    }

    private fun ensureHostRuntimeDirectories() {
        val directories = linkedSetOf(
            layout.configRoot,
            layout.dataRoot,
            layout.cacheRoot,
            layout.logRoot,
            layout.runRoot,
            layout.homeRoot,
            File(layout.runRoot, "tmp"),
            File(layout.runRoot, "proot-tmp"),
            File(layout.dataRoot, "security"),
            File(layout.dataRoot, "workspaces"),
            File(layout.dataRoot, "providers/qdrant/storage"),
            File(layout.configRoot, "providers/qdrant"),
        )
        for (directory in directories) {
            if (directory.exists()) {
                if (!directory.isDirectory) {
                    throw ProotEnvironmentException(
                        RuntimeEnvironmentErrorCode.HOST_LAYOUT_INVALID,
                        "runtime host path is not a directory: ${directory.absolutePath}",
                    )
                }
                continue
            }
            if (!directory.mkdirs() && !directory.isDirectory) {
                throw ProotEnvironmentException(
                    RuntimeEnvironmentErrorCode.HOST_LAYOUT_INVALID,
                    "failed to create runtime host directory: ${directory.absolutePath}",
                )
            }
        }
    }

    private fun runtimePath(file: File): String = file.canonicalFile.absolutePath
}

internal class ProotEnvironmentException(
    val code: RuntimeEnvironmentErrorCode,
    override val message: String,
) : RuntimeException(message)
