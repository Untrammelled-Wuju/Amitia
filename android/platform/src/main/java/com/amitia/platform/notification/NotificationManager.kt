package com.amitia.platform.notification

import kotlinx.coroutines.flow.Flow

interface NotificationManager {

    suspend fun createChannel(channel: NotificationChannelConfig): Boolean

    suspend fun deleteChannel(channelId: String)

    suspend fun notify(notification: NotificationPayload): Int

    suspend fun cancel(notificationId: Int)

    suspend fun cancelAll()

    suspend fun cancelByTag(tag: String)

    fun observeNotificationClicks(): Flow<NotificationClick>

    data class NotificationChannelConfig(
        val id: String,
        val name: String,
        val description: String,
        val importance: ChannelImportance = ChannelImportance.DEFAULT,
        val showBadge: Boolean = false,
        val vibrationEnabled: Boolean = false,
        val soundEnabled: Boolean = false
    )

    data class NotificationPayload(
        val id: Int,
        val channelId: String,
        val title: String,
        val contentText: String?,
        val subText: String? = null,
        val ticker: String? = null,
        val ongoing: Boolean = false,
        val autoCancel: Boolean = true,
        val priority: NotificationPriority = NotificationPriority.DEFAULT,
        val category: NotificationCategory = NotificationCategory.MESSAGE,
        val smallIconRes: Int? = null,
        val largeIconRes: Int? = null,
        val actions: List<NotificationAction> = emptyList(),
        val progress: ProgressInfo? = null
    )

    data class NotificationAction(
        val id: String,
        val title: String,
        val iconRes: Int? = null
    )

    data class ProgressInfo(
        val current: Int,
        val total: Int,
        val indeterminate: Boolean = false
    )

    data class NotificationClick(
        val notificationId: Int,
        val actionId: String?,
        val tag: String?
    )

    enum class ChannelImportance(val value: Int) {
        NONE(0), MIN(1), LOW(2), DEFAULT(3), HIGH(4), MAX(5)
    }

    enum class NotificationPriority(val value: Int) {
        MIN(-2), LOW(-1), DEFAULT(0), HIGH(1), MAX(2)
    }

    enum class NotificationCategory {
        MESSAGE, CALL, ALARM, REMINDER, EVENT, PROGRESS, STATUS, ERROR
    }
}
