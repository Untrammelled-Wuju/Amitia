package com.amitia.platform.audio

import android.content.Context
import android.media.MediaRecorder
import android.os.Build
import android.os.SystemClock
import com.amitia.core.logging.Logger
import com.amitia.platform.permissions.PermissionBroker
import dagger.hilt.android.qualifiers.ApplicationContext
import java.io.File
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

@Singleton
class AudioRecorderImpl @Inject constructor(
    @ApplicationContext private val context: Context,
    private val permissionBroker: PermissionBroker,
    private val logger: Logger
) : AudioRecorder {

    private val appContext = context.applicationContext
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)
    private val stateFlow = MutableStateFlow<AudioRecorder.RecordingState>(AudioRecorder.RecordingState.Idle)

    @Volatile
    private var recorder: MediaRecorder? = null
    @Volatile
    private var outputFile: File? = null
    @Volatile
    private var startElapsedMs: Long = 0L
    @Volatile
    private var accumulatedMs: Long = 0L
    @Volatile
    private var configuredMaxDuration: Long? = null

    override val state: AudioRecorder.RecordingState get() = stateFlow.value

    override fun observeState(): StateFlow<AudioRecorder.RecordingState> = stateFlow.asStateFlow()

    override suspend fun prepare(config: AudioRecorder.RecordingConfig): Boolean {
        if (stateFlow.value is AudioRecorder.RecordingState.Recording) {
            return false
        }
        val granted = permissionBroker.isGranted(PermissionBroker.Permissions.RECORD_AUDIO)
        if (!granted) {
            val result = permissionBroker.request(PermissionBroker.Permissions.RECORD_AUDIO)
            if (result !is PermissionBroker.PermissionResult.Granted) {
                stateFlow.value = AudioRecorder.RecordingState.Failed("mic_permission_denied")
                return false
            }
        }
        return runCatching {
            releaseRecorder()
            val file = File(config.outputFile)
            file.parentFile?.mkdirs()
            outputFile = file
            configuredMaxDuration = config.maxDurationMillis
            val rec = createRecorder()
            rec.setAudioSource(MediaRecorder.AudioSource.MIC)
            rec.setOutputFormat(mapOutputFormat(config.encoding))
            rec.setAudioEncoder(mapAudioEncoder(config.encoding))
            rec.setAudioSamplingRate(config.sampleRate)
            rec.setAudioChannels(config.channels)
            rec.setAudioEncodingBitRate(config.bitrate)
            rec.setOutputFile(file.absolutePath)
            config.maxDurationMillis?.let { rec.setMaxDuration(it.toInt()) }
            rec.setOnErrorListener { _, what, extra ->
                logger.e(TAG, "MediaRecorder error what=$what extra=$extra")
                stateFlow.value = AudioRecorder.RecordingState.Failed("recorder_error_$what")
                releaseRecorder()
            }
            rec.setOnInfoListener { _, what, _ ->
                if (what == MediaRecorder.MEDIA_RECORDER_INFO_MAX_DURATION_REACHED) {
                    scope.launch { stop() }
                }
            }
            rec.prepare()
            recorder = rec
            accumulatedMs = 0L
            stateFlow.value = AudioRecorder.RecordingState.Prepared
            true
        }.getOrElse { t ->
            logger.e(TAG, "prepare failed", t)
            stateFlow.value = AudioRecorder.RecordingState.Failed(t.message ?: "prepare_failed")
            releaseRecorder()
            false
        }
    }

    override suspend fun start() {
        val rec = recorder ?: run {
            stateFlow.value = AudioRecorder.RecordingState.Failed("not_prepared")
            return
        }
        runCatching {
            rec.start()
            startElapsedMs = SystemClock.elapsedRealtime()
            stateFlow.value = AudioRecorder.RecordingState.Recording
        }.onFailure { t ->
            logger.e(TAG, "start failed", t)
            stateFlow.value = AudioRecorder.RecordingState.Failed(t.message ?: "start_failed")
            releaseRecorder()
        }
    }

    override suspend fun pause() {
        val rec = recorder ?: return
        runCatching {
            rec.pause()
            accumulatedMs += SystemClock.elapsedRealtime() - startElapsedMs
            stateFlow.value = AudioRecorder.RecordingState.Paused
        }.onFailure { t ->
            logger.w(TAG, "pause failed: ${t.message}")
        }
    }

    override suspend fun resume() {
        val rec = recorder ?: return
        runCatching {
            rec.resume()
            startElapsedMs = SystemClock.elapsedRealtime()
            stateFlow.value = AudioRecorder.RecordingState.Recording
        }.onFailure { t ->
            logger.w(TAG, "resume failed: ${t.message}")
        }
    }

    override suspend fun stop(): AudioRecorder.RecordingResult {
        val rec = recorder
        val file = outputFile
        if (rec == null || file == null) {
            stateFlow.value = AudioRecorder.RecordingState.Idle
            return AudioRecorder.RecordingResult(
                success = false,
                filePath = null,
                error = "not_recording"
            )
        }
        val duration = accumulatedMs + (SystemClock.elapsedRealtime() - startElapsedMs).coerceAtLeast(0L)
        return runCatching {
            rec.stop()
            rec.release()
            recorder = null
            val size = file.length()
            stateFlow.value = AudioRecorder.RecordingState.Stopped(duration)
            AudioRecorder.RecordingResult(
                success = true,
                filePath = file.absolutePath,
                durationMillis = duration,
                sizeBytes = size
            )
        }.getOrElse { t ->
            logger.e(TAG, "stop failed", t)
            releaseRecorder()
            runCatching { file.delete() }
            stateFlow.value = AudioRecorder.RecordingState.Failed(t.message ?: "stop_failed")
            AudioRecorder.RecordingResult(
                success = false,
                filePath = null,
                durationMillis = duration,
                error = t.message ?: "stop_failed"
            )
        }
    }

    override suspend fun cancel() {
        releaseRecorder()
        outputFile?.let { runCatching { it.delete() } }
        outputFile = null
        accumulatedMs = 0L
        stateFlow.value = AudioRecorder.RecordingState.Idle
    }

    override suspend fun getMaxAmplitude(): Int {
        return runCatching { recorder?.maxAmplitude ?: 0 }.getOrDefault(0)
    }

    private fun createRecorder(): MediaRecorder {
        return if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            MediaRecorder(appContext)
        } else {
            @Suppress("DEPRECATION")
            MediaRecorder()
        }
    }

    private fun mapOutputFormat(encoding: AudioRecorder.AudioEncoding): Int {
        return when (encoding) {
            AudioRecorder.AudioEncoding.AAC, AudioRecorder.AudioEncoding.MPEG_4 -> MediaRecorder.OutputFormat.MPEG_4
            AudioRecorder.AudioEncoding.AMR_NB -> MediaRecorder.OutputFormat.AMR_NB
            AudioRecorder.AudioEncoding.AMR_WB -> MediaRecorder.OutputFormat.AMR_WB
            AudioRecorder.AudioEncoding.THREE_GPP -> MediaRecorder.OutputFormat.THREE_GPP
            AudioRecorder.AudioEncoding.OPUS -> if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                MediaRecorder.OutputFormat.OGG
            } else {
                MediaRecorder.OutputFormat.MPEG_4
            }
        }
    }

    private fun mapAudioEncoder(encoding: AudioRecorder.AudioEncoding): Int {
        return when (encoding) {
            AudioRecorder.AudioEncoding.AAC -> MediaRecorder.AudioEncoder.AAC
            AudioRecorder.AudioEncoding.AMR_NB -> MediaRecorder.AudioEncoder.AMR_NB
            AudioRecorder.AudioEncoding.AMR_WB -> MediaRecorder.AudioEncoder.AMR_WB
            AudioRecorder.AudioEncoding.MPEG_4 -> MediaRecorder.AudioEncoder.AAC
            AudioRecorder.AudioEncoding.THREE_GPP -> MediaRecorder.AudioEncoder.AMR_NB
            AudioRecorder.AudioEncoding.OPUS -> if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                MediaRecorder.AudioEncoder.OPUS
            } else {
                MediaRecorder.AudioEncoder.AAC
            }
        }
    }

    private fun releaseRecorder() {
        runCatching { recorder?.release() }
        recorder = null
    }

    companion object {
        private const val TAG = "AudioRecorderImpl"
    }
}
