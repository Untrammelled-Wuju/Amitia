package com.amitia.amitia_app.notifications

import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.graphics.Color
import android.graphics.Bitmap
import android.graphics.Canvas
import android.graphics.Paint
import android.graphics.Typeface
import android.net.Uri
import android.os.Build
import android.os.Handler
import android.os.Looper
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidx.core.app.Person
import androidx.core.app.RemoteInput
import androidx.core.graphics.drawable.IconCompat
import com.amitia.amitia_app.MainActivity
import com.amitia.amitia_app.R
import org.json.JSONArray
import org.json.JSONObject
import kotlin.math.roundToInt

object NotificationRenderer {
    const val CHANNEL_MESSAGES = "amitia_messages"
    const val CHANNEL_TASKS = "amitia_tasks"
    const val CHANNEL_CALLS = "amitia_calls"
    const val CHANNEL_REMINDERS = "amitia_reminders"
    const val CHANNEL_SYSTEM = "amitia_system"
    const val REPLY_KEY = "amitia_reply_text"
    private const val PREFS = "amitia_notification_runtime"
    private const val MAX_HISTORY = 6
    private const val MAX_RECENT_EVENTS = 24

    @Synchronized
    private fun shouldDisplayMessage(context: Context, conversationId: String, data: Map<String, String>): Boolean {
        val eventId = data["messageId"].orEmpty()
            .ifBlank { data["eventId"].orEmpty() }
            .ifBlank { data["notificationId"].orEmpty() }
            .trim()
        if (eventId.isEmpty() || conversationId.isEmpty()) return true
        val prefs = context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
        val key = "recent_events:$conversationId"
        val history = runCatching { JSONArray(prefs.getString(key, "[]")) }.getOrDefault(JSONArray())
        val previous = prefs.getString("last_event:$conversationId", null)
        if (previous == eventId) return false
        for (index in 0 until history.length()) {
            if (history.optString(index) == eventId) return false
        }
        val recent = JSONArray()
        for (index in (history.length() - MAX_RECENT_EVENTS + 1).coerceAtLeast(0) until history.length()) {
            val value = history.optString(index)
            if (value.isNotBlank()) recent.put(value)
        }
        recent.put(eventId)
        prefs.edit().putString(key, recent.toString()).remove("last_event:$conversationId").apply()
        return true
    }

    private fun publicNotification(context: Context, channel: String, title: String, text: String): android.app.Notification {
        return NotificationCompat.Builder(context, channel)
            .setSmallIcon(R.drawable.ic_notification)
            .setContentTitle(title)
            .setContentText(text)
            .build()
    }

    private fun avatarBitmap(context: Context, name: String): Bitmap {
        val size = (48 * context.resources.displayMetrics.density).toInt().coerceAtLeast(48)
        val bitmap = Bitmap.createBitmap(size, size, Bitmap.Config.ARGB_8888)
        val canvas = Canvas(bitmap)
        val paint = Paint(Paint.ANTI_ALIAS_FLAG)
        paint.color = Color.rgb(108, 115, 193)
        canvas.drawCircle(size / 2f, size / 2f, size / 2f, paint)
        paint.color = Color.WHITE
        paint.typeface = Typeface.create(Typeface.DEFAULT, Typeface.BOLD)
        paint.textSize = size * 0.43f
        paint.textAlign = Paint.Align.CENTER
        val initial = name.trim().take(1).ifEmpty { "A" }
        val center = (paint.ascent() + paint.descent()) / 2f
        canvas.drawText(initial, size / 2f, size / 2f - center, paint)
        return bitmap
    }

    fun ensureChannels(context: Context) {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return
        val manager = context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        val channels = listOf(
            NotificationChannel(CHANNEL_MESSAGES, "聊天消息", NotificationManager.IMPORTANCE_HIGH).apply {
                description = "AI 角色聊天消息"
                lockscreenVisibility = android.app.Notification.VISIBILITY_PRIVATE
            },
            NotificationChannel(CHANNEL_TASKS, "任务执行", NotificationManager.IMPORTANCE_DEFAULT).apply {
                description = "Agent 和工作流执行状态"
                lockscreenVisibility = android.app.Notification.VISIBILITY_PRIVATE
            },
            NotificationChannel(CHANNEL_CALLS, "语音通话", NotificationManager.IMPORTANCE_HIGH).apply {
                description = "实时语音来电和通话状态"
                lockscreenVisibility = android.app.Notification.VISIBILITY_PRIVATE
            },
            NotificationChannel(CHANNEL_REMINDERS, "提醒", NotificationManager.IMPORTANCE_DEFAULT).apply {
                description = "日程、学习与角色主动提醒"
                lockscreenVisibility = android.app.Notification.VISIBILITY_PRIVATE
            },
            NotificationChannel(CHANNEL_SYSTEM, "系统", NotificationManager.IMPORTANCE_DEFAULT).apply {
                description = "Amitia 系统状态"
            },
        )
        manager.createNotificationChannels(channels)
    }

    fun handleRemoteMessage(context: Context, data: Map<String, String>) {
        ensureChannels(context)
        if (!NotificationManagerCompat.from(context).areNotificationsEnabled()) return
        when (data["type"].orEmpty()) {
            "message.received", "message.completed" -> showMessage(context, data)
            "reminder.triggered", "proactive.message" -> showReminder(context, data)
            "call.incoming" -> showIncomingCall(context, data)
            "call.ended" -> endCall(context, data)
            "run.start", "run.started", "run.update", "run.updated" -> showExecution(context, data, false)
            "run.end", "run.completed", "run.failed", "run.cancelled", "run.interrupted" -> showExecution(context, data, true)
            "run.dismiss" -> dismissExecution(context, data)
            "system.test" -> showSystem(context, data)
        }
    }

    private fun showIncomingCall(context: Context, data: Map<String, String>) {
        val callId = data["callId"].orEmpty()
        val conversationId = data["conversationId"].orEmpty()
        if (callId.isBlank() || conversationId.isBlank()) return
        val callerName = data["callerName"].orEmpty().ifBlank {
            data["title"].orEmpty().ifBlank { "Amitia" }
        }
        val callType = data["callType"].orEmpty().ifBlank { "audio" }
        val person = Person.Builder()
            .setName(callerName)
            .setIcon(IconCompat.createWithBitmap(avatarBitmap(context, callerName)))
            .setKey(data["characterId"].orEmpty().ifBlank { callerName })
            .setImportant(true)
            .build()

        val incomingLink = callDeepLink(
            conversationId = conversationId,
            callId = callId,
            callType = callType,
            callerName = callerName,
            action = "incoming",
        )
        val answerLink = callDeepLink(
            conversationId = conversationId,
            callId = callId,
            callType = callType,
            callerName = callerName,
            action = "answer",
        )
        val answerIntent = Intent(context, MainActivity::class.java)
            .setAction(Intent.ACTION_VIEW)
            .setData(Uri.parse(answerLink))
            .putExtra("amitia.deepLink", answerLink)
            .addFlags(Intent.FLAG_ACTIVITY_CLEAR_TOP or Intent.FLAG_ACTIVITY_SINGLE_TOP)
        val answerPendingIntent = PendingIntent.getActivity(
            context,
            stableId("call-answer:$callId"),
            answerIntent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
        )
        val declineIntent = Intent(context, NotificationCallActionReceiver::class.java)
            .setAction("com.amitia.amitia_app.CALL_DECLINE.$callId")
            .putExtra("callId", callId)
            .putExtra("conversationId", conversationId)
            .putExtra(
                "deepLink",
                callDeepLink(
                    conversationId = conversationId,
                    callId = callId,
                    callType = callType,
                    callerName = callerName,
                    action = "decline",
                ),
            )
        val declinePendingIntent = PendingIntent.getBroadcast(
            context,
            stableId("call-decline:$callId"),
            declineIntent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
        )
        val fullscreenIntent = contentIntent(context, incomingLink, "call:$callId")
        val style = NotificationCompat.CallStyle
            .forIncomingCall(person, declinePendingIntent, answerPendingIntent)
            .setIsVideo(callType == "video")
        val builder = NotificationCompat.Builder(context, CHANNEL_CALLS)
            .setSmallIcon(R.drawable.ic_notification)
            .setContentTitle(callerName)
            .setContentText(if (callType == "video") "视频通话邀请" else "语音通话邀请")
            .setCategory(NotificationCompat.CATEGORY_CALL)
            .setPriority(NotificationCompat.PRIORITY_MAX)
            .setStyle(style)
            .setOngoing(true)
            .setAutoCancel(false)
            .setOnlyAlertOnce(false)
            .setContentIntent(fullscreenIntent)
            .setVisibility(NotificationCompat.VISIBILITY_PRIVATE)
            .setPublicVersion(publicNotification(context, CHANNEL_CALLS, "Amitia", "有新的语音或视频通话邀请"))
        if (canUseFullScreenCall(context)) {
            builder.setFullScreenIntent(fullscreenIntent, true)
        }
        NotificationManagerCompat.from(context)
            .notify(stableId("call:$callId"), builder.build())
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
            .edit()
            .putBoolean("call_active:$callId", true)
            .apply()
    }

    private fun endCall(context: Context, data: Map<String, String>) {
        val callId = data["callId"].orEmpty()
        if (callId.isBlank()) return
        clearCall(context, callId)
        NotificationPlatformPlugin.emitCallEnded(
            context = context.applicationContext,
            callId = callId,
            conversationId = data["conversationId"].orEmpty(),
            reason = data["reason"].orEmpty(),
        )
    }

    private fun callDeepLink(
        conversationId: String,
        callId: String,
        callType: String,
        callerName: String,
        action: String,
    ): String {
        return Uri.Builder()
            .scheme("amitia")
            .authority("call")
            .appendPath(conversationId)
            .appendQueryParameter("call", callId)
            .appendQueryParameter("type", callType)
            .appendQueryParameter("caller", callerName)
            .appendQueryParameter("action", action)
            .build()
            .toString()
    }

    private fun showReminder(context: Context, data: Map<String, String>) {
        val conversationId = data["conversationId"].orEmpty()
        val title = data["title"].orEmpty().ifBlank { "Amitia 提醒" }
        val body = data["body"].orEmpty().ifBlank { "你有一条提醒" }
        val deepLink = data["deepLink"].orEmpty().ifBlank {
            if (conversationId.isNotBlank()) "amitia://chat/$conversationId" else "amitia://reminder"
        }
        val builder = NotificationCompat.Builder(context, CHANNEL_REMINDERS)
            .setSmallIcon(R.drawable.ic_notification)
            .setContentTitle(title)
            .setContentText(body)
            .setStyle(NotificationCompat.BigTextStyle().bigText(body))
            .setCategory(NotificationCompat.CATEGORY_REMINDER)
            .setPriority(NotificationCompat.PRIORITY_DEFAULT)
            .setAutoCancel(true)
            .setGroup("amitia:reminders")
            .setContentIntent(contentIntent(context, deepLink, "reminder:$conversationId"))
            .setVisibility(NotificationCompat.VISIBILITY_PRIVATE)
            .setPublicVersion(publicNotification(context, CHANNEL_REMINDERS, "Amitia 提醒", "你有一条新提醒"))
        if (data["sound"] == "false") {
            builder.setSilent(true)
        }
        val reminderKey = (data["messageId"] ?: conversationId).ifBlank { "default" }
        val notificationId = stableId("reminder:$reminderKey")
        NotificationManagerCompat.from(context).notify(notificationId, builder.build())
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
            .edit()
            .putInt("reminder_active:$notificationId", notificationId)
            .apply()
    }

    private fun showSystem(context: Context, data: Map<String, String>) {
        val title = data["title"].orEmpty().ifBlank { "Amitia" }
        val body = data["body"].orEmpty().ifBlank { "通知链路可用" }
        val deepLink = data["deepLink"].orEmpty().ifBlank { "amitia://settings/notifications" }
        val builder = NotificationCompat.Builder(context, CHANNEL_SYSTEM)
            .setSmallIcon(R.drawable.ic_notification)
            .setContentTitle(title)
            .setContentText(body)
            .setStyle(NotificationCompat.BigTextStyle().bigText(body))
            .setCategory(NotificationCompat.CATEGORY_STATUS)
            .setPriority(NotificationCompat.PRIORITY_DEFAULT)
            .setAutoCancel(true)
            .setContentIntent(contentIntent(context, deepLink, "system-test"))
        if (data["sound"] == "false") {
            builder.setSilent(true)
        }
        NotificationManagerCompat.from(context)
            .notify(stableId("system:test"), builder.build())
    }

    private fun showMessage(context: Context, data: Map<String, String>) {
        val conversationId = data["conversationId"].orEmpty()
        val characterId = data["characterId"].orEmpty()
        val title = data["title"].orEmpty().ifBlank { "Amitia" }
        val body = data["body"].orEmpty()
        if (body.isBlank() || conversationId.isBlank() || !shouldDisplayMessage(context, conversationId, data)) return
        val deepLink = data["deepLink"].orEmpty().ifBlank { "amitia://chat/$conversationId" }
        val previewMode = data["previewMode"].orEmpty().trim().lowercase().ifBlank { "sender_only" }
        val visibleTitle = if (previewMode == "hidden") "Amitia" else title
        val visibleBody = if (previewMode == "full") body else "你有一条新消息"
        val avatar = avatarBitmap(context, visibleTitle)
        val sender = Person.Builder()
            .setName(visibleTitle)
            .setIcon(IconCompat.createWithBitmap(avatar))
            .setKey(characterId.ifBlank { visibleTitle })
            .build()
        val self = Person.Builder().setName("我").setKey("amitia-user").build()
        val style = NotificationCompat.MessagingStyle(self)
            .setConversationTitle(visibleTitle)
            .setGroupConversation(false)
        if (previewMode == "full") {
            loadMessageHistory(context, conversationId).forEach { item ->
                style.addMessage(item.second, item.first, sender)
            }
        } else {
            clearMessageHistory(context, conversationId)
        }
        val now = System.currentTimeMillis()
        style.addMessage(visibleBody, now, sender)
        if (previewMode == "full") {
            appendMessageHistory(context, conversationId, now, body)
        }

        val replyIntent = Intent(context, NotificationReplyReceiver::class.java)
            .setAction("com.amitia.amitia_app.NOTIFICATION_REPLY.$conversationId")
            .putExtra("conversationId", conversationId)
            .putExtra("deepLink", deepLink)
        val replyPendingIntent = PendingIntent.getBroadcast(
            context,
            stableId("reply:$conversationId"),
            replyIntent,
            PendingIntent.FLAG_UPDATE_CURRENT or pendingIntentMutableFlag(),
        )
        val remoteInput = RemoteInput.Builder(REPLY_KEY).setLabel("回复").build()
        val replyAction = NotificationCompat.Action.Builder(
            R.drawable.ic_notification,
            "回复",
            replyPendingIntent,
        ).addRemoteInput(remoteInput).setAllowGeneratedReplies(true).build()

        val builder = NotificationCompat.Builder(context, CHANNEL_MESSAGES)
            .setSmallIcon(R.drawable.ic_notification)
            .setContentTitle(visibleTitle)
            .setContentText(visibleBody)
            .setLargeIcon(avatar)
            .setStyle(style)
            .setCategory(NotificationCompat.CATEGORY_MESSAGE)
            .setPriority(NotificationCompat.PRIORITY_HIGH)
            .setAutoCancel(true)
            .setOnlyAlertOnce(false)
            .setGroup("amitia:conversation:$conversationId")
            .setContentIntent(contentIntent(context, deepLink, conversationId))
            .addAction(replyAction)
        val publicVersion = NotificationCompat.Builder(context, CHANNEL_MESSAGES)
            .setSmallIcon(R.drawable.ic_notification)
            .setContentTitle(if (previewMode == "sender_only") title else "Amitia")
            .setContentText("你有一条新消息")
            .build()
        builder.setPublicVersion(publicVersion)
        builder.setVisibility(if (previewMode == "hidden") NotificationCompat.VISIBILITY_SECRET else NotificationCompat.VISIBILITY_PRIVATE)
        if (data["sound"] == "false") {
            builder.setSilent(true)
        }
        NotificationManagerCompat.from(context).notify(stableId("message:$conversationId"), builder.build())
        FloatingChatBubbleService.onMessage(context, conversationId, visibleTitle, visibleBody)
        if (conversationId.isNotBlank()) {
            context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
                .edit()
                .putBoolean("message_active:$conversationId", true)
                .apply()
        }
    }

    private fun showExecution(context: Context, data: Map<String, String>, terminal: Boolean) {
        val runId = data["runId"].orEmpty()
        if (runId.isBlank()) return
        val revision = data["revision"]?.toLongOrNull() ?: 0L
        if (!acceptRevision(context, runId, revision, terminal)) return
        val phase = data["phase"].orEmpty()
        val finishedSuccessfully = terminal && phase == "completed"
        val detail = data["summary"].orEmpty().trim()
        val summary = when {
            finishedSuccessfully && (detail.isBlank() || detail == "已完成") -> "已完成 · 100%"
            finishedSuccessfully -> "已完成 · 100% · $detail"
            else -> detail.ifBlank { if (terminal) "已结束" else "正在执行" }
        }
        val title = data["title"].orEmpty().ifBlank {
            data["agentId"].orEmpty().ifBlank { "Amitia" }
        }
        val conversationTitle = data["conversationTitle"].orEmpty().trim()
            .ifBlank { data["conversation_title"].orEmpty().trim() }
            .ifBlank { data["chatTitle"].orEmpty().trim() }
            .ifBlank { title }
        val islandRoute = AgentTaskIslandRouting.choose(context)
        val timestamps = context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
        val startKey = "run_started_at:$runId"
        var startedAt = timestamps.getLong(startKey, 0L)
        if (startedAt <= 0L || revision <= 1L) {
            startedAt = System.currentTimeMillis()
            timestamps.edit().putLong(startKey, startedAt).apply()
        }
        val timeLabel = SimpleDateFormat("HH:mm", Locale.getDefault()).format(Date(startedAt))
        val deepLink = data["deepLink"].orEmpty().ifBlank { "amitia://run/$runId" }
        val currentStep = data["currentStep"]?.toIntOrNull()?.coerceAtLeast(0) ?: 0
        val totalSteps = data["totalSteps"]?.toIntOrNull()?.coerceAtLeast(0) ?: 0
        val progressFraction = data["progress"]?.toDoubleOrNull()?.coerceIn(0.0, 1.0) ?: 0.0
        val totalTokens = data["totalTokens"]?.toIntOrNull()?.coerceAtLeast(0) ?: 0
        val progress = when {
            finishedSuccessfully -> 100
            totalSteps > 0 -> ((currentStep.toDouble() / totalSteps.toDouble()) * 100.0).roundToInt().coerceIn(0, 100)
            progressFraction > 0 -> (progressFraction * 100.0).roundToInt().coerceIn(0, 100)
            terminal && phase == "completed" -> 100
            else -> 0
        }

        val showPercent = totalSteps > 0 || progressFraction > 0
        val notificationTitle = if (
            !terminal && showPercent && XiaomiIslandTaskCompat.supported(context)
        ) "$progress% · $title" else if (finishedSuccessfully) "✓ 已完成 · $title" else title
        val builder = NotificationCompat.Builder(context, CHANNEL_TASKS)
            .setSmallIcon(R.drawable.ic_notification)
            .setContentTitle(notificationTitle)
            .setContentText(summary)
            .setSubText("$conversationTitle · $timeLabel")
            .setWhen(startedAt)
            .setShowWhen(true)
            .setCategory(NotificationCompat.CATEGORY_PROGRESS)
            .setColor(android.graphics.Color.WHITE)
            .setColorized(false)
            .setOnlyAlertOnce(true)
            .setContentIntent(contentIntent(context, deepLink, stableId(runId).toString()))
            .setGroup("amitia:run")
            .setOngoing(!terminal || finishedSuccessfully)
            // System-owned expiry survives process death, unlike the main-thread Handler.
            // Keep the completed island for two minutes without leaving stale chips.
            .setTimeoutAfter(
                when (phase) {
                    "completed" -> 120_000L
                    "cancelled", "interrupted" -> 1_000L
                    "failed" -> 45_000L
                    else -> 0L
                },
            )
            .setAutoCancel(false)
            .setVisibility(NotificationCompat.VISIBILITY_PRIVATE)
            .setPublicVersion(
                publicNotification(
                    context,
                    CHANNEL_TASKS,
                    "Amitia 任务",
                    if (terminal) "任务状态已更新" else "后台任务正在执行",
                ),
            )

        if (Build.VERSION.SDK_INT >= 36 && (!terminal || finishedSuccessfully)) {
            val style = NotificationCompat.ProgressStyle()
                .setStyledByProgress(true)
            if (finishedSuccessfully) {
                style.addProgressSegment(NotificationCompat.ProgressStyle.Segment(100))
                style.setProgress(100)
            } else if (totalSteps > 0) {
                style.addProgressSegment(NotificationCompat.ProgressStyle.Segment(totalSteps.coerceAtLeast(1)))
                style.setProgress(currentStep.coerceIn(0, totalSteps))
            } else if (progress > 0) {
                style.addProgressSegment(NotificationCompat.ProgressStyle.Segment(100))
                style.setProgress(progress)
            } else {
                style.setProgressIndeterminate(true)
            }
            builder.setStyle(style)
            // Send vendor island extras when available, plus Android 16 Live Update
            // for cases where the ROM rejects this app's native island scene.
            // Android owns the fallback card theme and notification header.
            builder.setRequestPromotedOngoing(
                AgentTaskIslandRouting.shouldRequestLiveUpdate(),
            )
        } else if (!terminal) {
            if (progress > 0) {
                builder.setProgress(100, progress, false)
            } else {
                builder.setProgress(100, 0, true)
            }
        } else if (finishedSuccessfully) {
            builder.setProgress(100, 100, false)
        } else {
            builder.setProgress(0, 0, false)
        }

        if (islandRoute == AgentTaskIslandRouting.Route.XIAOMI_SUPER_ISLAND) {
            XiaomiIslandTaskCompat.attach(
                context, builder, conversationTitle, summary, progress, revision, terminal,
                timeLabel, phase, showPercent,
            )
        } else if (islandRoute == AgentTaskIslandRouting.Route.VIVO_ORIGIN_ISLAND) {
            VivoOriginIslandTaskCompat.attach(
                context, builder, conversationTitle, summary, progress, terminal,
                timeLabel, phase, showPercent,
            )
        }

        val manager = NotificationManagerCompat.from(context)
        val notificationId = stableId("run:$runId")
        manager.notify(notificationId, builder.build())
        if (terminal) {
            val dismissalDelayMs = when (phase) {
                "cancelled", "interrupted" -> 0L
                "completed" -> 120_000L
                else -> 45_000L
            }
            val dismiss = Runnable {
                manager.cancel(notificationId)
                timestamps.edit().remove(startKey).apply()
            }
            if (dismissalDelayMs == 0L) {
                dismiss.run()
            } else {
                Handler(Looper.getMainLooper()).postDelayed(
                    dismiss,
                    dismissalDelayMs,
                )
            }
        }
    }

    private fun dismissExecution(context: Context, data: Map<String, String>) {
        val runId = data["runId"].orEmpty().trim()
        if (runId.isEmpty()) return
        NotificationManagerCompat.from(context).cancel(stableId("run:$runId"))
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).edit()
            .putLong("run_terminal:$runId", System.currentTimeMillis())
            .apply()
    }

    fun clearAllExecution(context: Context) {
        val prefs = context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
        val runIds = prefs.all.keys
            .asSequence()
            .filter { it.startsWith("run_revision:") || it.startsWith("run_terminal:") }
            .map { it.substringAfter(':').trim() }
            .filter { it.isNotEmpty() }
            .toList()
        val manager = NotificationManagerCompat.from(context)
        runIds.forEach { runId ->
            manager.cancel(stableId("run:$runId"))
        }
        if (runIds.isNotEmpty()) {
            val editor = prefs.edit()
            runIds.forEach { runId ->
                editor.remove("run_revision:$runId")
                editor.remove("run_terminal:$runId")
                editor.remove("run_started_at:$runId")
            }
            editor.apply()
        }
    }

    private fun contentIntent(context: Context, deepLink: String, identity: String): PendingIntent {
        val intent = Intent(context, MainActivity::class.java)
            .setAction(Intent.ACTION_VIEW)
            .setData(Uri.parse(deepLink))
            .putExtra("amitia.deepLink", deepLink)
            .addFlags(Intent.FLAG_ACTIVITY_CLEAR_TOP or Intent.FLAG_ACTIVITY_SINGLE_TOP)
        return PendingIntent.getActivity(
            context,
            stableId("open:$identity"),
            intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
        )
    }

    private fun acceptRevision(context: Context, runId: String, revision: Long, terminal: Boolean): Boolean {
        val prefs = context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
        val key = "run_revision:$runId"
        val terminalKey = "run_terminal:$runId"
        val now = System.currentTimeMillis()
        val terminalAt = prefs.getLong(terminalKey, 0L)
        if (terminalAt > 0L && now - terminalAt in 0L..86_400_000L) return false
        if (terminalAt > 0L) {
            prefs.edit().remove(key).remove(terminalKey).apply()
        }
        val previous = prefs.getLong(key, -1L)
        if (revision > 0 && revision <= previous) return false
        val editor = prefs.edit().putLong(key, revision.coerceAtLeast(previous + 1))
        if (terminal) editor.putLong(terminalKey, now)
        editor.apply()
        return true
    }

    private fun clearRevision(context: Context, runId: String) {
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
            .edit()
            .remove("run_revision:$runId")
            .apply()
    }

    fun clearCall(context: Context, callId: String) {
        val normalized = callId.trim()
        if (normalized.isEmpty()) return
        NotificationManagerCompat.from(context)
            .cancel(stableId("call:$normalized"))
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
            .edit()
            .remove("call_active:$normalized")
            .apply()
    }

    fun clearAllCalls(context: Context) {
        val prefs = context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
        val callIds = prefs.all.keys
            .asSequence()
            .filter { it.startsWith("call_active:") }
            .map { it.removePrefix("call_active:").trim() }
            .filter { it.isNotEmpty() }
            .toList()
        val manager = NotificationManagerCompat.from(context)
        val editor = prefs.edit()
        callIds.forEach { callId ->
            manager.cancel(stableId("call:$callId"))
            editor.remove("call_active:$callId")
        }
        editor.apply()
    }

    fun clearAllMessages(context: Context) {
        val prefs = context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
        val conversationIds = prefs.all.keys
            .asSequence()
            .filter { it.startsWith("message_active:") || it.startsWith("history:") }
            .map {
                if (it.startsWith("message_active:")) {
                    it.removePrefix("message_active:")
                } else {
                    it.removePrefix("history:")
                }
            }
            .map { it.trim() }
            .filter { it.isNotEmpty() }
            .toSet()
        val manager = NotificationManagerCompat.from(context)
        val editor = prefs.edit()
        conversationIds.forEach { conversationId ->
            manager.cancel(stableId("message:$conversationId"))
            FloatingChatBubbleService.onConversationRead(conversationId)
            editor.remove("message_active:$conversationId")
            editor.remove("history:$conversationId")
        }
        prefs.all.keys
            .filter { it.startsWith("last_event:") || it.startsWith("recent_events:") }
            .forEach { editor.remove(it) }
        editor.apply()
    }

    fun clearAllReminders(context: Context) {
        val prefs = context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
        val ids = prefs.all.keys
            .asSequence()
            .filter { it.startsWith("reminder_active:") }
            .mapNotNull { prefs.getInt(it, Int.MIN_VALUE).takeIf { id -> id != Int.MIN_VALUE } }
            .toList()
        val manager = NotificationManagerCompat.from(context)
        val editor = prefs.edit()
        ids.forEach { manager.cancel(it) }
        prefs.all.keys
            .filter { it.startsWith("reminder_active:") }
            .forEach { editor.remove(it) }
        editor.apply()
    }

    fun clearConversation(context: Context, conversationId: String) {
        val normalized = conversationId.trim()
        if (normalized.isEmpty()) return
        clearMessageHistory(context, normalized)
        FloatingChatBubbleService.onConversationRead(normalized)
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
            .edit()
            .remove("message_active:$normalized")
            .apply()
        NotificationManagerCompat.from(context)
            .cancel(stableId("message:$normalized"))
    }

    private fun clearMessageHistory(context: Context, conversationId: String) {
        val normalized = conversationId.trim()
        if (normalized.isEmpty()) return
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
            .edit()
            .remove("history:$normalized")
            .apply()
    }

    private fun appendMessageHistory(context: Context, conversationId: String, timestamp: Long, body: String) {
        if (conversationId.isBlank()) return
        val prefs = context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
        val key = "history:$conversationId"
        val array = runCatching { JSONArray(prefs.getString(key, "[]")) }.getOrDefault(JSONArray())
        array.put(JSONObject().put("timestamp", timestamp).put("body", body))
        val trimmed = JSONArray()
        val start = (array.length() - MAX_HISTORY).coerceAtLeast(0)
        for (index in start until array.length()) {
            trimmed.put(array.getJSONObject(index))
        }
        prefs.edit().putString(key, trimmed.toString()).apply()
    }

    private fun loadMessageHistory(context: Context, conversationId: String): List<Pair<Long, String>> {
        if (conversationId.isBlank()) return emptyList()
        val prefs = context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
        val array = runCatching { JSONArray(prefs.getString("history:$conversationId", "[]")) }.getOrDefault(JSONArray())
        return buildList {
            for (index in 0 until array.length()) {
                val item = array.optJSONObject(index) ?: continue
                val body = item.optString("body")
                if (body.isNotBlank()) add(item.optLong("timestamp") to body)
            }
        }
    }

    private fun formatTokenCount(value: Int): String {
        return when {
            value >= 1_000_000 -> {
                val amount = value.toDouble() / 1_000_000.0
                val formatted = if (amount >= 10) "%.0fM" else "%.1fM"
                formatted.format(amount) + " tokens"
            }
            value >= 1_000 -> {
                val amount = value.toDouble() / 1_000.0
                val formatted = if (amount >= 10) "%.0fK" else "%.1fK"
                formatted.format(amount) + " tokens"
            }
            else -> "$value tokens"
        }
    }

    private fun stableId(value: String): Int = value.hashCode() and 0x7fffffff

    private fun canUseFullScreenCall(context: Context): Boolean {
        if (Build.VERSION.SDK_INT < 34) return true
        val manager = context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        return manager.canUseFullScreenIntent()
    }

    private fun pendingIntentMutableFlag(): Int {
        return if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) PendingIntent.FLAG_MUTABLE else 0
    }
}
