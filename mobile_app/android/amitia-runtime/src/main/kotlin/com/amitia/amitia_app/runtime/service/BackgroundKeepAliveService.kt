package com.amitia.amitia_app.runtime.service

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Intent
import android.content.pm.ServiceInfo
import android.os.Build
import android.os.IBinder

class BackgroundKeepAliveService : Service() {
    override fun onBind(intent: Intent?): IBinder? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (!BackgroundKeepAlive.enabled(this) || BackgroundKeepAlive.usingRuntime) {
            stopSelf()
            return START_NOT_STICKY
        }
        return try {
            val manager = getSystemService(NOTIFICATION_SERVICE) as NotificationManager
            val channelId = "amitia_background_keepalive"
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                manager.createNotificationChannel(NotificationChannel(channelId, "后台保活", NotificationManager.IMPORTANCE_LOW).apply { setShowBadge(false) })
            }
            val builder = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) Notification.Builder(this, channelId) else Notification.Builder(this)
            builder.setSmallIcon(android.R.drawable.ic_dialog_info)
                .setContentTitle("Amitia 后台保活")
                .setContentText("正在维持后台运行，可在系统设置中关闭")
                .setOngoing(true)
                .setCategory(Notification.CATEGORY_SERVICE)
            packageManager.getLaunchIntentForPackage(packageName)?.let { launch ->
                builder.setContentIntent(PendingIntent.getActivity(this, 0, launch, PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE))
            }
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) {
                startForeground(NOTIFICATION_ID, builder.build(), ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE)
            } else {
                startForeground(NOTIFICATION_ID, builder.build())
            }
            BackgroundKeepAlive.standaloneStarted()
            START_STICKY
        } catch (error: Exception) {
            BackgroundKeepAlive.standaloneFailed(error.message ?: "后台保活启动失败")
            stopSelf()
            START_NOT_STICKY
        }
    }

    override fun onDestroy() {
        BackgroundKeepAlive.standaloneStopped()
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.N) stopForeground(STOP_FOREGROUND_REMOVE)
        else stopForeground(true)
        super.onDestroy()
    }

    companion object { private const val NOTIFICATION_ID = 0x6A0501 }
}
