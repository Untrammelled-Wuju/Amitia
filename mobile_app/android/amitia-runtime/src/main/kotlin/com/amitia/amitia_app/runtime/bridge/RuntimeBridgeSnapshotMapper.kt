package com.amitia.amitia_app.runtime.bridge

import com.amitia.amitia_app.runtime.api.RuntimeSnapshot
import com.amitia.amitia_app.runtime.api.RuntimeState
import com.amitia.amitia_app.runtime.manifest.RuntimeManifest

object RuntimeBridgeSnapshotMapper {

    fun toBridgeSnapshot(
        snapshot: RuntimeSnapshot,
        manifest: RuntimeManifest?,
        runtimeInstalled: Boolean,
        runtimeAvailable: Boolean,
    ): Map<String, Any?> {
        val result = LinkedHashMap<String, Any?>()
        result["schemaVersion"] = RuntimeBridgeContract.SCHEMA_VERSION
        result["state"] = mapState(snapshot.state).name
        result["generation"] = snapshot.generation
        result["runtimeInstalled"] = runtimeInstalled
        result["runtimeAvailable"] = runtimeAvailable
        result["lastError"] = snapshot.lastError?.let { mapError(it) }
        result["manifest"] = manifest?.let { mapManifest(it) }
        return result
    }

    private fun mapState(state: RuntimeState): BridgeState {
        return when (state) {
            RuntimeState.UNKNOWN -> BridgeState.UNAVAILABLE
            RuntimeState.NOT_INSTALLED -> BridgeState.NOT_INSTALLED
            RuntimeState.INSTALLED -> BridgeState.STOPPED
            RuntimeState.INSTALLING -> BridgeState.INSTALLING
            RuntimeState.VERIFYING -> BridgeState.INSTALLING
            RuntimeState.STARTING -> BridgeState.STARTING
            RuntimeState.READY -> BridgeState.READY
            RuntimeState.DEGRADED -> BridgeState.READY
            RuntimeState.STOPPING -> BridgeState.STOPPING
            RuntimeState.STOPPED -> BridgeState.STOPPED
            RuntimeState.REPAIRING -> BridgeState.INSTALLING
            RuntimeState.CORRUPTED -> BridgeState.FAILED
            RuntimeState.FAILED -> BridgeState.FAILED
        }
    }

    private fun mapError(error: com.amitia.amitia_app.runtime.api.RuntimeError): Map<String, Any?> {
        val result = LinkedHashMap<String, Any?>()
        result["code"] = error.code.name
        result["message"] = sanitizeErrorMessage(error.message)
        result["retryable"] = error.recoverable
        return result
    }

    private fun sanitizeErrorMessage(message: String): String {
        return message
            .replace(Regex("/data/user/\\d+/[^ \n]*"), "[redacted]")
            .replace(Regex("/data/data/[^ \n]*"), "[redacted]")
            .replace(Regex("noBackupFilesDir"), "[redacted]")
            .replace(Regex("filesDir"), "[redacted]")
    }

    private fun mapManifest(manifest: RuntimeManifest): Map<String, Any?> {
        val result = LinkedHashMap<String, Any?>()
        result["schemaVersion"] = manifest.schemaVersion
        result["runtimeVersion"] = manifest.runtimeVersion
        result["packageId"] = manifest.packageId
        result["targetPlatform"] = manifest.target.hostPlatform
        result["targetArch"] = manifest.target.hostAbi
        result["verified"] = manifest.verification.allVerified()
        return result
    }

    private enum class BridgeState {
        UNAVAILABLE,
        NOT_INSTALLED,
        STOPPED,
        INSTALLING,
        STARTING,
        READY,
        STOPPING,
        FAILED
    }
}
