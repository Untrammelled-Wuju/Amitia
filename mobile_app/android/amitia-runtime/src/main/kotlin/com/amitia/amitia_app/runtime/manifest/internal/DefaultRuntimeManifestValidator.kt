package com.amitia.amitia_app.runtime.manifest.internal

import com.amitia.amitia_app.runtime.manifest.RuntimeManifest
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestComponent
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestError
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestErrorCode
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestInstallation
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestPaths
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestPayload
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestTarget
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestVerification
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestValidator

internal class DefaultRuntimeManifestValidator : RuntimeManifestValidator {

    override fun validateSchema(schemaVersion: Int): RuntimeManifestError? {
        if (schemaVersion != RuntimeManifest.SCHEMA_VERSION) {
            return RuntimeManifestError(
                RuntimeManifestErrorCode.UNSUPPORTED_MANIFEST_SCHEMA,
                "unsupported schemaVersion: $schemaVersion"
            )
        }
        return null
    }

    override fun validateRuntimeVersion(runtimeVersion: String): RuntimeManifestError? {
        if (runtimeVersion.isBlank()) {
            return RuntimeManifestError(
                RuntimeManifestErrorCode.MANIFEST_FIELD_INVALID,
                "runtimeVersion must not be blank"
            )
        }
        return null
    }

    override fun validateSourceCommit(sourceCommit: String): RuntimeManifestError? {
        val trimmed = sourceCommit.trim()
        if (trimmed.length != 40 && trimmed.length != 64) {
            return RuntimeManifestError(
                RuntimeManifestErrorCode.MANIFEST_FIELD_INVALID,
                "sourceCommit must be 40 or 64 hex chars"
            )
        }
        if (!trimmed.matches(Regex("[0-9a-fA-F]+"))) {
            return RuntimeManifestError(
                RuntimeManifestErrorCode.MANIFEST_FIELD_INVALID,
                "sourceCommit must be hex chars only"
            )
        }
        return null
    }

    override fun validatePackageSha256(packageSha256: String): RuntimeManifestError? {
        val trimmed = packageSha256.trim()
        if (trimmed.length != 64) {
            return RuntimeManifestError(
                RuntimeManifestErrorCode.MANIFEST_FIELD_INVALID,
                "packageSha256 must be 64 hex chars"
            )
        }
        if (!trimmed.matches(Regex("[0-9a-fA-F]+"))) {
            return RuntimeManifestError(
                RuntimeManifestErrorCode.MANIFEST_FIELD_INVALID,
                "packageSha256 must be hex chars only"
            )
        }
        return null
    }

    override fun validateTarget(target: RuntimeManifestTarget): RuntimeManifestError? {
        if (target.hostPlatform.isBlank()) {
            return RuntimeManifestError(
                RuntimeManifestErrorCode.MANIFEST_TARGET_MISMATCH,
                "hostPlatform must not be blank"
            )
        }
        if (target.hostAbi.isBlank()) {
            return RuntimeManifestError(
                RuntimeManifestErrorCode.MANIFEST_TARGET_MISMATCH,
                "hostAbi must not be blank"
            )
        }
        if (target.runtimeKind.isBlank()) {
            return RuntimeManifestError(
                RuntimeManifestErrorCode.MANIFEST_TARGET_MISMATCH,
                "runtimeKind must not be blank"
            )
        }
        if (target.guestPlatform.isBlank()) {
            return RuntimeManifestError(
                RuntimeManifestErrorCode.MANIFEST_TARGET_MISMATCH,
                "guestPlatform must not be blank"
            )
        }
        if (target.guestArchitecture.isBlank()) {
            return RuntimeManifestError(
                RuntimeManifestErrorCode.MANIFEST_TARGET_MISMATCH,
                "guestArchitecture must not be blank"
            )
        }
        if (target.distribution.isBlank()) {
            return RuntimeManifestError(
                RuntimeManifestErrorCode.MANIFEST_TARGET_MISMATCH,
                "distribution must not be blank"
            )
        }
        if (target.distributionRelease.isBlank()) {
            return RuntimeManifestError(
                RuntimeManifestErrorCode.MANIFEST_TARGET_MISMATCH,
                "distributionRelease must not be blank"
            )
        }
        return null
    }

    override fun validateInstallation(installation: RuntimeManifestInstallation, runtimeVersion: String): RuntimeManifestError? {
        if (installation.activeVersion.isBlank()) {
            return RuntimeManifestError(
                RuntimeManifestErrorCode.MANIFEST_ACTIVE_VERSION_MISMATCH,
                "activeVersion must not be blank"
            )
        }
        if (installation.activeVersion != runtimeVersion) {
            return RuntimeManifestError(
                RuntimeManifestErrorCode.MANIFEST_ACTIVE_VERSION_MISMATCH,
                "activeVersion must equal runtimeVersion"
            )
        }
        if (installation.rootfsId.isBlank()) {
            return RuntimeManifestError(
                RuntimeManifestErrorCode.MANIFEST_FIELD_INVALID,
                "rootfsId must not be blank"
            )
        }
        if (installation.runtimeRootId.isBlank()) {
            return RuntimeManifestError(
                RuntimeManifestErrorCode.MANIFEST_FIELD_INVALID,
                "runtimeRootId must not be blank"
            )
        }
        val sha = installation.runtimeRootTreeSha256.trim()
        if (sha.length != 64 || !sha.matches(Regex("[0-9a-fA-F]+"))) {
            return RuntimeManifestError(
                RuntimeManifestErrorCode.MANIFEST_FIELD_INVALID,
                "runtimeRootTreeSha256 must be 64 hex chars"
            )
        }
        return null
    }

    override fun validateComponents(components: List<RuntimeManifestComponent>): RuntimeManifestError? {
        val ids = mutableSetOf<String>()
        for (component in components) {
            if (component.id.isBlank()) {
                return RuntimeManifestError(
                    RuntimeManifestErrorCode.MANIFEST_FIELD_INVALID,
                    "component id must not be blank"
                )
            }
            if (component.root.isBlank()) {
                return RuntimeManifestError(
                    RuntimeManifestErrorCode.MANIFEST_FIELD_INVALID,
                    "component root must not be blank"
                )
            }
            if (component.source.isBlank()) {
                return RuntimeManifestError(
                    RuntimeManifestErrorCode.MANIFEST_FIELD_INVALID,
                    "component source must not be blank"
                )
            }
            if (component.sha256 != null && (component.sha256.length != 64 || !component.sha256.matches(Regex("[0-9a-fA-F]+")))) {
                return RuntimeManifestError(
                    RuntimeManifestErrorCode.MANIFEST_FIELD_INVALID,
                    "component sha256 must be 64 hex chars"
                )
            }
            if (component.treeSha256 != null && (component.treeSha256.length != 64 || !component.treeSha256.matches(Regex("[0-9a-fA-F]+")))) {
                return RuntimeManifestError(
                    RuntimeManifestErrorCode.MANIFEST_FIELD_INVALID,
                    "component treeSha256 must be 64 hex chars"
                )
            }
            if (!ids.add(component.id)) {
                return RuntimeManifestError(
                    RuntimeManifestErrorCode.MANIFEST_COMPONENT_DUPLICATE,
                    "duplicate component id: ${component.id}"
                )
            }
        }
        return null
    }

    override fun validatePayloads(payloads: List<RuntimeManifestPayload>): RuntimeManifestError? {
        val ids = mutableSetOf<String>()
        for (payload in payloads) {
            if (payload.id.isBlank()) {
                return RuntimeManifestError(
                    RuntimeManifestErrorCode.MANIFEST_FIELD_INVALID,
                    "payload id must not be blank"
                )
            }
            if (payload.sha256.isBlank() || payload.sha256.length != 64 || !payload.sha256.matches(Regex("[0-9a-fA-F]+"))) {
                return RuntimeManifestError(
                    RuntimeManifestErrorCode.MANIFEST_FIELD_INVALID,
                    "payload sha256 must be 64 hex chars"
                )
            }
            if (!ids.add(payload.id)) {
                return RuntimeManifestError(
                    RuntimeManifestErrorCode.MANIFEST_PAYLOAD_DUPLICATE,
                    "duplicate payload id: ${payload.id}"
                )
            }
        }
        return null
    }

    override fun validatePaths(paths: RuntimeManifestPaths): RuntimeManifestError? {
        val hostFields = listOf(
            paths.rootfsHostPath,
            paths.runtimeRootHostPath,
            paths.configHostPath,
            paths.dataHostPath,
            paths.cacheHostPath,
            paths.logHostPath,
            paths.runHostPath,
        )
        for (f in hostFields) {
            if (f.isBlank()) {
                return RuntimeManifestError(
                    RuntimeManifestErrorCode.MANIFEST_PATH_INVALID,
                    "host path must not be blank"
                )
            }
        }
        val guestFields = listOf(
            paths.guestRuntimeRoot,
            paths.guestConfigRoot,
            paths.guestDataRoot,
            paths.guestCacheRoot,
            paths.guestLogRoot,
            paths.guestRunRoot,
        )
        for (f in guestFields) {
            if (f.isBlank()) {
                return RuntimeManifestError(
                    RuntimeManifestErrorCode.MANIFEST_PATH_INVALID,
                    "guest path must not be blank"
                )
            }
            if (!f.startsWith("/")) {
                return RuntimeManifestError(
                    RuntimeManifestErrorCode.MANIFEST_PATH_INVALID,
                    "guest path must be absolute: $f"
                )
            }
        }
        validateHostPathOverlap(paths)?.let { return it }
        return null
    }

    private fun validateHostPathOverlap(paths: RuntimeManifestPaths): RuntimeManifestError? {
        val hostPaths = listOf(
            "rootfs" to paths.rootfsHostPath,
            "runtimeRoot" to paths.runtimeRootHostPath,
            "config" to paths.configHostPath,
            "data" to paths.dataHostPath,
            "cache" to paths.cacheHostPath,
            "logs" to paths.logHostPath,
            "run" to paths.runHostPath,
        )
        for (i in hostPaths.indices) {
            for (j in hostPaths.indices) {
                if (i >= j) continue
                val a = normalize(hostPaths[i].second)
                val b = normalize(hostPaths[j].second)
                if (a == b) {
                    return RuntimeManifestError(
                        RuntimeManifestErrorCode.MANIFEST_PATH_OVERLAP,
                        "${hostPaths[i].first} and ${hostPaths[j].first} share the same path"
                    )
                }
                if (a.startsWith(b + "/") || b.startsWith(a + "/")) {
                    return RuntimeManifestError(
                        RuntimeManifestErrorCode.MANIFEST_PATH_OVERLAP,
                        "${hostPaths[i].first} path overlaps with ${hostPaths[j].first}"
                    )
                }
            }
        }
        return null
    }

    private fun normalize(path: String): String {
        val p = path.replace('\\', '/')
        return if (p.endsWith("/") && p.length > 1) p.dropLast(1) else p
    }

    override fun validateVerification(verification: RuntimeManifestVerification): RuntimeManifestError? {
        if (!verification.allVerified()) {
            return RuntimeManifestError(
                RuntimeManifestErrorCode.MANIFEST_FIELD_INVALID,
                "not all verification flags are true"
            )
        }
        return null
    }
}
