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
import android.os.Build
import android.os.IBinder
import android.os.SystemClock
import android.os.UserManager
import android.provider.Settings
import android.view.Gravity
import android.view.MotionEvent
import android.view.View
import android.view.WindowManager
import android.widget.TextView
import com.amitia.amitia_app.MainActivity
import com.amitia.amitia_app.R
import kotlin.math.abs

class FloatingChatBubbleService : Service() {
    companion object {
        private const val PREFS = "amitia_floating_chat"
        private const val ENABLED = "enabled"
        private const val CHANNEL = "amitia_floating_chat"
        private const val NOTIFICATION_ID = 172004
        @Volatile private var current: FloatingChatBubbleService? = null
        @Volatile private var appForeground = false

        fun status(context: Context): Map<String, Any> {
            val permitted = Build.VERSION.SDK_INT < Build.VERSION_CODES.M || Settings.canDrawOverlays(context)
            return mapOf(
                "supported" to true,
                "permissionGranted" to permitted,
                "enabled" to context.getSharedPreferences(PREFS, MODE_PRIVATE).getBoolean(ENABLED, false),
                "visible" to (current?.bubble?.visibility == View.VISIBLE),
                "active" to (current != null),
            )
        }

        fun enable(context: Context): Boolean {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M && !Settings.canDrawOverlays(context)) {
                return false
            }
            context.getSharedPreferences(PREFS, MODE_PRIVATE).edit().putBoolean(ENABLED, true).apply()
            start(context)
            return true
        }

        fun disable(context: Context) {
            context.getSharedPreferences(PREFS, MODE_PRIVATE).edit().putBoolean(ENABLED, false).apply()
            context.stopService(Intent(context, FloatingChatBubbleService::class.java))
        }

        fun restore(context: Context) {
            if (context.getSharedPreferences(PREFS, MODE_PRIVATE).getBoolean(ENABLED, false) &&
                (Build.VERSION.SDK_INT < Build.VERSION_CODES.M || Settings.canDrawOverlays(context))
            ) {
                start(context)
            }
        }

        fun setAppForeground(foreground: Boolean) {
            appForeground = foreground
            current?.refreshVisibility()
        }

        private fun start(context: Context) {
            val intent = Intent(context, FloatingChatBubbleService::class.java)
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                context.startForegroundService(intent)
            } else {
                context.startService(intent)
            }
        }
    }

    private lateinit var windowManager: WindowManager
    private var bubble: TextView? = null
    private var params: WindowManager.LayoutParams? = null
    private var receiverRegistered = false
    private val displayReceiver = object : BroadcastReceiver() {
        override fun onReceive(context: Context, intent: Intent) {
            refreshVisibility()
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
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            registerReceiver(displayReceiver, filter, Context.RECEIVER_NOT_EXPORTED)
        } else {
            registerReceiver(displayReceiver, filter)
        }
        receiverRegistered = true
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (!getSharedPreferences(PREFS, MODE_PRIVATE).getBoolean(ENABLED, false) ||
            (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M && !Settings.canDrawOverlays(this))
        ) {
            stopSelf()
            return START_NOT_STICKY
        }
        val manager = getSystemService(NOTIFICATION_SERVICE) as NotificationManager
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            manager.createNotificationChannel(
                NotificationChannel(CHANNEL, "Amitia 悬浮球", NotificationManager.IMPORTANCE_LOW)
            )
        }
        val open = PendingIntent.getActivity(
            this, NOTIFICATION_ID,
            Intent(this, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_SINGLE_TOP),
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
        )
        val builder = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            Notification.Builder(this, CHANNEL)
        } else {
            Notification.Builder(this)
        }
        val notification = builder.setSmallIcon(R.drawable.ic_notification)
            .setContentTitle("Amitia 悬浮球已启用")
            .setContentText("返回桌面查看，长按悬浮球可关闭")
            .setContentIntent(open)
            .setOngoing(true)
            .build()
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) {
            startForeground(NOTIFICATION_ID, notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE)
        } else {
            startForeground(NOTIFICATION_ID, notification)
        }
        if (bubble == null) createBubble()
        refreshVisibility()
        return START_STICKY
    }

    private fun createBubble() {
        val size = (58 * resources.displayMetrics.density).toInt()
        val initialX = getSharedPreferences(PREFS, MODE_PRIVATE).getInt("x", 8)
        val initialY = getSharedPreferences(PREFS, MODE_PRIVATE).getInt("y", 260)
        val lp = WindowManager.LayoutParams(
            size, size,
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O)
                WindowManager.LayoutParams.TYPE_APPLICATION_OVERLAY
            else WindowManager.LayoutParams.TYPE_PHONE,
            WindowManager.LayoutParams.FLAG_NOT_FOCUSABLE or
                WindowManager.LayoutParams.FLAG_NOT_TOUCH_MODAL or
                WindowManager.LayoutParams.FLAG_LAYOUT_IN_SCREEN,
            PixelFormat.TRANSLUCENT,
        ).apply {
            gravity = Gravity.TOP or Gravity.START
            x = initialX
            y = initialY
        }
        val control = TextView(this).apply {
            text = "AI"
            textSize = 19f
            setTextColor(Color.WHITE)
            gravity = Gravity.CENTER
            contentDescription = "Amitia 悬浮聊天球，点击打开应用，长按关闭"
            background = GradientDrawable().apply {
                shape = GradientDrawable.OVAL
                setColor(Color.rgb(94, 125, 233))
                setStroke((2 * resources.displayMetrics.density).toInt(), Color.WHITE)
            }
            elevation = (6 * resources.displayMetrics.density)
        }
        var startRawX = 0f
        var startRawY = 0f
        var originX = 0
        var originY = 0
        var started = 0L
        var moved = false
        control.setOnTouchListener { _: View, event: MotionEvent ->
            when (event.actionMasked) {
                MotionEvent.ACTION_DOWN -> {
                    startRawX = event.rawX
                    startRawY = event.rawY
                    originX = lp.x
                    originY = lp.y
                    started = SystemClock.uptimeMillis()
                    moved = false
                    true
                }
                MotionEvent.ACTION_MOVE -> {
                    val dx = event.rawX - startRawX
                    val dy = event.rawY - startRawY
                    if (abs(dx) > 12 || abs(dy) > 12) moved = true
                    if (moved) {
                        lp.x = (originX + dx).toInt().coerceAtLeast(0)
                        lp.y = (originY + dy).toInt().coerceAtLeast(0)
                        runCatching { windowManager.updateViewLayout(control, lp) }
                    }
                    true
                }
                MotionEvent.ACTION_UP -> {
                    if (moved) {
                        getSharedPreferences(PREFS, MODE_PRIVATE).edit()
                            .putInt("x", lp.x).putInt("y", lp.y).apply()
                    } else if (SystemClock.uptimeMillis() - started >= 700L) {
                        disable(this)
                    } else {
                        startActivity(
                            Intent(this, MainActivity::class.java)
                                .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_SINGLE_TOP)
                        )
                    }
                    true
                }
                else -> true
            }
        }
        try {
            windowManager.addView(control, lp)
            bubble = control
            params = lp
        } catch (_: Exception) {
            stopSelf()
        }
    }

    private fun refreshVisibility() {
        val view = bubble ?: return
        val power = getSystemService(POWER_SERVICE) as android.os.PowerManager
        val user = getSystemService(USER_SERVICE) as UserManager
        val keyguard = getSystemService(KEYGUARD_SERVICE) as android.app.KeyguardManager
        val canShow = !appForeground && power.isInteractive && user.isUserUnlocked &&
            !keyguard.isKeyguardLocked &&
            (Build.VERSION.SDK_INT < Build.VERSION_CODES.M || Settings.canDrawOverlays(this))
        view.visibility = if (canShow) View.VISIBLE else View.GONE
    }

    override fun onDestroy() {
        if (receiverRegistered) {
            unregisterReceiver(displayReceiver)
            receiverRegistered = false
        }
        bubble?.let { runCatching { windowManager.removeView(it) } }
        bubble = null
        params = null
        if (current === this) current = null
        super.onDestroy()
    }
}
