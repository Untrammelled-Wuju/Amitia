package com.amitia.amitia_app.runtime.internal

import com.amitia.amitia_app.runtime.api.RuntimeState
import com.amitia.amitia_app.runtime.install.ActiveRuntimeManager
import com.amitia.amitia_app.runtime.install.ActiveRuntimeResult
import com.amitia.amitia_app.runtime.install.InstalledRuntimeVerifier
import com.amitia.amitia_app.runtime.install.RuntimeHostLayout
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestErrorCode
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestResult
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestStore
import java.io.File

internal sealed interface RuntimeBootstrapResult {
    data object NotInstalled : RuntimeBootstrapResult

    data class InstalledStopped(
        val runtimeVersion: String,
    ) : RuntimeBootstrapResult

    data class Failed(
        val code: RuntimeBootstrapErrorCode,
        val message: String,
    ) : RuntimeBootstrapResult
}

internal enum class RuntimeBootstrapErrorCode {
    ACTIVE_WITHOUT_MANIFEST,
    MANIFEST_WITHOUT_ACTIVE,
    MANIFEST_CORRUPT,
    ACTIVE_CORRUPT,
    ACTIVE_VERSION_MISSING,
    INSTALLED_VERSION_MISSING,
    PROGRAM_ROOT_INVALID,
    INSTALLED_RUNTIME_CORRUPT,
    PACKAGE_IDENTITY_MISMATCH,
    RUNTIME_TREE_DRIFT,
}

internal class DefaultRuntimeBootstrapper(
    private val manifestStore: RuntimeManifestStore,
    private val activeRuntimeManager: ActiveRuntimeManager,
    private val hostLayout: RuntimeHostLayout,
    private val installedRuntimeVerifier: InstalledRuntimeVerifier,
) : RuntimeBootstrapper {

    override fun bootstrap(): RuntimeBootstrapResult {
        val manifestResult = manifestStore.read()
        val activeResult = activeRuntimeManager.current()

        val manifestMissing = manifestResult is RuntimeManifestResult.Failure &&
            manifestResult.error.code == RuntimeManifestErrorCode.MANIFEST_NOT_FOUND
        val manifestCorrupt = manifestResult is RuntimeManifestResult.Failure &&
            manifestResult.error.code != RuntimeManifestErrorCode.MANIFEST_NOT_FOUND
        val hasManifest = manifestResult is RuntimeManifestResult.Success

        val activeMissing = activeResult is ActiveRuntimeResult.NoActiveRuntime
        val activeCorrupt = activeResult is ActiveRuntimeResult.Failure
        val hasActive = activeResult is ActiveRuntimeResult.Active

        if (manifestCorrupt) {
            val errorMessage = (manifestResult as RuntimeManifestResult.Failure).error.manifestMessage
            return RuntimeBootstrapResult.Failed(
                code = RuntimeBootstrapErrorCode.MANIFEST_CORRUPT,
                message = "manifest corrupt: $errorMessage"
            )
        }

        if (activeCorrupt) {
            val errorMessage = (activeResult as ActiveRuntimeResult.Failure).message
            return RuntimeBootstrapResult.Failed(
                code = RuntimeBootstrapErrorCode.ACTIVE_CORRUPT,
                message = "active runtime corrupt: $errorMessage"
            )
        }

        if (manifestMissing && activeMissing) {
            return RuntimeBootstrapResult.NotInstalled
        }

        if (!hasManifest && hasActive) {
            return RuntimeBootstrapResult.Failed(
                code = RuntimeBootstrapErrorCode.ACTIVE_WITHOUT_MANIFEST,
                message = "active runtime exists without manifest"
            )
        }

        if (hasManifest && !hasActive) {
            return RuntimeBootstrapResult.Failed(
                code = RuntimeBootstrapErrorCode.MANIFEST_WITHOUT_ACTIVE,
                message = "manifest exists without active runtime"
            )
        }

        val manifest = (manifestResult as RuntimeManifestResult.Success).manifest
        val activeInfo = (activeResult as ActiveRuntimeResult.Active).info

        if (manifest.runtimeVersion != activeInfo.version) {
            return RuntimeBootstrapResult.Failed(
                code = RuntimeBootstrapErrorCode.PACKAGE_IDENTITY_MISMATCH,
                message = "manifest version ${manifest.runtimeVersion} does not match active version ${activeInfo.version}"
            )
        }

        val versionDir = hostLayout.runtimeVersionRoot(activeInfo.version)
        if (!versionDir.exists() || !versionDir.isDirectory) {
            return RuntimeBootstrapResult.Failed(
                code = RuntimeBootstrapErrorCode.ACTIVE_VERSION_MISSING,
                message = "active runtime version directory missing: ${activeInfo.version}"
            )
        }

        return when (val verifyResult = installedRuntimeVerifier.verify(versionDir)) {
            is com.amitia.amitia_app.runtime.install.InstalledRuntimeVerificationResult.Success ->
                RuntimeBootstrapResult.InstalledStopped(
                    runtimeVersion = manifest.runtimeVersion
                )
            is com.amitia.amitia_app.runtime.install.InstalledRuntimeVerificationResult.Failure ->
                RuntimeBootstrapResult.Failed(
                    code = mapVerifyFailureToErrorCode(verifyResult.code),
                    message = verifyResult.message
                )
        }
    }

    private fun mapVerifyFailureToErrorCode(code: com.amitia.amitia_app.runtime.install.RuntimeInstallErrorCode): RuntimeBootstrapErrorCode {
        return when (code) {
            com.amitia.amitia_app.runtime.install.RuntimeInstallErrorCode.RUNTIME_VERIFY_FAILED ->
                RuntimeBootstrapErrorCode.INSTALLED_RUNTIME_CORRUPT
            else -> RuntimeBootstrapErrorCode.PROGRAM_ROOT_INVALID
        }
    }
}

internal interface RuntimeBootstrapper {
    fun bootstrap(): RuntimeBootstrapResult
}
