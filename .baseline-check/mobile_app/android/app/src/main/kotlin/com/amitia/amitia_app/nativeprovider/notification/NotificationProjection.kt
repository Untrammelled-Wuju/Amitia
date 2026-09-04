package com.amitia.amitia_app.nativeprovider.notification

import android.app.Notification
import android.service.notification.StatusBarNotification
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.atomic.AtomicLong

internal class NotificationProjectionInternal {

    private val refToKey = ConcurrentHashMap<String, InternalRefMapping>()
    private val keyToRef = ConcurrentHashMap<String, String>()
    private val generation = AtomicLong(0L)

    fun currentGeneration(): Long = generation.get()

    fun bumpGeneration() {
        generation.incrementAndGet()
        refToKey.clear()
        keyToRef.clear()
    }

    fun assignRef(key: String): String {
        keyToRef[key]?.let { return it }
        val ref = generateRef("ntf_")
        keyToRef[key] = ref
        refToKey[ref] = InternalRefMapping(key = key, generation = generation.get())
        return ref
    }

    fun lookupKey(ref: String): String? {
        val mapping = refToKey[ref] ?: return null
        if (mapping.generation != generation.get()) return null
        return mapping.key
    }

    fun invalidateRef(ref: String) {
        val mapping = refToKey.remove(ref) ?: return
        keyToRef.remove(mapping.key)
    }

    fun project(
        sbn: StatusBarNotification,
        appLabel: String,
        gen: Long,
    ): NotificationProjection {
        val notification = sbn.notification
        val extras = notification.extras

        val title = extras.getCharSequence("android.title")?.toString() ?: ""
        val text = extras.getCharSequence("android.text")?.toString() ?: ""
        val subText = extras.getCharSequence("android.subText")?.toString() ?: ""
        val category = extras.getString("android.category") ?: ""
        val groupKey = sbn.groupKey ?: ""

        val actions = notification.actions?.map { action ->
            NotificationActionProjection(
                actionRef = generateRef("act_"),
                title = action.title?.toString() ?: "",
                hasRemoteInput = action.remoteInputs?.isNotEmpty() == true,
            )
        } ?: emptyList()

        return NotificationProjection(
            notificationRef = assignRef(sbn.key),
            packageName = sbn.packageName,
            appLabel = appLabel,
            postedAt = sbn.postTime,
            title = title.truncate(TITLE_MAX),
            text = text.truncate(TEXT_MAX),
            subText = subText.truncate(SUBTEXT_MAX),
            category = category,
            ongoing = sbn.isOngoing,
            clearable = (notification.flags and Notification.FLAG_ONGOING_EVENT) == 0,
            groupKey = groupKey,
            channelId = notification.channelId ?: "",
            importance = notification.priority,
            hasContentAction = notification.contentIntent != null,
            actions = actions,
            generation = gen,
        )
    }

    fun isRuntimeService(packageName: String, channelId: String): Boolean {
        if (packageName == "com.amitia.amitia_app" && channelId == "runtime_service") return true
        if (channelId == "runtime_service") return true
        return false
    }

    private fun generateRef(prefix: String): String {
        val random = java.util.UUID.randomUUID().toString().replace("-", "")
        return "$prefix$random"
    }

    private fun String.truncate(maxLen: Int): String {
        return if (this.length > maxLen) this.substring(0, maxLen) else this
    }

    companion object {
        const val TITLE_MAX = 512
        const val TEXT_MAX = 4096
        const val SUBTEXT_MAX = 1024
    }
}
