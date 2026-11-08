package com.amitia.amitia_app.runtime.bridge

import com.amitia.amitia_app.runtime.api.RuntimeController
import com.amitia.amitia_app.runtime.api.RuntimeInstallRequest
import com.amitia.amitia_app.runtime.api.RuntimeListener
import com.amitia.amitia_app.runtime.api.RuntimeOperationCallback
import com.amitia.amitia_app.runtime.api.RuntimeOperationHandle
import com.amitia.amitia_app.runtime.api.RuntimeRepairRequest
import com.amitia.amitia_app.runtime.api.RuntimeSnapshot
import com.amitia.amitia_app.runtime.api.RuntimeStartRequest
import com.amitia.amitia_app.runtime.api.RuntimeStopRequest
import com.amitia.amitia_app.runtime.api.RuntimeSubscription
import com.amitia.amitia_app.runtime.api.RuntimeVerifyRequest
import com.amitia.amitia_app.runtime.connection.BackendConnectionAvailability
import com.amitia.amitia_app.runtime.connection.BackendConnectionProvider
import com.amitia.amitia_app.runtime.packagetrusted.RuntimePackageSource
import com.amitia.amitia_app.runtime.packagetrusted.RuntimePackageSourceErrorCode
import com.amitia.amitia_app.runtime.packagetrusted.RuntimePackageSourceResult
import io.flutter.plugin.common.MethodCall
import io.flutter.plugin.common.MethodChannel
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicReference

class RuntimeBridgeHandlerWorkTest {

    @Test
    fun installPackagePreparationDoesNotBlockCallerThread() {
        val entered = CountDownLatch(1)
        val release = CountDownLatch(1)
        val completed = CountDownLatch(1)
        val workThread = AtomicReference<String>()
        val failure = AtomicReference<String>()
        val source = object : RuntimePackageSource {
            override fun materialize(): RuntimePackageSourceResult {
                workThread.set(Thread.currentThread().name)
                entered.countDown()
                release.await(2, TimeUnit.SECONDS)
                return RuntimePackageSourceResult.Failed(
                    RuntimePackageSourceErrorCode.PACKAGE_MATERIALIZE_FAILED,
                    "preparation failed",
                )
            }
        }
        val handler = handler(source)
        val startedAt = System.nanoTime()

        handler.onMethodCall(MethodCall(RuntimeBridgeContract.METHOD_INSTALL, null), result { code ->
            failure.set(code)
            completed.countDown()
        })

        val elapsedMillis = TimeUnit.NANOSECONDS.toMillis(System.nanoTime() - startedAt)
        assertTrue("install dispatch blocked caller for ${elapsedMillis}ms", elapsedMillis < 1000)
        assertTrue("package work did not start", entered.await(1, TimeUnit.SECONDS))
        assertTrue("package work used caller thread", workThread.get() != Thread.currentThread().name)
        assertEquals("RuntimeBridgeWork", workThread.get())

        release.countDown()
        assertTrue("failure result did not complete", completed.await(2, TimeUnit.SECONDS))
        assertEquals(RuntimePackageSourceErrorCode.PACKAGE_MATERIALIZE_FAILED.name, failure.get())
    }

    @Test
    fun packagePreparationExceptionPreservesOriginalError() {
        val completed = CountDownLatch(1)
        val receivedCode = AtomicReference<String>()
        val receivedMessage = AtomicReference<String>()
        val receivedDetails = AtomicReference<String>()
        val source = object : RuntimePackageSource {
            override fun materialize(): RuntimePackageSourceResult {
                throw IllegalStateException("materialize exploded")
            }
        }
        val handler = handler(source)

        handler.onMethodCall(
            MethodCall(RuntimeBridgeContract.METHOD_REPAIR, null),
            object : MethodChannel.Result {
                override fun success(result: Any?) = Unit

                override fun error(errorCode: String, errorMessage: String?, errorDetails: Any?) {
                    receivedCode.set(errorCode)
                    receivedMessage.set(errorMessage.orEmpty())
                    receivedDetails.set(errorDetails?.toString().orEmpty())
                    completed.countDown()
                }

                override fun notImplemented() = Unit
            },
        )

        assertTrue("exception result did not complete", completed.await(2, TimeUnit.SECONDS))
        assertEquals("INTERNAL_ERROR", receivedCode.get())
        assertTrue(receivedMessage.get().contains("materialize exploded"))
        assertTrue(receivedDetails.get().contains("IllegalStateException"))
        assertTrue(receivedDetails.get().contains("materialize exploded"))
    }

    private fun handler(source: RuntimePackageSource): RuntimeBridgeHandler {
        return RuntimeBridgeHandler(
            controller = UnusedRuntimeController,
            backendConnectionProvider = UnusedBackendConnectionProvider,
            manifestStore = null,
            runtimePackageSource = source,
            mainDispatcher = { block -> block() },
            logEmitter = { _, _ -> Unit },
        )
    }

    private fun result(onError: (String) -> Unit): MethodChannel.Result {
        return object : MethodChannel.Result {
            override fun success(result: Any?) = Unit

            override fun error(errorCode: String, errorMessage: String?, errorDetails: Any?) {
                onError(errorCode)
            }

            override fun notImplemented() = Unit
        }
    }

    private object UnusedBackendConnectionProvider : BackendConnectionProvider {
        override fun current(): BackendConnectionAvailability =
            error("backend connection provider must not be used")
    }

    private object UnusedRuntimeController : RuntimeController {
        override fun snapshot(): RuntimeSnapshot = error("controller must not be used")

        override fun subscribe(listener: RuntimeListener): RuntimeSubscription =
            error("controller must not be used")

        override fun install(
            request: RuntimeInstallRequest,
            callback: RuntimeOperationCallback,
        ): RuntimeOperationHandle = error("controller must not be used")

        override fun verify(
            request: RuntimeVerifyRequest,
            callback: RuntimeOperationCallback,
        ): RuntimeOperationHandle = error("controller must not be used")

        override fun start(
            request: RuntimeStartRequest,
            callback: RuntimeOperationCallback,
        ): RuntimeOperationHandle = error("controller must not be used")

        override fun stop(
            request: RuntimeStopRequest,
            callback: RuntimeOperationCallback,
        ): RuntimeOperationHandle = error("controller must not be used")

        override fun repair(
            request: RuntimeRepairRequest,
            callback: RuntimeOperationCallback,
        ): RuntimeOperationHandle = error("controller must not be used")
    }
}
