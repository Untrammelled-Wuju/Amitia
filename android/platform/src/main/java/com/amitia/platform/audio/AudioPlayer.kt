package com.amitia.platform.audio

import kotlinx.coroutines.flow.Flow

interface AudioPlayer {

    val state: PlaybackState

    fun observeState(): Flow<PlaybackState>

    fun observeProgress(): Flow<PlaybackProgress>

    suspend fun play(source: PlaybackSource)

    suspend fun pause()

    suspend fun resume()

    suspend fun stop()

    suspend fun seekTo(positionMillis: Long)

    suspend fun setPlaybackSpeed(speed: Float)

    suspend fun setVolume(left: Float, right: Float)

    data class PlaybackProgress(
        val currentPosition: Long,
        val duration: Long,
        val bufferedPercentage: Int
    )

    sealed class PlaybackState {
        object Idle : PlaybackState()
        object Buffering : PlaybackState()
        object Ready : PlaybackState()
        object Playing : PlaybackState()
        object Paused : PlaybackState()
        object Ended : PlaybackState()
        data class Failed(val error: String) : PlaybackState()
    }

    sealed class PlaybackSource {
        data class LocalFile(val path: String) : PlaybackSource()
        data class ContentUri(val uri: String) : PlaybackSource()
        data class RemoteUrl(val url: String) : PlaybackSource()
        data class ResourceId(val resId: Int) : PlaybackSource()
    }
}
