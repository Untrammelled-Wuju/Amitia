package com.amitia.amitia_app.runtime.manifest.internal

import com.amitia.amitia_app.runtime.manifest.RuntimeManifest
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestComponent
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestError
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestErrorCode
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestResult
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestVerification
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestVerifyMode
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestVerifier
import java.io.File

internal class DefaultRuntimeManifestVerifier(
    private val hasher: InstalledFileHasher = InstalledFileHasher,
    private val treeHasher: InstalledTreeHasher = InstalledTreeHasher,
) : RuntimeManifestVerifier {

    override fun verify(manifest: RuntimeManifest, mode: RuntimeManifestVerifyMode): RuntimeManifestResult {
        if (!manifest.verification.allVerified()) {
            return RuntimeManifestResult.failure(
                RuntimeManifestError(
                    RuntimeManifestErrorCode.MANIFEST_FIELD_INVALID,
                    "manifest verification flags not all true"
                )
            )
        }

        val pathsResult = verifyPathsExist(manifest)
        if (pathsResult != null) {
            return RuntimeManifestResult.failure(pathsResult)
        }

        if (mode == RuntimeManifestVerifyMode.FULL) {
            val hashesResult = verifyComponentHashes(manifest)
            if (hashesResult != null) {
                return RuntimeManifestResult.failure(hashesResult)
            }
        }

        return RuntimeManifestResult.success(manifest)
    }

    private fun verifyPathsExist(manifest: RuntimeManifest): RuntimeManifestError? {
        val rootfsDir = File(manifest.paths.rootfsHostPath)
        if (!rootfsDir.exists() || !rootfsDir.isDirectory) {
            return RuntimeManifestError(
                RuntimeManifestErrorCode.ROOTFS_MISSING,
                "rootfs directory missing: ${manifest.paths.rootfsHostPath}"
            )
        }

        val runtimeRootDir = File(manifest.paths.runtimeRootHostPath)
        if (!runtimeRootDir.exists() || !runtimeRootDir.isDirectory) {
            return RuntimeManifestError(
                RuntimeManifestErrorCode.RUNTIME_ROOT_MISSING,
                "runtime root directory missing: ${manifest.paths.runtimeRootHostPath}"
            )
        }

        for (component in manifest.components) {
            if (component.id == RuntimeManifestComponent.ID_PROOT) {
                if (component.source != RuntimeManifestComponent.SOURCE_PROOT) {
                    return RuntimeManifestError(
                        RuntimeManifestErrorCode.PROOT_COMPONENT_MISSING,
                        "proot component source must be android-proot"
                    )
                }
                continue
            }
            val componentDir = File(manifest.paths.runtimeRootHostPath, component.root)
            if (!componentDir.exists()) {
                return RuntimeManifestError(
                    RuntimeManifestErrorCode.MANIFEST_COMPONENT_MISSING,
                    "component directory missing: ${component.id} at ${component.root}",
                    componentId = component.id
                )
            }
            if (!component.entry.isNullOrBlank()) {
                val entryFile = File(componentDir, component.entry)
                if (!entryFile.exists()) {
                    return RuntimeManifestError(
                        RuntimeManifestErrorCode.MANIFEST_COMPONENT_MISSING,
                        "component entry missing: ${component.id}/${component.entry}",
                        componentId = component.id
                    )
                }
            }
        }

        return null
    }

    private fun verifyComponentHashes(manifest: RuntimeManifest): RuntimeManifestError? {
        for (component in manifest.components) {
            if (component.source == RuntimeManifestComponent.SOURCE_PROOT) continue
            if (component.id == RuntimeManifestComponent.ID_PROOT) continue
            val componentDir = File(manifest.paths.runtimeRootHostPath, component.root)

            if (!component.entry.isNullOrBlank() && component.sha256 != null) {
                val entryFile = File(componentDir, component.entry)
                if (entryFile.exists()) {
                    val actualSha = hasher.sha256(entryFile)
                    if (!actualSha.equals(component.sha256, ignoreCase = true)) {
                        return RuntimeManifestError(
                            RuntimeManifestErrorCode.MANIFEST_COMPONENT_HASH_MISMATCH,
                            "component ${component.id} entry hash mismatch",
                            componentId = component.id
                        )
                    }
                }
            }

            if (component.treeSha256 != null) {
                val actualTree = treeHasher.computeTreeSha256(componentDir)
                if (!actualTree.equals(component.treeSha256, ignoreCase = true)) {
                    return RuntimeManifestError(
                        RuntimeManifestErrorCode.MANIFEST_TREE_HASH_MISMATCH,
                        "component ${component.id} tree hash mismatch",
                        componentId = component.id
                    )
                }
            }
        }

        val guestLayoutFile = File(manifest.paths.runtimeRootHostPath, "manifest/guest-layout.json")
        if (guestLayoutFile.exists()) {
            val actualSha = hasher.sha256(guestLayoutFile)
        }

        return null
    }
}
