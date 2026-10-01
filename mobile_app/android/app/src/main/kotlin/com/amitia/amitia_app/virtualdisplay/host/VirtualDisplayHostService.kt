package com.amitia.amitia_app.virtualdisplay.host

import android.annotation.SuppressLint
import android.app.UiAutomation
import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.graphics.PixelFormat
import android.graphics.Rect
import android.hardware.display.DisplayManager
import android.hardware.display.VirtualDisplay
import android.media.Image
import android.media.ImageReader
import android.os.Build
import android.os.Looper
import android.os.Process
import android.os.SystemClock
import android.util.Base64
import android.util.SparseArray
import android.view.KeyEvent
import android.view.accessibility.AccessibilityNodeInfo
import android.view.accessibility.AccessibilityWindowInfo
import org.json.JSONArray
import org.json.JSONObject
import org.xmlpull.v1.XmlPullParser
import org.xmlpull.v1.XmlPullParserFactory
import java.io.BufferedReader
import java.io.ByteArrayOutputStream
import java.io.File
import java.io.InputStreamReader
import java.io.StringReader
import java.lang.reflect.Constructor
import java.security.MessageDigest
import java.util.UUID
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit

internal class VirtualDisplayHostService(
    private val context: Context,
    private val hostPackage: String,
    private val token: String,
) : IVirtualDisplayHost.Stub() {

    private data class DisplaySession(
        val ref: String,
        val name: String,
        var width: Int,
        var height: Int,
        var densityDpi: Int,
        var packageName: String?,
        var generation: Long,
        val display: VirtualDisplay,
        var reader: ImageReader,
        var lastFrameAt: Long,
    )

    private val displayManager = createDisplayManager(context)
    private val inputController by lazy {
        runCatching { VirtualDisplayHostInputController() }.getOrNull()
    }
    private val uiAutomation by lazy { createUiAutomation() }
    private val sessions = ConcurrentHashMap<Int, DisplaySession>()
    private val scheduler = Executors.newSingleThreadScheduledExecutor()
    private val lock = Any()
    private var generation = 0L
    private var stopped = false

    init {
        scheduler.scheduleAtFixedRate(
            { broadcastBinder() },
            0L,
            5L,
            TimeUnit.SECONDS,
        )
    }

    override fun execute(requestJson: String): String {
        val request = try {
            JSONObject(requestJson)
        } catch (error: Throwable) {
            return errorResponse("", "VIRTUAL_DISPLAY_INVALID_REQUEST", error.message ?: "invalid request")
        }
        val requestId = request.optString("requestId")
        return try {
            when (request.optString("operation")) {
                "virtual_display.status", "virtual_display.ping" -> successResponse(requestId, status())
                "virtual_display.create" -> successResponse(requestId, create(request))
                "virtual_display.get" -> successResponse(requestId, get(request))
                "virtual_display.list" -> successResponse(requestId, list())
                "virtual_display.resize" -> successResponse(requestId, resize(request))
                "virtual_display.release" -> successResponse(requestId, release(request))
                "virtual_display.launch" -> successResponse(requestId, launch(request))
                "virtual_display.capture" -> successResponse(requestId, capture(request))
                "virtual_display.tap" -> successResponse(requestId, tap(request))
                "virtual_display.swipe" -> successResponse(requestId, swipe(request))
                "virtual_display.key" -> successResponse(requestId, key(request))
                "virtual_display.text" -> successResponse(requestId, text(request))
                "virtual_display.clear_text" -> successResponse(requestId, clearText(request))
                "virtual_display.ui_tree" -> successResponse(requestId, uiTree(request))
                "virtual_display.shutdown" -> {
                    successResponse(requestId, shutdownHost())
                }
                else -> errorResponse(requestId, "VIRTUAL_DISPLAY_OPERATION_NOT_SUPPORTED", "unsupported operation")
            }
        } catch (error: HostError) {
            errorResponse(requestId, error.code, error.message ?: "virtual display host error")
        } catch (error: Throwable) {
            errorResponse(requestId, "VIRTUAL_DISPLAY_NATIVE_ERROR", error.message ?: error.javaClass.simpleName)
        }
    }

    override fun shutdown() {
        shutdownHost()
    }

    private fun create(request: JSONObject): JSONObject {
        val name = request.optString("name").trim().ifEmpty { "amitia_virtual" }
        val width = request.optInt("width", 1080).coerceIn(320, 2560)
        val height = request.optInt("height", 1920).coerceIn(320, 2560)
        val densityDpi = request.optInt("densityDpi", 420).coerceIn(72, 640)
        synchronized(lock) {
            val reader = ImageReader.newInstance(width, height, PixelFormat.RGBA_8888, 3)
            val flags = DisplayManager.VIRTUAL_DISPLAY_FLAG_PUBLIC or
                DisplayManager.VIRTUAL_DISPLAY_FLAG_PRESENTATION
            val display = displayManager.createVirtualDisplay(
                name,
                width,
                height,
                densityDpi,
                reader.surface,
                flags,
            ) ?: run {
                reader.close()
                throw HostError("VIRTUAL_DISPLAY_CREATE_FAILED", "DisplayManager returned null")
            }
            val displayId = display.display?.displayId ?: -1
            if (displayId < 0) {
                display.release()
                reader.close()
                throw HostError("VIRTUAL_DISPLAY_CREATE_FAILED", "system did not assign a display id")
            }
            generation += 1
            val session = DisplaySession(
                ref = "vd_${displayId}_${generation}",
                name = name,
                width = width,
                height = height,
                densityDpi = densityDpi,
                packageName = null,
                generation = generation,
                display = display,
                reader = reader,
                lastFrameAt = 0L,
            )
            sessions[displayId] = session
            return sessionJson(displayId, session).put("ref", session.ref)
        }
    }

    private fun get(request: JSONObject): JSONObject {
        val pair = resolveSession(request)
        return sessionJson(pair.first, pair.second)
    }

    private fun list(): JSONObject {
        val array = JSONArray()
        sessions.entries.sortedBy { it.key }.forEach { entry ->
            array.put(sessionJson(entry.key, entry.value))
        }
        return JSONObject()
            .put("displays", array)
            .put("count", array.length())
    }

    private fun resize(request: JSONObject): JSONObject {
        val pair = resolveSession(request)
        val displayId = pair.first
        val session = pair.second
        val width = request.optInt("width", session.width).coerceIn(320, 2560)
        val height = request.optInt("height", session.height).coerceIn(320, 2560)
        val densityDpi = request.optInt("densityDpi", session.densityDpi).coerceIn(72, 640)
        synchronized(lock) {
            val nextReader = ImageReader.newInstance(width, height, PixelFormat.RGBA_8888, 3)
            try {
                session.display.setSurface(nextReader.surface)
                session.display.resize(width, height, densityDpi)
            } catch (error: Throwable) {
                nextReader.close()
                throw HostError("VIRTUAL_DISPLAY_RESIZE_FAILED", error.message ?: "resize failed")
            }
            session.reader.close()
            session.reader = nextReader
            session.width = width
            session.height = height
            session.densityDpi = densityDpi
            generation += 1
            session.generation = generation
        }
        return sessionJson(displayId, session)
    }

    private fun release(request: JSONObject): JSONObject {
        val pair = resolveSession(request)
        sessions.remove(pair.first)
        closeSession(pair.second)
        return JSONObject()
            .put("released", true)
            .put("displayId", pair.first)
    }

    private fun launch(request: JSONObject): JSONObject {
        val pair = resolveSession(request)
        val displayId = pair.first
        val packageName = request.optString("packageName").trim()
        if (packageName.isBlank()) {
            throw HostError("VIRTUAL_DISPLAY_INVALID_REQUEST", "packageName is required")
        }
        val component = resolveLaunchComponent(packageName, request.optString("component").trim())
            ?: throw HostError("VIRTUAL_DISPLAY_LAUNCH_FAILED", "no launch activity for $packageName")
        ensureDisplayReady(displayId)
        val result = runCommand(
            listOf(
                "/system/bin/am",
                "start",
                "--display",
                displayId.toString(),
                "-a",
                Intent.ACTION_MAIN,
                "-c",
                Intent.CATEGORY_LAUNCHER,
                "-n",
                component,
            ),
            12000L,
        )
        if (result.exitCode != 0) {
            throw HostError(
                "VIRTUAL_DISPLAY_LAUNCH_FAILED",
                result.stderr.ifBlank { result.stdout }.ifBlank { "am start failed" },
            )
        }
        ensureDisplayReady(displayId)
        pair.second.packageName = packageName
        return sessionJson(displayId, pair.second)
            .put("component", component)
    }

    private fun capture(request: JSONObject): JSONObject {
        val pair = resolveSession(request)
        ensureDisplayReady(pair.first)
        val format = request.optString("format", "jpeg").lowercase()
        val quality = request.optInt("quality", 82).coerceIn(1, 100)
        val maxWidth = request.optInt("maxWidth", 1280).coerceIn(320, 2560)
        val maxHeight = request.optInt("maxHeight", 1280).coerceIn(320, 2560)
        val image = acquireImage(pair.second.reader, 3000L)
        if (image == null) {
            return captureWithSystemScreencap(
                pair.first,
                pair.second.ref,
                format,
                quality,
                maxWidth,
                maxHeight,
            )
        }
        val bitmap = try {
            imageToBitmap(image)
        } finally {
            image.close()
        }
        val output = try {
            val scaled = scale(bitmap, maxWidth, maxHeight)
            if (scaled !== bitmap) bitmap.recycle()
            encode(scaled, format, quality).also { scaled.recycle() }
        } catch (error: Throwable) {
            bitmap.recycle()
            throw HostError("VIRTUAL_DISPLAY_CAPTURE_FAILED", error.message ?: "encode failed")
        }
        pair.second.lastFrameAt = System.currentTimeMillis()
        return JSONObject()
            .put("displayId", pair.first)
            .put("ref", pair.second.ref)
            .put("width", output.width)
            .put("height", output.height)
            .put("mimeType", output.mimeType)
            .put("format", format)
            .put("quality", quality)
            .put("dataBase64", Base64.encodeToString(output.bytes, Base64.NO_WRAP))
            .put("capturedAt", pair.second.lastFrameAt)
    }

    private fun captureWithSystemScreencap(
        displayId: Int,
        ref: String,
        format: String,
        quality: Int,
        maxWidth: Int,
        maxHeight: Int,
    ): JSONObject {
        val surfaceFlingerDisplayId = resolveSurfaceFlingerDisplayId(displayId)
            ?: throw HostError("VIRTUAL_DISPLAY_CAPTURE_FAILED", "surfaceflinger display id not found")
        val file = File(
            "/data/local/tmp",
            "amitia_vd_capture_${UUID.randomUUID().toString().replace("-", "")}.png",
        )
        val result = runCommand(
            listOf(
                "/system/bin/screencap",
                "-p",
                "-d",
                surfaceFlingerDisplayId,
                file.absolutePath,
            ),
            10000L,
        )
        if (result.exitCode != 0 || !file.isFile) {
            file.delete()
            throw HostError(
                "VIRTUAL_DISPLAY_CAPTURE_FAILED",
                result.stderr.ifBlank { result.stdout }.ifBlank { "screencap failed" },
            )
        }
        val bitmap = try {
            BitmapFactory.decodeFile(file.absolutePath)
                ?: throw HostError("VIRTUAL_DISPLAY_CAPTURE_FAILED", "screencap bitmap decode failed")
        } finally {
            file.delete()
        }
        val output = try {
            val scaled = scale(bitmap, maxWidth, maxHeight)
            if (scaled !== bitmap) bitmap.recycle()
            encode(scaled, format, quality).also { scaled.recycle() }
        } catch (error: Throwable) {
            bitmap.recycle()
            throw HostError("VIRTUAL_DISPLAY_CAPTURE_FAILED", error.message ?: "encode failed")
        }
        val capturedAt = System.currentTimeMillis()
        return JSONObject()
            .put("displayId", displayId)
            .put("ref", ref)
            .put("width", output.width)
            .put("height", output.height)
            .put("mimeType", output.mimeType)
            .put("format", format)
            .put("quality", quality)
            .put("dataBase64", Base64.encodeToString(output.bytes, Base64.NO_WRAP))
            .put("capturedAt", capturedAt)
    }

    private fun resolveSurfaceFlingerDisplayId(displayId: Int): String? {
        val result = runCommand(
            listOf("/system/bin/dumpsys", "SurfaceFlinger", "--display-id"),
            5000L,
        )
        if (result.exitCode != 0) return null
        val surfaceFlingerIds = result.stdout.lineSequence()
            .filter { it.contains("displayName=\"amitia_virtual\"") }
            .mapNotNull { line ->
                Regex("Display\\s+(\\d+)").find(line)?.groupValues?.get(1)
            }
            .toList()
        if (surfaceFlingerIds.isEmpty()) return null
        val sortedSessionIds = sessions.keys.sorted()
        val index = sortedSessionIds.indexOf(displayId)
        if (index < 0) return null
        return surfaceFlingerIds.getOrNull(index)
    }

    private fun tap(request: JSONObject): JSONObject {
        val pair = resolveSession(request)
        val x = request.optDouble("x", Double.NaN).toFloat()
        val y = request.optDouble("y", Double.NaN).toFloat()
        if (x.isNaN() || y.isNaN()) {
            throw HostError("VIRTUAL_DISPLAY_INVALID_REQUEST", "x and y are required")
        }
        val injected = inputController?.tap(pair.first, x, y) == true
        val result = if (injected) null else runCommand(
            listOf(
                "/system/bin/input",
                "-d",
                pair.first.toString(),
                "tap",
                x.toString(),
                y.toString(),
            ),
            5000L,
        )
        if (!injected && result?.exitCode != 0) {
            throw HostError(
                "VIRTUAL_DISPLAY_INPUT_FAILED",
                result?.stderr.orEmpty().ifBlank { result?.stdout.orEmpty() }.ifBlank { "tap injection failed" },
            )
        }
        return JSONObject().put("displayId", pair.first).put("x", x.toDouble()).put("y", y.toDouble())
    }

    private fun swipe(request: JSONObject): JSONObject {
        val pair = resolveSession(request)
        val startX = request.optDouble("startX", Double.NaN).toFloat()
        val startY = request.optDouble("startY", Double.NaN).toFloat()
        val endX = request.optDouble("endX", Double.NaN).toFloat()
        val endY = request.optDouble("endY", Double.NaN).toFloat()
        val durationMs = request.optLong("durationMs", 300L).coerceIn(1L, 5000L)
        if (startX.isNaN() || startY.isNaN() || endX.isNaN() || endY.isNaN()) {
            throw HostError("VIRTUAL_DISPLAY_INVALID_REQUEST", "swipe coordinates are required")
        }
        val injected = inputController?.swipe(pair.first, startX, startY, endX, endY, durationMs) == true
        val result = if (injected) null else runCommand(
            listOf(
                "/system/bin/input",
                "-d",
                pair.first.toString(),
                "swipe",
                startX.toString(),
                startY.toString(),
                endX.toString(),
                endY.toString(),
                durationMs.toString(),
            ),
            8000L,
        )
        if (!injected && result?.exitCode != 0) {
            throw HostError(
                "VIRTUAL_DISPLAY_INPUT_FAILED",
                result?.stderr.orEmpty().ifBlank { result?.stdout.orEmpty() }.ifBlank { "swipe injection failed" },
            )
        }
        return JSONObject()
            .put("displayId", pair.first)
            .put("startX", startX.toDouble())
            .put("startY", startY.toDouble())
            .put("endX", endX.toDouble())
            .put("endY", endY.toDouble())
            .put("durationMs", durationMs)
    }

    private fun key(request: JSONObject): JSONObject {
        val pair = resolveSession(request)
        val keyCode = request.optInt("keyCode", KeyEvent.KEYCODE_UNKNOWN)
        val metaState = request.optInt("metaState", 0)
        if (keyCode == KeyEvent.KEYCODE_UNKNOWN) {
            throw HostError("VIRTUAL_DISPLAY_INVALID_REQUEST", "keyCode is required")
        }
        val injected = inputController?.key(pair.first, keyCode, metaState) == true
        val result = if (injected) null else runCommand(
            listOf(
                "/system/bin/input",
                "-d",
                pair.first.toString(),
                "keyevent",
                keyCode.toString(),
            ),
            5000L,
        )
        if (!injected && result?.exitCode != 0) {
            throw HostError(
                "VIRTUAL_DISPLAY_INPUT_FAILED",
                result?.stderr.orEmpty().ifBlank { result?.stdout.orEmpty() }.ifBlank { "key injection failed" },
            )
        }
        return JSONObject()
            .put("displayId", pair.first)
            .put("keyCode", keyCode)
            .put("metaState", metaState)
    }

    private fun text(request: JSONObject): JSONObject {
        val pair = resolveSession(request)
        val value = request.optString("text")
        if (value.isBlank()) {
            throw HostError("VIRTUAL_DISPLAY_INVALID_REQUEST", "text is required")
        }
        if (value.length > 4096) {
            throw HostError("VIRTUAL_DISPLAY_INVALID_REQUEST", "text exceeds maximum length")
        }
        if (request.optBoolean("replace", false) && !clearTextSession(pair.first)) {
            throw HostError("VIRTUAL_DISPLAY_INPUT_FAILED", "failed to clear existing text")
        }
        if (pasteText(pair.first, value)) {
            return JSONObject()
                .put("displayId", pair.first)
                .put("length", value.length)
        }
        val result = runCommand(
            listOf("/system/bin/input", "-d", pair.first.toString(), "text", value),
            8000L,
        )
        if (result.exitCode != 0) {
            throw HostError(
                "VIRTUAL_DISPLAY_INPUT_FAILED",
                result.stderr.ifBlank { result.stdout }.ifBlank { "text input failed" },
            )
        }
        return JSONObject()
            .put("displayId", pair.first)
            .put("length", value.length)
    }

    private fun pasteText(displayId: Int, value: String): Boolean {
        val clipboard = context.getSystemService(Context.CLIPBOARD_SERVICE) as? ClipboardManager ?: return false
        return try {
            clipboard.setPrimaryClip(ClipData.newPlainText("amitia_virtual", value))
            inputController?.key(displayId, KeyEvent.KEYCODE_PASTE, 0) == true
        } catch (_: Throwable) {
            false
        }
    }

    private fun clearText(request: JSONObject): JSONObject {
        val pair = resolveSession(request)
        if (!clearTextSession(pair.first)) {
            throw HostError("VIRTUAL_DISPLAY_INPUT_FAILED", "text clear failed")
        }
        return JSONObject()
            .put("displayId", pair.first)
            .put("cleared", true)
    }

    private fun clearTextSession(displayId: Int): Boolean {
        val moveResult = runCommand(
            listOf(
                "/system/bin/input",
                "-d",
                displayId.toString(),
                "keyevent",
                KeyEvent.KEYCODE_MOVE_END.toString(),
            ),
            5000L,
        )
        val moveSucceeded = moveResult.exitCode == 0 ||
            inputController?.key(displayId, KeyEvent.KEYCODE_MOVE_END, 0) == true
        if (!moveSucceeded) return false
        val deleteCommand = mutableListOf(
            "/system/bin/input",
            "-d",
            displayId.toString(),
            "keyevent",
        )
        repeat(128) {
            deleteCommand.add(KeyEvent.KEYCODE_DEL.toString())
        }
        val deleteResult = runCommand(deleteCommand, 10000L)
        if (deleteResult.exitCode == 0) return true
        repeat(128) {
            if (inputController?.key(displayId, KeyEvent.KEYCODE_DEL, 0) != true) return false
        }
        return true
    }

    private fun uiTree(request: JSONObject): JSONObject {
        val pair = resolveSession(request)
        val displayId = pair.first
        ensureDisplayReady(displayId)
        uiAutomationTree(displayId)?.let { return it }
        return uiautomatorTree(pair)
    }

    private fun uiautomatorTree(pair: Pair<Int, DisplaySession>): JSONObject {
        val displayId = pair.first
        val dumpFile = File(
            "/data/local/tmp",
            "amitia_vd_ui_${UUID.randomUUID().toString().replace("-", "")}.xml",
        )
        val result = runCommand(
            listOf(
                "/system/bin/uiautomator",
                "dump",
                "--display-id",
                displayId.toString(),
                dumpFile.absolutePath,
            ),
            12000L,
        )
        if (result.exitCode != 0 || !dumpFile.isFile) {
            dumpFile.delete()
            throw HostError(
                "VIRTUAL_DISPLAY_UI_TREE_FAILED",
                result.stderr.ifBlank { result.stdout }.ifBlank { "uiautomator dump failed" },
            )
        }
        val xml = try {
            dumpFile.readText()
        } finally {
            dumpFile.delete()
        }
        val tree = parseUiTree(displayId, xml)
        validateUiTreeBounds(tree, pair.second)
        return tree
    }

    private fun uiAutomationTree(displayId: Int): JSONObject? {
        val automation = uiAutomation ?: return null
        return try {
            automation.clearCache()
            val allWindows: SparseArray<List<AccessibilityWindowInfo>> =
                automation.getWindowsOnAllDisplays()
            val displayWindows = allWindows[displayId] ?: return null
            if (displayWindows.isEmpty()) return null
            val nodes = JSONArray()
            val windows = JSONArray()
            val capturedAt = System.currentTimeMillis()
            for (window in displayWindows.sortedBy { it.layer }) {
                val root = window.root ?: continue
                val windowId = "vd-ui-window-$displayId-${window.id}"
                val rootNodeId = uiAutomationNodeId(displayId, window.id, emptyList())
                val bounds = Rect().also { window.getBoundsInScreen(it) }
                windows.put(
                    JSONObject()
                        .put("windowId", windowId)
                        .put("type", uiAutomationWindowType(window.type))
                        .put("packageName", root.packageName?.toString().orEmpty())
                        .put("title", window.title?.toString().orEmpty())
                        .put("active", window.isActive)
                        .put("focused", window.isFocused)
                        .put("displayId", displayId)
                        .put("layer", window.layer)
                        .put("left", bounds.left)
                        .put("top", bounds.top)
                        .put("right", bounds.right)
                        .put("bottom", bounds.bottom)
                        .put("rootNodeId", rootNodeId),
                )
                appendUiAutomationNode(
                    node = root,
                    nodes = nodes,
                    displayId = displayId,
                    windowId = window.id,
                    windowRef = windowId,
                    path = emptyList(),
                    parentId = null,
                    depth = 0,
                )
            }
            if (windows.length() == 0 || nodes.length() == 0) return null
            val activeWindowId = (0 until windows.length())
                .map { windows.getJSONObject(it) }
                .firstOrNull { it.optBoolean("active", false) }
                ?.optString("windowId")
                .orEmpty()
            JSONObject()
                .put("nodes", nodes)
                .put("windows", windows)
                .put("windowCount", windows.length())
                .put("activeWindowId", activeWindowId)
                .put("generation", capturedAt)
                .put("capturedAt", capturedAt)
                .put("accessibilityConnected", true)
                .put("multiWindow", windows.length() > 1)
                .put("stableNodeReference", true)
                .put("truncated", false)
                .put("source", "ui_automation")
        } catch (_: Throwable) {
            null
        }
    }

    private fun appendUiAutomationNode(
        node: AccessibilityNodeInfo,
        nodes: JSONArray,
        displayId: Int,
        windowId: Int,
        windowRef: String,
        path: List<Int>,
        parentId: String?,
        depth: Int,
    ) {
        val nodeId = uiAutomationNodeId(displayId, windowId, path)
        val bounds = Rect().also { node.getBoundsInScreen(it) }
        val className = node.className?.toString().orEmpty()
        val resourceId = node.viewIdResourceName.orEmpty()
        val editable = node.isEditable
        val actions = JSONArray()
        if (node.isClickable) actions.put("ACTION_CLICK")
        if (node.isLongClickable) actions.put("ACTION_LONG_CLICK")
        if (node.isScrollable) {
            actions.put("ACTION_SCROLL_FORWARD")
            actions.put("ACTION_SCROLL_BACKWARD")
        }
        if (editable) actions.put("ACTION_SET_TEXT")
        val item = JSONObject()
            .put("nodeId", nodeId)
            .put("parentId", parentId ?: JSONObject.NULL)
            .put("windowId", windowRef)
            .put("className", className)
            .put("packageName", node.packageName?.toString().orEmpty())
            .put("text", node.text?.toString().orEmpty())
            .put("contentDescription", node.contentDescription?.toString().orEmpty())
            .put("resourceId", resourceId)
            .put("left", bounds.left)
            .put("top", bounds.top)
            .put("right", bounds.right)
            .put("bottom", bounds.bottom)
            .put("visibleToUser", node.isVisibleToUser)
            .put("enabled", node.isEnabled)
            .put("focusable", node.isFocusable)
            .put("focused", node.isFocused)
            .put("selected", node.isSelected)
            .put("checked", node.isChecked)
            .put("checkable", node.isCheckable)
            .put("clickable", node.isClickable)
            .put("longClickable", node.isLongClickable)
            .put("scrollable", node.isScrollable)
            .put("editable", editable)
            .put("password", node.isPassword)
            .put("actions", actions)
            .put("depth", depth)
            .put("sourceRef", virtualNodeReference(displayId, bounds))
        nodes.put(item)
        for (index in 0 until node.childCount) {
            val child = node.getChild(index) ?: continue
            appendUiAutomationNode(
                node = child,
                nodes = nodes,
                displayId = displayId,
                windowId = windowId,
                windowRef = windowRef,
                path = path + index,
                parentId = nodeId,
                depth = depth + 1,
            )
        }
    }

    private fun uiAutomationNodeId(displayId: Int, windowId: Int, path: List<Int>): String {
        val digest = MessageDigest.getInstance("SHA-256")
            .digest("ui:$displayId:$windowId:${path.joinToString("/")}".toByteArray())
        return "vd_ui_node_" + digest.take(12).joinToString("") { "%02x".format(it) }
    }

    private fun uiAutomationWindowType(type: Int): String = when (type) {
        AccessibilityWindowInfo.TYPE_APPLICATION -> "application"
        AccessibilityWindowInfo.TYPE_INPUT_METHOD -> "input_method"
        AccessibilityWindowInfo.TYPE_SYSTEM -> "system"
        AccessibilityWindowInfo.TYPE_ACCESSIBILITY_OVERLAY -> "accessibility_overlay"
        AccessibilityWindowInfo.TYPE_SPLIT_SCREEN_DIVIDER -> "split_screen_divider"
        else -> "unknown"
    }

    private fun ensureDisplayReady(displayId: Int) {
        if (inputController?.key(displayId, KeyEvent.KEYCODE_WAKEUP, 0) != true) {
            runCommand(
                listOf(
                    "/system/bin/input",
                    "-d",
                    displayId.toString(),
                    "keyevent",
                    KeyEvent.KEYCODE_WAKEUP.toString(),
                ),
                5000L,
            )
        }
        if (inputController?.key(displayId, KeyEvent.KEYCODE_UNKNOWN, 0) != true) {
            runCommand(
                listOf(
                    "/system/bin/input",
                    "-d",
                    displayId.toString(),
                    "keyevent",
                    KeyEvent.KEYCODE_UNKNOWN.toString(),
                ),
                5000L,
            )
        }
        if (isDisplayKeyguardShowing(displayId)) {
            runCommand(listOf("/system/bin/wm", "dismiss-keyguard"), 5000L)
            Thread.sleep(250L)
        }
    }

    private fun isDisplayKeyguardShowing(displayId: Int): Boolean {
        val result = runCommand(
            listOf("/system/bin/dumpsys", "window", "displays"),
            5000L,
        )
        if (result.exitCode != 0) return false
        val lines = result.stdout.lines()
        val start = lines.indexOfFirst { it.contains("Display: mDisplayId=$displayId") }
        if (start < 0) return false
        val end = (start + 1 until lines.size).firstOrNull { index ->
            lines[index].contains("Display: mDisplayId=")
        } ?: lines.size
        return lines.subList(start, end).any { line ->
            line.contains("isKeyguardShowing=true") ||
                (line.contains("mCurrentFocus=Window{") && line.contains("Keyguard"))
        }
    }

    private fun validateUiTreeBounds(tree: JSONObject, session: DisplaySession) {
        val windows = tree.optJSONArray("windows") ?: JSONArray()
        if (windows.length() == 0) {
            throw HostError(
                "VIRTUAL_DISPLAY_UI_TREE_FAILED",
                "no window found for display ${session.name}",
            )
        }
        val root = windows.optJSONObject(0)
        val right = root?.optInt("right", 0) ?: 0
        val bottom = root?.optInt("bottom", 0) ?: 0
        if (
            right < session.width * 0.8 ||
            bottom < session.height * 0.8 ||
            right > session.width + 8 ||
            bottom > session.height + 8
        ) {
            throw HostError(
                "VIRTUAL_DISPLAY_UI_TREE_FAILED",
                "uiautomator returned a tree for a different display",
            )
        }
    }

    private fun parseUiTree(displayId: Int, xml: String): JSONObject {
        if (xml.isBlank()) {
            throw HostError("VIRTUAL_DISPLAY_UI_TREE_FAILED", "uiautomator returned empty XML")
        }
        val parser = XmlPullParserFactory.newInstance().newPullParser()
        parser.setInput(StringReader(xml))
        val nodes = JSONArray()
        val stack = ArrayDeque<JSONObject>()
        val windowId = "vd-window-$displayId"
        var rootNodeId = ""
        var rootBounds = Rect()
        var rootPackage = ""
        var event = parser.eventType
        while (event != XmlPullParser.END_DOCUMENT) {
            if (event == XmlPullParser.START_TAG && parser.name == "node") {
                val parent = stack.lastOrNull()
                val index = parser.getAttributeValue(null, "index").orEmpty().ifBlank { "0" }
                val parentPath = parent?.optString("_path").orEmpty()
                val path = if (parentPath.isBlank()) index else "$parentPath/$index"
                val className = parser.getAttributeValue(null, "class").orEmpty()
                val packageName = parser.getAttributeValue(null, "package").orEmpty()
                val resourceId = parser.getAttributeValue(null, "resource-id").orEmpty()
                val bounds = parseUiBounds(parser.getAttributeValue(null, "bounds").orEmpty())
                val clickable = parser.booleanAttribute("clickable")
                val longClickable = parser.booleanAttribute("long-clickable")
                val scrollable = parser.booleanAttribute("scrollable")
                val editable = className.contains("EditText", ignoreCase = true) ||
                    className.contains("Editor", ignoreCase = true)
                val actions = JSONArray()
                if (clickable) actions.put("ACTION_CLICK")
                if (longClickable) actions.put("ACTION_LONG_CLICK")
                if (scrollable) {
                    actions.put("ACTION_SCROLL_FORWARD")
                    actions.put("ACTION_SCROLL_BACKWARD")
                }
                if (editable) actions.put("ACTION_SET_TEXT")
                val nodeId = uiNodeId(displayId, path, className, resourceId, bounds)
                val item = JSONObject()
                    .put("nodeId", nodeId)
                    .put("parentId", parent?.optString("nodeId")?.takeIf { it.isNotBlank() } ?: JSONObject.NULL)
                    .put("windowId", windowId)
                    .put("className", className)
                    .put("packageName", packageName)
                    .put("text", parser.getAttributeValue(null, "text").orEmpty())
                    .put("contentDescription", parser.getAttributeValue(null, "content-desc").orEmpty())
                    .put("resourceId", resourceId)
                    .put("left", bounds.left)
                    .put("top", bounds.top)
                    .put("right", bounds.right)
                    .put("bottom", bounds.bottom)
                    .put("visibleToUser", true)
                    .put("enabled", parser.booleanAttribute("enabled", true))
                    .put("focusable", parser.booleanAttribute("focusable"))
                    .put("focused", parser.booleanAttribute("focused"))
                    .put("selected", parser.booleanAttribute("selected"))
                    .put("checked", parser.booleanAttribute("checked"))
                    .put("checkable", parser.booleanAttribute("checkable"))
                    .put("clickable", clickable)
                    .put("longClickable", longClickable)
                    .put("scrollable", scrollable)
                    .put("editable", editable)
                    .put("password", parser.booleanAttribute("password"))
                    .put("actions", actions)
                    .put("depth", stack.size)
                    .put("sourceRef", virtualNodeReference(displayId, bounds))
                    .put("_path", path)
                if (parent == null) {
                    rootNodeId = nodeId
                    rootBounds = bounds
                    rootPackage = packageName
                }
                nodes.put(item)
                stack.addLast(item)
            } else if (event == XmlPullParser.END_TAG && parser.name == "node") {
                if (stack.isNotEmpty()) stack.removeLast()
            }
            event = parser.next()
        }
        for (index in 0 until nodes.length()) {
            nodes.getJSONObject(index).remove("_path")
        }
        val windows = JSONArray()
        if (rootNodeId.isNotBlank() || nodes.length() > 0) {
            windows.put(
                JSONObject()
                    .put("windowId", windowId)
                    .put("type", "application")
                    .put("packageName", rootPackage)
                    .put("title", "")
                    .put("active", true)
                    .put("focused", true)
                    .put("displayId", displayId)
                    .put("left", rootBounds.left)
                    .put("top", rootBounds.top)
                    .put("right", rootBounds.right)
                    .put("bottom", rootBounds.bottom)
                    .put("rootNodeId", rootNodeId),
            )
        }
        val capturedAt = System.currentTimeMillis()
        return JSONObject()
            .put("nodes", nodes)
            .put("windows", windows)
            .put("windowCount", windows.length())
            .put("activeWindowId", if (windows.length() > 0) windowId else "")
            .put("generation", capturedAt)
            .put("capturedAt", capturedAt)
            .put("accessibilityConnected", false)
            .put("multiWindow", false)
            .put("stableNodeReference", true)
            .put("truncated", false)
            .put("source", "uiautomator")
    }

    private fun XmlPullParser.booleanAttribute(name: String, fallback: Boolean = false): Boolean {
        val value = getAttributeValue(null, name) ?: return fallback
        return value.equals("true", ignoreCase = true)
    }

    private fun parseUiBounds(value: String): Rect {
        val values = Regex("-?\\d+").findAll(value).take(4).map { it.value.toIntOrNull() ?: 0 }.toList()
        if (values.size != 4) return Rect()
        return Rect(values[0], values[1], values[2], values[3])
    }

    private fun virtualNodeReference(displayId: Int, bounds: Rect): String =
        "vd:$displayId:${bounds.left},${bounds.top},${bounds.right},${bounds.bottom}"

    private fun uiNodeId(displayId: Int, path: String, className: String, resourceId: String, bounds: Rect): String {
        val digest = MessageDigest.getInstance("SHA-256")
            .digest("$displayId|$path|$className|$resourceId|$bounds".toByteArray())
        return "vd_node_" + digest.take(12).joinToString("") { "%02x".format(it) }
    }

    private fun status(): JSONObject {
        val array = JSONArray()
        sessions.entries.sortedBy { it.key }.forEach { entry ->
            array.put(sessionJson(entry.key, entry.value))
        }
        return JSONObject()
            .put("supported", true)
            .put("available", true)
            .put("state", "ready")
            .put("provider", "app_process")
            .put("hostPid", Process.myPid())
            .put("hostUid", Process.myUid())
            .put("inputReady", true)
            .put("gestureSupported", true)
            .put("keySupported", true)
            .put("textSupported", true)
            .put("thirdPartyLaunchSupported", true)
            .put("frameSourceSupported", true)
            .put("uiTreeSupported", true)
            .put("activeCount", array.length())
            .put("displays", array)
    }

    private fun resolveSession(request: JSONObject): Pair<Int, DisplaySession> {
        val ref = request.optString("ref").trim()
        if (ref.isNotBlank()) {
            sessions.entries.firstOrNull { it.value.ref == ref }?.let { return it.key to it.value }
            throw HostError("VIRTUAL_DISPLAY_NOT_FOUND", "virtual display not found")
        }
        val displayId = if (request.has("displayId")) request.optInt("displayId") else -1
        if (displayId >= 0) {
            sessions[displayId]?.let { return displayId to it }
            throw HostError("VIRTUAL_DISPLAY_NOT_FOUND", "virtual display not found")
        }
        if (sessions.size == 1) {
            val entry = sessions.entries.first()
            return entry.key to entry.value
        }
        throw HostError("VIRTUAL_DISPLAY_ID_MISMATCH", "displayId or ref is required")
    }

    private fun sessionJson(displayId: Int, session: DisplaySession): JSONObject {
        return JSONObject()
            .put("ref", session.ref)
            .put("displayId", displayId)
            .put("name", session.name)
            .put("width", session.width)
            .put("height", session.height)
            .put("densityDpi", session.densityDpi)
            .put("generation", session.generation)
            .put("packageName", session.packageName ?: JSONObject.NULL)
            .put("surfaceAttached", session.display.surface != null)
            .put("active", sessions.containsKey(displayId))
            .put("lastFrameAt", session.lastFrameAt)
            .put("provider", "app_process")
    }

    private fun closeSession(session: DisplaySession) {
        try {
            session.display.setSurface(null)
        } catch (_: Throwable) {
        }
        try {
            session.display.release()
        } catch (_: Throwable) {
        }
        try {
            session.reader.close()
        } catch (_: Throwable) {
        }
    }

    private fun createDisplayManager(context: Context): DisplayManager {
        val constructor: Constructor<DisplayManager> =
            DisplayManager::class.java.getDeclaredConstructor(Context::class.java)
        constructor.isAccessible = true
        return constructor.newInstance(context)
    }

    @SuppressLint("BlockedPrivateApi")
    private fun createUiAutomation(): UiAutomation? = runCatching {
        val connectionClass = Class.forName("android.app.UiAutomationConnection")
        val connection = connectionClass.getDeclaredConstructor().newInstance()
        val automationClass = UiAutomation::class.java
        val connectionInterface = Class.forName("android.app.IUiAutomationConnection")
        val constructor = automationClass.getDeclaredConstructor(Looper::class.java, connectionInterface)
        constructor.isAccessible = true
        val automation = constructor.newInstance(Looper.getMainLooper(), connection) as UiAutomation
        val connect = automationClass.getDeclaredMethod(
            "connectWithTimeout",
            Int::class.javaPrimitiveType,
            Long::class.javaPrimitiveType,
        )
        connect.isAccessible = true
        connect.invoke(automation, 1, 5000L)
        automation
    }.getOrNull()

    private fun acquireImage(reader: ImageReader, timeoutMs: Long): Image? {
        val deadline = SystemClock.uptimeMillis() + timeoutMs
        while (SystemClock.uptimeMillis() <= deadline) {
            val image = reader.acquireLatestImage()
            if (image != null) return image
            Thread.sleep(20L)
        }
        return null
    }

    private fun imageToBitmap(image: Image): Bitmap {
        val plane = image.planes.firstOrNull()
            ?: throw HostError("VIRTUAL_DISPLAY_CAPTURE_FAILED", "image plane unavailable")
        val width = image.width
        val height = image.height
        val pixelStride = plane.pixelStride
        val rowStride = plane.rowStride
        val rowPadding = rowStride - pixelStride * width
        val paddedWidth = width + rowPadding / pixelStride.coerceAtLeast(1)
        val padded = Bitmap.createBitmap(paddedWidth, height, Bitmap.Config.ARGB_8888)
        plane.buffer.rewind()
        padded.copyPixelsFromBuffer(plane.buffer)
        if (paddedWidth == width) return padded
        val cropped = Bitmap.createBitmap(padded, 0, 0, width, height)
        padded.recycle()
        return cropped
    }

    private fun scale(bitmap: Bitmap, maxWidth: Int, maxHeight: Int): Bitmap {
        val ratio = minOf(
            maxWidth.toDouble() / bitmap.width.toDouble(),
            maxHeight.toDouble() / bitmap.height.toDouble(),
            1.0,
        )
        if (ratio >= 1.0) return bitmap
        return Bitmap.createScaledBitmap(
            bitmap,
            (bitmap.width * ratio).toInt().coerceAtLeast(1),
            (bitmap.height * ratio).toInt().coerceAtLeast(1),
            true,
        )
    }

    private fun encode(bitmap: Bitmap, format: String, quality: Int): EncodedFrame {
        val compressFormat = when (format) {
            "png" -> Bitmap.CompressFormat.PNG
            "webp" -> if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
                Bitmap.CompressFormat.WEBP_LOSSY
            } else {
                @Suppress("DEPRECATION")
                Bitmap.CompressFormat.WEBP
            }
            else -> Bitmap.CompressFormat.JPEG
        }
        val mime = when (format) {
            "png" -> "image/png"
            "webp" -> "image/webp"
            else -> "image/jpeg"
        }
        val stream = ByteArrayOutputStream()
        if (!bitmap.compress(compressFormat, quality, stream)) {
            stream.close()
            throw HostError("VIRTUAL_DISPLAY_CAPTURE_FAILED", "bitmap compression failed")
        }
        val bytes = stream.toByteArray()
        stream.close()
        if (bytes.isEmpty()) {
            throw HostError("VIRTUAL_DISPLAY_CAPTURE_FAILED", "encoded frame is empty")
        }
        if (bytes.size > 16 * 1024 * 1024) {
            throw HostError("VIRTUAL_DISPLAY_CAPTURE_TOO_LARGE", "encoded frame exceeds bridge limit")
        }
        return EncodedFrame(bytes, bitmap.width, bitmap.height, mime)
    }

    private fun resolveLaunchComponent(packageName: String, requested: String): String? {
        if (requested.isNotBlank()) return requested
        val launchIntent = context.packageManager.getLaunchIntentForPackage(packageName)
        val component = launchIntent?.component
        if (component != null) {
            return component.flattenToShortString()
        }
        val result = runCommand(
            listOf(
                "/system/bin/cmd",
                "package",
                "resolve-activity",
                "--brief",
                "-c",
                Intent.CATEGORY_LAUNCHER,
                packageName,
            ),
            8000L,
        )
        if (result.exitCode != 0) return null
        return result.stdout.lineSequence()
            .map { it.trim() }
            .lastOrNull { it.contains("/") }
    }

    private fun runCommand(command: List<String>, timeoutMs: Long): CommandResult {
        val process = ProcessBuilder(command).start()
        val stdout = StringBuilder()
        val stderr = StringBuilder()
        val stdoutThread = Thread {
            try {
                BufferedReader(InputStreamReader(process.inputStream)).use { reader ->
                    reader.forEachLine { stdout.append(it).append('\n') }
                }
            } catch (_: Throwable) {
            }
        }
        val stderrThread = Thread {
            try {
                BufferedReader(InputStreamReader(process.errorStream)).use { reader ->
                    reader.forEachLine { stderr.append(it).append('\n') }
                }
            } catch (_: Throwable) {
            }
        }
        stdoutThread.start()
        stderrThread.start()
        val finished = process.waitFor(timeoutMs, TimeUnit.MILLISECONDS)
        if (!finished) {
            process.destroy()
            if (!process.waitFor(300L, TimeUnit.MILLISECONDS)) {
                process.destroyForcibly()
            }
            stdoutThread.join(500L)
            stderrThread.join(500L)
            return CommandResult(-1, stdout.toString(), stderr.toString())
        }
        stdoutThread.join(500L)
        stderrThread.join(500L)
        return CommandResult(process.exitValue(), stdout.toString(), stderr.toString())
    }

    private fun broadcastBinder() {
        if (stopped) return
        try {
            val intent = Intent(VirtualDisplayHostContract.ACTION_READY)
                .setPackage(hostPackage)
                .putExtra(VirtualDisplayHostContract.EXTRA_TOKEN, token)
                .putExtra(
                    VirtualDisplayHostContract.EXTRA_CONTAINER,
                    VirtualDisplayHostBinderContainer(asBinder()),
                )
            context.sendBroadcast(intent)
        } catch (_: Throwable) {
        }
    }

    private fun shutdownHost(): JSONObject {
        if (stopped) return JSONObject().put("stopped", true)
        stopped = true
        sessions.keys.toList().forEach { displayId ->
            sessions.remove(displayId)?.let { closeSession(it) }
        }
        scheduler.shutdownNow()
        Thread {
            Thread.sleep(150L)
            Process.killProcess(Process.myPid())
        }.apply {
            isDaemon = true
            start()
        }
        return JSONObject().put("stopped", true)
    }

    private fun successResponse(requestId: String, result: JSONObject): String {
        return JSONObject()
            .put("requestId", requestId)
            .put("status", "success")
            .put("result", result)
            .toString()
    }

    private fun errorResponse(requestId: String, code: String, message: String): String {
        return JSONObject()
            .put("requestId", requestId)
            .put("status", "error")
            .put("error", JSONObject().put("code", code).put("message", message))
            .toString()
    }

    private data class EncodedFrame(
        val bytes: ByteArray,
        val width: Int,
        val height: Int,
        val mimeType: String,
    )

    private data class CommandResult(
        val exitCode: Int,
        val stdout: String,
        val stderr: String,
    )

    private class HostError(
        val code: String,
        message: String,
    ) : RuntimeException(message)
}
