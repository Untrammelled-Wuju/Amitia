package com.amitia.amitia_app.update

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Context
import android.content.Intent
import android.content.pm.ServiceInfo
import android.net.ConnectivityManager
import android.net.NetworkCapabilities
import android.os.Build
import android.os.Environment
import android.os.IBinder
import com.amitia.amitia_app.MainActivity
import com.amitia.amitia_app.R
import java.io.File
import java.util.concurrent.Executors
import java.util.concurrent.atomic.AtomicBoolean

class SegmentedApkDownloadService : Service() {
    companion object {
        const val ACTION_START = "com.amitia.amitia_app.update.DOWNLOAD"
        const val EXTRA_DOWNLOAD_ID = "downloadId"
        const val EXTRA_URL = "url"
        const val EXTRA_FILE_NAME = "fileName"
        const val EXTRA_EXPECTED_SIZE = "expectedSize"
        const val EXTRA_EXPECTED_SHA256 = "expectedSha256"
        private const val CHANNEL_ID = "amitia_app_update"
        private const val NOTIFICATION_ID = 7315

        fun start(
            context: Context,
            downloadId: Long,
            url: String,
            fileName: String,
            expectedSize: Long,
            expectedSha256: String,
        ) {
            val intent = Intent(context, SegmentedApkDownloadService::class.java)
                .setAction(ACTION_START)
                .putExtra(EXTRA_DOWNLOAD_ID, downloadId)
                .putExtra(EXTRA_URL, url)
                .putExtra(EXTRA_FILE_NAME, fileName)
                .putExtra(EXTRA_EXPECTED_SIZE, expectedSize)
                .putExtra(EXTRA_EXPECTED_SHA256, expectedSha256)
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                context.startForegroundService(intent)
            } else {
                context.startService(intent)
            }
        }
    }

    private val executor = Executors.newSingleThreadExecutor()
    private val running = AtomicBoolean(false)
    @Volatile private var cancelled = false
    private var currentDownloadId = 0L

    override fun onCreate() {
        super.onCreate()
        createNotificationChannel()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (intent?.action != ACTION_START) {
            stopSelf(startId)
            return START_NOT_STICKY
        }
        val downloadId = intent.getLongExtra(EXTRA_DOWNLOAD_ID, 0L)
        val url = intent.getStringExtra(EXTRA_URL).orEmpty()
        val fileName = intent.getStringExtra(EXTRA_FILE_NAME).orEmpty()
        val expectedSize = intent.getLongExtra(EXTRA_EXPECTED_SIZE, 0L)
        val expectedSha256 = intent.getStringExtra(EXTRA_EXPECTED_SHA256).orEmpty()
        if (downloadId <= 0L || url.isEmpty() || fileName.isEmpty() || expectedSize <= 0L) {
            stopSelf(startId)
            return START_NOT_STICKY
        }
        if (!running.compareAndSet(false, true)) {
            AppUpdateDownloadStore.markFailed(this, downloadId, "已有更新下载任务正在运行", 1001)
            return START_NOT_STICKY
        }
        currentDownloadId = downloadId
        startForegroundCompat(buildNotification(0, expectedSize))
        AppUpdateDownloadStore.markRunning(this, downloadId)
        executor.execute {
            try {
                val root = getExternalFilesDir(Environment.DIRECTORY_DOWNLOADS)
                    ?: File(filesDir, "downloads")
                root.mkdirs()
                val target = File(root, fileName)
                val downloader = SegmentedDownloader(
                    concurrency = resolveConcurrency(),
                )
                var lastStoreUpdate = 0L
                downloader.download(
                    url = url,
                    target = target,
                    expectedSize = expectedSize,
                    expectedSha256 = expectedSha256,
                    onProgress = { downloaded, total ->
                        val now = System.currentTimeMillis()
                        if (now - lastStoreUpdate >= 500 || downloaded == total) {
                            lastStoreUpdate = now
                            AppUpdateDownloadStore.updateProgress(this, downloadId, downloaded, total)
                            notificationManager().notify(
                                NOTIFICATION_ID,
                                buildNotification(downloaded, total),
                            )
                        }
                    },
                    isCancelled = { cancelled },
                )
                AppUpdateDownloadStore.markSucceeded(this, downloadId, target, expectedSize)
            } catch (error: Throwable) {
                AppUpdateDownloadStore.markFailed(
                    this,
                    downloadId,
                    error.message ?: "更新下载失败",
                    1002,
                )
            } finally {
                running.set(false)
                stopForegroundCompat()
                stopSelf()
            }
        }
        return START_NOT_STICKY
    }

    override fun onDestroy() {
        cancelled = true
        executor.shutdownNow()
        if (running.get() && currentDownloadId > 0L) {
            AppUpdateDownloadStore.markFailed(this, currentDownloadId, "更新下载已停止", 1003)
        }
        running.set(false)
        super.onDestroy()
    }

    override fun onBind(intent: Intent?): IBinder? = null

    private fun resolveConcurrency(): Int {
        val manager = getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager
        val capabilities = manager.getNetworkCapabilities(manager.activeNetwork)
        return when {
            capabilities?.hasTransport(NetworkCapabilities.TRANSPORT_ETHERNET) == true -> 6
            capabilities?.hasTransport(NetworkCapabilities.TRANSPORT_WIFI) == true -> 4
            capabilities?.hasTransport(NetworkCapabilities.TRANSPORT_CELLULAR) == true -> 2
            else -> 3
        }
    }

    private fun buildNotification(downloaded: Long, total: Long): Notification {
        val builder = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            Notification.Builder(this, CHANNEL_ID)
        } else {
            Notification.Builder(this)
        }
        val contentIntent = PendingIntent.getActivity(
            this,
            0,
            Intent(this, MainActivity::class.java),
            PendingIntent.FLAG_UPDATE_CURRENT or pendingIntentImmutableFlag(),
        )
        val percent = if (total > 0) {
            (downloaded * 100 / total).coerceIn(0, 100).toInt()
        } else {
            0
        }
        return builder
            .setSmallIcon(R.drawable.ic_notification)
            .setContentTitle("正在下载 Amitia 更新")
            .setContentText("$percent% · ${formatBytes(downloaded)} / ${formatBytes(total)}")
            .setContentIntent(contentIntent)
            .setOngoing(true)
            .setOnlyAlertOnce(true)
            .setProgress(100, percent, total <= 0)
            .build()
    }

    private fun startForegroundCompat(notification: Notification) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            startForeground(
                NOTIFICATION_ID,
                notification,
                ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC,
            )
        } else {
            startForeground(NOTIFICATION_ID, notification)
        }
    }

    private fun stopForegroundCompat() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.N) {
            stopForeground(STOP_FOREGROUND_REMOVE)
        } else {
            @Suppress("DEPRECATION")
            stopForeground(true)
        }
    }

    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return
        val manager = notificationManager()
        if (manager.getNotificationChannel(CHANNEL_ID) == null) {
            manager.createNotificationChannel(
                NotificationChannel(
                    CHANNEL_ID,
                    "应用更新下载",
                    NotificationManager.IMPORTANCE_LOW,
                ).apply {
                    description = "显示 Amitia 应用更新下载进度"
                    setShowBadge(false)
                },
            )
        }
    }

    private fun notificationManager() =
        getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager

    private fun pendingIntentImmutableFlag(): Int =
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) PendingIntent.FLAG_IMMUTABLE else 0

    private fun formatBytes(value: Long): String {
        if (value < 1024) return "$value B"
        if (value < 1024 * 1024) return String.format("%.1f KB", value / 1024.0)
        if (value < 1024L * 1024 * 1024) return String.format("%.1f MB", value / 1024.0 / 1024.0)
        return String.format("%.2f GB", value / 1024.0 / 1024.0 / 1024.0)
    }
}
