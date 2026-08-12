package com.amitia.amitia_app.runtime.proot.internal

import com.amitia.amitia_app.runtime.connection.BackendEndpointPolicy
import com.amitia.amitia_app.runtime.connection.embeddedAndroidBackendPolicy
import com.amitia.amitia_app.runtime.install.RuntimeHostLayout
import com.amitia.amitia_app.runtime.proot.ProotBindMount
import com.amitia.amitia_app.runtime.proot.ProotEnvironment
import com.amitia.amitia_app.runtime.proot.ProotLaunchRequest
import com.amitia.amitia_app.runtime.proot.ProotLaunchSpec
import com.amitia.amitia_app.runtime.proot.RuntimeEnvironment
import com.amitia.amitia_app.runtime.proot.RuntimeEnvironmentBuilder
import com.amitia.amitia_app.runtime.proot.RuntimeEnvironmentRequest
import com.amitia.amitia_app.runtime.proot.RuntimeEnvironmentResult

internal class ProotEnvironmentAssembler(
    private val layout: RuntimeHostLayout,
    private val environmentBuilder: RuntimeEnvironmentBuilder,
) {

    fun assembleRootfsProbe(): ProotLaunchSpec {
        val environment = buildEnvironment()
        val bindMounts = buildBindMounts(layout)

        return ProotLaunchSpec(
            binaryPath = "",
            rootfsPath = layout.rootfsRoot.absolutePath,
            workingDirectory = ProotLaunchSpec.DEFAULT_WORKDIR,
            command = listOf("/usr/bin/env"),
            bindMounts = bindMounts,
            environment = environment,
        )
    }

    fun assembleBackendLaunch(): ProotLaunchSpec {
        val environment = buildEnvironment()
        val bindMounts = buildBindMounts(layout)

        return ProotLaunchSpec(
            binaryPath = "",
            rootfsPath = layout.rootfsRoot.absolutePath,
            workingDirectory = ProotLaunchSpec.DEFAULT_WORKDIR,
            command = listOf("/opt/amitia/backend/amitia-server"),
            bindMounts = bindMounts,
            environment = environment,
        )
    }

    fun toProotLaunchRequest(spec: ProotLaunchSpec, binaryPath: String): ProotLaunchRequest {
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
            is RuntimeEnvironmentResult.Success -> ProotEnvironment.of(envResult.environment.guestRuntime)
            is RuntimeEnvironmentResult.Failure -> throw ProotEnvironmentException(envResult.code, envResult.message)
        }
    }

    private fun buildBindMounts(layout: RuntimeHostLayout): List<ProotBindMount> {
        val mounts = mutableListOf<ProotBindMount>()

        mounts.add(ProotBindMount.create(layout.versionsRoot.absolutePath, "/opt/amitia"))
        mounts.add(ProotBindMount.create(layout.configRoot.absolutePath, "/etc/amitia"))
        mounts.add(ProotBindMount.create(layout.dataRoot.absolutePath, "/var/lib/amitia"))
        mounts.add(ProotBindMount.create(layout.cacheRoot.absolutePath, "/var/cache/amitia"))
        mounts.add(ProotBindMount.create(layout.logRoot.absolutePath, "/var/log/amitia"))
        mounts.add(ProotBindMount.create(layout.runRoot.absolutePath, "/run/amitia"))
        mounts.add(ProotBindMount.create(layout.homeRoot.absolutePath, "/home/amitia"))
        mounts.add(ProotBindMount.create("/system", "/system"))
        mounts.add(ProotBindMount.create("/proc", "/proc"))

        return mounts
    }
}

internal class ProotEnvironmentException(
    val code: com.amitia.amitia_app.runtime.proot.RuntimeEnvironmentErrorCode,
    override val message: String,
) : RuntimeException(message)
