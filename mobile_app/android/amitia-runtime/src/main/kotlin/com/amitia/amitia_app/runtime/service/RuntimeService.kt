package com.amitia.amitia_app.runtime.service

import android.app.Notification
import android.app.Service
import android.content.Context
import android.content.Intent
import android.os.IBinder
import androidx.core.app.NotificationCompat
import androidx.core.app.ServiceCompat
import androidx.core.content.ContextCompat
import com.amitia.amitia_app.runtime.service.internal.DefaultRuntimeServiceEndpoint
import com.amitia.amitia_app.runtime.service.internal.RuntimeForegroundNotification
import com.amitia.amitia_app.runtime.service.internal.RuntimeForegroundNotificationResult
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicReference
import java.util.concurrent.locks.ReentrantLock
import kotlin.concurrent.withLock

class RuntimeService : Service() {
    private val lock = ReentrantLock()
    private val destroyed = AtomicBoolean(false)
    private val serviceState = AtomicReference(ServiceHostState.CREATED)
    private val expectedStopRequested = AtomicBoolean(false)
    private val notificationManager by lazy { RuntimeForegroundNotification(this) }

    private val endpoint by lazy { DefaultRuntimeServiceEndpoint { this@RuntimeService } }
    private val binder by lazy { RuntimeServiceBinder(endpoint) }

    override fun onCreate() {
        super.onCreate()
        lock.withLock {
            destroyed.set(false)
            serviceState.set(ServiceHostState.CREATED)
        }
    }

    override fun onBind(intent: Intent?): IBinder {
        return binder
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (intent == null) {
            stopSelfSafely()
            return START_NOT_STICKY
        }

        when (intent.action) {
            RuntimeServiceContract.ACTION_START_HOST -> handleStartHost()
            RuntimeServiceContract.ACTION_STOP_HOST -> handleStopHost()
            else -> {
                stopSelfSafely()
                return START_NOT_STICKY
            }
        }

        return START_NOT_STICKY
    }

    private fun handleStartHost() {
        if (!serviceState.compareAndSet(ServiceHostState.CREATED, ServiceHostState.FOREGROUND)) {
            return
        }

        when (val notificationResult = notificationManager.createNotification()) {
            is RuntimeForegroundNotificationResult.Success -> {
                try {
                    ServiceCompat.startForeground(
                        this,
                        RuntimeServiceContract.NOTIFICATION_ID,
                        notificationResult.notification,
                        RuntimeServiceContract.FOREGROUND_SERVICE_TYPE
                    )
                    endpoint.notify(RuntimeServiceHostEvent.ForegroundStarted)
                } catch (e: Exception) {
                    serviceState.set(ServiceHostState.DESTROYED)
                    endpoint.notify(
                        RuntimeServiceHostEvent.UnexpectedTermination(
                            RuntimeServiceTerminationCause.FOREGROUND_FAILED
                        )
                    )
                    stopSelfSafely()
                }
            }
            is RuntimeForegroundNotificationResult.Failure -> {
                serviceState.set(ServiceHostState.DESTROYED)
                endpoint.notify(
                    RuntimeServiceHostEvent.UnexpectedTermination(
                        RuntimeServiceTerminationCause.NOTIFICATION_FAILED
                    )
                )
                stopSelfSafely()
            }
        }
    }

    private fun handleStopHost() {
        expectedStopRequested.set(true)
        stopForegroundSafely()
        stopSelfSafely()
    }

    private fun stopForegroundSafely() {
        try {
            stopForeground(STOP_FOREGROUND_REMOVE)
        } catch (_: Exception) {
        }
    }

    private fun stopSelfSafely() {
        try {
            stopSelf()
        } catch (_: Exception) {
        }
    }

    override fun onDestroy() {
        val wasExpected = expectedStopRequested.get()
        lock.withLock {
            destroyed.set(true)
            serviceState.set(ServiceHostState.DESTROYED)
        }
        if (wasExpected) {
            endpoint.notify(RuntimeServiceHostEvent.ExpectedStopped)
        } else {
            endpoint.notify(
                RuntimeServiceHostEvent.UnexpectedTermination(
                    RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR
                )
            )
        }
        super.onDestroy()
    }

    override fun onTaskRemoved(rootIntent: Intent?) {
        super.onTaskRemoved(rootIntent)
    }

    internal fun snapshot(): RuntimeServiceSnapshot {
        return RuntimeServiceSnapshot(
            created = serviceState.get() != ServiceHostState.DESTROYED,
            foreground = serviceState.get() == ServiceHostState.FOREGROUND,
            boundClients = 0
        )
    }

    private fun handleHostEvent(event: RuntimeServiceHostEvent) {
        lock.withLock {
            when (event) {
                is RuntimeServiceHostEvent.ForegroundStarted -> {
                    if (serviceState.get() != ServiceHostState.DESTROYED) {
                        serviceState.set(ServiceHostState.FOREGROUND)
                    }
                }
                is RuntimeServiceHostEvent.ExpectedStopped -> {
                    destroyed.set(true)
                    serviceState.set(ServiceHostState.DESTROYED)
                }
                is RuntimeServiceHostEvent.UnexpectedTermination -> {
                    destroyed.set(true)
                    serviceState.set(ServiceHostState.DESTROYED)
                }
            }
        }
    }

    private enum class ServiceHostState {
        CREATED,
        FOREGROUND,
        DESTROYED
    }

    internal companion object {
        internal fun startHost(context: Context): RuntimeServiceResult {
            return try {
                val intent = Intent(context, RuntimeService::class.java).apply {
                    action = RuntimeServiceContract.ACTION_START_HOST
                }
                ContextCompat.startForegroundService(context, intent)
                RuntimeServiceResult.Success
            } catch (e: Exception) {
                RuntimeServiceResult.Failure(
                    RuntimeServiceError(
                        code = RuntimeServiceErrorCode.SERVICE_START_FAILED,
                        message = "failed to start runtime service: ${e.message}",
                        cause = e
                    )
                )
            }
        }

        internal fun stopHost(context: Context): RuntimeServiceResult {
            return try {
                val intent = Intent(context, RuntimeService::class.java).apply {
                    action = RuntimeServiceContract.ACTION_STOP_HOST
                }
                context.startService(intent)
                RuntimeServiceResult.Success
            } catch (e: Exception) {
                RuntimeServiceResult.Failure(
                    RuntimeServiceError(
                        code = RuntimeServiceErrorCode.SERVICE_STOP_FAILED,
                        message = "failed to stop runtime service: ${e.message}",
                        cause = e
                    )
                )
            }
        }
    }
}

internal data class RuntimeServiceSnapshot(
    val created: Boolean,
    val foreground: Boolean,
    val boundClients: Int
)
