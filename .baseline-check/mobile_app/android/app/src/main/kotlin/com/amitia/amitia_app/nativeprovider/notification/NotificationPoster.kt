package com.amitia.amitia_app.nativeprovider.notification

import android.app.NotificationChannel
import android.app.NotificationManager
import android.content.Context
import android.os.Build
import androidx.core.app.NotificationCompat
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.atomic.AtomicInteger

internal class NotificationPoster(private val context: Context) {

    private val ownRefToTag = ConcurrentHashMap<String, String>()
    private val tagToOwnRef = ConcurrentHashMap<String, String>()
    private val idCounter = AtomicInteger(NOTIFICATION_ID_BASE)

    init {
        ensureChannels()
    }

    fun channelsExist(): Boolean {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return true
        val nm = context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        return nm.getNotificationChannel(CHANNEL_AGENT_ID) != null
    }

    fun post(title: String, body: String, channel: String, silent: Boolean): String {
        val actualChannel = if (channel == CHANNEL_TASK_ID) CHANNEL_TASK_ID else CHANNEL_AGENT_ID
        val tag = generateTag()
        val id = idCounter.incrementAndGet()

        val builder = NotificationCompat.Builder(context, actualChannel)
            .setSmallIcon(android.R.drawable.ic_dialog_info)
            .setContentTitle(title.ifBlank { null })
            .setContentText(body.ifBlank { null })
            .setPriority(NotificationCompat.PRIORITY_DEFAULT)
            .setAutoCancel(true)
            .setOnlyAlertOnce(true)

        if (silent) {
            builder.setSilent(true)
        }

        val nm = context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        nm.notify(tag, id, builder.build())

        val ref = generateOwnRef()
        ownRefToTag[ref] = tag
        tagToOwnRef[tag] = ref

        return ref
    }

    fun cancelOwn(ref: String): Boolean {
        val tag = ownRefToTag.remove(ref) ?: return false
        tagToOwnRef.remove(tag)
        val nm = context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        nm.cancel(tag, OWN_NOTIFICATION_ID)
        return true
    }

    fun lookupOwnTag(ref: String): String? = ownRefToTag[ref]

    private fun ensureChannels() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return
        val nm = context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager

        val agentChannel = NotificationChannel(
            CHANNEL_AGENT_ID,
            CHANNEL_AGENT_NAME,
            NotificationManager.IMPORTANCE_DEFAULT,
        )
        nm.createNotificationChannel(agentChannel)

        val taskChannel = NotificationChannel(
            CHANNEL_TASK_ID,
            CHANNEL_TASK_NAME,
            NotificationManager.IMPORTANCE_DEFAULT,
        )
        nm.createNotificationChannel(taskChannel)
    }

    private fun generateTag(): String {
        return "amitia_" + java.util.UUID.randomUUID().toString().replace("-", "").take(16)
    }

    private fun generateOwnRef(): String {
        val random = java.util.UUID.randomUUID().toString().replace("-", "")
        return "own_$random"
    }

    companion object {
        const val NOTIFICATION_ID_BASE = 0x6E740000
        const val OWN_NOTIFICATION_ID = 1001
        const val CHANNEL_AGENT_ID = "amitia_agent"
        const val CHANNEL_AGENT_NAME = "Amitia"
        const val CHANNEL_TASK_ID = "amitia_task"
        const val CHANNEL_TASK_NAME = "Amitia Task"
        const val TITLE_MAX = 256
        const val BODY_MAX = 4096
    }
}
