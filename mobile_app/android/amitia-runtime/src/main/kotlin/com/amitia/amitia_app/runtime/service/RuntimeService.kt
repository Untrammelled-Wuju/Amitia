package com.amitia.amitia_app.runtime.service

import android.app.Service
import android.content.Context
import android.content.Intent
import android.os.IBinder
import androidx.core.app.ServiceCompat
import androidx.core.content.ContextCompat
import com.amitia.amitia_app.runtime.AndroidRuntimeModule
import com.amitia.amitia_app.runtime.service.internal.DefaultRuntimeServiceEndpoint
import com.amitia.amitia_app.runtime.service.internal.RuntimeForegroundNotification
import com.amitia.amitia_app.runtime.service.internal.RuntimeForegroundNotificationResult
import com.amitia.amitia_app.runtime.proot.ProotEnvironment
import com.amitia.amitia_app.runtime.proot.ProotEvent
import com.amitia.amitia_app.runtime.proot.ProotLaunchRequest
import com.amitia.amitia_app.runtime.proot.ProotObserver
import com.amitia.amitia_app.runtime.proot.ProotSession
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicLong
import java.util.concurrent.atomic.AtomicReference
import java.util.concurrent.locks.ReentrantLock
import kotlin.concurrent.withLock

class RuntimeService : Service() {
    private val lock = ReentrantLock()
    private val destroyed = AtomicBoolean(false)
    private val serviceState = AtomicReference(ServiceHostState.CREATED)
    private val expectedStopRequested = AtomicBoolean(false)
    private val terminalEventEmitted = AtomicBoolean(false)
    private val notificationManager by lazy { RuntimeForegroundNotification(this) }

    private val endpoint by lazy { DefaultRuntimeServiceEndpoint { this@RuntimeService } }
    private val binder by lazy { RuntimeServiceBinder(endpoint) }

    private val sessionGeneration = AtomicLong(0L)
    private val currentSessionRef: AtomicReference<ProotSession?> = AtomicReference(null)
    private val currentSessionIdRef: AtomicReference<String?> = AtomicReference(null)

    init {
        instanceRef.set(this)
    }

    override fun onCreate() {
        super.onCreate()
        instanceRef.set(this)
        lock.withLock {
            destroyed.set(false)
            serviceState.set(ServiceHostState.CREATED)
            sessionGeneration.set(0L)
            currentSessionRef.set(null)
            currentSessionIdRef.set(null)
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
                    startProotSessionLocked()
                } catch (e: Exception) {
                    terminalEventEmitted.compareAndSet(false, true)
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
                terminalEventEmitted.compareAndSet(false, true)
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

    private fun startProotSessionLocked() {
        val component = AndroidRuntimeModule.prootComponent ?: return
        val rootfsPath = AndroidRuntimeModule.prootRootfsPath ?: return
        val nextGen = sessionGeneration.incrementAndGet()
        val request = ProotLaunchRequest.create(
            rootfsPath = rootfsPath,
            workingDirectory = WORKING_DIRECTORY,
            command = listOf(GUEST_SERVER_COMMAND),
            bindMountsSource = emptyList(),
            environmentSource = ProotEnvironment.EMPTY
        )
        val observer = ProotObserver { event -> onProotEvent(event, nextGen) }
        val session = component.launch(request, observer)
        currentSessionRef.set(session)
        currentSessionIdRef.set(session.sessionId)
    }

    private fun onProotEvent(event: ProotEvent, generation: Long) {
        if (sessionGeneration.get() != generation) return
        when (event) {
            is ProotEvent.Started -> {
                if (sessionGeneration.get() == generation) {
                    endpoint.notify(
                        RuntimeServiceHostEvent.SessionReady(
                            generation = generation,
                            sessionId = event.sessionId
                        )
                    )
                }
            }
            is ProotEvent.Exited -> {
                lock.withLock {
                    if (destroyed.get()) return
                    if (serviceState.get() == ServiceHostState.DESTROYED) return
                    endpoint.notify(
                        RuntimeServiceHostEvent.SessionExited(
                            sessionId = event.sessionId,
                            exitCode = event.exitCode,
                            forced = event.forced
                        )
                    )
                    if (serviceState.get() != ServiceHostState.FOREGROUND) return
                    expectedStopRequested.set(true)
                    terminalEventEmitted.compareAndSet(false, true)
                    serviceState.set(ServiceHostState.DESTROYED)
                    endpoint.notify(
                        RuntimeServiceHostEvent.UnexpectedTermination(
                            RuntimeServiceTerminationCause.SESSION_EXITED
                        )
                    )
                    stopSelfSafely()
                }
            }
            else -> {}
        }
    }

    private fun handleStopHost() {
        runCatching {
            val session = currentSessionRef.get()
            if (session != null && session.isAlive()) {
                AndroidRuntimeModule.prootComponent?.stop()
            }
        }
        expectedStopRequested.set(true)
        terminalEventEmitted.compareAndSet(false, true)
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
        val wasTerminalAlreadyEmitted = terminalEventEmitted.get()
        val wasExpected = expectedStopRequested.get()
        lock.withLock {
            destroyed.set(true)
            serviceState.set(ServiceHostState.DESTROYED)
            currentSessionRef.set(null)
            currentSessionIdRef.set(null)
        }
        if (!wasTerminalAlreadyEmitted) {
            if (wasExpected) {
                endpoint.notify(RuntimeServiceHostEvent.ExpectedStopped)
            } else {
                endpoint.notify(
                    RuntimeServiceHostEvent.UnexpectedTermination(
                        RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR
                    )
                )
            }
        }
        instanceRef.compareAndSet(this, null)
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

    internal fun currentProotSession(): ProotSession? = currentSessionRef.get()

    private enum class ServiceHostState {
        CREATED,
        FOREGROUND,
        DESTROYED
    }

    internal companion object {
        private val instanceRef = AtomicReference<RuntimeService?>(null)

        fun currentSession(context: Context): ProotSession? {
            return instanceRef.get()?.currentProotSession()
        }

        fun currentGeneration(context: Context): Long {
            return instanceRef.get()?.sessionGeneration?.get() ?: 0L
        }

        private const val WORKING_DIRECTORY = "/opt/amitia/backend"
        private const val GUEST_SERVER_COMMAND = "/opt/amitia/backend/amitia-server"

        internal fun startHost(context: Context): RuntimeServiceResult {
            return try {
                val intent = Intent(context, RuntimeService::class.java).apply {
                    action = RuntimeServiceContract.ACTION_START_HOST
                }
                ContextCompat.startForegroundService(context, intent)
                RuntimeServiceResult.Success
            } catch (e: Exception) {
                instanceRef.get()?.let { svc ->
                    synchronized(svc) {
                        svc.destroyed.set(true)
                        svc.serviceState.set(ServiceHostState.DESTROYED)
                        svc.currentSessionRef.set(null)
                        svc.currentSessionIdRef.set(null)
                    }
                }
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
            val svc = instanceRef.get()
            if (svc != null) {
                svc.lock.withLock {
                    svc.expectedStopRequested.set(true)
                    svc.terminalEventEmitted.compareAndSet(false, true)
                }
            }
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

        fun clearInstanceRef() {
            instanceRef.set(null)
        }
    }
}

internal data class RuntimeServiceSnapshot(
    val created: Boolean,
    val foreground: Boolean,
    val boundClients: Int
)
