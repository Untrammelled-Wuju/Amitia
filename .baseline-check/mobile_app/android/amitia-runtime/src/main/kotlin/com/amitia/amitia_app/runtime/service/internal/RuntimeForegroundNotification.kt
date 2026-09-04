package com.amitia.amitia_app.runtime.service.internal

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.content.Context
import android.os.Build
import androidx.core.app.NotificationCompat
import com.amitia.amitia_app.runtime.service.RuntimeServiceContract

internal class RuntimeForegroundNotification(
    private val context: Context
) {
    private val manager: NotificationManager? =
        context.getSystemService(Context.NOTIFICATION_SERVICE) as? NotificationManager

    fun createNotification(): RuntimeForegroundNotificationResult {
        val currentManager = manager ?: return RuntimeForegroundNotificationResult.Failure(
            "NotificationManager not available"
        )

        try {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                val channel = NotificationChannel(
                    RuntimeServiceContract.NOTIFICATION_CHANNEL_ID,
                    "Amitia Runtime",
                    NotificationManager.IMPORTANCE_LOW
                ).apply {
                    description = "Keeps Amitia Runtime running"
                    setShowBadge(false)
                }
                currentManager.createNotificationChannel(channel)
            }

            val notification = NotificationCompat.Builder(context, RuntimeServiceContract.NOTIFICATION_CHANNEL_ID)
                .setContentTitle("Amitia")
                .setContentText("Amitia Runtime 正在运行")
                .setSmallIcon(android.R.drawable.ic_dialog_info)
                .setOngoing(true)
                .setPriority(NotificationCompat.PRIORITY_LOW)
                .setCategory(NotificationCompat.CATEGORY_SERVICE)
                .build()

            return RuntimeForegroundNotificationResult.Success(notification)
        } catch (e: Exception) {
            return RuntimeForegroundNotificationResult.Failure("failed to create notification: ${e.message}")
        }
    }
}

internal sealed interface RuntimeForegroundNotificationResult {
    data class Success(val notification: Notification) : RuntimeForegroundNotificationResult
    data class Failure(val reason: String) : RuntimeForegroundNotificationResult
}
