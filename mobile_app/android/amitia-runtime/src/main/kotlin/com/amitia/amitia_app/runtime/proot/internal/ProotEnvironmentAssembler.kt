package com.amitia.amitia_app.runtime.proot.internal

import com.amitia.amitia_app.runtime.connection.BackendEndpointPolicy
import com.amitia.amitia_app.runtime.connection.embeddedAndroidBackendPolicy
import com.amitia.amitia_app.runtime.install.RuntimeHostLayout
import com.amitia.amitia_app.runtime.proot.ProotBindMount
import com.amitia.amitia_app.runtime.proot.ProotLaunchRequest
import com.amitia.amitia_app.runtime.proot.RuntimeEnvironment
import com.amitia.amitia_app.runtime.proot.RuntimeEnvironmentBuilder
import com.amitia.amitia_app.runtime.proot.RuntimeEnvironmentRequest
import com.amitia.amitia_app.runtime.proot.RuntimeEnvironmentResult

internal data class ProotLaunchSpec(
    val rootfsPath: String,
    val workingDirectory: String,
    val command: List<String>,
    val bindMounts: List<ProotBindMount>,
    val environment: RuntimeEnvironment,
    val fakeRoot: Boolean = true,
    val killOnExit: Boolean = true,
) {
    companion object {
        const val DEFAULT_WORKDIR = "/opt/amitia"
    }
}

internal class ProotEnvironmentAssembler(
    private val layout: RuntimeHostLayout,
    private val environmentBuilder: RuntimeEnvironmentBuilder,
) {

    fun assembleRootfsProbe(): ProotLaunchSpec {
        val envRequest = RuntimeEnvironmentRequest(
            hostLayout = layout,
            endpoint = embeddedAndroidBackendPolicy(),
        )
        val envResult = environmentBuilder.build(envRequest)
        val environment = when (envResult) {
            is RuntimeEnvironmentResult.Success -> envResult.environment
            is RuntimeEnvironmentResult.Failure -> throw ProotEnvironmentException(envResult.code, envResult.message)
        }

        val bindMounts = buildBindMounts(layout)

        return ProotLaunchSpec(
            rootfsPath = layout.rootfsRoot.absolutePath,
            workingDirectory = ProotLaunchSpec.DEFAULT_WORKDIR,
            command = listOf("/usr/bin/env"),
            bindMounts = bindMounts,
            environment = environment,
        )
    }

    fun assembleBackendLaunch(): ProotLaunchSpec {
        val envRequest = RuntimeEnvironmentRequest(
            hostLayout = layout,
            endpoint = embeddedAndroidBackendPolicy(),
        )
        val envResult = environmentBuilder.build(envRequest)
        val environment = when (envResult) {
            is RuntimeEnvironmentResult.Success -> envResult.environment
            is RuntimeEnvironmentResult.Failure -> throw ProotEnvironmentException(envResult.code, envResult.message)
        }

        val bindMounts = buildBindMounts(layout)

        return ProotLaunchSpec(
            rootfsPath = layout.rootfsRoot.absolutePath,
            workingDirectory = ProotLaunchSpec.DEFAULT_WORKDIR,
            command = listOf("/opt/amitia/backend/amitia-server"),
            bindMounts = bindMounts,
            environment = environment,
        )
    }

    fun toProotLaunchRequest(spec: ProotLaunchSpec): ProotLaunchRequest {
        return ProotLaunchRequest.create(
            rootfsPath = spec.rootfsPath,
            workingDirectory = spec.workingDirectory,
            command = spec.command,
            bindMountsSource = spec.bindMounts,
            environmentSource = com.amitia.amitia_app.runtime.proot.ProotEnvironment.of(spec.environment.guestRuntime),
            fakeRoot = spec.fakeRoot,
            killOnExit = spec.killOnExit,
        )
    }

    private fun buildBindMounts(layout: RuntimeHostLayout): List<ProotBindMount> {
        val mounts = mutableListOf<ProotBindMount>()

        mounts.add(ProotBindMount.create(layout.versionsRoot.absolutePath, "/opt/amitia"))
        mounts.add(ProotBindMount.create(layout.configRoot.absolutePath, "/etc/amitia"))
        mounts.add(ProotBindMount.create(layout.dataRoot.absolutePath, "/var/lib/amitia"))
        mounts.add(ProotBindMount.create(layout.cacheRoot.absolutePath, "/var/cache/amitia"))
        mounts.add(ProotBindMount.create(layout.logRoot.absolutePath, "/var/log/amitia"))
        mounts.add(ProotBindMount.create(layout.runRoot.absolutePath, "/run/amitia"))
        mounts.add(ProotBindMount.create("/system", "/system"))
        mounts.add(ProotBindMount.create("/proc", "/proc"))

        return mounts
    }
}

internal class ProotEnvironmentException(
    val code: com.amitia.amitia_app.runtime.proot.RuntimeEnvironmentErrorCode,
    override val message: String,
) : RuntimeException(message)
