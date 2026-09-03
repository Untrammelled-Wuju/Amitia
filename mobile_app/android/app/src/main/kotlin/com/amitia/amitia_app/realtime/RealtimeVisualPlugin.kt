package com.amitia.amitia_app.realtime

import android.Manifest
import android.app.Activity
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.content.ServiceConnection
import android.content.pm.PackageManager
import android.graphics.Bitmap
import android.graphics.ImageFormat
import android.graphics.PixelFormat
import android.hardware.camera2.CameraCaptureSession
import android.hardware.camera2.CameraCharacteristics
import android.hardware.camera2.CameraDevice
import android.hardware.camera2.CameraManager
import android.hardware.camera2.CaptureRequest
import android.media.ImageReader
import android.media.projection.MediaProjection
import android.media.projection.MediaProjectionManager
import android.os.Build
import android.os.Handler
import android.os.HandlerThread
import android.os.IBinder
import android.os.Looper
import android.util.DisplayMetrics
import io.flutter.embedding.engine.plugins.FlutterPlugin
import io.flutter.embedding.engine.plugins.activity.ActivityAware
import io.flutter.embedding.engine.plugins.activity.ActivityPluginBinding
import io.flutter.plugin.common.EventChannel
import io.flutter.plugin.common.MethodCall
import io.flutter.plugin.common.MethodChannel
import io.flutter.plugin.common.PluginRegistry
import java.io.ByteArrayOutputStream
import java.nio.ByteBuffer
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicLong
import kotlin.math.max
import kotlin.math.min

class RealtimeVisualPlugin : FlutterPlugin,
    ActivityAware,
    EventChannel.StreamHandler,
    PluginRegistry.RequestPermissionsResultListener,
    PluginRegistry.ActivityResultListener {

    private var applicationContext: Context? = null
    private var activityBinding: ActivityPluginBinding? = null
    private var activity: Activity? = null
    private var controlChannel: MethodChannel? = null
    private var frameChannel: EventChannel? = null
    private var eventSink: EventChannel.EventSink? = null
    private val mainHandler = Handler(Looper.getMainLooper())

    private val workerThread = HandlerThread("amitia-realtime-visual").apply { start() }
    private val worker = Handler(workerThread.looper)
    private val cameraActive = AtomicBoolean(false)
    private val screenActive = AtomicBoolean(false)
    private val cameraSequence = AtomicLong(0)
    private val screenSequence = AtomicLong(0)

    private var cameraDevice: CameraDevice? = null
    private var cameraSession: CameraCaptureSession? = null
    private var cameraReader: ImageReader? = null
    private var cameraFacing = CameraCharacteristics.LENS_FACING_FRONT
    private var pendingCameraResult: MethodChannel.Result? = null
    private var cameraCaptureRunnable: Runnable? = null

    private var pendingScreenResult: MethodChannel.Result? = null
    private var pendingProjectionResultCode: Int? = null
    private var pendingProjectionData: Intent? = null
    private var projectionServiceBound = false
    private var projection: MediaProjection? = null
    private var screenReader: ImageReader? = null
    private var screenDisplay: android.hardware.display.VirtualDisplay? = null
    private var lastScreenFrameAt = 0L
    @Volatile private var forceScreenFrame = false

    private val projectionServiceConnection = object : ServiceConnection {
        override fun onServiceConnected(name: ComponentName?, service: IBinder?) {
            projectionServiceBound = true
            val resultCode = pendingProjectionResultCode
            val data = pendingProjectionData
            val result = pendingScreenResult
            pendingProjectionResultCode = null
            pendingProjectionData = null
            pendingScreenResult = null
            if (resultCode == null || data == null || result == null) {
                stopProjectionService()
                return
            }
            val context = applicationContext ?: run {
                result.error("SCREEN_UNAVAILABLE", "Screen capture context unavailable", null)
                stopProjectionService()
                return
            }
            val manager = context.getSystemService(Context.MEDIA_PROJECTION_SERVICE) as MediaProjectionManager
            try {
                val mediaProjection = manager.getMediaProjection(resultCode, data)
                startProjection(mediaProjection)
                result.success(null)
            } catch (error: Throwable) {
                stopProjectionService()
                result.error("SCREEN_START_FAILED", error.message, null)
            }
        }

        override fun onServiceDisconnected(name: ComponentName?) {
            projectionServiceBound = false
            if (screenActive.get()) releaseScreenResources(stopProjection = true)
        }
    }

    override fun onAttachedToEngine(binding: FlutterPlugin.FlutterPluginBinding) {
        applicationContext = binding.applicationContext
        controlChannel = MethodChannel(binding.binaryMessenger, CONTROL_CHANNEL).also {
            it.setMethodCallHandler(::handleMethodCall)
        }
        frameChannel = EventChannel(binding.binaryMessenger, FRAME_CHANNEL).also {
            it.setStreamHandler(this)
        }
    }

    override fun onDetachedFromEngine(binding: FlutterPlugin.FlutterPluginBinding) {
        reset()
        controlChannel?.setMethodCallHandler(null)
        frameChannel?.setStreamHandler(null)
        controlChannel = null
        frameChannel = null
        eventSink = null
        applicationContext = null
        workerThread.quitSafely()
    }

    override fun onAttachedToActivity(binding: ActivityPluginBinding) {
        activityBinding = binding
        activity = binding.activity
        binding.addRequestPermissionsResultListener(this)
        binding.addActivityResultListener(this)
    }

    override fun onDetachedFromActivityForConfigChanges() = detachActivity()
    override fun onDetachedFromActivity() = detachActivity()
    override fun onReattachedToActivityForConfigChanges(binding: ActivityPluginBinding) = onAttachedToActivity(binding)

    private fun detachActivity() {
        activityBinding?.removeRequestPermissionsResultListener(this)
        activityBinding?.removeActivityResultListener(this)
        activityBinding = null
        activity = null
    }

    override fun onListen(arguments: Any?, events: EventChannel.EventSink?) {
        eventSink = events
    }

    override fun onCancel(arguments: Any?) {
        eventSink = null
    }

    private fun handleMethodCall(call: MethodCall, result: MethodChannel.Result) {
        when (call.method) {
            "startCamera" -> {
                val facing = (call.arguments as? Map<*, *>)?.get("facing")?.toString()?.lowercase()
                cameraFacing = if (facing == "back") CameraCharacteristics.LENS_FACING_BACK else CameraCharacteristics.LENS_FACING_FRONT
                startCameraWithPermission(result)
            }
            "stopCamera" -> {
                stopCamera()
                result.success(null)
            }
            "switchCamera" -> {
                cameraFacing = if (cameraFacing == CameraCharacteristics.LENS_FACING_FRONT) CameraCharacteristics.LENS_FACING_BACK else CameraCharacteristics.LENS_FACING_FRONT
                val wasActive = cameraActive.get()
                stopCamera()
                if (wasActive) startCameraWithPermission(result) else result.success(null)
            }
            "startScreen" -> startScreen(result)
            "stopScreen" -> {
                stopScreen()
                result.success(null)
            }
            "requestImmediateFrame" -> {
                val source = (call.arguments as? Map<*, *>)?.get("source")?.toString()
                if (source == "camera") requestCameraCapture(immediate = true)
                if (source == "screen") forceScreenFrame = true
                result.success(null)
            }
            "status" -> result.success(
                mapOf(
                    "cameraActive" to cameraActive.get(),
                    "screenActive" to screenActive.get(),
                    "cameraSupported" to hasCamera(),
                    "screenSupported" to (Build.VERSION.SDK_INT >= Build.VERSION_CODES.LOLLIPOP),
                    "crossAppScreenSupported" to (Build.VERSION.SDK_INT >= Build.VERSION_CODES.LOLLIPOP),
                ),
            )
            "reset" -> {
                reset()
                result.success(null)
            }
            else -> result.notImplemented()
        }
    }

    private fun startCameraWithPermission(result: MethodChannel.Result) {
        val context = applicationContext ?: run {
            result.error("CAMERA_UNAVAILABLE", "Camera context unavailable", null)
            return
        }
        if (context.checkSelfPermission(Manifest.permission.CAMERA) == PackageManager.PERMISSION_GRANTED) {
            startCamera(result)
            return
        }
        val currentActivity = activity ?: run {
            result.error("CAMERA_PERMISSION_UNAVAILABLE", "Activity unavailable for camera permission", null)
            return
        }
        if (pendingCameraResult != null) {
            result.error("CAMERA_PERMISSION_PENDING", "Camera permission request already pending", null)
            return
        }
        pendingCameraResult = result
        currentActivity.requestPermissions(arrayOf(Manifest.permission.CAMERA), CAMERA_PERMISSION_REQUEST)
    }

    override fun onRequestPermissionsResult(requestCode: Int, permissions: Array<out String>, grantResults: IntArray): Boolean {
        if (requestCode != CAMERA_PERMISSION_REQUEST) return false
        val result = pendingCameraResult
        pendingCameraResult = null
        if (result == null) return true
        if (grantResults.isNotEmpty() && grantResults[0] == PackageManager.PERMISSION_GRANTED) {
            startCamera(result)
        } else {
            result.error("CAMERA_PERMISSION_DENIED", "Camera permission denied", null)
        }
        return true
    }

    private fun startCamera(result: MethodChannel.Result) {
        if (cameraActive.get()) {
            result.success(null)
            return
        }
        val context = applicationContext ?: run {
            result.error("CAMERA_UNAVAILABLE", "Camera context unavailable", null)
            return
        }
        val manager = context.getSystemService(Context.CAMERA_SERVICE) as CameraManager
        val cameraId = manager.cameraIdList.firstOrNull { id ->
            manager.getCameraCharacteristics(id).get(CameraCharacteristics.LENS_FACING) == cameraFacing
        } ?: manager.cameraIdList.firstOrNull()
        if (cameraId == null) {
            result.error("CAMERA_NOT_FOUND", "No camera device available", null)
            return
        }
        val characteristics = manager.getCameraCharacteristics(cameraId)
        val sizes = characteristics.get(CameraCharacteristics.SCALER_STREAM_CONFIGURATION_MAP)
            ?.getOutputSizes(ImageFormat.JPEG)
            ?.toList()
            .orEmpty()
        val target = sizes
            .filter { it.width <= 1920 && it.height <= 1080 }
            .maxByOrNull { it.width * it.height }
            ?: sizes.minByOrNull { kotlin.math.abs(it.width - 1280) + kotlin.math.abs(it.height - 720) }
            ?: android.util.Size(1280, 720)
        cameraReader = ImageReader.newInstance(target.width, target.height, ImageFormat.JPEG, 2).also { reader ->
            reader.setOnImageAvailableListener({ source ->
                val image = source.acquireLatestImage() ?: return@setOnImageAvailableListener
                try {
                    val buffer = image.planes.firstOrNull()?.buffer ?: return@setOnImageAvailableListener
                    val bytes = ByteArray(buffer.remaining())
                    buffer.get(bytes)
                    emitFrame("camera", cameraSequence.incrementAndGet(), target.width, target.height, bytes)
                } finally {
                    image.close()
                }
            }, worker)
        }
        try {
            manager.openCamera(cameraId, object : CameraDevice.StateCallback() {
                override fun onOpened(camera: CameraDevice) {
                    cameraDevice = camera
                    val reader = cameraReader ?: run {
                        camera.close()
                        finishCameraStart(result, "CAMERA_READER_FAILED", "Camera image reader unavailable")
                        return
                    }
                    camera.createCaptureSession(listOf(reader.surface), object : CameraCaptureSession.StateCallback() {
                        override fun onConfigured(session: CameraCaptureSession) {
                            cameraSession = session
                            cameraActive.set(true)
                            scheduleCameraCaptures()
                            mainHandler.post { result.success(null) }
                        }

                        override fun onConfigureFailed(session: CameraCaptureSession) {
                            finishCameraStart(result, "CAMERA_SESSION_FAILED", "Unable to configure camera capture session")
                            stopCamera()
                        }
                    }, worker)
                }

                override fun onDisconnected(camera: CameraDevice) {
                    camera.close()
                    stopCamera()
                }

                override fun onError(camera: CameraDevice, error: Int) {
                    camera.close()
                    if (!cameraActive.get()) {
                        finishCameraStart(result, "CAMERA_OPEN_FAILED", "Camera open failed: $error")
                    }
                    stopCamera()
                }
            }, worker)
        } catch (security: SecurityException) {
            result.error("CAMERA_PERMISSION_DENIED", security.message, null)
            stopCamera()
        } catch (error: Throwable) {
            result.error("CAMERA_OPEN_FAILED", error.message, null)
            stopCamera()
        }
    }

    private fun finishCameraStart(result: MethodChannel.Result, code: String, message: String) {
        mainHandler.post { result.error(code, message, null) }
    }

    private fun scheduleCameraCaptures() {
        cameraCaptureRunnable?.let(worker::removeCallbacks)
        cameraCaptureRunnable = object : Runnable {
            override fun run() {
                if (!cameraActive.get()) return
                requestCameraCapture(immediate = false)
                worker.postDelayed(this, CAMERA_INTERVAL_MS)
            }
        }.also { worker.post(it) }
    }

    private fun requestCameraCapture(immediate: Boolean) {
        val device = cameraDevice ?: return
        val session = cameraSession ?: return
        val reader = cameraReader ?: return
        worker.post {
            try {
                val request = device.createCaptureRequest(CameraDevice.TEMPLATE_STILL_CAPTURE).apply {
                    addTarget(reader.surface)
                    set(CaptureRequest.CONTROL_AF_MODE, CaptureRequest.CONTROL_AF_MODE_CONTINUOUS_PICTURE)
                    set(CaptureRequest.JPEG_QUALITY, if (immediate) 82.toByte() else 72.toByte())
                }.build()
                session.capture(request, null, worker)
            } catch (_: Throwable) {
            }
        }
    }

    private fun stopCamera() {
        cameraActive.set(false)
        cameraCaptureRunnable?.let(worker::removeCallbacks)
        cameraCaptureRunnable = null
        try { cameraSession?.close() } catch (_: Throwable) {}
        try { cameraDevice?.close() } catch (_: Throwable) {}
        try { cameraReader?.close() } catch (_: Throwable) {}
        cameraSession = null
        cameraDevice = null
        cameraReader = null
    }

    private fun startScreen(result: MethodChannel.Result) {
        if (screenActive.get()) {
            result.success(null)
            return
        }
        val currentActivity = activity ?: run {
            result.error("SCREEN_PERMISSION_UNAVAILABLE", "Activity unavailable for screen capture", null)
            return
        }
        if (pendingScreenResult != null) {
            result.error("SCREEN_PERMISSION_PENDING", "Screen capture permission request already pending", null)
            return
        }
        val manager = currentActivity.getSystemService(Context.MEDIA_PROJECTION_SERVICE) as MediaProjectionManager
        pendingScreenResult = result
        currentActivity.startActivityForResult(manager.createScreenCaptureIntent(), SCREEN_CAPTURE_REQUEST)
    }

    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?): Boolean {
        if (requestCode != SCREEN_CAPTURE_REQUEST) return false
        val result = pendingScreenResult
        if (result == null) return true
        if (resultCode != Activity.RESULT_OK || data == null) {
            pendingScreenResult = null
            result.error("SCREEN_PERMISSION_DENIED", "Screen capture permission denied", null)
            return true
        }
        val context = applicationContext ?: run {
            result.error("SCREEN_UNAVAILABLE", "Screen capture context unavailable", null)
            return true
        }
        // For modern Android the mediaProjection foreground service must already be
        // running before getMediaProjection/createVirtualDisplay is used. Bind to
        // the service and only create the projection after onServiceConnected.
        pendingScreenResult = result
        pendingProjectionResultCode = resultCode
        pendingProjectionData = Intent(data)
        val serviceIntent = Intent(context, RealtimeProjectionService::class.java)
        try {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                context.startForegroundService(serviceIntent)
            } else {
                context.startService(serviceIntent)
            }
            if (!context.bindService(serviceIntent, projectionServiceConnection, Context.BIND_AUTO_CREATE)) {
                pendingScreenResult = null
                pendingProjectionResultCode = null
                pendingProjectionData = null
                context.stopService(serviceIntent)
                result.error("SCREEN_SERVICE_FAILED", "Unable to bind screen capture foreground service", null)
            }
        } catch (error: Throwable) {
            pendingScreenResult = null
            pendingProjectionResultCode = null
            pendingProjectionData = null
            context.stopService(serviceIntent)
            result.error("SCREEN_SERVICE_FAILED", error.message, null)
        }
        return true
    }

    private fun startProjection(mediaProjection: MediaProjection) {
        val context = applicationContext ?: return
        projection = mediaProjection
        mediaProjection.registerCallback(object : MediaProjection.Callback() {
            override fun onStop() {
                releaseScreenResources(stopProjection = false)
            }
        }, mainHandler)

        val metrics = DisplayMetrics()
        val currentActivity = activity
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R && currentActivity != null) {
            val bounds = currentActivity.windowManager.currentWindowMetrics.bounds
            metrics.widthPixels = bounds.width()
            metrics.heightPixels = bounds.height()
            metrics.densityDpi = context.resources.displayMetrics.densityDpi
        } else {
            @Suppress("DEPRECATION")
            currentActivity?.windowManager?.defaultDisplay?.getRealMetrics(metrics)
            if (metrics.widthPixels == 0) {
                val fallback = context.resources.displayMetrics
                metrics.widthPixels = fallback.widthPixels
                metrics.heightPixels = fallback.heightPixels
                metrics.densityDpi = fallback.densityDpi
            }
        }
        val width = max(1, metrics.widthPixels)
        val height = max(1, metrics.heightPixels)
        val density = max(1, metrics.densityDpi)
        screenReader = ImageReader.newInstance(width, height, PixelFormat.RGBA_8888, 2).also { reader ->
            reader.setOnImageAvailableListener({ source ->
                val image = source.acquireLatestImage() ?: return@setOnImageAvailableListener
                try {
                    val now = System.currentTimeMillis()
                    if (!forceScreenFrame && now - lastScreenFrameAt < SCREEN_INTERVAL_MS) return@setOnImageAvailableListener
                    forceScreenFrame = false
                    lastScreenFrameAt = now
                    val plane = image.planes.firstOrNull() ?: return@setOnImageAvailableListener
                    val pixelStride = plane.pixelStride
                    val rowStride = plane.rowStride
                    val rowPadding = rowStride - pixelStride * width
                    val paddedWidth = width + rowPadding / max(1, pixelStride)
                    val bitmap = Bitmap.createBitmap(paddedWidth, height, Bitmap.Config.ARGB_8888)
                    bitmap.copyPixelsFromBuffer(plane.buffer)
                    val cropped = Bitmap.createBitmap(bitmap, 0, 0, width, height)
                    bitmap.recycle()
                    val scaled = scaleBitmap(cropped, 1280, 800)
                    if (scaled !== cropped) cropped.recycle()
                    val output = ByteArrayOutputStream()
                    scaled.compress(Bitmap.CompressFormat.JPEG, 78, output)
                    val outWidth = scaled.width
                    val outHeight = scaled.height
                    scaled.recycle()
                    emitFrame("screen", screenSequence.incrementAndGet(), outWidth, outHeight, output.toByteArray())
                } finally {
                    image.close()
                }
            }, worker)
        }
        screenDisplay = mediaProjection.createVirtualDisplay(
            "AmitiaRealtimeScreen",
            width,
            height,
            density,
            android.hardware.display.DisplayManager.VIRTUAL_DISPLAY_FLAG_AUTO_MIRROR,
            screenReader!!.surface,
            null,
            worker,
        )
        screenActive.set(true)
        forceScreenFrame = true
    }

    private fun stopScreen() {
        releaseScreenResources(stopProjection = true)
    }

    private fun releaseScreenResources(stopProjection: Boolean) {
        screenActive.set(false)
        try { screenDisplay?.release() } catch (_: Throwable) {}
        try { screenReader?.close() } catch (_: Throwable) {}
        val activeProjection = projection
        screenDisplay = null
        screenReader = null
        projection = null
        if (stopProjection) {
            try { activeProjection?.stop() } catch (_: Throwable) {}
        }
        stopProjectionService()
    }

    private fun stopProjectionService() {
        val context = applicationContext ?: return
        if (projectionServiceBound) {
            try { context.unbindService(projectionServiceConnection) } catch (_: Throwable) {}
            projectionServiceBound = false
        }
        try { context.stopService(Intent(context, RealtimeProjectionService::class.java)) } catch (_: Throwable) {}
    }

    private fun scaleBitmap(bitmap: Bitmap, maxWidth: Int, maxHeight: Int): Bitmap {
        val scale = min(1.0, min(maxWidth.toDouble() / bitmap.width, maxHeight.toDouble() / bitmap.height))
        if (scale >= 0.999) return bitmap
        return Bitmap.createScaledBitmap(bitmap, max(1, (bitmap.width * scale).toInt()), max(1, (bitmap.height * scale).toInt()), true)
    }

    private fun emitFrame(source: String, sequence: Long, width: Int, height: Int, bytes: ByteArray) {
        if (bytes.isEmpty() || bytes.size > MAX_FRAME_BYTES) return
        val payload = mapOf(
            "source" to source,
            "sequence" to sequence,
            "capturedAtMs" to System.currentTimeMillis(),
            "mime" to "image/jpeg",
            "width" to width,
            "height" to height,
            "data" to bytes,
        )
        mainHandler.post { eventSink?.success(payload) }
    }

    private fun hasCamera(): Boolean {
        val context = applicationContext ?: return false
        return context.packageManager.hasSystemFeature(PackageManager.FEATURE_CAMERA_ANY)
    }

    private fun reset() {
        pendingCameraResult = null
        pendingScreenResult = null
        pendingProjectionResultCode = null
        pendingProjectionData = null
        stopCamera()
        stopScreen()
    }

    companion object {
        private const val CONTROL_CHANNEL = "com.amitia.realtime_visual/control"
        private const val FRAME_CHANNEL = "com.amitia.realtime_visual/frames"
        private const val CAMERA_PERMISSION_REQUEST = 42031
        private const val SCREEN_CAPTURE_REQUEST = 42032
        private const val CAMERA_INTERVAL_MS = 650L
        private const val SCREEN_INTERVAL_MS = 700L
        private const val MAX_FRAME_BYTES = 2 * 1024 * 1024
    }
}
