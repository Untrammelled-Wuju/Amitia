package com.amitia.amitia_app.nativeprovider.overlay

import android.content.Context
import android.content.Intent
import android.graphics.BitmapFactory
import android.graphics.Color
import android.graphics.PixelFormat
import android.graphics.drawable.GradientDrawable
import android.net.Uri
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.provider.Settings
import android.util.Base64
import android.view.Gravity
import android.view.MotionEvent
import android.view.View
import android.view.ViewGroup
import android.view.WindowManager
import android.widget.FrameLayout
import android.widget.ImageView
import android.widget.LinearLayout
import android.widget.TextView
import com.amitia.amitia_app.nativeprovider.AndroidNativeOperationHandler
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeError
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeProtocol
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeRequest
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeResponse
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.io.File
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.atomic.AtomicLong
import kotlin.math.roundToInt

internal class OverlayNativeHandler(
    private val context: Context,
) : AndroidNativeOperationHandler {

    private data class OverlayRecord(
        var info: OverlayInfo,
        val root: FrameLayout,
        val params: WindowManager.LayoutParams,
        var ttlRunnable: Runnable? = null,
    )

    private val appContext = context.applicationContext
    private val windowManager = appContext.getSystemService(Context.WINDOW_SERVICE) as WindowManager
    private val generation = AtomicLong(0L)
    private val sequence = AtomicLong(0L)
    private val mainHandler = Handler(Looper.getMainLooper())
    private val activeOverlays = ConcurrentHashMap<String, OverlayRecord>()

    override val operations: Set<String> = setOf(
        OP_STATUS,
        OP_REQUEST_PERMISSION,
        OP_CREATE,
        OP_UPDATE,
        OP_SHOW,
        OP_HIDE,
        OP_CLOSE,
        OP_LIST,
        OP_CLOSE_ALL,
    )

    override suspend fun execute(request: NativeBridgeRequest): NativeBridgeResponse =
        withContext(Dispatchers.Main.immediate) {
            when (request.operation) {
                OP_STATUS -> handleStatus(request)
                OP_REQUEST_PERMISSION -> handleRequestPermission(request)
                OP_CREATE -> handleCreate(request)
                OP_UPDATE -> handleUpdate(request)
                OP_SHOW -> handleShow(request)
                OP_HIDE -> handleHide(request)
                OP_CLOSE -> handleClose(request)
                OP_LIST -> handleList(request)
                OP_CLOSE_ALL -> handleCloseAll(request)
                else -> unsupportedOperation(request)
            }
        }

    private fun handleStatus(request: NativeBridgeRequest): NativeBridgeResponse {
        val granted = overlayPermissionGranted()
        return success(
            request,
            mapOf(
                "supported" to true,
                "permissionRequired" to (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M),
                "permissionGranted" to granted,
                "nativeHostReady" to true,
                "canCreate" to granted,
                "canUpdate" to granted,
                "canInteract" to granted,
                "activeCount" to activeOverlays.size,
                "userActionRequired" to !granted,
                "state" to if (granted) "available" else "permission_required",
            ),
        )
    }

    private fun handleRequestPermission(request: NativeBridgeRequest): NativeBridgeResponse {
        if (overlayPermissionGranted()) {
            return success(
                request,
                mapOf(
                    "opened" to false,
                    "userActionRequired" to false,
                    "permissionGranted" to true,
                ),
            )
        }

        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.M) {
            return success(
                request,
                mapOf(
                    "opened" to false,
                    "userActionRequired" to false,
                    "permissionGranted" to true,
                ),
            )
        }

        return try {
            val intent = Intent(
                Settings.ACTION_MANAGE_OVERLAY_PERMISSION,
                Uri.parse("package:${appContext.packageName}"),
            ).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            appContext.startActivity(intent)
            success(
                request,
                mapOf(
                    "opened" to true,
                    "userActionRequired" to true,
                    "permissionGranted" to false,
                ),
            )
        } catch (error: Exception) {
            failure(
                request,
                "OVERLAY_PERMISSION_REQUEST_FAILED",
                "failed to open overlay permission settings: ${error.message}",
            )
        }
    }

    private fun handleCreate(request: NativeBridgeRequest): NativeBridgeResponse {
        ensurePermission(request)?.let { return it }

        val kind = request.payload.string("kind", "text").lowercase()
        if (kind !in SUPPORTED_KINDS) {
            return failure(request, "OVERLAY_INVALID_KIND", "invalid overlay kind: $kind")
        }

        val overlayId = request.payload.string("overlayId").ifBlank {
            "ovl_android_${System.currentTimeMillis()}_${sequence.incrementAndGet()}"
        }
        if (activeOverlays.containsKey(overlayId)) {
            return failure(request, "OVERLAY_ALREADY_EXISTS", "overlay already exists: $overlayId")
        }

        val now = System.currentTimeMillis()
        val info = OverlayInfo(
            overlayId = overlayId,
            kind = kind,
            visible = false,
            focusable = request.payload.boolean("focusable", false),
            touchable = request.payload.boolean("touchable", true),
            draggable = request.payload.boolean("draggable", false),
            x = request.payload.int("x", 0),
            y = request.payload.int("y", 0),
            width = request.payload.int("width", defaultWidth(kind)).coerceAtLeast(1),
            height = request.payload.int("height", defaultHeight(kind)).coerceAtLeast(1),
            gravity = normalizeGravity(request.payload.string("gravity", "top_left")),
            displayId = 0,
            createdAt = now,
            updatedAt = now,
            content = request.payload.map("content"),
        )

        val root = FrameLayout(appContext).apply {
            visibility = View.GONE
            setBackgroundColor(Color.TRANSPARENT)
            clipChildren = false
            clipToPadding = false
        }
        val params = buildLayoutParams(info)
        val record = OverlayRecord(info = info, root = root, params = params)
        render(record)
        configureDrag(record)

        return try {
            windowManager.addView(root, params)
            activeOverlays[overlayId] = record
            scheduleTtl(record, request.payload.longOrNull("ttlMs"))
            generation.incrementAndGet()
            success(request, instanceMap(record.info))
        } catch (error: Exception) {
            failure(request, "OVERLAY_CREATE_FAILED", "failed to create overlay: ${error.message}")
        }
    }

    private fun handleUpdate(request: NativeBridgeRequest): NativeBridgeResponse {
        ensurePermission(request)?.let { return it }
        val record = findRecord(request) ?: return notFound(request)
        val payload = request.payload
        val previous = record.info
        val now = System.currentTimeMillis()

        record.info = previous.copy(
            focusable = payload.booleanOrNull("focusable") ?: previous.focusable,
            touchable = payload.booleanOrNull("touchable") ?: previous.touchable,
            draggable = payload.booleanOrNull("draggable") ?: previous.draggable,
            x = payload.intOrNull("x") ?: previous.x,
            y = payload.intOrNull("y") ?: previous.y,
            width = (payload.intOrNull("width") ?: previous.width).coerceAtLeast(1),
            height = (payload.intOrNull("height") ?: previous.height).coerceAtLeast(1),
            gravity = payload.stringOrNull("gravity")?.let(::normalizeGravity) ?: previous.gravity,
            updatedAt = now,
            content = if (payload.containsKey("content")) payload.map("content") else previous.content,
        )

        applyInfoToParams(record)
        render(record)
        configureDrag(record)
        record.root.alpha = contentAlpha(record.info.content)

        return try {
            windowManager.updateViewLayout(record.root, record.params)
            scheduleTtl(record, payload.longOrNull("ttlMs"))
            generation.incrementAndGet()
            success(request, instanceMap(record.info))
        } catch (error: Exception) {
            failure(request, "OVERLAY_UPDATE_FAILED", "failed to update overlay: ${error.message}")
        }
    }

    private fun handleShow(request: NativeBridgeRequest): NativeBridgeResponse {
        ensurePermission(request)?.let { return it }
        val record = findRecord(request) ?: return notFound(request)
        record.root.visibility = View.VISIBLE
        record.info = record.info.copy(visible = true, updatedAt = System.currentTimeMillis())
        generation.incrementAndGet()
        return success(request, instanceMap(record.info))
    }

    private fun handleHide(request: NativeBridgeRequest): NativeBridgeResponse {
        val record = findRecord(request) ?: return notFound(request)
        record.root.visibility = View.GONE
        record.info = record.info.copy(visible = false, updatedAt = System.currentTimeMillis())
        generation.incrementAndGet()
        return success(request, instanceMap(record.info))
    }

    private fun handleClose(request: NativeBridgeRequest): NativeBridgeResponse {
        val overlayId = request.payload.string("overlayId")
        if (overlayId.isBlank()) {
            return failure(request, "OVERLAY_INVALID_REQUEST", "overlayId is required")
        }
        val record = activeOverlays.remove(overlayId) ?: return notFound(request, overlayId)
        closeRecord(record)
        generation.incrementAndGet()
        return success(request, mapOf("closed" to true, "overlayId" to overlayId))
    }

    private fun handleList(request: NativeBridgeRequest): NativeBridgeResponse {
        val overlays = activeOverlays.values
            .map { instanceMap(it.info) }
            .sortedBy { it["createdAt"] as Long }
        return success(
            request,
            mapOf(
                "overlays" to overlays,
                "count" to overlays.size,
            ),
        )
    }

    private fun handleCloseAll(request: NativeBridgeRequest): NativeBridgeResponse {
        val records = activeOverlays.values.toList()
        activeOverlays.clear()
        records.forEach(::closeRecord)
        if (records.isNotEmpty()) generation.incrementAndGet()
        return success(request, mapOf("closedCount" to records.size))
    }

    private fun render(record: OverlayRecord) {
        val root = record.root
        root.removeAllViews()
        root.alpha = contentAlpha(record.info.content)
        val child = when (record.info.kind) {
            "image" -> buildImageView(record.info.content)
            "card" -> buildCardView(record.info.content)
            "status" -> buildStatusView(record.info.content)
            else -> buildTextView(record.info.content)
        }
        root.addView(
            child,
            FrameLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT,
                ViewGroup.LayoutParams.MATCH_PARENT,
            ),
        )
    }

    private fun buildTextView(content: Map<String, Any?>): View {
        val text = content.string("text").ifBlank { content.string("body", "Amitia") }
        return TextView(appContext).apply {
            this.text = text
            setTextColor(parseColor(content.string("textColor", "#FFFFFFFF"), Color.WHITE))
            textSize = (16f * content.float("fontScale", 1f).coerceIn(0.5f, 3f))
            maxLines = content.int("maxLines", 8).coerceIn(1, 32)
            gravity = Gravity.CENTER
            setPadding(dp(12), dp(8), dp(12), dp(8))
            background = roundedBackground(content)
        }
    }

    private fun buildStatusView(content: Map<String, Any?>): View {
        val body = content.string("text").ifBlank {
            content.string("body").ifBlank { content.string("status", "Amitia") }
        }
        return buildCardLikeView(content.string("title"), body, content)
    }

    private fun buildCardView(content: Map<String, Any?>): View {
        val imageUri = content.string("imageUri")
        val container = LinearLayout(appContext).apply {
            orientation = LinearLayout.VERTICAL
            gravity = Gravity.CENTER_VERTICAL
            setPadding(dp(12), dp(10), dp(12), dp(10))
            background = roundedBackground(content)
        }
        if (imageUri.isNotBlank()) {
            val image = buildImageView(mapOf("imageUri" to imageUri, "scaleType" to "center_inside"))
            container.addView(
                image,
                LinearLayout.LayoutParams(
                    ViewGroup.LayoutParams.MATCH_PARENT,
                    0,
                    1f,
                ),
            )
        }
        container.addView(
            buildCardLikeView(content.string("title"), content.string("body"), content),
            LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT,
                ViewGroup.LayoutParams.WRAP_CONTENT,
            ),
        )
        return container
    }

    private fun buildCardLikeView(title: String, body: String, content: Map<String, Any?>): View {
        return LinearLayout(appContext).apply {
            orientation = LinearLayout.VERTICAL
            gravity = Gravity.CENTER_VERTICAL
            if (title.isNotBlank()) {
                addView(TextView(appContext).apply {
                    text = title
                    setTextColor(parseColor(content.string("textColor", "#FFFFFFFF"), Color.WHITE))
                    textSize = 16f
                    setTypeface(typeface, android.graphics.Typeface.BOLD)
                })
            }
            if (body.isNotBlank()) {
                addView(TextView(appContext).apply {
                    text = body
                    setTextColor(parseColor(content.string("textColor", "#FFFFFFFF"), Color.WHITE))
                    textSize = 14f
                    if (title.isNotBlank()) setPadding(0, dp(4), 0, 0)
                })
            }
        }
    }

    private fun buildImageView(content: Map<String, Any?>): View {
        val view = ImageView(appContext).apply {
            adjustViewBounds = true
            scaleType = when (content.string("scaleType", "fit_center")) {
                "center_crop" -> ImageView.ScaleType.CENTER_CROP
                "center_inside" -> ImageView.ScaleType.CENTER_INSIDE
                "fit_xy" -> ImageView.ScaleType.FIT_XY
                else -> ImageView.ScaleType.FIT_CENTER
            }
        }
        val uri = content.string("imageUri")
        val bitmap = decodeBitmap(uri)
        if (bitmap != null) {
            view.setImageBitmap(bitmap)
        } else {
            view.setImageDrawable(null)
            view.contentDescription = if (uri.isBlank()) "Amitia overlay image" else "Overlay image unavailable"
        }
        return view
    }

    private fun decodeBitmap(source: String): android.graphics.Bitmap? {
        if (source.isBlank()) return null
        return try {
            when {
                source.startsWith("data:image/") -> {
                    val encoded = source.substringAfter(',', "")
                    val bytes = Base64.decode(encoded, Base64.DEFAULT)
                    BitmapFactory.decodeByteArray(bytes, 0, bytes.size)
                }
                source.startsWith("content://") -> {
                    appContext.contentResolver.openInputStream(Uri.parse(source))?.use(BitmapFactory::decodeStream)
                }
                source.startsWith("file://") -> BitmapFactory.decodeFile(Uri.parse(source).path)
                source.startsWith("/") -> BitmapFactory.decodeFile(source)
                else -> {
                    val runtimeDataFile = File(File(File(appContext.filesDir, "amitia"), "data"), source)
                    BitmapFactory.decodeFile(runtimeDataFile.canonicalPath)
                }
            }
        } catch (_: Exception) {
            null
        }
    }

    private fun roundedBackground(content: Map<String, Any?>): GradientDrawable = GradientDrawable().apply {
        shape = GradientDrawable.RECTANGLE
        cornerRadius = dp(12).toFloat()
        setColor(parseColor(content.string("backgroundColor", "#CC202124"), 0xCC202124.toInt()))
    }

    private fun configureDrag(record: OverlayRecord) {
        if (!record.info.draggable || !record.info.touchable) {
            record.root.setOnTouchListener(null)
            return
        }
        record.root.setOnTouchListener(object : View.OnTouchListener {
            private var startRawX = 0f
            private var startRawY = 0f
            private var startX = 0
            private var startY = 0

            override fun onTouch(view: View, event: MotionEvent): Boolean {
                when (event.actionMasked) {
                    MotionEvent.ACTION_DOWN -> {
                        startRawX = event.rawX
                        startRawY = event.rawY
                        startX = record.params.x
                        startY = record.params.y
                        return true
                    }
                    MotionEvent.ACTION_MOVE -> {
                        record.params.x = startX + (event.rawX - startRawX).roundToInt()
                        record.params.y = startY + (event.rawY - startRawY).roundToInt()
                        try {
                            windowManager.updateViewLayout(record.root, record.params)
                            record.info = record.info.copy(
                                x = pxToDp(record.params.x),
                                y = pxToDp(record.params.y),
                                updatedAt = System.currentTimeMillis(),
                            )
                        } catch (_: Exception) {
                            return false
                        }
                        return true
                    }
                    MotionEvent.ACTION_UP, MotionEvent.ACTION_CANCEL -> return true
                }
                return false
            }
        })
    }

    private fun buildLayoutParams(info: OverlayInfo): WindowManager.LayoutParams {
        val type = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            WindowManager.LayoutParams.TYPE_APPLICATION_OVERLAY
        } else {
            @Suppress("DEPRECATION")
            WindowManager.LayoutParams.TYPE_PHONE
        }
        return WindowManager.LayoutParams(
            dp(info.width),
            dp(info.height),
            type,
            buildFlags(info),
            PixelFormat.TRANSLUCENT,
        ).apply {
            gravity = gravityValue(info.gravity)
            x = dp(info.x)
            y = dp(info.y)
        }
    }

    private fun applyInfoToParams(record: OverlayRecord) {
        record.params.width = dp(record.info.width)
        record.params.height = dp(record.info.height)
        record.params.gravity = gravityValue(record.info.gravity)
        record.params.x = dp(record.info.x)
        record.params.y = dp(record.info.y)
        record.params.flags = buildFlags(record.info)
    }

    private fun buildFlags(info: OverlayInfo): Int {
        var flags = WindowManager.LayoutParams.FLAG_LAYOUT_NO_LIMITS or
            WindowManager.LayoutParams.FLAG_LAYOUT_IN_SCREEN
        if (!info.focusable) flags = flags or WindowManager.LayoutParams.FLAG_NOT_FOCUSABLE
        if (!info.touchable) flags = flags or WindowManager.LayoutParams.FLAG_NOT_TOUCHABLE
        return flags
    }

    private fun scheduleTtl(record: OverlayRecord, ttlMs: Long?) {
        record.ttlRunnable?.let { mainHandler.removeCallbacks(it) }
        record.ttlRunnable = null
        if (ttlMs == null || ttlMs <= 0L) return
        val runnable = Runnable {
            val removed = activeOverlays.remove(record.info.overlayId) ?: return@Runnable
            closeRecord(removed)
            generation.incrementAndGet()
        }
        record.ttlRunnable = runnable
        mainHandler.postDelayed(runnable, ttlMs)
    }

    private fun closeRecord(record: OverlayRecord) {
        record.ttlRunnable?.let { mainHandler.removeCallbacks(it) }
        record.ttlRunnable = null
        try {
            windowManager.removeViewImmediate(record.root)
        } catch (_: Exception) {
            // Already detached or process teardown; the logical record is still removed.
        }
    }

    private fun findRecord(request: NativeBridgeRequest): OverlayRecord? {
        val overlayId = request.payload.string("overlayId")
        if (overlayId.isBlank()) return null
        return activeOverlays[overlayId]
    }

    private fun notFound(request: NativeBridgeRequest, explicitId: String = ""): NativeBridgeResponse {
        val overlayId = explicitId.ifBlank { request.payload.string("overlayId") }
        if (overlayId.isBlank()) {
            return failure(request, "OVERLAY_INVALID_REQUEST", "overlayId is required")
        }
        return failure(request, "OVERLAY_NOT_FOUND", "overlay not found: $overlayId")
    }

    private fun ensurePermission(request: NativeBridgeRequest): NativeBridgeResponse? {
        if (overlayPermissionGranted()) return null
        return failure(
            request,
            "OVERLAY_PERMISSION_REQUIRED",
            "overlay permission not granted",
            "OVERLAY_PERMISSION_REQUIRED",
        )
    }

    private fun overlayPermissionGranted(): Boolean =
        Build.VERSION.SDK_INT < Build.VERSION_CODES.M || Settings.canDrawOverlays(appContext)

    private fun pxToDp(value: Int): Int =
        (value / appContext.resources.displayMetrics.density).roundToInt()

    private fun instanceMap(info: OverlayInfo): Map<String, Any?> = mapOf(
        "overlayId" to info.overlayId,
        "kind" to info.kind,
        "visible" to info.visible,
        "focusable" to info.focusable,
        "touchable" to info.touchable,
        "x" to info.x,
        "y" to info.y,
        "width" to info.width,
        "height" to info.height,
        "gravity" to info.gravity,
        "displayId" to info.displayId,
        "createdAt" to info.createdAt,
        "updatedAt" to info.updatedAt,
    )

    private fun success(request: NativeBridgeRequest, result: Map<String, Any?>): NativeBridgeResponse =
        NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_SUCCESS,
            result = result,
        )

    private fun failure(
        request: NativeBridgeRequest,
        code: String,
        message: String,
        domainCode: String? = null,
    ): NativeBridgeResponse = NativeBridgeResponse(
        protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
        requestId = request.requestId,
        status = NativeBridgeProtocol.STATUS_ERROR,
        error = NativeBridgeError(code = code, message = message, domainCode = domainCode),
    )

    private fun unsupportedOperation(request: NativeBridgeRequest): NativeBridgeResponse = failure(
        request,
        NativeBridgeProtocol.ERR_OPERATION_NOT_SUPPORTED,
        "unknown overlay operation: ${request.operation}",
    )

    private fun normalizeGravity(value: String): String = when (value.lowercase()) {
        "top_left", "top_right", "bottom_left", "bottom_right", "center", "top_center", "bottom_center" -> value.lowercase()
        else -> "top_left"
    }

    private fun gravityValue(value: String): Int = when (normalizeGravity(value)) {
        "top_right" -> Gravity.TOP or Gravity.END
        "bottom_left" -> Gravity.BOTTOM or Gravity.START
        "bottom_right" -> Gravity.BOTTOM or Gravity.END
        "center" -> Gravity.CENTER
        "top_center" -> Gravity.TOP or Gravity.CENTER_HORIZONTAL
        "bottom_center" -> Gravity.BOTTOM or Gravity.CENTER_HORIZONTAL
        else -> Gravity.TOP or Gravity.START
    }

    private fun defaultWidth(kind: String): Int = if (kind == "image") 180 else 220
    private fun defaultHeight(kind: String): Int = if (kind == "image") 180 else 120
    private fun dp(value: Int): Int = (value * appContext.resources.displayMetrics.density).roundToInt()

    private fun contentAlpha(content: Map<String, Any?>): Float = when (val raw = content["alpha"]) {
        is Number -> raw.toFloat().coerceIn(0.2f, 1f)
        is String -> raw.toFloatOrNull()?.coerceIn(0.2f, 1f) ?: 1f
        else -> 1f
    }

    private fun parseColor(raw: String, fallback: Int): Int = try {
        Color.parseColor(raw)
    } catch (_: Exception) {
        fallback
    }

    private fun Map<String, Any?>.string(key: String, fallback: String = ""): String =
        this[key]?.toString()?.trim().orEmpty().ifBlank { fallback }

    private fun Map<String, Any?>.stringOrNull(key: String): String? =
        if (!containsKey(key)) null else this[key]?.toString()?.trim()?.takeIf { it.isNotEmpty() }

    private fun Map<String, Any?>.boolean(key: String, fallback: Boolean): Boolean =
        booleanOrNull(key) ?: fallback

    private fun Map<String, Any?>.booleanOrNull(key: String): Boolean? = when (val raw = this[key]) {
        is Boolean -> raw
        is Number -> raw.toInt() != 0
        is String -> when (raw.lowercase()) {
            "true", "1", "yes", "on" -> true
            "false", "0", "no", "off" -> false
            else -> null
        }
        else -> null
    }

    private fun Map<String, Any?>.int(key: String, fallback: Int): Int = intOrNull(key) ?: fallback

    private fun Map<String, Any?>.intOrNull(key: String): Int? = when (val raw = this[key]) {
        is Number -> raw.toInt()
        is String -> raw.toIntOrNull()
        else -> null
    }

    private fun Map<String, Any?>.longOrNull(key: String): Long? = when (val raw = this[key]) {
        is Number -> raw.toLong()
        is String -> raw.toLongOrNull()
        else -> null
    }

    private fun Map<String, Any?>.float(key: String, fallback: Float): Float = when (val raw = this[key]) {
        is Number -> raw.toFloat()
        is String -> raw.toFloatOrNull() ?: fallback
        else -> fallback
    }

    @Suppress("UNCHECKED_CAST")
    private fun Map<String, Any?>.map(key: String): Map<String, Any?> {
        val raw = this[key] as? Map<*, *> ?: return emptyMap()
        return raw.entries.associate { it.key.toString() to it.value }
    }

    companion object {
        private val SUPPORTED_KINDS = setOf("text", "image", "card", "status")

        const val OP_STATUS = "system.overlay.status"
        const val OP_REQUEST_PERMISSION = "system.overlay.permission.request"
        const val OP_CREATE = "system.overlay.create"
        const val OP_UPDATE = "system.overlay.update"
        const val OP_SHOW = "system.overlay.show"
        const val OP_HIDE = "system.overlay.hide"
        const val OP_CLOSE = "system.overlay.close"
        const val OP_LIST = "system.overlay.list"
        const val OP_CLOSE_ALL = "system.overlay.close_all"
    }
}
