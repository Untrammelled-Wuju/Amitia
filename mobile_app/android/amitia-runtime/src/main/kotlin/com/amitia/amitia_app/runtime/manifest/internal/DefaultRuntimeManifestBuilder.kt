package com.amitia.amitia_app.runtime.manifest.internal

import com.amitia.amitia_app.runtime.abi.RuntimeAbiStatus
import com.amitia.amitia_app.runtime.install.RuntimeHostLayout
import com.amitia.amitia_app.runtime.manifest.RuntimeManifest
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestBuilder
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestComponent
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestError
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestErrorCode
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestInstallation
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestPaths
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestPayload
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestResult
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestTarget
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestVerification
import java.io.File

internal class DefaultRuntimeManifestBuilder(
    private val layout: RuntimeHostLayout,
    private val abiStatus: RuntimeAbiStatus.Supported,
) : RuntimeManifestBuilder {

    private var manifest: RuntimeManifest? = null

    fun setManifest(manifest: RuntimeManifest): DefaultRuntimeManifestBuilder {
        this.manifest = manifest
        return this
    }

    override fun buildFromInstalledTree(
        runtimeVersion: String,
        sourceCommit: String,
        packageId: String,
        packageSha256: String,
        rootfsId: String,
        runtimeRootTreeSha256: String,
        payloads: List<RuntimeManifestPayload>,
        components: List<RuntimeManifestComponent>,
    ): RuntimeManifestResult {
        val target = RuntimeManifestTarget(
            hostPlatform = RuntimeManifestTarget.HOST_PLATFORM_ANDROID,
            hostAbi = abiStatus.abi,
            runtimeKind = RuntimeManifestTarget.RUNTIME_KIND_PROOT,
            guestPlatform = RuntimeManifestTarget.GUEST_PLATFORM_LINUX,
            guestArchitecture = "arm64",
            distribution = "amitia",
            distributionRelease = runtimeVersion,
        )

        val installation = RuntimeManifestInstallation(
            activeVersion = runtimeVersion,
            rootfsId = rootfsId,
            runtimeRootId = runtimeVersion,
            runtimeRootTreeSha256 = runtimeRootTreeSha256,
        )

        val paths = RuntimeManifestPaths(
            rootfsHostPath = layout.rootfsRoot.absolutePath,
            runtimeRootHostPath = layout.runtimeVersionRoot(runtimeVersion).absolutePath,
            configHostPath = layout.configRoot.absolutePath,
            dataHostPath = layout.dataRoot.absolutePath,
            cacheHostPath = layout.cacheRoot.absolutePath,
            logHostPath = layout.logRoot.absolutePath,
            runHostPath = layout.runRoot.absolutePath,
            guestRuntimeRoot = RuntimeManifestPaths.GUEST_RUNTIME_ROOT,
            guestConfigRoot = RuntimeManifestPaths.GUEST_CONFIG_ROOT,
            guestDataRoot = RuntimeManifestPaths.GUEST_DATA_ROOT,
            guestCacheRoot = RuntimeManifestPaths.GUEST_CACHE_ROOT,
            guestLogRoot = RuntimeManifestPaths.GUEST_LOG_ROOT,
            guestRunRoot = RuntimeManifestPaths.GUEST_RUN_ROOT,
        )

        val verification = RuntimeManifestVerification(
            packageVerified = true,
            rootfsVerified = true,
            runtimeRootVerified = true,
            componentsVerified = true,
            guestLayoutVerified = true,
            mountContractVerified = true,
        )

        val manifest = RuntimeManifest(
            schemaVersion = RuntimeManifest.SCHEMA_VERSION,
            runtimeVersion = runtimeVersion,
            sourceCommit = sourceCommit,
            packageId = packageId,
            packageSha256 = packageSha256,
            target = target,
            installation = installation,
            payloads = payloads,
            components = components,
            paths = paths,
            verification = verification,
        )

        return RuntimeManifestResult.success(manifest)
    }

    override fun build(): RuntimeManifestResult {
        val m = manifest ?: return RuntimeManifestResult.failure(
            RuntimeManifestError(
                RuntimeManifestErrorCode.MANIFEST_INVALID_JSON,
                "manifest is null"
            )
        )
        return RuntimeManifestResult.success(m)
    }
}
