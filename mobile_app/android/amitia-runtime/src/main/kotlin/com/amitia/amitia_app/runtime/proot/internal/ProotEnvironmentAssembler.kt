package com.amitia.amitia_app.runtime.proot.internal

import com.amitia.amitia_app.runtime.connection.BackendEndpointPolicy
import com.amitia.amitia_app.runtime.connection.embeddedAndroidBackendPolicy
import com.amitia.amitia_app.runtime.install.RuntimeHostLayout
import com.amitia.amitia_app.runtime.proot.GuestLayout
import com.amitia.amitia_app.runtime.proot.MountContract
import com.amitia.amitia_app.runtime.proot.ProotBindMount
import com.amitia.amitia_app.runtime.proot.ProotEnvironment
import com.amitia.amitia_app.runtime.proot.ProotLaunchRequest
import com.amitia.amitia_app.runtime.proot.ProotLaunchSpec
import com.amitia.amitia_app.runtime.proot.RuntimeEnvironmentBuilder
import com.amitia.amitia_app.runtime.proot.RuntimeEnvironmentRequest
import com.amitia.amitia_app.runtime.proot.RuntimeEnvironmentResult
import java.io.File

internal class ProotEnvironmentAssembler(
    private val layout: RuntimeHostLayout,
    private val environmentBuilder: RuntimeEnvironmentBuilder,
) {

    fun assembleRootfsProbe(activeProgramSource: File): ProotLaunchSpec {
        val environment = buildEnvironment()
        val bindMounts = buildBindMounts(activeProgramSource)

        return ProotLaunchSpec(
            binaryPath = "",
            rootfsPath = layout.rootfsRoot.absolutePath,
            workingDirectory = GuestLayout.BACKEND_DIR,
            command = listOf("/usr/bin/env"),
            bindMounts = bindMounts,
            environment = environment,
        )
    }

    fun assembleBackendLaunch(activeProgramSource: File, runtimeProfile: String): ProotLaunchSpec {
        val environment = buildEnvironment()
        val bindMounts = buildBindMounts(activeProgramSource)

        return ProotLaunchSpec(
            binaryPath = "",
            rootfsPath = layout.rootfsRoot.absolutePath,
            workingDirectory = GuestLayout.BACKEND_DIR,
            command = listOf(GuestLayout.BACKEND_SERVER, "--runtime-profile=$runtimeProfile"),
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

    private fun buildBindMounts(activeProgramSource: File): List<ProotBindMount> {
        val contract = MountContract.build(layout, activeProgramSource)
        return contract.toProotBindMounts()
    }
}

internal class ProotEnvironmentException(
    val code: com.amitia.amitia_app.runtime.proot.RuntimeEnvironmentErrorCode,
    override val message: String,
) : RuntimeException(message)
