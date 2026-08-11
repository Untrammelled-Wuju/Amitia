package com.amitia.amitia_app.runtime.bridge

import com.amitia.amitia_app.runtime.api.RuntimeController
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
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestResult
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestStore
import io.flutter.plugin.common.MethodCall
import io.flutter.plugin.common.MethodChannel

internal class RuntimeBridgeHandler(
    private val controller: RuntimeController,
    private val backendConnectionProvider: BackendConnectionProvider,
    private val manifestStore: RuntimeManifestStore?,
) : MethodChannel.MethodCallHandler {

    override fun onMethodCall(call: MethodCall, result: MethodChannel.Result) {
        try {
            when (call.method) {
                RuntimeBridgeContract.METHOD_SNAPSHOT -> handleSnapshot(result)
                RuntimeBridgeContract.METHOD_START -> handleStart(result)
                RuntimeBridgeContract.METHOD_STOP -> handleStop(result)
                RuntimeBridgeContract.METHOD_INSTALL -> handleInstall(result)
                RuntimeBridgeContract.METHOD_VERIFY -> handleVerify(result)
                RuntimeBridgeContract.METHOD_REPAIR -> handleRepair(result)
                RuntimeBridgeContract.METHOD_MANIFEST_SUMMARY -> handleManifestSummary(result)
                RuntimeBridgeContract.METHOD_GET_BACKEND_CONNECTION -> handleGetBackendConnection(result)
                else -> result.notImplemented()
            }
        } catch (e: Exception) {
            result.error(
                "INTERNAL_ERROR",
                "Internal error: ${e.message}",
                null
            )
        }
    }

    private fun handleSnapshot(result: MethodChannel.Result) {
        val snapshot = controller.snapshot()
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
        val mapped = BackendConnectionMapper.toPayload(
            available = availability is BackendConnectionAvailability.Available,
            descriptor = (availability as? BackendConnectionAvailability.Available)?.descriptor,
            error = null,
        )
        result.success(mapped)
    }

    private fun handleStart(result: MethodChannel.Result) {
        val request = RuntimeStartRequest(reason = RuntimeStartReason.USER_REQUEST)
        controller.start(request, object : RuntimeOperationCallback {
            override fun onCompleted(operationResult: RuntimeOperationResult) {
                handleOperationResult(operationResult, result)
            }
        })
    }

    private fun handleStop(result: MethodChannel.Result) {
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
        val request = RuntimeInstallRequest(
            packageUri = "",
            expectedVersion = null,
            allowRepairExisting = false
        )
        controller.install(request, object : RuntimeOperationCallback {
            override fun onCompleted(operationResult: RuntimeOperationResult) {
                handleOperationResult(operationResult, result)
            }
        })
    }

    private fun handleVerify(result: MethodChannel.Result) {
        val request = RuntimeVerifyRequest(deep = false)
        controller.verify(request, object : RuntimeOperationCallback {
            override fun onCompleted(operationResult: RuntimeOperationResult) {
                handleOperationResult(operationResult, result)
            }
        })
    }

    private fun handleRepair(result: MethodChannel.Result) {
        val request = RuntimeRepairRequest(
            packageUri = null,
            preserveUserData = true
        )
        controller.repair(request, object : RuntimeOperationCallback {
            override fun onCompleted(operationResult: RuntimeOperationResult) {
                handleOperationResult(operationResult, result)
            }
        })
    }

    private fun handleManifestSummary(result: MethodChannel.Result) {
        val manifestResult = manifestStore?.read()
        if (manifestResult is RuntimeManifestResult.Success) {
            val manifest = manifestResult.manifest
            val mapped = RuntimeBridgeSnapshotMapper.toBridgeSnapshot(
                snapshot = controller.snapshot(),
                manifest = manifest,
                runtimeInstalled = true,
                runtimeAvailable = true,
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
        response["accepted"] = when (operationResult) {
            is RuntimeOperationResult.Success -> true
            is RuntimeOperationResult.Failure -> operationResult.error.recoverable
            is RuntimeOperationResult.Cancelled -> false
        }
        response["snapshot"] = mappedSnapshot
        if (operationResult is RuntimeOperationResult.Failure) {
            response["error"] = RuntimeBridgeErrorMapper.mapToBridgeError(operationResult.error)
        }
        result.success(response)
    }
}
