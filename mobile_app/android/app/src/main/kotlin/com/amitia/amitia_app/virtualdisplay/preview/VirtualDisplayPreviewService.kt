package com.amitia.amitia_app.virtualdisplay.preview

import android.animation.ValueAnimator
import android.app.KeyguardManager
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
import android.graphics.Bitmap
import android.graphics.Color
import android.graphics.BitmapFactory
import android.graphics.PixelFormat
import android.graphics.drawable.GradientDrawable
import android.net.Uri
import android.os.Build
import android.os.Handler
import android.os.IBinder
import android.os.Looper
import android.os.PowerManager
import android.provider.Settings
import android.util.Log
import android.view.Gravity
import android.view.HapticFeedbackConstants
import android.view.MotionEvent
import android.view.View
import android.view.ViewGroup
import android.view.ViewOutlineProvider
import android.view.WindowManager
import android.view.animation.DecelerateInterpolator
import android.widget.FrameLayout
import android.widget.ImageView
import android.widget.TextView
import com.amitia.amitia_app.MainActivity
import com.amitia.amitia_app.R
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeRequest
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeResponse
import com.amitia.amitia_app.virtualdisplay.host.VirtualDisplayHostError
import com.amitia.amitia_app.virtualdisplay.host.VirtualDisplayHostManager
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import kotlinx.coroutines.Job
import kotlin.math.abs
import kotlin.math.max
import kotlin.math.min
import kotlin.math.roundToInt

class VirtualDisplayPreviewService : Service() {
    companion object {
        private const val PREFS = "amitia_virtual_display_preview"
        private const val KEY_ENABLED = "enabled"
        private const val KEY_SUPPRESSED = "suppressed"
        private const val KEY_TARGET_DISPLAY_ID = "target_display_id"
        private const val KEY_WIDTH_RATIO = "width_ratio"
        private const val KEY_X_RATIO = "x_ratio"
        private const val KEY_Y_RATIO = "y_ratio"
        private const val KEY_LAST_ERROR = "last_error"
        private const val CHANNEL_ID = "amitia_virtual_display_preview"
        private const val NOTIFICATION_ID = 172005
        private const val TAG = "AmitiaVirtualPreview"
        private const val ACTION_OPEN_PERMISSION = "open_permission"
        private const val ACTION_SHOW_DISPLAY = "show_display"
        private const val EXTRA_DISPLAY_ID = "display_id"

        @Volatile
        private var current: VirtualDisplayPreviewService? = null

        @Volatile
        private var runtimeState: String = "disabled"

        fun isEnabled(context: Context): Boolean = preferences(context).getBoolean(KEY_ENABLED, false)

        fun setEnabled(context: Context, enabled: Boolean): Map<String, Any?> {
            val prefs = preferences(context)
            prefs.edit().putBoolean(KEY_ENABLED, enabled).apply()
            if (!enabled) {
                prefs.edit()
                    .putBoolean(KEY_SUPPRESSED, false)
                    .putInt(KEY_TARGET_DISPLAY_ID, -1)
                    .apply()
                context.stopService(Intent(context, VirtualDisplayPreviewService::class.java))
                runtimeState = "disabled"
                return status(context)
            }
            if (!overlayPermissionGranted(context)) {
                prefs.edit().putString(
                    KEY_LAST_ERROR,
                    "系统悬浮窗权限未授予",
                ).apply()
                runtimeState = "permission_required"
                return status(context)
            }
            ensureRunning(context)
            return status(context)
        }

        fun ensureRunning(context: Context) {
            if (!isEnabled(context)) return
            if (!overlayPermissionGranted(context)) {
                runtimeState = "permission_required"
                return
            }
            try {
                preferences(context).edit().remove(KEY_LAST_ERROR).apply()
                start(context)
                if (runtimeState == "disabled" || runtimeState == "failed") {
                    runtimeState = "waiting"
                }
            } catch (error: Throwable) {
                preferences(context).edit().putString(
                    KEY_LAST_ERROR,
                    "${error.javaClass.name}: ${error.message}",
                ).apply()
                runtimeState = "failed"
            }
        }

        fun requestOverlayPermission(context: Context): Map<String, Any?> {
            if (overlayPermissionGranted(context)) return status(context)
            val intent = Intent(
                Settings.ACTION_MANAGE_OVERLAY_PERMISSION,
                Uri.parse("package:${context.packageName}"),
            ).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            context.startActivity(intent)
            return status(context)
        }

        fun resetWindowPosition(context: Context): Map<String, Any?> {
            preferences(context).edit()
                .remove(KEY_X_RATIO)
                .remove(KEY_Y_RATIO)
                .apply()
            current?.mainHandler?.post { current?.relayout(resetPosition = true) }
            return status(context)
        }

        fun observeHostRequest(
            context: Context,
            request: NativeBridgeRequest,
            response: NativeBridgeResponse? = null,
        ) {
            if (!isEnabled(context)) return
            if (request.operation == "virtual_display.release") {
                if (response != null && response.status == "success") onDisplayReleased(context)
                return
            }
            if (response == null && !VirtualDisplayPreviewPolicy.shouldObserveBefore(request.operation)) {
                return
            }
            if (response != null && !VirtualDisplayPreviewPolicy.shouldObserveAfter(request.operation)) {
                return
            }
            val displayId = VirtualDisplayPreviewPolicy.displayId(request.payload).takeIf { it > 0 }
                ?: targetDisplayId(context)
            if (request.operation == "virtual_display.create") {
                if (response?.status == "success") {
                    val createdDisplayId = response.result?.get("displayId") as? Number
                    val created = createdDisplayId?.toInt()
                        ?: (response.result?.get("display") as? Map<*, *>)
                            ?.get("displayId")
                            ?.let { (it as? Number)?.toInt() }
                    if (created != null && created >= 0) onVirtualDisplayOperation(context, created)
                }
                return
            }
            if (displayId > 0) {
                onVirtualDisplayOperation(context, displayId)
            }
        }

        fun onVirtualDisplayOperation(context: Context, displayId: Int) {
            if (!isEnabled(context) || displayId <= 0) return
            val prefs = preferences(context)
            prefs.edit()
                .putBoolean(KEY_SUPPRESSED, false)
                .putInt(KEY_TARGET_DISPLAY_ID, displayId)
                .remove(KEY_LAST_ERROR)
                .apply()
            if (!overlayPermissionGranted(context)) {
                runtimeState = "permission_required"
                prefs.edit().putString(KEY_LAST_ERROR, "系统悬浮窗权限未授予").apply()
                return
            }
            ensureRunning(context)
            start(context, displayId)
            runtimeState = "connecting"
            current?.mainHandler?.post {
                current?.presentDisplay(displayId)
            }
        }

        fun onDisplayReleased(context: Context) {
            val prefs = preferences(context)
            prefs.edit().putInt(KEY_TARGET_DISPLAY_ID, -1).apply()
            runtimeState = "waiting"
            current?.mainHandler?.post {
                current?.hideOverlay()
                current?.updateNotification()
            }
        }

        fun onVirtualDisplayPermissionChanged(context: Context, enabled: Boolean) {
            if (!enabled) {
                preferences(context).edit()
                    .putInt(KEY_TARGET_DISPLAY_ID, -1)
                    .putBoolean(KEY_SUPPRESSED, false)
                    .apply()
                runtimeState = "waiting"
                current?.mainHandler?.post {
                    current?.hideOverlay()
                    current?.updateNotification()
                }
                context.stopService(Intent(context, VirtualDisplayPreviewService::class.java))
                return
            }
            if (isEnabled(context)) ensureRunning(context)
        }

        fun status(context: Context): Map<String, Any?> {
            val prefs = preferences(context)
            val enabled = prefs.getBoolean(KEY_ENABLED, false)
            val permissionGranted = overlayPermissionGranted(context)
            val service = current
            val target = prefs.getInt(KEY_TARGET_DISPLAY_ID, -1)
            val suppressed = prefs.getBoolean(KEY_SUPPRESSED, false)
            val state = when {
                !enabled -> "disabled"
                !permissionGranted -> "permission_required"
                suppressed -> "suppressed"
                service?.overlayVisible() == true -> "visible"
                service != null && target > 0 -> runtimeState.ifBlank { "connecting" }
                else -> runtimeState.takeIf { it != "disabled" } ?: "waiting"
            }
            return mapOf(
                "supported" to true,
                "enabled" to enabled,
                "overlayPermissionGranted" to permissionGranted,
                "serviceRunning" to (service != null),
                "visible" to (service?.overlayVisible() == true),
                "suppressed" to suppressed,
                "targetDisplayId" to target,
                "state" to state,
                "lastError" to prefs.getString(KEY_LAST_ERROR, "").orEmpty(),
            )
        }

        private fun start(context: Context, displayId: Int? = null) {
            val intent = Intent(context, VirtualDisplayPreviewService::class.java)
            if (displayId != null && displayId > 0) {
                intent.action = ACTION_SHOW_DISPLAY
                intent.putExtra(EXTRA_DISPLAY_ID, displayId)
            }
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                context.startForegroundService(intent)
            } else {
                context.startService(intent)
            }
        }

        private fun targetDisplayId(context: Context): Int {
            return preferences(context).getInt(KEY_TARGET_DISPLAY_ID, -1)
        }

        private fun overlayPermissionGranted(context: Context): Boolean =
            Build.VERSION.SDK_INT < Build.VERSION_CODES.M || Settings.canDrawOverlays(context)

        private fun preferences(context: Context) =
            context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
    }

    private val mainHandler = Handler(Looper.getMainLooper())
    private val serviceScope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)
    private lateinit var windowManager: WindowManager
    private lateinit var manager: VirtualDisplayHostManager
    private var rootView: FrameLayout? = null
    private var imageView: ImageView? = null
    private var closeButton: TextView? = null
    private var layoutParams: WindowManager.LayoutParams? = null
    private var targetDisplayId = -1
    private var displayWidth = 1080
    private var displayHeight = 1920
    private var receiverRegistered = false
    private var previewJob: Job? = null
    private var snapAnimator: ValueAnimator? = null
    private var frameBitmap: Bitmap? = null

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
        manager = VirtualDisplayHostManager(applicationContext)
        targetDisplayId = preferences().getInt(KEY_TARGET_DISPLAY_ID, -1)
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
        if (intent?.action == ACTION_OPEN_PERMISSION) {
            openPermissionSettings()
        }
        val requestedDisplayId = intent?.getIntExtra(EXTRA_DISPLAY_ID, -1) ?: -1
        if (intent?.action == ACTION_SHOW_DISPLAY && requestedDisplayId > 0) {
            targetDisplayId = requestedDisplayId
            preferences().edit()
                .putBoolean(KEY_SUPPRESSED, false)
                .putInt(KEY_TARGET_DISPLAY_ID, requestedDisplayId)
                .apply()
        }
        if (!isEnabled(this) || !overlayPermissionGranted(this)) {
            runtimeState = if (!isEnabled(this)) "disabled" else "permission_required"
            stopSelf()
            return START_NOT_STICKY
        }
        return try {
            startForegroundCompat()
            targetDisplayId = preferences().getInt(KEY_TARGET_DISPLAY_ID, -1)
            runtimeState = if (targetDisplayId > 0) "connecting" else "waiting"
            refreshVisibility()
            START_STICKY
        } catch (error: Throwable) {
            preferences().edit().putString(
                KEY_LAST_ERROR,
                "${error.javaClass.name}: ${error.message}",
            ).apply()
            runtimeState = "failed"
            stopSelf()
            START_NOT_STICKY
        }
    }

    override fun onDestroy() {
        if (receiverRegistered) {
            unregisterReceiver(displayReceiver)
            receiverRegistered = false
        }
        hideOverlay()
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.N) {
            stopForeground(STOP_FOREGROUND_REMOVE)
        } else {
            @Suppress("DEPRECATION")
            stopForeground(true)
        }
        if (current === this) current = null
        runtimeState = if (isEnabled(this)) "waiting" else "disabled"
        serviceScope.cancel()
        super.onDestroy()
    }

    fun presentDisplay(displayId: Int) {
        if (displayId <= 0 || !isEnabled(this)) return
        if (preferences().getBoolean(KEY_SUPPRESSED, false)) return
        if (displayId != targetDisplayId) {
            hideOverlay()
        }
        targetDisplayId = displayId
        preferences().edit()
            .putInt(KEY_TARGET_DISPLAY_ID, displayId)
            .remove(KEY_LAST_ERROR)
            .apply()
        refreshVisibility()
        updateNotification()
    }

    fun relayout(resetPosition: Boolean = false) {
        val view = rootView ?: return
        val params = layoutParams ?: return
        val size = windowSize()
        params.width = size.first
        params.height = size.second
        val bounds = positionBounds(params.width, params.height)
        if (resetPosition) {
            params.x = bounds.first
            params.y = bounds.second
            savePosition(params.x, params.y, params.width, params.height)
        } else {
            val saved = savedPosition(params.width, params.height)
            params.x = saved?.first ?: params.x.coerceIn(0, bounds.first)
            params.y = saved?.second ?: params.y.coerceIn(0, bounds.second)
        }
        try {
            windowManager.updateViewLayout(view, params)
        } catch (error: Throwable) {
            recordError(error)
        }
    }

    private fun refreshVisibility() {
        if (!isEnabled(this) || !overlayPermissionGranted(this)) {
            hideOverlay()
            return
        }
        if (targetDisplayId <= 0 || preferences().getBoolean(KEY_SUPPRESSED, false)) {
            hideOverlay()
            return
        }
        if (!canShowContent()) {
            hideOverlay()
            return
        }
        if (rootView == null) createOverlay()
    }

    private fun createOverlay() {
        if (rootView != null) return
        val params = WindowManager.LayoutParams(
            1,
            1,
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                WindowManager.LayoutParams.TYPE_APPLICATION_OVERLAY
            } else {
                @Suppress("DEPRECATION")
                WindowManager.LayoutParams.TYPE_PHONE
            },
            WindowManager.LayoutParams.FLAG_NOT_FOCUSABLE or
                WindowManager.LayoutParams.FLAG_NOT_TOUCH_MODAL or
                WindowManager.LayoutParams.FLAG_LAYOUT_IN_SCREEN or
                WindowManager.LayoutParams.FLAG_LAYOUT_NO_LIMITS,
            PixelFormat.TRANSLUCENT,
        ).apply {
            gravity = Gravity.TOP or Gravity.START
        }
        val size = windowSize()
        params.width = size.first
        params.height = size.second
        val bounds = positionBounds(params.width, params.height)
        val saved = savedPosition(params.width, params.height)
        params.x = saved?.first ?: bounds.first
        params.y = saved?.second ?: bounds.second

        val root = FrameLayout(this).apply {
            background = GradientDrawable().apply {
                shape = GradientDrawable.RECTANGLE
                cornerRadius = dp(16).toFloat()
                setColor(0xF21A1A1A.toInt())
            }
            foreground = GradientDrawable().apply {
                shape = GradientDrawable.RECTANGLE
                cornerRadius = dp(16).toFloat()
                setColor(Color.TRANSPARENT)
                setStroke(dp(2), 0xB3FFFFFF.toInt())
            }
            foregroundGravity = Gravity.FILL
            elevation = dp(12).toFloat()
            clipToOutline = true
            outlineProvider = ViewOutlineProvider.BACKGROUND
            contentDescription = "Amitia 虚拟屏实时画面，可拖动，长按角落缩放"
        }
        val image = ImageView(this).apply {
            scaleType = ImageView.ScaleType.FIT_CENTER
            setBackgroundColor(Color.TRANSPARENT)
            contentDescription = "Amitia 虚拟屏实时画面，可拖动，长按角落缩放"
        }
        root.addView(
            image,
            FrameLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT,
                ViewGroup.LayoutParams.MATCH_PARENT,
            ),
        )
        val connecting = TextView(this).apply {
            text = "正在连接虚拟屏"
            textSize = 13f
            gravity = Gravity.CENTER
            setTextColor(Color.WHITE)
            contentDescription = "正在连接虚拟屏"
            tag = "connecting"
        }
        root.addView(
            connecting,
            FrameLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT,
                ViewGroup.LayoutParams.MATCH_PARENT,
            ),
        )
        val gestureLayer = View(this).apply {
            setBackgroundColor(Color.TRANSPARENT)
        }
        root.addView(
            gestureLayer,
            FrameLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT,
                ViewGroup.LayoutParams.MATCH_PARENT,
            ),
        )
        val controlSize = dp(32)
        val close = TextView(this).apply {
            text = "×"
            textSize = 21f
            gravity = Gravity.CENTER
            setTextColor(Color.WHITE)
            background = GradientDrawable().apply {
                shape = GradientDrawable.OVAL
                setColor(0xD9000000.toInt())
                setStroke(dp(1), 0x88FFFFFF.toInt())
            }
            visibility = View.GONE
            contentDescription = "临时关闭虚拟屏画面"
            elevation = dp(4).toFloat()
            isClickable = false
        }
        root.addView(
            close,
            FrameLayout.LayoutParams(
                controlSize,
                controlSize,
                Gravity.TOP or Gravity.END,
            ).apply {
                topMargin = dp(7)
                marginEnd = dp(7)
            },
        )
        configureGestures(root, gestureLayer, close)
        rootView = root
        imageView = image
        closeButton = close
        layoutParams = params
        try {
            windowManager.addView(root, params)
            Log.i(TAG, "overlay added x=${params.x} y=${params.y} width=${params.width} height=${params.height}")
            runtimeState = "connecting"
            startFrameLoop(targetDisplayId)
        } catch (error: Throwable) {
            recordError(error)
            removeViewOnly()
        }
    }

    private fun startFrameLoop(displayId: Int) {
        previewJob?.cancel()
        if (displayId <= 0) return
        previewJob = serviceScope.launch {
            var consecutiveFailures = 0
            while (isActive && targetDisplayId == displayId && rootView != null) {
                if (preferences().getBoolean(KEY_SUPPRESSED, false)) break
                try {
                    val result = manager.execute(
                        "virtual_display.capture",
                        mapOf(
                            "displayId" to displayId,
                            "format" to "jpeg",
                            "quality" to 55,
                            "maxWidth" to 540,
                            "maxHeight" to 960,
                        ),
                    )
                    val encoded = result["dataBase64"] as? String
                    if (encoded.isNullOrBlank()) {
                        throw IllegalStateException("virtual display capture did not return dataBase64")
                    }
                    if (encoded.length > 4 * 1024 * 1024) {
                        throw IllegalStateException("virtual display capture payload exceeds bridge limit")
                    }
                    val bytes = android.util.Base64.decode(encoded, android.util.Base64.NO_WRAP)
                    val bitmap = BitmapFactory.decodeByteArray(bytes, 0, bytes.size)
                        ?: throw IllegalStateException("virtual display frame decode failed")
                    val previousBitmap = frameBitmap
                    frameBitmap = bitmap
                    imageView?.setImageBitmap(bitmap)
                    previousBitmap?.recycle()
                    rootView?.findViewWithTag<TextView>("connecting")?.visibility = View.GONE
                    preferences().edit().remove(KEY_LAST_ERROR).apply()
                    runtimeState = "visible"
                    consecutiveFailures = 0
                    updateNotification()
                } catch (error: CancellationException) {
                    throw error
                } catch (error: Throwable) {
                    consecutiveFailures += 1
                    recordError(error)
                    if (consecutiveFailures >= 3) {
                        runtimeState = "failed"
                        hideOverlay()
                        updateNotification()
                        break
                    }
                    delay(250L)
                }
                delay(100L)
            }
        }
    }

    private fun configureGestures(root: FrameLayout, gestureLayer: View, close: TextView) {
        val touchSlop = dp(6).toFloat()
        val cornerRadius = dp(28).toFloat()
        var downRawX = 0f
        var downRawY = 0f
        var startFrame = VirtualDisplayWindowFrame(0, 0, 1, 1)
        var activeCorner: VirtualDisplayResizeCorner? = null
        var dragging = false
        var resizing = false
        val activateResize = Runnable {
            val corner = activeCorner
            if (!dragging && corner != null && rootView === root) {
                resizing = true
                close.visibility = View.GONE
                root.removeCallbacks(hideCloseRunnable)
                root.performHapticFeedback(HapticFeedbackConstants.LONG_PRESS)
                Log.i(TAG, "overlay resize activated corner=$corner")
            }
        }
        gestureLayer.setOnTouchListener { _, event ->
            val params = layoutParams ?: return@setOnTouchListener false
            when (event.actionMasked) {
                MotionEvent.ACTION_DOWN -> {
                    snapAnimator?.cancel()
                    snapAnimator = null
                    downRawX = event.rawX
                    downRawY = event.rawY
                    startFrame = VirtualDisplayWindowFrame(
                        params.x,
                        params.y,
                        params.width,
                        params.height,
                    )
                    activeCorner = VirtualDisplayPreviewGeometry.detectCorner(
                        event.x,
                        event.y,
                        root.width,
                        root.height,
                        cornerRadius,
                    )
                    dragging = false
                    resizing = false
                    if (activeCorner != null) {
                        root.postDelayed(activateResize, 250L)
                    }
                    true
                }

                MotionEvent.ACTION_MOVE -> {
                    val dx = event.rawX - downRawX
                    val dy = event.rawY - downRawY
                    if (resizing) {
                        val corner = activeCorner ?: return@setOnTouchListener true
                        val metrics = resources.displayMetrics
                        val constraints = windowConstraints()
                        val frame = VirtualDisplayPreviewGeometry.resize(
                            start = startFrame,
                            corner = corner,
                            deltaX = dx,
                            deltaY = dy,
                            aspectRatio = displayAspectRatio(),
                            minWidth = constraints.first,
                            maxWidth = constraints.second,
                            screenWidth = metrics.widthPixels,
                            screenHeight = metrics.heightPixels,
                        )
                        params.x = frame.x
                        params.y = frame.y
                        params.width = frame.width
                        params.height = frame.height
                        updateOverlayLayout(root, params)
                        return@setOnTouchListener true
                    }
                    if (!dragging && abs(dx) + abs(dy) > touchSlop) {
                        root.removeCallbacks(activateResize)
                        dragging = true
                        close.visibility = View.GONE
                    }
                    if (dragging) {
                        val bounds = positionBounds(params.width, params.height)
                        params.x = (startFrame.x + dx.roundToInt()).coerceIn(0, bounds.first)
                        params.y = (startFrame.y + dy.roundToInt()).coerceIn(0, bounds.second)
                        updateOverlayLayout(root, params)
                    }
                    true
                }

                MotionEvent.ACTION_UP -> {
                    root.removeCallbacks(activateResize)
                    if (resizing) {
                        settleOverlay(root, params, "resized")
                    } else if (dragging) {
                        settleOverlay(root, params, "dragged")
                    } else {
                        val closeVisible = close.visibility == View.VISIBLE
                        val insideClose = closeVisible &&
                            event.x >= close.left &&
                            event.x <= close.right &&
                            event.y >= close.top &&
                            event.y <= close.bottom
                        if (insideClose) {
                            dismissPreview()
                        } else {
                            close.visibility =
                                if (closeVisible) View.GONE else View.VISIBLE
                            Log.i(TAG, "overlay tapped closeVisible=${close.visibility == View.VISIBLE}")
                            root.removeCallbacks(hideCloseRunnable)
                            if (close.visibility == View.VISIBLE) {
                                root.postDelayed(hideCloseRunnable, 5000L)
                            }
                        }
                    }
                    true
                }

                MotionEvent.ACTION_CANCEL -> {
                    root.removeCallbacks(activateResize)
                    if (resizing || dragging) {
                        settleOverlay(root, params, "cancelled")
                    }
                    true
                }

                else -> false
            }
        }
    }

    private fun updateOverlayLayout(root: FrameLayout, params: WindowManager.LayoutParams) {
        try {
            windowManager.updateViewLayout(root, params)
        } catch (error: Throwable) {
            recordError(error)
        }
    }

    private fun settleOverlay(
        root: FrameLayout,
        params: WindowManager.LayoutParams,
        action: String,
    ) {
        val metrics = resources.displayMetrics
        val frame = VirtualDisplayWindowFrame(
            params.x,
            params.y,
            params.width,
            params.height,
        )
        val snapTarget = VirtualDisplayPreviewGeometry.horizontalSnapTarget(
            frame,
            metrics.widthPixels,
            dp(32),
        )
        if (snapTarget != null && snapTarget != params.x) {
            animateSnap(root, params, snapTarget, action)
        } else {
            savePosition(params.x, params.y, params.width, params.height)
            Log.i(
                TAG,
                "overlay $action x=${params.x} y=${params.y} width=${params.width} height=${params.height}",
            )
        }
    }

    private fun animateSnap(
        root: FrameLayout,
        params: WindowManager.LayoutParams,
        targetX: Int,
        action: String,
    ) {
        snapAnimator?.cancel()
        val startX = params.x
        val animator = ValueAnimator.ofInt(startX, targetX).apply {
            duration = 180L
            interpolator = DecelerateInterpolator()
            addUpdateListener { animation ->
                params.x = (animation.animatedValue as Number).toInt()
                updateOverlayLayout(root, params)
            }
            addListener(object : android.animation.AnimatorListenerAdapter() {
                override fun onAnimationEnd(animation: android.animation.Animator) {
                    if (snapAnimator === animation) snapAnimator = null
                    savePosition(params.x, params.y, params.width, params.height)
                    Log.i(
                        TAG,
                        "overlay snapped $action x=${params.x} y=${params.y} width=${params.width} height=${params.height}",
                    )
                }
            })
        }
        snapAnimator = animator
        animator.start()
    }

    private val hideCloseRunnable = Runnable {
        closeButton?.visibility = View.GONE
    }

    private fun dismissPreview() {
        Log.i(TAG, "overlay dismissed")
        preferences().edit()
            .putBoolean(KEY_SUPPRESSED, true)
            .remove(KEY_LAST_ERROR)
            .apply()
        runtimeState = "suppressed"
        closeButton?.visibility = View.GONE
        hideOverlay()
        updateNotification()
    }

    private fun hideOverlay() {
        snapAnimator?.cancel()
        snapAnimator = null
        previewJob?.cancel()
        previewJob = null
        rootView?.removeCallbacks(hideCloseRunnable)
        removeViewOnly()
    }

    private fun removeViewOnly() {
        val view = rootView ?: return
        rootView = null
        imageView?.setImageDrawable(null)
        imageView = null
        frameBitmap?.recycle()
        frameBitmap = null
        closeButton = null
        layoutParams = null
        try {
            windowManager.removeViewImmediate(view)
        } catch (error: Throwable) {
            recordError(error)
        }
    }

    private fun windowSize(): Pair<Int, Int> {
        val metrics = resources.displayMetrics
        val ratio = displayAspectRatio()
        val constraints = windowConstraints()
        val defaultWidth = min(
            dp(176),
            (metrics.widthPixels * 0.48f).roundToInt(),
        ).coerceAtLeast(constraints.first)
        val savedRatio = preferences().getFloat(
            KEY_WIDTH_RATIO,
            defaultWidth.toFloat() / metrics.widthPixels.toFloat(),
        )
        val width = (savedRatio * metrics.widthPixels)
            .roundToInt()
            .coerceIn(constraints.first, constraints.second)
        val height = (width * ratio).roundToInt()
        return width to height
    }

    private fun displayAspectRatio(): Float {
        return displayHeight.toFloat() / displayWidth.toFloat().coerceAtLeast(1f)
    }

    private fun windowConstraints(): Pair<Int, Int> {
        val metrics = resources.displayMetrics
        val ratio = displayAspectRatio()
        val minWidth = dp(120)
        val widthLimit = (metrics.widthPixels * 0.88f).roundToInt()
        val heightLimit = (metrics.heightPixels * 0.82f / ratio).roundToInt()
        val maxWidth = max(minWidth, min(widthLimit, heightLimit))
        return minWidth.coerceAtMost(maxWidth) to maxWidth
    }

    private fun positionBounds(width: Int, height: Int): Pair<Int, Int> {
        val metrics = resources.displayMetrics
        return max(0, metrics.widthPixels - width) to
            max(0, metrics.heightPixels - height)
    }

    private fun savedPosition(width: Int, height: Int): Pair<Int, Int>? {
        val prefs = preferences()
        if (!prefs.contains(KEY_X_RATIO) || !prefs.contains(KEY_Y_RATIO)) return null
        val bounds = positionBounds(width, height)
        val x = (prefs.getFloat(KEY_X_RATIO, 0f) * bounds.first).roundToInt()
        val y = (prefs.getFloat(KEY_Y_RATIO, 0f) * bounds.second).roundToInt()
        return x.coerceIn(0, bounds.first) to y.coerceIn(0, bounds.second)
    }

    private fun savePosition(x: Int, y: Int, width: Int, height: Int) {
        val metrics = resources.displayMetrics
        val bounds = positionBounds(width, height)
        val widthRatio = width.toFloat() / metrics.widthPixels.toFloat().coerceAtLeast(1f)
        val xRatio = if (bounds.first <= 0) 0f else x.toFloat() / bounds.first.toFloat()
        val yRatio = if (bounds.second <= 0) 0f else y.toFloat() / bounds.second.toFloat()
        preferences().edit()
            .putFloat(KEY_WIDTH_RATIO, widthRatio.coerceIn(0f, 1f))
            .putFloat(KEY_X_RATIO, xRatio.coerceIn(0f, 1f))
            .putFloat(KEY_Y_RATIO, yRatio.coerceIn(0f, 1f))
            .apply()
    }

    private fun canShowContent(): Boolean {
        val power = getSystemService(POWER_SERVICE) as PowerManager
        val keyguard = getSystemService(KEYGUARD_SERVICE) as KeyguardManager
        return power.isInteractive && !keyguard.isKeyguardLocked
    }

    private fun startForegroundCompat() {
        val notificationManager = getSystemService(NOTIFICATION_SERVICE) as NotificationManager
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            notificationManager.createNotificationChannel(
                NotificationChannel(
                    CHANNEL_ID,
                    "虚拟屏实时画面",
                    NotificationManager.IMPORTANCE_LOW,
                ).apply {
                    setShowBadge(false)
                    description = "AI 操作虚拟屏时显示可拖动、可缩放的实时画面"
                },
            )
        }
        val notification = buildNotification()
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) {
            startForeground(
                NOTIFICATION_ID,
                notification,
                ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE,
            )
        } else {
            startForeground(NOTIFICATION_ID, notification)
        }
    }

    private fun updateNotification() {
        val manager = getSystemService(NOTIFICATION_SERVICE) as NotificationManager
        manager.notify(NOTIFICATION_ID, buildNotification())
    }

    private fun buildNotification(): Notification {
        val openApp = PendingIntent.getActivity(
            this,
            NOTIFICATION_ID,
            Intent(this, MainActivity::class.java)
                .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_SINGLE_TOP),
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
        )
        val builder = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            Notification.Builder(this, CHANNEL_ID)
        } else {
            Notification.Builder(this)
        }
        val detail = when (runtimeState) {
            "visible" -> "正在显示 AI 虚拟屏实时画面"
            "suppressed" -> "虚拟屏画面已临时关闭，等待下一次操作"
            "permission_required" -> "需要系统悬浮窗权限"
            "failed" -> preferences().getString(KEY_LAST_ERROR, "虚拟屏画面连接失败")
            else -> "等待 AI 操作虚拟屏"
        }
        return builder
            .setSmallIcon(R.drawable.ic_notification)
            .setContentTitle("Amitia 虚拟屏实时画面")
            .setContentText(detail)
            .setContentIntent(openApp)
            .setOngoing(true)
            .setOnlyAlertOnce(true)
            .setCategory(Notification.CATEGORY_SERVICE)
            .setVisibility(Notification.VISIBILITY_SECRET)
            .build()
    }

    private fun openPermissionSettings() {
        val intent = Intent(
            Settings.ACTION_MANAGE_OVERLAY_PERMISSION,
            Uri.parse("package:$packageName"),
        ).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        startActivity(intent)
    }

    private fun recordError(error: Throwable) {
        val message = if (error is VirtualDisplayHostError) {
            "${error.code}: ${error.message}"
        } else {
            "${error.javaClass.name}: ${error.message}"
        }
        preferences().edit().putString(KEY_LAST_ERROR, message).apply()
    }

    private fun overlayVisible(): Boolean = rootView != null

    private fun preferences() = getSharedPreferences(PREFS, MODE_PRIVATE)

    private fun dp(value: Int): Int =
        (value * resources.displayMetrics.density).roundToInt()
}
