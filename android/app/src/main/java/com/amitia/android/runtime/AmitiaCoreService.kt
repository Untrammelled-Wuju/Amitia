package com.amitia.android.runtime

import android.app.Notification
import android.app.Service
import android.content.Intent
import android.content.pm.ServiceInfo
import android.os.Build
import android.os.IBinder
import com.amitia.core.logging.Logger
import com.amitia.platform.foreground.ForegroundServiceManagerImpl
import com.amitia.runtime.api.RuntimeState
import com.amitia.runtime.manager.RuntimeManager
import dagger.hilt.android.AndroidEntryPoint
import javax.inject.Inject
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.flow.collectLatest
import kotlinx.coroutines.launch

@AndroidEntryPoint
class AmitiaCoreService : Service() {

    @Inject
    lateinit var runtimeManager: RuntimeManager

    @Inject
    lateinit var foregroundServiceManager: ForegroundServiceManagerImpl

    @Inject
    lateinit var logger: Logger

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Default)
    private var stateJob: Job? = null

    override fun onCreate() {
        super.onCreate()
        logger.i(TAG, "AmitiaCoreService onCreate")
        startForegroundCompat(buildRuntimeNotification(initialSnapshot()))
        observeRuntimeState()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        logger.i(TAG, "AmitiaCoreService onStartCommand action=${intent?.action}")
        when (intent?.action) {
            ACTION_STOP_FOREGROUND -> {
                stopForegroundAndSelf()
                return START_NOT_STICKY
            }
            else -> {
                startForegroundCompat(buildRuntimeNotification(initialSnapshot()))
            }
        }
        return START_STICKY
    }

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onDestroy() {
        logger.i(TAG, "AmitiaCoreService onDestroy")
        stateJob?.cancel()
        scope.cancel()
        scope.launch {
            runCatching { runtimeManager.stop() }
        }
        super.onDestroy()
    }

    private fun observeRuntimeState() {
        stateJob?.cancel()
        stateJob = scope.launch {
            runtimeManager.observeState().collectLatest { state ->
                val snapshot = toSnapshot(state)
                foregroundServiceManager.updateNotification(snapshot)
                val notification = buildRuntimeNotification(snapshot)
                startForegroundCompat(notification)
            }
        }
    }

    private fun stopForegroundAndSelf() {
        logger.i(TAG, "stopForegroundAndSelf")
        stateJob?.cancel()
        scope.launch {
            runCatching { runtimeManager.stop() }
            stopForeground(STOP_FOREGROUND_REMOVE)
            stopSelf()
        }
    }

    private fun startForegroundCompat(notification: Notification) {
        runCatching {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                startForeground(
                    ForegroundServiceManagerImpl.NOTIFICATION_ID_CORE,
                    notification,
                    ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC
                )
            } else {
                startForeground(ForegroundServiceManagerImpl.NOTIFICATION_ID_CORE, notification)
            }
        }.onFailure { t ->
            logger.e(TAG, "startForeground failed", t)
        }
    }

    private fun buildRuntimeNotification(snapshot: ForegroundServiceManagerImpl.ForegroundRuntimeSnapshot): Notification {
        return foregroundServiceManager.buildRuntimeNotification(snapshot)
    }

    private fun initialSnapshot(): ForegroundServiceManagerImpl.ForegroundRuntimeSnapshot {
        val current = runtimeManager.observeState().value
        return toSnapshot(current)
    }

    private fun toSnapshot(state: RuntimeState): ForegroundServiceManagerImpl.ForegroundRuntimeSnapshot {
        return ForegroundServiceManagerImpl.ForegroundRuntimeSnapshot(
            phase = state.phase.name,
            readableMessage = state.readableMessage,
            isRunning = state is RuntimeState.Running,
            isStarting = state is RuntimeState.Starting || state is RuntimeState.Installing,
            isFailed = state is RuntimeState.Failed,
            isStopped = state is RuntimeState.Stopped || state is RuntimeState.NotInstalled
        )
    }

    companion object {
        private const val TAG = "AmitiaCoreService"
        const val ACTION_STOP_FOREGROUND = ForegroundServiceManagerImpl.ACTION_STOP_FOREGROUND
    }
}
