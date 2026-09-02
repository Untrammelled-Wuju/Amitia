package com.amitia.amitia_app.runtime.workflow

import android.Manifest
import android.content.Context
import android.content.pm.PackageManager
import android.media.AudioFormat
import android.media.AudioRecord
import android.media.MediaRecorder
import androidx.core.content.ContextCompat
import org.json.JSONArray
import org.json.JSONObject
import java.util.concurrent.ArrayBlockingQueue
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicLong
import kotlin.math.max

/**
 * Device-local microphone producer for Workflow wake triggers.
 *
 * This lives in the Android runtime process rather than Flutter. It polls the
 * loopback Device Agent for whether an enabled wake trigger currently needs
 * PCM, captures mono PCM16/16 kHz only while needed, and forwards bounded
 * chunks through the root local token. No cloud endpoint or Flutter engine is
 * involved in the wake path.
 */
internal class WorkflowWakeAudioMonitor(
    context: Context,
    private val beforeCapture: () -> Boolean,
    private val afterCapture: () -> Unit,
) {
    private data class AudioChunk(
        val pcm: ByteArray,
        val sequence: Long,
        val capturedAtMs: Long,
    )

    private val appContext = context.applicationContext
    private val client = WorkflowDeviceEventClient(appContext)
    private val running = AtomicBoolean(false)
    private val capturing = AtomicBoolean(false)
    private val wakeCaptureLeaseHeld = AtomicBoolean(false)
    private val sequence = AtomicLong(0L)
    private val lock = Any()
    private val queue = ArrayBlockingQueue<AudioChunk>(MAX_QUEUED_CHUNKS)

    private val statusExecutor = Executors.newSingleThreadScheduledExecutor { runnable ->
        Thread(runnable, "amitia-workflow-wake-status").apply { isDaemon = true }
    }
    private val captureExecutor = Executors.newSingleThreadExecutor { runnable ->
        Thread(runnable, "amitia-workflow-wake-capture").apply { isDaemon = true }
    }
    private val senderExecutor = Executors.newSingleThreadExecutor { runnable ->
        Thread(runnable, "amitia-workflow-wake-sender").apply { isDaemon = true }
    }

    @Volatile
    private var audioRecord: AudioRecord? = null

    @Volatile
    private var lastCapabilityState: String = ""

    @Volatile
    private var lastWakeMonitorState: String = ""

    fun start() {
        if (!running.compareAndSet(false, true)) return
        reportWakeMonitorState("idle")
        senderExecutor.execute(::senderLoop)
        statusExecutor.scheduleWithFixedDelay(
            ::refreshStatus,
            0L,
            STATUS_POLL_SECONDS,
            TimeUnit.SECONDS,
        )
    }

    fun stop() {
        if (!running.getAndSet(false)) return
        stopCapture()
        reportWakeMonitorState("idle")
        clearQueuedAudio()
        statusExecutor.shutdownNow()
        captureExecutor.shutdownNow()
        senderExecutor.shutdownNow()
    }

    private fun refreshStatus() {
        if (!running.get()) return
        if (WorkflowMicrophoneArbiter.isRealtimeCaptureActive()) {
            stopCapture()
            reportWakeMonitorState("wake_suspended", "Realtime voice capture has microphone priority")
            return
        }
        val hasPermission = ContextCompat.checkSelfPermission(
            appContext,
            Manifest.permission.RECORD_AUDIO,
        ) == PackageManager.PERMISSION_GRANTED
        if (!hasPermission) {
            stopCapture()
            reportWakeMonitorState("wake_permission_missing", "Microphone permission is not granted")
            reportWakeCapability(
                available = false,
                reason = "Microphone permission is not granted",
            )
            return
        }
        val status = client.getWakeRuntimeStatus().getOrNull()
        if (status == null) {
            stopCapture()
            reportWakeMonitorState("wake_required", "Device Agent wake runtime is unavailable")
            reportWakeCapability(
                available = false,
                reason = "Device Agent wake runtime is unavailable",
            )
            return
        }
        if (!status.required) {
            stopCapture()
            reportWakeMonitorState("idle")
            // With microphone permission granted and the local Device Agent
            // reachable, the Android side is available. The backend capability
            // snapshot separately checks whether a real wake-word recognizer
            // configuration exists before exposing the trigger as usable.
            reportWakeCapability(
                available = true,
                reason = "",
            )
            return
        }
        if (!status.ready) {
            stopCapture()
            reportWakeMonitorState("wake_required", status.reason.ifBlank { "Wake-word runtime is not ready" })
            reportWakeCapability(
                available = false,
                reason = status.reason.ifBlank { "Wake-word runtime is not ready" },
            )
            return
        }
        reportWakeMonitorState("wake_required")
        startCapture()
    }

    private fun startCapture() {
        synchronized(lock) {
            if (!running.get() || capturing.get()) return
            if (!beforeCapture()) {
                reportWakeMonitorState("wake_blocked_by_android", "Android blocked microphone foreground activation")
                reportWakeCapability(
                    available = false,
                    reason = "Android blocked microphone foreground activation; open Amitia to activate wake listening",
                )
                return
            }
            if (!WorkflowMicrophoneArbiter.tryAcquireWakeCapture()) {
                afterCapture()
                reportWakeMonitorState("wake_suspended", "Microphone is in use by a higher-priority capture")
                return
            }
            wakeCaptureLeaseHeld.set(true)

            val minimum = AudioRecord.getMinBufferSize(
                SAMPLE_RATE,
                AudioFormat.CHANNEL_IN_MONO,
                AudioFormat.ENCODING_PCM_16BIT,
            )
            if (minimum <= 0) {
                releaseWakeCaptureLease()
                afterCapture()
                reportWakeMonitorState("wake_blocked_by_android", "Microphone recorder is unavailable")
                reportWakeCapability(false, "Microphone recorder is unavailable")
                return
            }
            val recorder = try {
                AudioRecord(
                    MediaRecorder.AudioSource.VOICE_RECOGNITION,
                    SAMPLE_RATE,
                    AudioFormat.CHANNEL_IN_MONO,
                    AudioFormat.ENCODING_PCM_16BIT,
                    max(minimum, CHUNK_BYTES * 4),
                )
            } catch (_: Throwable) {
                releaseWakeCaptureLease()
                afterCapture()
                reportWakeMonitorState("wake_blocked_by_android", "Microphone recorder initialization failed")
                reportWakeCapability(false, "Microphone recorder initialization failed")
                return
            }
            if (recorder.state != AudioRecord.STATE_INITIALIZED) {
                recorder.release()
                releaseWakeCaptureLease()
                afterCapture()
                reportWakeMonitorState("wake_blocked_by_android", "Microphone recorder failed to initialize")
                reportWakeCapability(false, "Microphone recorder failed to initialize")
                return
            }
            try {
                recorder.startRecording()
            } catch (_: Throwable) {
                recorder.release()
                releaseWakeCaptureLease()
                afterCapture()
                reportWakeMonitorState("wake_blocked_by_android", "Microphone recording could not start")
                reportWakeCapability(false, "Microphone recording could not start")
                return
            }
            audioRecord = recorder
            capturing.set(true)
            reportWakeMonitorState("wake_active")
            reportWakeCapability(true, "")
            captureExecutor.execute { captureLoop(recorder) }
        }
    }

    private fun captureLoop(recorder: AudioRecord) {
        val buffer = ByteArray(CHUNK_BYTES)
        try {
            while (running.get() && capturing.get() && audioRecord === recorder && !WorkflowMicrophoneArbiter.isRealtimeCaptureActive()) {
                val count = try {
                    recorder.read(buffer, 0, buffer.size)
                } catch (_: Throwable) {
                    break
                }
                if (count > 0) {
                    val evenCount = count - (count % 2)
                    if (evenCount <= 0) continue
                    val chunk = AudioChunk(
                        pcm = buffer.copyOf(evenCount),
                        sequence = sequence.incrementAndGet(),
                        capturedAtMs = System.currentTimeMillis(),
                    )
                    if (!queue.offer(chunk)) {
                        queue.poll()?.pcm?.fill(0)
                        if (!queue.offer(chunk)) chunk.pcm.fill(0)
                    }
                    continue
                }
                if (count == AudioRecord.ERROR_INVALID_OPERATION || count == AudioRecord.ERROR_BAD_VALUE) {
                    break
                }
            }
        } finally {
            buffer.fill(0)
            stopCapture(recorder)
            if (running.get() && !WorkflowMicrophoneArbiter.isRealtimeCaptureActive()) {
                reportWakeMonitorState("wake_required", "Wake audio capture stopped; retrying")
            }
        }
    }

    private fun senderLoop() {
        while (running.get() || queue.isNotEmpty()) {
            val chunk = try {
                queue.poll(500L, TimeUnit.MILLISECONDS)
            } catch (_: InterruptedException) {
                Thread.currentThread().interrupt()
                return
            } ?: continue
            if (!running.get() || !capturing.get()) {
                chunk.pcm.fill(0)
                continue
            }
            try {
                client.postWakeAudio(
                    pcm = chunk.pcm,
                    sequence = chunk.sequence,
                    capturedAtMs = chunk.capturedAtMs,
                )
            } finally {
                chunk.pcm.fill(0)
            }
        }
    }

    private fun reportWakeMonitorState(state: String, reason: String = "") {
        val normalizedState = state.trim().lowercase()
        val normalizedReason = reason.trim().take(512)
        val fingerprint = "$normalizedState:$normalizedReason"
        if (fingerprint == lastWakeMonitorState) return
        lastWakeMonitorState = fingerprint
        client.postWakeDeviceStatus(normalizedState, normalizedReason)
    }

    private fun reportWakeCapability(available: Boolean, reason: String) {
        val state = "$available:${reason.trim()}"
        if (state == lastCapabilityState) return
        lastCapabilityState = state
        val item = JSONObject()
            .put("id", "workflow.trigger.voice_wake.v1")
            .put("supported", true)
            .put("available", available)
            .put("permissionRequired", true)
            .put("permission", Manifest.permission.RECORD_AUDIO)
            .put("reason", reason.take(512))
        client.postCapabilityStatus(
            JSONObject().put("items", JSONArray().put(item)),
        )
    }

    private fun stopCapture(expected: AudioRecord? = null) {
        synchronized(lock) {
            val recorder = audioRecord ?: return
            if (expected != null && recorder !== expected) return
            audioRecord = null
            val wasCapturing = capturing.getAndSet(false)
            clearQueuedAudio()
            try {
                recorder.stop()
            } catch (_: Throwable) {
            }
            try {
                recorder.release()
            } catch (_: Throwable) {
            }
            releaseWakeCaptureLease()
            if (wasCapturing) {
                afterCapture()
            }
        }
    }

    private fun releaseWakeCaptureLease() {
        if (wakeCaptureLeaseHeld.compareAndSet(true, false)) {
            WorkflowMicrophoneArbiter.releaseWakeCapture()
        }
    }

    private fun clearQueuedAudio() {
        while (true) {
            val chunk = queue.poll() ?: break
            chunk.pcm.fill(0)
        }
    }

    companion object {
        private const val SAMPLE_RATE = 16_000
        private const val CHUNK_BYTES = 3_200
        private const val MAX_QUEUED_CHUNKS = 8
        private const val STATUS_POLL_SECONDS = 5L
    }
}
