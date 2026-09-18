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
    private val dnsServersProvider: () -> List<String> = { emptyList() },
    private val caCertificateDirectories: List<File> = listOf(
        File("/system/etc/security/cacerts"),
        File("/apex/com.android.conscrypt/cacerts"),
    ),
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
        ensureCaBundleFile()
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
            ProotBindMount.create(ensureResolverFile().absolutePath, "/etc/resolv.conf", readOnly = true),
        )
    }

    private fun ensureResolverFile(): File {
        val servers = dnsServersProvider()
            .map(String::trim)
            .filter(String::isNotEmpty)
            .distinct()
            .ifEmpty { listOf("223.5.5.5", "119.29.29.29") }
        val resolver = File(layout.runRoot, "resolv.conf")
        val content = buildString {
            for (server in servers) {
                append("nameserver ")
                append(server)
                append('\n')
            }
            append("options timeout:2 attempts:3 rotate\n")
        }
        if (resolver.exists() && !resolver.delete()) {
            throw ProotEnvironmentException(
                RuntimeEnvironmentErrorCode.BUILD_FAILED,
                "failed to replace resolver file: ${resolver.absolutePath}",
            )
        }
        resolver.writeText(content, Charsets.UTF_8)
        return resolver
    }

    private fun ensureCaBundleFile(): File {
        val certificates = caCertificateDirectories
            .asSequence()
            .flatMap { directory ->
                directory.listFiles()
                    ?.asSequence()
                    ?.filter { it.isFile }
                    ?.sortedBy { it.name }
                    ?: emptySequence()
            }
            .mapNotNull { file ->
                runCatching { file.readText(Charsets.US_ASCII).trim() }.getOrNull()
            }
            .filter { it.isNotEmpty() }
            .toList()

        val bundle = File(layout.dataRoot, "security/ca-certificates.crt")
        if (certificates.isEmpty()) {
            if (bundle.exists()) bundle.delete()
            return bundle
        }

        val content = buildString {
            for (certificate in certificates) {
                append(certificate)
                if (!certificate.endsWith('\n')) append('\n')
            }
        }
        if (bundle.exists() && !bundle.delete()) {
            throw ProotEnvironmentException(
                RuntimeEnvironmentErrorCode.BUILD_FAILED,
                "failed to replace CA bundle: ${bundle.absolutePath}",
            )
        }
        bundle.writeText(content, Charsets.UTF_8)
        return bundle
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
