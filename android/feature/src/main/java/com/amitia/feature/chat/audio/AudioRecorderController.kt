package com.amitia.feature.chat.audio

import android.content.Context
import android.media.MediaRecorder
import android.os.Build
import android.os.SystemClock
import dagger.hilt.android.qualifiers.ApplicationContext
import java.io.File
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch

@Singleton
class AudioRecorderController @Inject constructor(
    @ApplicationContext private val context: Context
) {

    private val cacheDir: File by lazy {
        File(context.cacheDir, "voice").apply { mkdirs() }
    }

    private val _state = MutableStateFlow(RecorderState())
    val state: StateFlow<RecorderState> = _state.asStateFlow()

    private var recorder: MediaRecorder? = null
    private var currentFile: File? = null
    private var startElapsedMs: Long = 0L
    private var tickerJob: Job? = null

    fun start(scope: kotlinx.coroutines.CoroutineScope): File? {
        return runCatching {
            stopInternal(cleanup = false)
            val file = File(cacheDir, "voice_${System.currentTimeMillis()}.aac")
            currentFile = file
            val rec = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
                MediaRecorder(context)
            } else {
                @Suppress("DEPRECATION")
                MediaRecorder()
            }
            rec.setAudioSource(MediaRecorder.AudioSource.MIC)
            rec.setOutputFormat(MediaRecorder.OutputFormat.AAC_ADTS)
            rec.setAudioEncoder(MediaRecorder.AudioEncoder.AAC)
            rec.setAudioSamplingRate(SAMPLE_RATE)
            rec.setAudioEncodingBitRate(BIT_RATE)
            rec.setOutputFile(file.absolutePath)
            rec.prepare()
            rec.start()
            recorder = rec
            startElapsedMs = SystemClock.elapsedRealtime()
            _state.value = RecorderState(recording = true, durationSec = 0, error = null)
            tickerJob = scope.launch {
                while (isActive) {
                    val elapsed = ((SystemClock.elapsedRealtime() - startElapsedMs) / 1000).toInt()
                    _state.value = _state.value.copy(durationSec = elapsed)
                    delay(500)
                }
            }
            file
        }.getOrElse { e ->
            _state.value = _state.value.copy(
                recording = false,
                error = e.message ?: "录音启动失败"
            )
            null
        }
    }

    fun stop(): File? {
        val file = currentFile
        stopInternal(cleanup = false)
        return file?.takeIf { it.exists() }
    }

    fun cancel() {
        stopInternal(cleanup = true)
    }

    private fun stopInternal(cleanup: Boolean) {
        tickerJob?.cancel()
        tickerJob = null
        runCatching { recorder?.stop() }
        runCatching { recorder?.release() }
        recorder = null
        if (cleanup) {
            currentFile?.delete()
        }
        currentFile = null
        _state.value = _state.value.copy(recording = false, durationSec = 0)
    }

    data class RecorderState(
        val recording: Boolean = false,
        val durationSec: Int = 0,
        val error: String? = null
    )

    companion object {
        const val SAMPLE_RATE = 44100
        const val BIT_RATE = 128000
    }
}
