package com.amitia.feature.chat.audio

import android.content.Context
import androidx.media3.common.AudioAttributes
import androidx.media3.common.C
import androidx.media3.common.MediaItem
import androidx.media3.common.PlaybackParameters
import androidx.media3.exoplayer.ExoPlayer
import dagger.hilt.android.qualifiers.ApplicationContext
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
class AudioPlayerController @Inject constructor(
    @ApplicationContext private val context: Context
) {

    private var player: ExoPlayer? = null
    private var progressJob: Job? = null
    private var currentUrl: String? = null

    private val _state = MutableStateFlow(PlayerState())
    val state: StateFlow<PlayerState> = _state.asStateFlow()

    private fun ensurePlayer(): ExoPlayer {
        return player ?: run {
            val attributes = AudioAttributes.Builder()
                .setContentType(C.AUDIO_CONTENT_TYPE_SPEECH)
                .setUsage(C.USAGE_MEDIA)
                .build()
            ExoPlayer.Builder(context)
                .setAudioAttributes(attributes, true)
                .build()
                .also { newPlayer -> player = newPlayer }
        }
    }

    fun play(
        url: String,
        scope: kotlinx.coroutines.CoroutineScope,
        onCompleted: (() -> Unit)? = null
    ) {
        val exo = ensurePlayer()
        if (currentUrl == url && _state.value.isPlaying) {
            exo.pause()
            _state.value = _state.value.copy(isPlaying = false)
            return
        }
        if (currentUrl != url) {
            exo.setMediaItem(MediaItem.fromUri(url))
            exo.prepare()
            currentUrl = url
        }
        exo.playWhenReady = true
        exo.playbackParameters = PlaybackParameters(1f)
        _state.value = _state.value.copy(currentUrl = url, isPlaying = true, durationMs = exo.duration.coerceAtLeast(0L).toInt())
        progressJob?.cancel()
        progressJob = scope.launch {
            while (isActive) {
                val pos = exo.currentPosition.toInt()
                val dur = exo.duration.toInt().coerceAtLeast(1)
                _state.value = _state.value.copy(
                    positionMs = pos,
                    durationMs = dur,
                    isPlaying = exo.isPlaying
                )
                if (pos >= dur && dur > 1) {
                    _state.value = _state.value.copy(isPlaying = false, positionMs = 0)
                    onCompleted?.invoke()
                    break
                }
                delay(200)
            }
        }
    }

    fun pause() {
        player?.pause()
        _state.value = _state.value.copy(isPlaying = false)
    }

    fun release() {
        progressJob?.cancel()
        progressJob = null
        player?.release()
        player = null
        currentUrl = null
        _state.value = PlayerState()
    }

    data class PlayerState(
        val currentUrl: String? = null,
        val isPlaying: Boolean = false,
        val positionMs: Int = 0,
        val durationMs: Int = 0
    )
}
