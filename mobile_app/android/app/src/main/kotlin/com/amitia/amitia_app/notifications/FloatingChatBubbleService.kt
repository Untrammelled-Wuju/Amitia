package com.amitia.amitia_app.notifications

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.content.pm.ServiceInfo
import android.graphics.Color
import android.graphics.PixelFormat
import android.graphics.drawable.GradientDrawable
import android.net.Uri
import android.os.Build
import android.os.Handler
import android.os.IBinder
import android.os.Looper
import android.os.PowerManager
import android.os.UserManager
import android.provider.Settings
import android.util.Log
import android.view.Gravity
import android.view.View
import android.view.WindowManager
import android.widget.LinearLayout
import android.widget.TextView
import com.amitia.amitia_app.MainActivity
import com.amitia.amitia_app.R

class FloatingChatBubbleService : Service() {
    companion object {
        private const val PREFS = "amitia_floating_chat"
        private const val ENABLED = "enabled"
        private const val PREVIEW = "preview_enabled"
        private const val LAST_ERROR = "last_error"
        private const val TAG = "AmitiaFloatingChat"
        private const val CHANNEL = "amitia_floating_chat"
        private const val NOTIFICATION_ID = 172004
        @Volatile private var current: FloatingChatBubbleService? = null
        @Volatile private var appForeground = false

        fun status(context: Context): Map<String, Any> {
            val prefs = context.getSharedPreferences(PREFS, MODE_PRIVATE)
            val permitted = Build.VERSION.SDK_INT < Build.VERSION_CODES.M || Settings.canDrawOverlays(context)
            return mapOf(
                "supported" to true,
                "permissionGranted" to permitted,
                "enabled" to prefs.getBoolean(ENABLED, false),
                "previewEnabled" to prefs.getBoolean(PREVIEW, true),
                "lastError" to prefs.getString(LAST_ERROR, "").orEmpty(),
                "visible" to (current?.preview != null),
                "active" to (current != null),
                "unreadCount" to (current?.messages?.size ?: 0),
            )
        }

        fun configure(context: Context, previewEnabled: Boolean): Map<String, Any> {
            context.getSharedPreferences(PREFS, MODE_PRIVATE).edit()
                .putBoolean(PREVIEW, previewEnabled).apply()
            current?.mainHandler?.post {
                if (!previewEnabled) current?.hidePreview()
            }
            return status(context)
        }

        fun enable(context: Context): Boolean {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M && !Settings.canDrawOverlays(context)) return false
            val prefs = context.getSharedPreferences(PREFS, MODE_PRIVATE)
            prefs.edit().putBoolean(ENABLED, true).remove(LAST_ERROR).apply()
            return try {
                start(context)
                true
            } catch (error: Exception) {
                Log.e(TAG, "Unable to start message preview service", error)
                prefs.edit().putBoolean(ENABLED, false)
                    .putString(LAST_ERROR, error.javaClass.simpleName + ": " + error.message).apply()
                false
            }
        }

        fun disable(context: Context) {
            context.getSharedPreferences(PREFS, MODE_PRIVATE).edit()
                .putBoolean(ENABLED, false).apply()
            context.stopService(Intent(context, FloatingChatBubbleService::class.java))
        }

        fun restore(context: Context) {
            if (context.getSharedPreferences(PREFS, MODE_PRIVATE).getBoolean(ENABLED, false) &&
                (Build.VERSION.SDK_INT < Build.VERSION_CODES.M || Settings.canDrawOverlays(context))
            ) start(context)
        }

        fun setAppForeground(foreground: Boolean) {
            appForeground = foreground
            if (foreground) current?.mainHandler?.post { current?.hidePreview() }
        }

        fun onMessage(context: Context, conversationId: String, title: String, body: String) {
            if (conversationId.isBlank()) return
            val service = current ?: return
            service.mainHandler.post {
                if (current !== service ||
                    !context.getSharedPreferences(PREFS, MODE_PRIVATE).getBoolean(ENABLED, false)
                ) return@post
                service.messages.remove(conversationId)
                service.messages[conversationId] = PreviewMessage(
                    conversationId, title.take(50), body.take(220),
                )
                while (service.messages.size > 9) service.messages.remove(service.messages.keys.first())
                service.showPreview()
            }
        }

        fun onConversationRead(conversationId: String) {
            val service = current ?: return
            service.mainHandler.post {
                service.messages.remove(conversationId)
                if (service.messages.isEmpty()) service.hidePreview()
            }
        }

        private fun start(context: Context) {
            val intent = Intent(context, FloatingChatBubbleService::class.java)
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) context.startForegroundService(intent)
            else context.startService(intent)
        }
    }

    private data class PreviewMessage(val id: String, val title: String, val body: String)
    private val mainHandler = Handler(Looper.getMainLooper())
    private val messages = linkedMapOf<String, PreviewMessage>()
    private lateinit var windowManager: WindowManager
    private var preview: LinearLayout? = null
    private var hideTask: Runnable? = null
    private var receiverRegistered = false

    private fun dp(value: Int): Int = (value * resources.displayMetrics.density + 0.5f).toInt()

    private val displayReceiver = object : BroadcastReceiver() {
        override fun onReceive(context: Context, intent: Intent) {
            if (!canShowPreview()) hidePreview()
        }
    }

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onCreate() {
        super.onCreate()
        current = this
        windowManager = getSystemService(WINDOW_SERVICE) as WindowManager
        val filter = IntentFilter().apply {
            addAction(Intent.ACTION_SCREEN_OFF)
            addAction(Intent.ACTION_SCREEN_ON)
            addAction(Intent.ACTION_USER_PRESENT)
        }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU)
            registerReceiver(displayReceiver, filter, Context.RECEIVER_NOT_EXPORTED)
        else registerReceiver(displayReceiver, filter)
        receiverRegistered = true
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (!getSharedPreferences(PREFS, MODE_PRIVATE).getBoolean(ENABLED, false) ||
            (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M && !Settings.canDrawOverlays(this))) {
            stopSelf()
            return START_NOT_STICKY
        }
        val manager = getSystemService(NOTIFICATION_SERVICE) as NotificationManager
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            manager.createNotificationChannel(
                NotificationChannel(CHANNEL, "Amitia 悬浮消息预览", NotificationManager.IMPORTANCE_LOW),
            )
        }
        val open = PendingIntent.getActivity(
            this, NOTIFICATION_ID,
            Intent(this, MainActivity::class.java)
                .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_SINGLE_TOP),
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
        )
        val builder = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O)
            Notification.Builder(this, CHANNEL) else Notification.Builder(this)
        val notification = builder.setSmallIcon(R.drawable.ic_notification)
            .setContentTitle("Amitia 消息预览")
            .setContentText("仅在解锁且应用处于后台时显示顶部消息预览")
            .setContentIntent(open)
            .setOngoing(true)
            .setVisibility(Notification.VISIBILITY_SECRET)
            .build()
        try {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE)
                startForeground(NOTIFICATION_ID, notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE)
            else startForeground(NOTIFICATION_ID, notification)
        } catch (error: Exception) {
            Log.e(TAG, "Unable to start message preview service", error)
            getSharedPreferences(PREFS, MODE_PRIVATE).edit()
                .putBoolean(ENABLED, false)
                .putString(LAST_ERROR, error.javaClass.simpleName + ": " + error.message).apply()
            stopSelf()
            return START_NOT_STICKY
        }
        return START_STICKY
    }

    private fun canShowPreview(): Boolean {
        if (appForeground || !getSharedPreferences(PREFS, MODE_PRIVATE).getBoolean(PREVIEW, true)) return false
        val power = getSystemService(POWER_SERVICE) as PowerManager
        val user = getSystemService(USER_SERVICE) as UserManager
        val keyguard = getSystemService(KEYGUARD_SERVICE) as android.app.KeyguardManager
        return power.isInteractive && user.isUserUnlocked && !keyguard.isKeyguardLocked &&
            (Build.VERSION.SDK_INT < Build.VERSION_CODES.M || Settings.canDrawOverlays(this))
    }

    private fun showPreview() {
        hidePreview()
        if (!canShowPreview()) return
        val item = messages.values.lastOrNull() ?: return
        val panel = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(dp(15), dp(11), dp(15), dp(11))
            background = GradientDrawable().apply {
                setColor(Color.BLACK)
                cornerRadius = dp(19).toFloat()
                setStroke(dp(1), Color.rgb(45, 45, 45))
            }
            elevation = dp(8).toFloat()
            contentDescription = "Amitia 新消息：" + item.title
        }
        panel.addView(TextView(this).apply {
            text = item.title
            textSize = 14f
            setTextColor(Color.WHITE)
            typeface = android.graphics.Typeface.DEFAULT_BOLD
            maxLines = 1
            ellipsize = android.text.TextUtils.TruncateAt.END
        })
        panel.addView(TextView(this).apply {
            text = item.body.ifBlank { "收到一条新消息" }
            textSize = 13f
            setTextColor(Color.rgb(225, 225, 225))
            maxLines = 2
            ellipsize = android.text.TextUtils.TruncateAt.END
            setPadding(0, dp(4), 0, 0)
        })
        panel.setOnClickListener {
            hidePreview()
            val deepLink = "amitia://chat/" + Uri.encode(item.id)
            val intent = Intent(this, MainActivity::class.java)
                .setAction(Intent.ACTION_VIEW)
                .setData(Uri.parse(deepLink))
                .putExtra("amitia.deepLink", deepLink)
                .putExtra("amitia.conversationId", item.id)
                .addFlags(
                    Intent.FLAG_ACTIVITY_NEW_TASK or
                        Intent.FLAG_ACTIVITY_CLEAR_TOP or Intent.FLAG_ACTIVITY_SINGLE_TOP,
                )
            runCatching { startActivity(intent) }
                .onFailure { Log.w(TAG, "Unable to open full conversation", it) }
        }
        val width = dp(300).coerceAtMost(
            (resources.displayMetrics.widthPixels - dp(28)).coerceAtLeast(dp(160)),
        )
        val lp = WindowManager.LayoutParams(
            width,
            WindowManager.LayoutParams.WRAP_CONTENT,
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O)
                WindowManager.LayoutParams.TYPE_APPLICATION_OVERLAY
            else WindowManager.LayoutParams.TYPE_PHONE,
            WindowManager.LayoutParams.FLAG_NOT_FOCUSABLE or
                WindowManager.LayoutParams.FLAG_NOT_TOUCH_MODAL or
                WindowManager.LayoutParams.FLAG_LAYOUT_IN_SCREEN,
            PixelFormat.TRANSLUCENT,
        ).apply {
            gravity = Gravity.TOP or Gravity.START
            y = dp(44)
            x = ((resources.displayMetrics.widthPixels - width) / 2).coerceAtLeast(0)
        }
        try {
            panel.alpha = 0f
            panel.translationY = -dp(10).toFloat()
            windowManager.addView(panel, lp)
            panel.animate().alpha(1f).translationY(0f).setDuration(220L).start()
            preview = panel
            hideTask = Runnable { hidePreview() }.also { mainHandler.postDelayed(it, 4200L) }
        } catch (error: Exception) {
            Log.w(TAG, "Unable to show message preview", error)
        }
    }

    private fun hidePreview() {
        hideTask?.let { mainHandler.removeCallbacks(it) }
        hideTask = null
        preview?.let { runCatching { windowManager.removeView(it) } }
        preview = null
    }

    override fun onDestroy() {
        hidePreview()
        if (receiverRegistered) {
            unregisterReceiver(displayReceiver)
            receiverRegistered = false
        }
        messages.clear()
        if (current === this) current = null
        super.onDestroy()
    }
}
