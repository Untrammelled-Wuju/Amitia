package com.amitia.platform.foreground

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.os.Build
import androidx.core.app.NotificationCompat
import com.amitia.core.common.Constants
import com.amitia.core.logging.Logger
import com.amitia.core.model.RuntimeStrategy
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow

@Singleton
class ForegroundServiceManagerImpl @Inject constructor(
    @ApplicationContext private val context: Context,
    private val logger: Logger
) : ForegroundServiceManager {

    private val notificationManager = context.getSystemService(NotificationManager::class.java)
    private val serviceStateFlow = MutableStateFlow<Map<ForegroundServiceManager.ServiceId, ForegroundServiceManager.ServiceState>>(emptyMap())
    private var currentStrategy = RuntimeStrategy.ON_DEMAND
    private var currentSnapshot: ForegroundRuntimeSnapshot = ForegroundRuntimeSnapshot()

    override suspend fun start(
        serviceId: ForegroundServiceManager.ServiceId,
        config: ForegroundServiceManager.ForegroundConfig
    ): Boolean {
        return try {
            ensureChannel(config.channelId)
            val notification = buildNotification(config)
            val intent = Intent().apply {
                component = ComponentName(context, AMITIA_CORE_SERVICE_CLASS)
                action = ACTION_START_FOREGROUND
                putExtra(EXTRA_NOTIFICATION_TITLE, config.notificationTitle)
                putExtra(EXTRA_NOTIFICATION_TEXT, config.notificationText)
            }
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                context.startForegroundService(intent)
            } else {
                context.startService(intent)
            }
            serviceStateFlow.value = serviceStateFlow.value + (serviceId to ForegroundServiceManager.ServiceState.Running)
            logger.i(TAG, "foreground service started: $serviceId")
            true
        } catch (t: Throwable) {
            logger.e(TAG, "start foreground service failed: $serviceId", t)
            serviceStateFlow.value = serviceStateFlow.value + (serviceId to ForegroundServiceManager.ServiceState.Failed(t.message ?: "unknown"))
            false
        }
    }

    override suspend fun updateNotification(
        serviceId: ForegroundServiceManager.ServiceId,
        config: ForegroundServiceManager.ForegroundConfig
    ): Boolean {
        return try {
            val notification = buildNotification(config)
            notificationManager?.notify(notificationId(serviceId), notification)
            true
        } catch (t: Throwable) {
            logger.e(TAG, "updateNotification failed: $serviceId", t)
            false
        }
    }

    override suspend fun stop(serviceId: ForegroundServiceManager.ServiceId): Boolean {
        return try {
            val intent = Intent().apply {
                component = ComponentName(context, AMITIA_CORE_SERVICE_CLASS)
                action = ACTION_STOP_FOREGROUND
            }
            context.startService(intent)
            serviceStateFlow.value = serviceStateFlow.value + (serviceId to ForegroundServiceManager.ServiceState.Stopped)
            logger.i(TAG, "foreground service stopped: $serviceId")
            true
        } catch (t: Throwable) {
            logger.e(TAG, "stop foreground service failed: $serviceId", t)
            false
        }
    }

    override suspend fun isRunning(serviceId: ForegroundServiceManager.ServiceId): Boolean {
        return serviceStateFlow.value[serviceId] is ForegroundServiceManager.ServiceState.Running
    }

    override fun observeServiceState(): StateFlow<Map<ForegroundServiceManager.ServiceId, ForegroundServiceManager.ServiceState>> {
        return serviceStateFlow.asStateFlow()
    }

    fun startForegroundService() {
        try {
            ensureChannel(Constants.NOTIFICATION_CHANNEL_CORE)
            val intent = Intent().apply {
                component = ComponentName(context, AMITIA_CORE_SERVICE_CLASS)
                action = ACTION_START_FOREGROUND
            }
            context.startForegroundService(intent)
        } catch (t: Throwable) {
            logger.e(TAG, "startForegroundService failed", t)
        }
    }

    fun stopForegroundService() {
        try {
            val intent = Intent().apply {
                component = ComponentName(context, AMITIA_CORE_SERVICE_CLASS)
                action = ACTION_STOP_FOREGROUND
            }
            context.startService(intent)
        } catch (t: Throwable) {
            logger.e(TAG, "stopForegroundService failed", t)
        }
    }

    fun updateNotification(state: ForegroundRuntimeSnapshot) {
        currentSnapshot = state
        try {
            ensureChannel(Constants.NOTIFICATION_CHANNEL_CORE)
            val notification = buildRuntimeNotification(state)
            notificationManager?.notify(NOTIFICATION_ID_CORE, notification)
        } catch (t: Throwable) {
            logger.e(TAG, "updateNotification(state) failed", t)
        }
    }

    fun setStrategy(strategy: RuntimeStrategy) {
        currentStrategy = strategy
        logger.i(TAG, "runtime strategy set: $strategy")
    }

    fun currentStrategy(): RuntimeStrategy = currentStrategy

    fun buildRuntimeNotification(state: ForegroundRuntimeSnapshot): Notification {
        val title = when {
            state.isRunning -> "Amitia 运行中"
            state.isFailed -> "Amitia 运行异常"
            state.isStarting -> "Amitia 正在启动"
            state.isStopped -> "Amitia 已停止"
            else -> "Amitia"
        }
        val text = state.readableMessage.ifBlank { "本地运行时正在维护 Amitia 核心" }
        val launchIntent = context.packageManager.getLaunchIntentForPackage(context.packageName)
        val pendingIntent = if (launchIntent != null) {
            PendingIntent.getActivity(
                context,
                0,
                launchIntent,
                PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT
            )
        } else null
        return NotificationCompat.Builder(context, Constants.NOTIFICATION_CHANNEL_CORE)
            .setSmallIcon(android.R.drawable.stat_notify_sync)
            .setContentTitle(title)
            .setContentText(text)
            .setOngoing(true)
            .setOnlyAlertOnce(true)
            .setCategory(NotificationCompat.CATEGORY_SERVICE)
            .setPriority(NotificationCompat.PRIORITY_LOW)
            .setContentIntent(pendingIntent)
            .build()
    }

    private fun ensureChannel(channelId: String) {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return
        val existing = notificationManager?.getNotificationChannel(channelId)
        if (existing != null) return
        val channel = NotificationChannel(
            channelId,
            channelId,
            NotificationManager.IMPORTANCE_LOW
        ).apply {
            description = "Amitia 核心服务通知"
            setShowBadge(false)
        }
        notificationManager?.createNotificationChannel(channel)
    }

    private fun buildNotification(config: ForegroundServiceManager.ForegroundConfig): Notification {
        val builder = NotificationCompat.Builder(context, config.channelId)
            .setSmallIcon(config.iconRes)
            .setContentTitle(config.notificationTitle)
            .setContentText(config.notificationText)
            .setOngoing(config.ongoing)
            .setOnlyAlertOnce(true)
            .setPriority(NotificationCompat.PRIORITY_LOW)
        config.notificationSubText?.let { builder.setSubText(it) }
        return builder.build()
    }

    private fun notificationId(serviceId: ForegroundServiceManager.ServiceId): Int = serviceId.ordinal + 1

    data class ForegroundRuntimeSnapshot(
        val phase: String = "IDLE",
        val readableMessage: String = "",
        val isRunning: Boolean = false,
        val isStarting: Boolean = false,
        val isFailed: Boolean = false,
        val isStopped: Boolean = false
    )

    companion object {
        private const val TAG = "ForegroundServiceManager"
        private const val AMITIA_CORE_SERVICE_CLASS = "com.amitia.android.runtime.AmitiaCoreService"
        const val ACTION_START_FOREGROUND = "com.amitia.android.action.START_FOREGROUND"
        const val ACTION_STOP_FOREGROUND = "com.amitia.android.action.STOP_FOREGROUND"
        const val EXTRA_NOTIFICATION_TITLE = "notification_title"
        const val EXTRA_NOTIFICATION_TEXT = "notification_text"
        const val NOTIFICATION_ID_CORE = 1001
    }
}
