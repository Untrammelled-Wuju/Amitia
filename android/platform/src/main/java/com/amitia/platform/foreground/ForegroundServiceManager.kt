package com.amitia.platform.foreground

import kotlinx.coroutines.flow.Flow

interface ForegroundServiceManager {

    suspend fun start(serviceId: ServiceId, config: ForegroundConfig): Boolean

    suspend fun updateNotification(serviceId: ServiceId, config: ForegroundConfig): Boolean

    suspend fun stop(serviceId: ServiceId): Boolean

    suspend fun isRunning(serviceId: ServiceId): Boolean

    fun observeServiceState(): Flow<Map<ServiceId, ServiceState>>

    enum class ServiceId {
        AMITIA_CORE, MEDIA_PLAYBACK, AUDIO_RECORDING, FILE_DOWNLOAD, RUNTIME_INSTALL
    }

    data class ForegroundConfig(
        val notificationTitle: String,
        val notificationText: String,
        val notificationSubText: String? = null,
        val channelId: String,
        val iconRes: Int,
        val ongoing: Boolean = true,
        val showForegroundImmediately: Boolean = true,
        val foregroundServiceType: Int? = null,
        val contentIntent: PendingIntentConfig? = null,
        val actions: List<ForegroundAction> = emptyList()
    )

    data class PendingIntentConfig(
        val action: String,
        val extras: Map<String, String> = emptyMap()
    )

    data class ForegroundAction(
        val id: String,
        val title: String,
        val iconRes: Int? = null
    )

    sealed class ServiceState {
        object Stopped : ServiceState()
        object Starting : ServiceState()
        object Running : ServiceState()
        object Stopping : ServiceState()
        data class Failed(val error: String) : ServiceState()
    }
}
