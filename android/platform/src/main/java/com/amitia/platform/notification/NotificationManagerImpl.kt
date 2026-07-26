package com.amitia.platform.notification

import android.app.NotificationChannel
import android.app.PendingIntent
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.os.Build
import androidx.core.app.NotificationCompat
import androidx.core.content.edit
import com.amitia.core.datastore.SettingsDataStore
import com.amitia.core.logging.Logger
import com.amitia.core.datastore.SettingsDataStore.NotificationPrivacy
import com.amitia.platform.notification.NotificationManager.ChannelImportance
import com.amitia.platform.notification.NotificationManager.NotificationAction
import com.amitia.platform.notification.NotificationManager.NotificationCategory
import com.amitia.platform.notification.NotificationManager.NotificationChannelConfig
import com.amitia.platform.notification.NotificationManager.NotificationClick
import com.amitia.platform.notification.NotificationManager.NotificationPayload
import com.amitia.platform.notification.NotificationManager.NotificationPriority
import com.amitia.platform.notification.NotificationManager.ProgressInfo
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.runBlocking

@Singleton
class NotificationManagerImpl @Inject constructor(
    @ApplicationContext private val context: Context,
    private val settingsDataStore: SettingsDataStore,
    private val intentBuilder: NotificationIntentBuilder,
    private val logger: Logger
) : NotificationManager {

    private val notificationManager = context.getSystemService(android.app.NotificationManager::class.java)
    private val dedupPrefs by lazy {
        context.getSharedPreferences(DEDUP_PREFS, Context.MODE_PRIVATE)
    }

    override suspend fun createChannel(channel: NotificationChannelConfig): Boolean {
        return try {
            val existing = notificationManager?.getNotificationChannel(channel.id)
            if (existing == null) {
                val androidChannel = NotificationChannel(
                    channel.id,
                    channel.name,
                    mapImportance(channel.importance)
                ).apply {
                    description = channel.description
                    setShowBadge(channel.showBadge)
                    enableVibration(channel.vibrationEnabled)
                    if (!channel.soundEnabled) setSound(null, null)
                }
                notificationManager?.createNotificationChannel(androidChannel)
            }
            true
        } catch (t: Throwable) {
            logger.e(TAG, "createChannel failed: ${channel.id}", t)
            false
        }
    }

    override suspend fun deleteChannel(channelId: String) {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return
        runCatching { notificationManager?.deleteNotificationChannel(channelId) }
    }

    override suspend fun notify(notification: NotificationPayload): Int {
        return try {
            ensureChannelExists(notification.channelId)
            val androidNotification = buildPayloadNotification(notification)
            notificationManager?.notify(notification.id, androidNotification)
            notification.id
        } catch (t: Throwable) {
            logger.e(TAG, "notify failed: ${notification.id}", t)
            -1
        }
    }

    override suspend fun cancel(notificationId: Int) {
        notificationManager?.cancel(notificationId)
    }

    override suspend fun cancelAll() {
        notificationManager?.cancelAll()
        dedupPrefs.edit { clear() }
    }

    override suspend fun cancelByTag(tag: String) {
        notificationManager?.cancel(tag, 0)
    }

    override fun observeNotificationClicks() = kotlinx.coroutines.flow.emptyFlow<NotificationClick>()

    fun showProactiveMessage(
        characterId: String,
        characterName: String,
        content: String,
        messageId: String,
        conversationId: String? = null
    ) {
        try {
            if (dedupPrefs.getBoolean(messageId, false)) {
                logger.d(TAG, "proactive message already notified: $messageId")
                return
            }
            val notificationEnabled = runBlocking { settingsDataStore.notificationEnabled.first() }
            if (!notificationEnabled) {
                logger.d(TAG, "notifications disabled, skip: $messageId")
                return
            }
            val charEnabled = runBlocking {
                settingsDataStore.characterNotificationEnabledNow(characterId)
            }
            if (!charEnabled) {
                logger.d(TAG, "character notifications disabled: $characterId")
                return
            }
            val privacy = runBlocking { settingsDataStore.notificationPrivacy.first() }
            ensureChannelExists(CHANNEL_PROACTIVE)
            val title = characterName.ifBlank { "Amitia" }
            val body = when (privacy) {
                NotificationPrivacy.CONTENT -> content
                NotificationPrivacy.ANNOUNCEMENT_ONLY -> "你收到一条新消息"
                NotificationPrivacy.HIDDEN -> "新消息"
            }
            val pendingIntent = intentBuilder.buildChatIntent(
                characterId = characterId,
                conversationId = conversationId,
                messageId = messageId
            )
            val notification = NotificationCompat.Builder(context, CHANNEL_PROACTIVE)
                .setSmallIcon(android.R.drawable.stat_notify_chat)
                .setContentTitle(title)
                .setContentText(body)
                .setStyle(NotificationCompat.BigTextStyle().bigText(body))
                .setAutoCancel(true)
                .setPriority(NotificationCompat.PRIORITY_HIGH)
                .setCategory(NotificationCompat.CATEGORY_MESSAGE)
                .setContentIntent(pendingIntent)
                .setDeleteIntent(intentBuilder.buildClearIntent())
                .build()
            notificationManager?.notify(messageId.hashCode(), notification)
            dedupPrefs.edit { putBoolean(messageId, true) }
            logger.i(TAG, "proactive notification shown: $messageId char=$characterId")
        } catch (t: Throwable) {
            logger.e(TAG, "showProactiveMessage failed: $messageId", t)
        }
    }

    fun cancelProactive(messageId: String) {
        notificationManager?.cancel(messageId.hashCode())
        dedupPrefs.edit { remove(messageId) }
    }

    fun isNotified(messageId: String): Boolean = dedupPrefs.getBoolean(messageId, false)

    private fun ensureChannelExists(channelId: String) {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return
        val existing = notificationManager?.getNotificationChannel(channelId)
        if (existing != null) return
        val channel = NotificationChannel(
            channelId,
            channelId,
            android.app.NotificationManager.IMPORTANCE_DEFAULT
        )
        notificationManager?.createNotificationChannel(channel)
    }

    private fun buildPayloadNotification(payload: NotificationPayload): android.app.Notification {
        val builder = NotificationCompat.Builder(context, payload.channelId)
            .setSmallIcon(payload.smallIconRes ?: android.R.drawable.stat_notify_chat)
            .setContentTitle(payload.title)
            .setContentText(payload.contentText)
            .setOngoing(payload.ongoing)
            .setAutoCancel(payload.autoCancel)
            .setPriority(mapPriority(payload.priority))
            .setCategory(mapCategory(payload.category))
        payload.subText?.let { builder.setSubText(it) }
        payload.ticker?.let { builder.setTicker(it) }
        payload.progress?.let { progress ->
            if (progress.total > 0) {
                builder.setProgress(progress.total, progress.current, progress.indeterminate)
            }
        }
        return builder.build()
    }

    private fun mapImportance(importance: ChannelImportance): Int = importance.value

    private fun mapPriority(priority: NotificationPriority): Int = priority.value

    private fun mapCategory(category: NotificationCategory): String {
        return when (category) {
            NotificationCategory.MESSAGE -> NotificationCompat.CATEGORY_MESSAGE
            NotificationCategory.CALL -> NotificationCompat.CATEGORY_CALL
            NotificationCategory.ALARM -> NotificationCompat.CATEGORY_ALARM
            NotificationCategory.REMINDER -> NotificationCompat.CATEGORY_REMINDER
            NotificationCategory.EVENT -> NotificationCompat.CATEGORY_EVENT
            NotificationCategory.PROGRESS -> NotificationCompat.CATEGORY_PROGRESS
            NotificationCategory.STATUS -> NotificationCompat.CATEGORY_STATUS
            NotificationCategory.ERROR -> NotificationCompat.CATEGORY_ERROR
        }
    }

    companion object {
        const val CHANNEL_PROACTIVE = "amitia_proactive"
        const val CHANNEL_SERVICE = "amitia_service"
        const val CHANNEL_ERROR = "amitia_error"

        const val DEDUP_PREFS = "amitia_notification_dedup"
        const val ACTION_OPEN_CONVERSATION = "com.amitia.android.action.OPEN_CONVERSATION"
        const val EXTRA_CHARACTER_ID = "character_id"
        const val EXTRA_CONVERSATION_ID = "conversation_id"
        const val EXTRA_MESSAGE_ID = "message_id"
        const val MAIN_ACTIVITY_CLASS = "com.amitia.android.MainActivity"

        private const val TAG = "NotificationManagerImpl"
    }
}
