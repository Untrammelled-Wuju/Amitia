package com.amitia.amitia_app.runtime.bridge

import android.os.Handler
import android.os.Looper
import com.amitia.amitia_app.runtime.api.RuntimeController
import com.amitia.amitia_app.runtime.api.RuntimeErrorCode
import com.amitia.amitia_app.runtime.api.RuntimeInstallRequest
import com.amitia.amitia_app.runtime.api.RuntimeOperationCallback
import com.amitia.amitia_app.runtime.api.RuntimeOperationResult
import com.amitia.amitia_app.runtime.api.RuntimeRepairRequest
import com.amitia.amitia_app.runtime.api.RuntimeStartRequest
import com.amitia.amitia_app.runtime.api.RuntimeStartReason
import com.amitia.amitia_app.runtime.api.RuntimeState
import com.amitia.amitia_app.runtime.api.RuntimeStopRequest
import com.amitia.amitia_app.runtime.api.RuntimeStopReason
import com.amitia.amitia_app.runtime.api.RuntimeVerifyRequest
import com.amitia.amitia_app.runtime.connection.BackendConnectionAvailability
import com.amitia.amitia_app.runtime.connection.BackendConnectionProvider
import com.amitia.amitia_app.runtime.connection.BackendConnectionError
import com.amitia.amitia_app.runtime.connection.BackendConnectionErrorCode
import com.amitia.amitia_app.runtime.connection.internal.BackendConnectionMapper
import com.amitia.amitia_app.runtime.bridge.RuntimeBridgeContract
import com.amitia.amitia_app.runtime.bridge.RuntimeBridgeErrorMapper
import com.amitia.amitia_app.runtime.bridge.RuntimeBridgeSnapshotMapper
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestResult
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestStore
import com.amitia.amitia_app.runtime.packagetrusted.RuntimePackageSource
import com.amitia.amitia_app.runtime.packagetrusted.RuntimePackageReference
import com.amitia.amitia_app.runtime.packagetrusted.RuntimePackageSourceResult
import com.amitia.amitia_app.runtime.packagetrusted.TrustedRuntimePackageSource
import io.flutter.plugin.common.MethodCall
import io.flutter.plugin.common.MethodChannel

internal class RuntimeBridgeHandler(
    private val controller: RuntimeController,
    private val backendConnectionProvider: BackendConnectionProvider,
    private val manifestStore: RuntimeManifestStore?,
    private val runtimePackageSource: RuntimePackageSource,
) : MethodChannel.MethodCallHandler {

    override fun onMethodCall(call: MethodCall, result: MethodChannel.Result) {
        try {
            when (call.method) {
                RuntimeBridgeContract.METHOD_SNAPSHOT -> handleSnapshot(result)
                RuntimeBridgeContract.METHOD_START -> handleStart(result)
                RuntimeBridgeContract.METHOD_START_WITH_PROFILE -> handleStartWithProfile(call, result)
                RuntimeBridgeContract.METHOD_STOP -> handleStop(result)
                RuntimeBridgeContract.METHOD_INSTALL -> handleInstall(result)
                RuntimeBridgeContract.METHOD_RECONCILE_EMBEDDED -> handleReconcileEmbedded(result)
                RuntimeBridgeContract.METHOD_VERIFY -> handleVerify(result)
                RuntimeBridgeContract.METHOD_REPAIR -> handleRepair(result)
                RuntimeBridgeContract.METHOD_MANIFEST_SUMMARY -> handleManifestSummary(result)
                RuntimeBridgeContract.METHOD_GET_BACKEND_CONNECTION -> handleGetBackendConnection(result)
                else -> result.notImplemented()
            }
        } catch (e: Exception) {
            emitRuntimeLog(
                "ERROR",
                "Runtime bridge ${call.method} failed: ${e.message ?: e.javaClass.simpleName}",
            )
            result.error(
                "INTERNAL_ERROR",
                "Internal error: ${e.message ?: e.javaClass.simpleName}",
                null
            )
        }
    }

    private fun handleSnapshot(result: MethodChannel.Result) {
        val snapshot = controller.snapshot()
        android.util.Log.d(
            BRIDGE_LOG_TAG,
            "snapshot: state=${snapshot.state.name} gen=${snapshot.generation} activeProfile=${snapshot.activeProfile} lastError=${snapshot.lastError?.code?.name}:${snapshot.lastError?.message}"
        )
        val manifest = manifestStore?.read()
        val runtimeInstalled = manifest is RuntimeManifestResult.Success
        val runtimeAvailable = snapshot.state == RuntimeState.READY ||
                snapshot.state == RuntimeState.DEGRADED
        val mapped = RuntimeBridgeSnapshotMapper.toBridgeSnapshot(
            snapshot = snapshot,
            manifest = (manifest as? RuntimeManifestResult.Success)?.manifest,
            runtimeInstalled = runtimeInstalled,
            runtimeAvailable = runtimeAvailable,
        )
        result.success(mapped)
    }

    private fun handleGetBackendConnection(result: MethodChannel.Result) {
        val availability = backendConnectionProvider.current()
        val snapshot = controller.snapshot()
        val error = when (availability) {
            is BackendConnectionAvailability.Available -> null
            is BackendConnectionAvailability.Resolving -> BackendConnectionError(
                BackendConnectionErrorCode.BACKEND_NOT_READY,
                "runtime backend connection is still resolving for generation ${snapshot.generation}",
            )
            is BackendConnectionAvailability.Unavailable -> {
                backendConnectionProvider.lastError() ?:
                    if (snapshot.state != RuntimeState.READY && snapshot.state != RuntimeState.DEGRADED) {
                        BackendConnectionError(
                            BackendConnectionErrorCode.RUNTIME_NOT_READY,
                            snapshot.lastError?.message
                                ?: "runtime is ${snapshot.state.name.lowercase()} for generation ${snapshot.generation}",
                        )
                    } else {
                        BackendConnectionError(
                            BackendConnectionErrorCode.ENDPOINT_UNAVAILABLE,
                            "runtime is ready but the backend endpoint is unavailable for generation ${snapshot.generation}",
                        )
                    }
            }
        }
        val mapped = BackendConnectionMapper.toPayload(
            available = availability is BackendConnectionAvailability.Available,
            descriptor = (availability as? BackendConnectionAvailability.Available)?.descriptor,
            error = error,
        )
        result.success(mapped)
    }

    private fun handleStart(result: MethodChannel.Result) {
        emitRuntimeLog("INFO", "Runtime start requested")
        val request = RuntimeStartRequest(reason = RuntimeStartReason.USER_REQUEST)
        controller.start(request, object : RuntimeOperationCallback {
            override fun onCompleted(operationResult: RuntimeOperationResult) {
                handleOperationResult(operationResult, result)
            }
        })
    }

    private fun handleStartWithProfile(call: MethodCall, result: MethodChannel.Result) {
        val profile = call.argument<String>("profile") ?: "local"
        emitRuntimeLog("INFO", "Runtime start requested with profile: $profile")
        val request = RuntimeStartRequest(
            reason = RuntimeStartReason.USER_REQUEST,
            profile = profile
        )
        controller.start(request, object : RuntimeOperationCallback {
            override fun onCompleted(operationResult: RuntimeOperationResult) {
                handleOperationResult(operationResult, result)
            }
        })
    }

    private fun handleStop(result: MethodChannel.Result) {
        emitRuntimeLog("INFO", "Runtime stop requested")
        val request = RuntimeStopRequest(
            reason = RuntimeStopReason.USER_REQUEST,
            force = false
        )
        controller.stop(request, object : RuntimeOperationCallback {
            override fun onCompleted(operationResult: RuntimeOperationResult) {
                handleOperationResult(operationResult, result)
            }
        })
    }

    private fun handleInstall(result: MethodChannel.Result) {
        emitRuntimeLog("INFO", "Runtime installation requested")
        when (val source = runtimePackageSource.materialize()) {
            is RuntimePackageSourceResult.Failed -> {
                emitRuntimeLog(
                    "ERROR",
                    "Runtime package preparation failed [${source.code.name}]: ${source.message}",
                )
                result.error(
                    source.code.name,
                    source.message,
                    null,
                )
            }
            is RuntimePackageSourceResult.Ready -> {
                val trustedRef = source.reference
                emitRuntimeLog(
                    "INFO",
                    "Runtime package prepared for version ${trustedRef.expectedRuntimeVersion}",
                )
                val request = RuntimeInstallRequest(
                    packageUri = trustedRef.packageFile.absolutePath,
                    expectedVersion = trustedRef.expectedRuntimeVersion,
                    allowRepairExisting = false,
                )
                controller.install(request, object : RuntimeOperationCallback {
                    override fun onCompleted(operationResult: RuntimeOperationResult) {
                        handleOperationResult(operationResult, result)
                    }
                })
            }
        }
    }

    private fun handleReconcileEmbedded(result: MethodChannel.Result) {
        emitRuntimeLog("INFO", "Runtime reconcile requested")
        val expectedRuntimeVersion = TrustedRuntimePackageSource.expectedRuntimeVersion()
        val expectedPackageSha256 = TrustedRuntimePackageSource.expectedPackageSha256()
        val manifestResult = manifestStore?.read()
        val installedManifest =
            (manifestResult as? RuntimeManifestResult.Success)?.manifest
        val identityMatches = installedManifest?.packageSha256
            ?.equals(expectedPackageSha256, ignoreCase = true) == true
        val snapshot = controller.snapshot()
        val runtimeHealthy = identityMatches &&
            snapshot.state != RuntimeState.CORRUPTED &&
            snapshot.state != RuntimeState.FAILED
        if (runtimeHealthy) {
            emitRuntimeLog(
                "INFO",
                "Runtime identity already matches embedded package $expectedRuntimeVersion",
            )
            result.success(buildSnapshotResponse())
            return
        }

        when (val source = runtimePackageSource.materialize()) {
            is RuntimePackageSourceResult.Failed -> {
                emitRuntimeLog(
                    "ERROR",
                    "Runtime package preparation failed [${source.code.name}]: ${source.message}",
                )
                result.error(
                    source.code.name,
                    source.message,
                    null,
                )
            }
            is RuntimePackageSourceResult.Ready -> {
                val trustedRef = source.reference
                continueReconcileEmbedded(trustedRef, result, 0)
            }
        }
    }

    private fun continueReconcileEmbedded(
        trustedRef: RuntimePackageReference,
        result: MethodChannel.Result,
        attempt: Int,
    ) {
        val snapshot = controller.snapshot()
        when (snapshot.state) {
            RuntimeState.READY,
            RuntimeState.DEGRADED,
            RuntimeState.STARTING -> {
                controller.stop(
                    RuntimeStopRequest(
                        reason = RuntimeStopReason.INSTALL_REPLACEMENT,
                        force = false,
                    ),
                    object : RuntimeOperationCallback {
                        override fun onCompleted(operationResult: RuntimeOperationResult) {
                            if (operationResult !is RuntimeOperationResult.Success) {
                                handleOperationResult(operationResult, result)
                                return
                            }
                            scheduleReconcileEmbedded(trustedRef, result, attempt + 1)
                        }
                    },
                )
            }
            RuntimeState.INSTALLED,
            RuntimeState.STOPPED,
            RuntimeState.CORRUPTED,
            RuntimeState.FAILED -> repairEmbeddedRuntime(trustedRef, result, attempt)
            RuntimeState.NOT_INSTALLED -> installEmbeddedRuntime(trustedRef, result, attempt)
            RuntimeState.UNKNOWN,
            RuntimeState.INSTALLING,
            RuntimeState.VERIFYING,
            RuntimeState.REPAIRING,
            RuntimeState.STOPPING -> scheduleReconcileEmbedded(
                trustedRef,
                result,
                attempt + 1,
            )
        }
    }

    private fun scheduleReconcileEmbedded(
        trustedRef: RuntimePackageReference,
        result: MethodChannel.Result,
        attempt: Int,
    ) {
        if (attempt > RECONCILE_STABLE_STATE_MAX_ATTEMPTS) {
            result.error(
                "RUNTIME_BUSY",
                "runtime did not reach a reconcile-safe state",
                null,
            )
            return
        }
        Handler(Looper.getMainLooper()).postDelayed(
            {
                continueReconcileEmbedded(trustedRef, result, attempt)
            },
            RECONCILE_STABLE_STATE_POLL_MS,
        )
    }

    private fun installEmbeddedRuntime(
        trustedRef: RuntimePackageReference,
        result: MethodChannel.Result,
        attempt: Int,
    ) {
        emitRuntimeLog(
            "INFO",
            "Installing embedded Runtime ${trustedRef.expectedRuntimeVersion}",
        )
        val request = RuntimeInstallRequest(
            packageUri = trustedRef.packageFile.absolutePath,
            expectedVersion = trustedRef.expectedRuntimeVersion,
            allowRepairExisting = true,
        )
        controller.install(request, object : RuntimeOperationCallback {
            override fun onCompleted(operationResult: RuntimeOperationResult) {
                if (retryTransientReconcileFailure(operationResult, trustedRef, result, attempt)) {
                    return
                }
                handleOperationResult(operationResult, result)
            }
        })
    }

    private fun repairEmbeddedRuntime(
        trustedRef: RuntimePackageReference,
        result: MethodChannel.Result,
        attempt: Int,
    ) {
        emitRuntimeLog(
            "INFO",
            "Repairing embedded Runtime ${trustedRef.expectedRuntimeVersion}",
        )
        val request = RuntimeRepairRequest(
            packageUri = trustedRef.packageFile.absolutePath,
            preserveUserData = true,
            expectedVersion = trustedRef.expectedRuntimeVersion,
        )
        controller.repair(request, object : RuntimeOperationCallback {
            override fun onCompleted(operationResult: RuntimeOperationResult) {
                if (retryTransientReconcileFailure(operationResult, trustedRef, result, attempt)) {
                    return
                }
                handleOperationResult(operationResult, result)
            }
        })
    }

    private fun retryTransientReconcileFailure(
        operationResult: RuntimeOperationResult,
        trustedRef: RuntimePackageReference,
        result: MethodChannel.Result,
        attempt: Int,
    ): Boolean {
        val failure = operationResult as? RuntimeOperationResult.Failure ?: return false
        if (failure.error.code !in setOf(
                RuntimeErrorCode.INVALID_STATE,
                RuntimeErrorCode.OPERATION_ALREADY_RUNNING,
                RuntimeErrorCode.RUNTIME_EXECUTION_NOT_AVAILABLE,
            )
        ) {
            return false
        }
        scheduleReconcileEmbedded(trustedRef, result, attempt + 1)
        return true
    }

    private fun handleVerify(result: MethodChannel.Result) {
        emitRuntimeLog("INFO", "Runtime verification requested")
        val request = RuntimeVerifyRequest(deep = false)
        controller.verify(request, object : RuntimeOperationCallback {
            override fun onCompleted(operationResult: RuntimeOperationResult) {
                handleOperationResult(operationResult, result)
            }
        })
    }

    private fun handleRepair(result: MethodChannel.Result) {
        emitRuntimeLog("INFO", "Runtime repair requested")
        when (val source = runtimePackageSource.materialize()) {
            is RuntimePackageSourceResult.Failed -> {
                emitRuntimeLog(
                    "ERROR",
                    "Runtime package preparation failed [${source.code.name}]: ${source.message}",
                )
                result.error(
                    source.code.name,
                    source.message,
                    null,
                )
            }
            is RuntimePackageSourceResult.Ready -> {
                val trustedRef = source.reference
                emitRuntimeLog(
                    "INFO",
                    "Runtime package prepared for repair",
                )
                val request = RuntimeRepairRequest(
                    packageUri = trustedRef.packageFile.absolutePath,
                    preserveUserData = true,
                    expectedVersion = trustedRef.expectedRuntimeVersion,
                )
                controller.repair(request, object : RuntimeOperationCallback {
                    override fun onCompleted(operationResult: RuntimeOperationResult) {
                        handleOperationResult(operationResult, result)
                    }
                })
            }
        }
    }

    private fun handleManifestSummary(result: MethodChannel.Result) {
        val manifestResult = manifestStore?.read()
        if (manifestResult is RuntimeManifestResult.Success) {
            val manifest = manifestResult.manifest
            val snapshot = controller.snapshot()
            val mapped = RuntimeBridgeSnapshotMapper.toBridgeSnapshot(
                snapshot = snapshot,
                manifest = manifest,
                runtimeInstalled = true,
                runtimeAvailable = snapshot.state == RuntimeState.READY ||
                    snapshot.state == RuntimeState.DEGRADED,
            )
            val manifestMap = mapped["manifest"] as? Map<String, Any?>
            result.success(manifestMap)
        } else {
            result.success(null)
        }
    }

    private fun handleOperationResult(
        operationResult: RuntimeOperationResult,
        result: MethodChannel.Result,
    ) {
        when (operationResult) {
            is RuntimeOperationResult.Success -> {
                emitRuntimeLog(
                    "INFO",
                    "Runtime ${operationResult.type.name.lowercase()} completed",
                )
            }
            is RuntimeOperationResult.Failure -> {
                emitRuntimeLog(
                    "ERROR",
                    "Runtime ${operationResult.type.name.lowercase()} failed [${operationResult.error.code.name}]: ${operationResult.error.message}",
                )
            }
            is RuntimeOperationResult.Cancelled -> {
                emitRuntimeLog(
                    "WARN",
                    "Runtime ${operationResult.type.name.lowercase()} cancelled",
                )
            }
        }
        val response = buildSnapshotResponse()
        response["accepted"] = when (operationResult) {
            is RuntimeOperationResult.Success -> true
            is RuntimeOperationResult.Failure -> false
            is RuntimeOperationResult.Cancelled -> false
        }
        if (operationResult is RuntimeOperationResult.Failure) {
            response["error"] = RuntimeBridgeErrorMapper.mapToBridgeError(operationResult.error)
        }
        result.success(response)
    }

    private fun buildSnapshotResponse(): LinkedHashMap<String, Any?> {
        val snapshot = controller.snapshot()
        val manifest = manifestStore?.read()
        val runtimeInstalled = manifest is RuntimeManifestResult.Success
        val runtimeAvailable = snapshot.state == RuntimeState.READY ||
                snapshot.state == RuntimeState.DEGRADED
        val mappedSnapshot = RuntimeBridgeSnapshotMapper.toBridgeSnapshot(
            snapshot = snapshot,
            manifest = (manifest as? RuntimeManifestResult.Success)?.manifest,
            runtimeInstalled = runtimeInstalled,
            runtimeAvailable = runtimeAvailable,
        )
        val response = LinkedHashMap<String, Any?>()
        response["accepted"] = true
        response["snapshot"] = mappedSnapshot
        return response
    }

    private fun emitRuntimeLog(level: String, message: String) {
        when (level) {
            "ERROR" -> android.util.Log.e(BRIDGE_LOG_TAG, message)
            "WARN" -> android.util.Log.w(BRIDGE_LOG_TAG, message)
            else -> android.util.Log.i(BRIDGE_LOG_TAG, message)
        }
        try {
            RuntimeLogCallback.instance?.onLog(level, message)
        } catch (_: Throwable) {
        }
    }

    private companion object {
        const val BRIDGE_LOG_TAG = "AmitiaRuntime"
        const val RECONCILE_STABLE_STATE_POLL_MS = 250L
        const val RECONCILE_STABLE_STATE_MAX_ATTEMPTS = 240
    }
}
