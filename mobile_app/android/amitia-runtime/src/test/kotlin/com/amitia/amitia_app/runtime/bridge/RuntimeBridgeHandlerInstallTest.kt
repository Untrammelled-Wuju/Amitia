package com.amitia.amitia_app.runtime.bridge

import com.amitia.amitia_app.runtime.api.RuntimeController
import com.amitia.amitia_app.runtime.api.RuntimeInstallRequest
import com.amitia.amitia_app.runtime.api.RuntimeOperationCallback
import com.amitia.amitia_app.runtime.api.RuntimeOperationHandle
import com.amitia.amitia_app.runtime.api.RuntimeOperationResult
import com.amitia.amitia_app.runtime.api.RuntimeOperationType
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
import com.amitia.amitia_app.runtime.packagetrusted.TrustedRuntimePackageSource
import io.flutter.plugin.common.MethodChannel
import io.flutter.plugin.common.MethodCall
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Test

class RuntimeBridgeHandlerInstallTest {

    private class FakeController : RuntimeController {
        var installRequest: RuntimeInstallRequest? = null
        var repairRequest: RuntimeRepairRequest? = null

        override fun snapshot() = com.amitia.amitia_app.runtime.api.RuntimeSnapshot(
            state = RuntimeState.NOT_INSTALLED,
            generation = 0,
            lastError = null,
        )

        override fun subscribe(listener: com.amitia.amitia_app.runtime.api.RuntimeListener) =
            object : com.amitia.amitia_app.runtime.api.RuntimeSubscription {
                override fun cancel() {}
            }

        override fun install(request: RuntimeInstallRequest, callback: RuntimeOperationCallback): RuntimeOperationHandle {
            installRequest = request
            callback.onCompleted(RuntimeOperationResult.Success("op-1", RuntimeOperationType.INSTALL, snapshot()))
            return object : RuntimeOperationHandle {
                override val operationId = "op-1"
                override val type = RuntimeOperationType.INSTALL
                override fun cancel() = false
                override fun isCancelled() = false
                override fun isCompleted() = true
            }
        }

        override fun verify(request: RuntimeVerifyRequest, callback: RuntimeOperationCallback): RuntimeOperationHandle {
            callback.onCompleted(RuntimeOperationResult.Success("op-2", RuntimeOperationType.VERIFY, snapshot()))
            return object : RuntimeOperationHandle {
                override val operationId = "op-2"
                override val type = RuntimeOperationType.VERIFY
                override fun cancel() = false
                override fun isCancelled() = false
                override fun isCompleted() = true
            }
        }

        override fun start(request: RuntimeStartRequest, callback: RuntimeOperationCallback): RuntimeOperationHandle {
            callback.onCompleted(RuntimeOperationResult.Success("op-3", RuntimeOperationType.START, snapshot()))
            return object : RuntimeOperationHandle {
                override val operationId = "op-3"
                override val type = RuntimeOperationType.START
                override fun cancel() = false
                override fun isCancelled() = false
                override fun isCompleted() = true
            }
        }

        override fun stop(request: RuntimeStopRequest, callback: RuntimeOperationCallback): RuntimeOperationHandle {
            callback.onCompleted(RuntimeOperationResult.Success("op-4", RuntimeOperationType.STOP, snapshot()))
            return object : RuntimeOperationHandle {
                override val operationId = "op-4"
                override val type = RuntimeOperationType.STOP
                override fun cancel() = false
                override fun isCancelled() = false
                override fun isCompleted() = true
            }
        }

        override fun repair(request: RuntimeRepairRequest, callback: RuntimeOperationCallback): RuntimeOperationHandle {
            repairRequest = request
            callback.onCompleted(RuntimeOperationResult.Success("op-5", RuntimeOperationType.REPAIR, snapshot()))
            return object : RuntimeOperationHandle {
                override val operationId = "op-5"
                override val type = RuntimeOperationType.REPAIR
                override fun cancel() = false
                override fun isCancelled() = false
                override fun isCompleted() = true
            }
        }
    }

    private class FakeManifestStore : RuntimeManifestStore {
        override fun read(): RuntimeManifestResult {
            return RuntimeManifestResult.failure(
                com.amitia.amitia_app.runtime.manifest.RuntimeManifestError(
                    com.amitia.amitia_app.runtime.manifest.RuntimeManifestErrorCode.MANIFEST_NOT_FOUND,
                    "not found"
                )
            )
        }

        override fun write(manifest: com.amitia.amitia_app.runtime.manifest.RuntimeManifest): RuntimeManifestResult {
            return RuntimeManifestResult.failure(
                com.amitia.amitia_app.runtime.manifest.RuntimeManifestError(
                    com.amitia.amitia_app.runtime.manifest.RuntimeManifestErrorCode.MANIFEST_WRITE_FAILED,
                    "not implemented"
                )
            )
        }

        override fun delete(): RuntimeManifestResult {
            return RuntimeManifestResult.failure(
                com.amitia.amitia_app.runtime.manifest.RuntimeManifestError(
                    com.amitia.amitia_app.runtime.manifest.RuntimeManifestErrorCode.MANIFEST_WRITE_FAILED,
                    "not implemented"
                )
            )
        }
    }

    private fun createHandler(
        controller: RuntimeController = FakeController(),
        manifestStore: RuntimeManifestStore? = FakeManifestStore(),
    ): RuntimeBridgeHandler {
        return RuntimeBridgeHandler(
            controller = controller,
            backendConnectionProvider = object : BackendConnectionProvider {
                override fun current() = BackendConnectionAvailability.Unavailable(null)
            },
            manifestStore = manifestStore,
        )
    }

    @Test
    fun handleInstall_doesNotUseEmptyPackageUri() {
        val controller = FakeController()
        val handler = createHandler(controller = controller)
        val result = FakeMethodChannelResult()

        handler.onMethodCall(
            MethodCall("runtime.install", null),
            result,
        )

        assertNotNull(controller.installRequest)
        assertFalse(
            "Bridge install must NOT use empty packageUri",
            controller.installRequest!!.packageUri.isEmpty()
        )
    }

    @Test
    fun handleInstall_usesTrustedRuntimePackageSource() {
        val controller = FakeController()
        val handler = createHandler(controller = controller)
        val result = FakeMethodChannelResult()

        handler.onMethodCall(
            MethodCall("runtime.install", null),
            result,
        )

        val request = controller.installRequest!!
        assertTrue(
            "Bridge install packageUri must contain runtime version from TrustedRuntimePackageSource",
            request.packageUri.contains(TrustedRuntimePackageSource.RUNTIME_VERSION)
        )
        assertEquals(
            "Bridge install expectedVersion must match TrustedRuntimePackageSource",
            TrustedRuntimePackageSource.expectedRuntimeVersion(),
            request.expectedVersion
        )
    }

    @Test
    fun handleRepair_usesTrustedPackageSource() {
        val controller = FakeController()
        val handler = createHandler(controller = controller)
        val result = FakeMethodChannelResult()

        handler.onMethodCall(
            MethodCall("runtime.repair", null),
            result,
        )

        val request = controller.repairRequest!!
        assertTrue(
            "Bridge repair packageUri must contain runtime version from TrustedRuntimePackageSource",
            request.packageUri.contains(TrustedRuntimePackageSource.RUNTIME_VERSION)
        )
    }

    @Test
    fun handleManifestSummary_usesManifestStore() {
        val handler = createHandler(manifestStore = FakeManifestStore())
        val result = FakeMethodChannelResult()

        handler.onMethodCall(
            MethodCall("runtime.manifestSummary", null),
            result,
        )

        assertTrue(result.receivedNull)
    }

    private class FakeMethodChannelResult : MethodChannel.Result {
        var receivedNull = false
        var.successValue: Any? = null
        var errorCode: String? = null

        override fun success(result: Any?) {
            successValue = result
            if (result == null) receivedNull = true
        }

        override fun error(errorCode: String, errorMessage: String?, errorDetails: Any?) {
            this.errorCode = errorCode
        }

        override fun notImplemented() {
            errorCode = "NOT_IMPLEMENTED"
        }
    }
}
