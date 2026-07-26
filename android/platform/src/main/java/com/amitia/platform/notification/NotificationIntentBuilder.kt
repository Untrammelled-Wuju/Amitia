package com.amitia.platform.notification

import android.app.PendingIntent
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class NotificationIntentBuilder @Inject constructor(
    @ApplicationContext private val context: Context
) {

    fun buildChatIntent(
        characterId: String,
        conversationId: String? = null,
        messageId: String? = null
    ): PendingIntent {
        val intent = Intent().apply {
            component = ComponentName(context, NotificationManagerImpl.MAIN_ACTIVITY_CLASS)
            action = NotificationManagerImpl.ACTION_OPEN_CONVERSATION
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP or Intent.FLAG_ACTIVITY_SINGLE_TOP)
            putExtra(NotificationManagerImpl.EXTRA_CHARACTER_ID, characterId)
            conversationId?.let { putExtra(NotificationManagerImpl.EXTRA_CONVERSATION_ID, it) }
            messageId?.let { putExtra(NotificationManagerImpl.EXTRA_MESSAGE_ID, it) }
        }
        val requestCode = (characterId + (conversationId ?: "") + (messageId ?: "")).hashCode()
        return PendingIntent.getActivity(
            context,
            requestCode,
            intent,
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT
        )
    }

    fun buildClearIntent(): PendingIntent {
        val intent = Intent().apply {
            component = ComponentName(context, NotificationManagerImpl.MAIN_ACTIVITY_CLASS)
            action = ACTION_CLEAR_NOTIFICATIONS
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP or Intent.FLAG_ACTIVITY_SINGLE_TOP)
        }
        return PendingIntent.getActivity(
            context,
            REQUEST_CLEAR,
            intent,
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT
        )
    }

    companion object {
        const val ACTION_CLEAR_NOTIFICATIONS = "com.amitia.android.action.CLEAR_NOTIFICATIONS"
        private const val REQUEST_CLEAR = 0x7FFF
    }
}
