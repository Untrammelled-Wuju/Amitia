package com.amitia.platform.audio

import kotlinx.coroutines.flow.Flow

interface AudioRecorder {

    val state: RecordingState

    fun observeState(): Flow<RecordingState>

    suspend fun prepare(config: RecordingConfig): Boolean

    suspend fun start()

    suspend fun pause()

    suspend fun resume()

    suspend fun stop(): RecordingResult

    suspend fun cancel()

    suspend fun getMaxAmplitude(): Int

    data class RecordingConfig(
        val outputFile: String,
        val sampleRate: Int = 44_100,
        val channels: Int = 1,
        val encoding: AudioEncoding = AudioEncoding.AAC,
        val bitrate: Int = 128_000,
        val maxDurationMillis: Long? = null
    )

    enum class AudioEncoding {
        AAC, AMR_NB, AMR_WB, MPEG_4, THREE_GPP, OPUS
    }

    sealed class RecordingState {
        object Idle : RecordingState()
        object Prepared : RecordingState()
        object Recording : RecordingState()
        object Paused : RecordingState()
        data class Stopped(val durationMillis: Long) : RecordingState()
        data class Failed(val error: String) : RecordingState()
    }

    data class RecordingResult(
        val success: Boolean,
        val filePath: String?,
        val durationMillis: Long = 0L,
        val sizeBytes: Long = 0L,
        val error: String? = null
    )
}
