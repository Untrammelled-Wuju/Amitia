package com.amitia.amitia_app.nativeprovider.accessibility

import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.content.ServiceConnection
import android.os.IBinder
import com.amitia.amitia_app.accessibility.IAmitiaAccessibilityProvider
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import kotlinx.coroutines.withTimeoutOrNull

internal object AccessibilityProviderClient {

    private const val PROVIDER_PACKAGE = "com.amitia.amitia_app.accessibility"
    private const val PROVIDER_SERVICE = "com.amitia.amitia_app.accessibility.RemoteBinderService"
    private const val BIND_TIMEOUT_MS = 2500L

    private val mutex = Mutex()
    private var binder: IAmitiaAccessibilityProvider? = null
    private var pending: CompletableDeferred<IAmitiaAccessibilityProvider>? = null
    private var bound = false

    private val connection = object : ServiceConnection {
        override fun onServiceConnected(name: ComponentName?, service: IBinder?) {
            val provider = IAmitiaAccessibilityProvider.Stub.asInterface(service)
            binder = provider
            pending?.complete(provider)
            pending = null
        }

        override fun onServiceDisconnected(name: ComponentName?) {
            binder = null
            pending?.completeExceptionally(IllegalStateException("accessibility provider disconnected"))
            pending = null
        }
    }

    fun isProviderReference(nativeRef: String): Boolean =
        nativeRef.startsWith("api:")

    fun isProviderPackage(packageName: String): Boolean =
        packageName == PROVIDER_PACKAGE

    suspend fun status(context: Context): String? =
        call(context) { it.status() }

    suspend fun snapshot(context: Context, payloadJson: String): String? =
        call(context) { it.snapshot(payloadJson) }

    suspend fun performNodeAction(context: Context, payloadJson: String): String? =
        call(context) { it.performNodeAction(payloadJson) }

    suspend fun click(context: Context, x: Int, y: Int): String? =
        call(context) { it.performClick(x, y) }

    suspend fun longPress(context: Context, x: Int, y: Int, durationMs: Long): String? =
        call(context) { it.performLongPress(x, y, durationMs) }

    suspend fun swipe(
        context: Context,
        startX: Int,
        startY: Int,
        endX: Int,
        endY: Int,
        durationMs: Long,
    ): String? = call(context) { it.performSwipe(startX, startY, endX, endY, durationMs) }

    suspend fun globalAction(context: Context, actionId: Int): String? =
        call(context) { it.performGlobalAction(actionId) }

    suspend fun isConnected(context: Context): Boolean {
        val status = status(context) ?: return false
        return status.contains("\"connected\":true")
    }

    private suspend fun call(
        context: Context,
        block: (IAmitiaAccessibilityProvider) -> String,
    ): String? {
        val provider = provider(context) ?: return null
        return withContext(Dispatchers.IO) {
            try {
                block(provider)
            } catch (_: Throwable) {
                null
            }
        }
    }

    private suspend fun provider(context: Context): IAmitiaAccessibilityProvider? {
        binder?.let { return it }
        return mutex.withLock {
            binder?.let { return@withLock it }
            val deferred = pending ?: CompletableDeferred<IAmitiaAccessibilityProvider>().also {
                pending = it
                bind(context.applicationContext)
            }
            withTimeoutOrNull(BIND_TIMEOUT_MS) { deferred.await() } ?: run {
                if (pending === deferred) pending = null
                null
            }
        }
    }

    private fun bind(context: Context) {
        if (bound) return
        val intent = Intent().setComponent(ComponentName(PROVIDER_PACKAGE, PROVIDER_SERVICE))
        try {
            bound = context.bindService(intent, connection, Context.BIND_AUTO_CREATE)
        } catch (_: Throwable) {
            bound = false
            pending?.completeExceptionally(IllegalStateException("accessibility provider bind failed"))
            pending = null
        }
    }
}
