package com.amitia.amitia_app.realtime

import android.Manifest
import android.app.Activity
import android.content.Context
import android.content.pm.PackageManager
import android.media.AudioAttributes
import android.media.AudioFormat
import android.media.AudioManager
import android.media.AudioRecord
import android.media.AudioTrack
import android.media.MediaRecorder
import android.os.Build
import android.os.Handler
import android.os.Looper
import com.amitia.amitia_app.runtime.workflow.WorkflowDeviceEventIngress
import com.amitia.amitia_app.runtime.workflow.WorkflowMicrophoneArbiter
import com.amitia.amitia_app.workflow.WorkflowTriggerCapabilityReporter
import io.flutter.embedding.engine.plugins.FlutterPlugin
import io.flutter.embedding.engine.plugins.activity.ActivityAware
import io.flutter.embedding.engine.plugins.activity.ActivityPluginBinding
import io.flutter.plugin.common.EventChannel
import io.flutter.plugin.common.MethodCall
import io.flutter.plugin.common.MethodChannel
import io.flutter.plugin.common.PluginRegistry
import java.util.concurrent.Executors
import org.json.JSONObject
import java.util.concurrent.atomic.AtomicBoolean
import kotlin.math.max

/**
 * Native PCM bridge for the realtime voice WebSocket.
 *
 * Input: mono PCM16 / 16 kHz.
 * Output: mono PCM16 / 24 kHz.
 */
class RealtimeAudioPlugin : FlutterPlugin,
    ActivityAware,
    EventChannel.StreamHandler,
    PluginRegistry.RequestPermissionsResultListener {

    private var applicationContext: Context? = null
    private var activityBinding: ActivityPluginBinding? = null
    private var activity: Activity? = null
    private var methodChannel: MethodChannel? = null
    private var eventChannel: EventChannel? = null
    private var eventSink: EventChannel.EventSink? = null
    private val mainHandler = Handler(Looper.getMainLooper())

    private val capturing = AtomicBoolean(false)
    private val realtimeCaptureLeaseHeld = AtomicBoolean(false)
    private var audioRecord: AudioRecord? = null
    private var captureThread: Thread? = null

    private val playbackExecutor = Executors.newSingleThreadExecutor { runnable ->
        Thread(runnable, "amitia-realtime-playback").apply { isDaemon = true }
    }
    private var audioTrack: AudioTrack? = null

    private var pendingStartResult: MethodChannel.Result? = null

    override fun onAttachedToEngine(binding: FlutterPlugin.FlutterPluginBinding) {
        applicationContext = binding.applicationContext
        WorkflowTriggerCapabilityReporter.report(binding.applicationContext)
        methodChannel = MethodChannel(binding.binaryMessenger, CONTROL_CHANNEL).also { channel ->
            channel.setMethodCallHandler(::handleMethodCall)
        }
        eventChannel = EventChannel(binding.binaryMessenger, INPUT_CHANNEL).also { channel ->
            channel.setStreamHandler(this)
        }
    }

    override fun onDetachedFromEngine(binding: FlutterPlugin.FlutterPluginBinding) {
        resetAudio()
        methodChannel?.setMethodCallHandler(null)
        eventChannel?.setStreamHandler(null)
        methodChannel = null
        eventChannel = null
        eventSink = null
        applicationContext = null
    }

    override fun onAttachedToActivity(binding: ActivityPluginBinding) {
        activityBinding = binding
        activity = binding.activity
        binding.addRequestPermissionsResultListener(this)
    }

    override fun onDetachedFromActivityForConfigChanges() {
        detachActivity()
    }

    override fun onReattachedToActivityForConfigChanges(binding: ActivityPluginBinding) {
        onAttachedToActivity(binding)
    }

    override fun onDetachedFromActivity() {
        detachActivity()
    }

    private fun detachActivity() {
        activityBinding?.removeRequestPermissionsResultListener(this)
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
            "startCapture" -> startCaptureWithPermission(result)
            "stopCapture" -> {
                stopCapture()
                result.success(null)
            }
            "playPcm" -> {
                val bytes = call.arguments as? ByteArray
                if (bytes == null) {
                    result.error("INVALID_AUDIO", "playPcm requires Uint8List PCM data", null)
                    return
                }
                enqueuePlayback(bytes)
                result.success(null)
            }
            "reset" -> {
                resetAudio()
                result.success(null)
            }
            "emitWorkflowASRFinal" -> emitWorkflowASRFinal(call, result)
            "status" -> result.success(
                mapOf(
                    "capturing" to capturing.get(),
                    "playing" to (audioTrack?.playState == AudioTrack.PLAYSTATE_PLAYING),
                ),
            )
            else -> result.notImplemented()
        }
    }

    private fun emitWorkflowASRFinal(call: MethodCall, result: MethodChannel.Result) {
        val context = applicationContext
        if (context == null) {
            result.error("WORKFLOW_INGRESS_UNAVAILABLE", "Android workflow ingress unavailable", null)
            return
        }
        val arguments = call.arguments as? Map<*, *>
        val transcript = arguments?.get("transcript")?.toString()?.trim().orEmpty()
        val eventID = arguments?.get("eventId")?.toString()?.trim().orEmpty()
        if (transcript.isEmpty() || transcript.length > MAX_ASR_TRANSCRIPT_CHARS) {
            result.error("INVALID_ASR_TRANSCRIPT", "Final ASR transcript is empty or too large", null)
            return
        }
        if (!EVENT_ID_PATTERN.matches(eventID)) {
            result.error("INVALID_EVENT_ID", "Invalid workflow ASR event id", null)
            return
        }

        val payload = JSONObject()
            .put("transcript", transcript)
            .put("final", true)
        arguments?.get("sessionId")?.toString()?.trim()?.takeIf { it.isNotEmpty() }?.let {
            payload.put("sessionId", it.take(MAX_CONTEXT_ID_CHARS))
        }
        arguments?.get("conversationId")?.toString()?.trim()?.takeIf { it.isNotEmpty() }?.let {
            payload.put("conversationId", it.take(MAX_CONTEXT_ID_CHARS))
        }

        WorkflowDeviceEventIngress(context).emit(
            eventType = "voice.asr.final",
            payload = payload,
            source = "voice.realtime.asr",
            eventID = eventID,
        ) { completion ->
            mainHandler.post {
                completion.fold(
                    onSuccess = { result.success(null) },
                    onFailure = { error ->
                        result.error("WORKFLOW_INGRESS_FAILED", error.message ?: "Workflow ingress failed", null)
                    },
                )
            }
        }
    }

    private fun startCaptureWithPermission(result: MethodChannel.Result) {
        val context = applicationContext
        if (context == null) {
            result.error("AUDIO_UNAVAILABLE", "Android audio context unavailable", null)
            return
        }
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.M ||
            context.checkSelfPermission(Manifest.permission.RECORD_AUDIO) == PackageManager.PERMISSION_GRANTED
        ) {
            startCapture(result)
            return
        }
        val currentActivity = activity
        if (currentActivity == null) {
            result.error("PERMISSION_UNAVAILABLE", "Activity unavailable for microphone permission", null)
            return
        }
        if (pendingStartResult != null) {
            result.error("PERMISSION_PENDING", "Microphone permission request already pending", null)
            return
        }
        pendingStartResult = result
        currentActivity.requestPermissions(arrayOf(Manifest.permission.RECORD_AUDIO), PERMISSION_REQUEST)
    }

    override fun onRequestPermissionsResult(
        requestCode: Int,
        permissions: Array<out String>,
        grantResults: IntArray,
    ): Boolean {
        if (requestCode != PERMISSION_REQUEST) return false
        val result = pendingStartResult
        pendingStartResult = null
        if (result == null) return true
        if (grantResults.isNotEmpty() && grantResults[0] == PackageManager.PERMISSION_GRANTED) {
            startCapture(result)
        } else {
            result.error("MICROPHONE_PERMISSION_DENIED", "Microphone permission denied", null)
        }
        applicationContext?.let(WorkflowTriggerCapabilityReporter::report)
        return true
    }

    private fun startCapture(result: MethodChannel.Result) {
        if (capturing.get()) {
            result.success(null)
            return
        }
        val context = applicationContext
        if (context == null) {
            result.error("AUDIO_UNAVAILABLE", "Android audio context unavailable", null)
            return
        }

        val minimum = AudioRecord.getMinBufferSize(
            CAPTURE_SAMPLE_RATE,
            AudioFormat.CHANNEL_IN_MONO,
            AudioFormat.ENCODING_PCM_16BIT,
        )
        if (minimum <= 0) {
            result.error("AUDIO_INIT_FAILED", "Unable to determine microphone buffer size", minimum)
            return
        }
        val bufferSize = max(minimum, CAPTURE_CHUNK_BYTES * 2)
        if (realtimeCaptureLeaseHeld.compareAndSet(false, true)) {
            WorkflowMicrophoneArbiter.acquireRealtimeCapture()
        }
        if (!WorkflowMicrophoneArbiter.awaitWakeCaptureReleased(REALTIME_WAKE_HANDOFF_TIMEOUT_MS)) {
            releaseRealtimeCaptureLease()
            result.error("AUDIO_BUSY", "Wake microphone capture did not release in time", null)
            return
        }
        val recorder = try {
            AudioRecord(
                MediaRecorder.AudioSource.VOICE_COMMUNICATION,
                CAPTURE_SAMPLE_RATE,
                AudioFormat.CHANNEL_IN_MONO,
                AudioFormat.ENCODING_PCM_16BIT,
                bufferSize,
            )
        } catch (error: Throwable) {
            releaseRealtimeCaptureLease()
            result.error("AUDIO_INIT_FAILED", error.message, null)
            return
        }
        if (recorder.state != AudioRecord.STATE_INITIALIZED) {
            recorder.release()
            releaseRealtimeCaptureLease()
            result.error("AUDIO_INIT_FAILED", "AudioRecord failed to initialize", null)
            return
        }

        val audioManager = context.getSystemService(Context.AUDIO_SERVICE) as? AudioManager
        audioManager?.mode = AudioManager.MODE_IN_COMMUNICATION

        audioRecord = recorder
        capturing.set(true)
        try {
            recorder.startRecording()
        } catch (error: Throwable) {
            capturing.set(false)
            recorder.release()
            audioRecord = null
            releaseRealtimeCaptureLease()
            result.error("AUDIO_START_FAILED", error.message, null)
            return
        }
        captureThread = Thread({ captureLoop(recorder) }, "amitia-realtime-capture").apply {
            isDaemon = true
            start()
        }
        result.success(null)
    }

    private fun captureLoop(recorder: AudioRecord) {
        val buffer = ByteArray(CAPTURE_CHUNK_BYTES)
        try {
            while (capturing.get() && audioRecord === recorder) {
                val count = try {
                    recorder.read(buffer, 0, buffer.size)
                } catch (_: Throwable) {
                    break
                }
                if (count > 0) {
                    val chunk = buffer.copyOf(count)
                    mainHandler.post {
                        try {
                            eventSink?.success(chunk)
                        } finally {
                            chunk.fill(0)
                        }
                    }
                } else if (count == AudioRecord.ERROR_INVALID_OPERATION || count == AudioRecord.ERROR_BAD_VALUE) {
                    break
                }
            }
        } finally {
            buffer.fill(0)
            if (capturing.compareAndSet(true, false)) {
                if (audioRecord === recorder) audioRecord = null
                runCatching { recorder.stop() }
                runCatching { recorder.release() }
                releaseRealtimeCaptureLease()
            }
        }
    }

    private fun stopCapture() {
        if (!capturing.getAndSet(false)) {
            releaseRealtimeCaptureLease()
            return
        }
        val recorder = audioRecord
        audioRecord = null
        try {
            recorder?.stop()
        } catch (_: Throwable) {
        }
        try {
            recorder?.release()
        } catch (_: Throwable) {
        }
        try {
            captureThread?.join(250)
        } catch (_: InterruptedException) {
            Thread.currentThread().interrupt()
        }
        captureThread = null
        releaseRealtimeCaptureLease()
    }

    private fun releaseRealtimeCaptureLease() {
        if (realtimeCaptureLeaseHeld.compareAndSet(true, false)) {
            WorkflowMicrophoneArbiter.releaseRealtimeCapture()
        }
    }

    private fun ensureAudioTrack(): AudioTrack? {
        val existing = audioTrack
        if (existing != null && existing.state == AudioTrack.STATE_INITIALIZED) return existing

        val minimum = AudioTrack.getMinBufferSize(
            PLAYBACK_SAMPLE_RATE,
            AudioFormat.CHANNEL_OUT_MONO,
            AudioFormat.ENCODING_PCM_16BIT,
        )
        if (minimum <= 0) return null
        val format = AudioFormat.Builder()
            .setEncoding(AudioFormat.ENCODING_PCM_16BIT)
            .setSampleRate(PLAYBACK_SAMPLE_RATE)
            .setChannelMask(AudioFormat.CHANNEL_OUT_MONO)
            .build()
        val attributes = AudioAttributes.Builder()
            .setUsage(AudioAttributes.USAGE_VOICE_COMMUNICATION)
            .setContentType(AudioAttributes.CONTENT_TYPE_SPEECH)
            .build()
        val track = try {
            AudioTrack.Builder()
                .setAudioAttributes(attributes)
                .setAudioFormat(format)
                .setTransferMode(AudioTrack.MODE_STREAM)
                .setBufferSizeInBytes(max(minimum, PLAYBACK_BUFFER_BYTES))
                .build()
        } catch (_: Throwable) {
            return null
        }
        if (track.state != AudioTrack.STATE_INITIALIZED) {
            track.release()
            return null
        }
        track.play()
        audioTrack = track
        return track
    }

    private fun enqueuePlayback(bytes: ByteArray) {
        if (bytes.isEmpty()) return
        val copy = bytes.copyOf()
        playbackExecutor.execute {
            val track = ensureAudioTrack() ?: return@execute
            var offset = 0
            while (offset < copy.size) {
                val written = try {
                    track.write(copy, offset, copy.size - offset, AudioTrack.WRITE_BLOCKING)
                } catch (_: Throwable) {
                    break
                }
                if (written <= 0) break
                offset += written
            }
        }
    }

    private fun stopPlayback() {
        val track = audioTrack
        audioTrack = null
        try {
            track?.pause()
            track?.flush()
            track?.stop()
        } catch (_: Throwable) {
        }
        try {
            track?.release()
        } catch (_: Throwable) {
        }
    }

    private fun resetAudio() {
        stopCapture()
        stopPlayback()
        val context = applicationContext
        val audioManager = context?.getSystemService(Context.AUDIO_SERVICE) as? AudioManager
        if (audioManager?.mode == AudioManager.MODE_IN_COMMUNICATION) {
            audioManager.mode = AudioManager.MODE_NORMAL
        }
    }

    companion object {
        private const val CONTROL_CHANNEL = "com.amitia.realtime_audio/control"
        private const val INPUT_CHANNEL = "com.amitia.realtime_audio/input"
        private const val PERMISSION_REQUEST = 43120
        private const val CAPTURE_SAMPLE_RATE = 16_000
        private const val PLAYBACK_SAMPLE_RATE = 24_000
        private const val CAPTURE_CHUNK_BYTES = 3_200
        private const val PLAYBACK_BUFFER_BYTES = 9_600
        private const val MAX_ASR_TRANSCRIPT_CHARS = 16_384
        private const val MAX_CONTEXT_ID_CHARS = 200
        private const val REALTIME_WAKE_HANDOFF_TIMEOUT_MS = 600L
        private val EVENT_ID_PATTERN = Regex("^[A-Za-z0-9._:-]{1,200}$")
    }
}
