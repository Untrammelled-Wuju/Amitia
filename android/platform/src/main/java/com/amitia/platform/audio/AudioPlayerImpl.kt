package com.amitia.platform.audio

import android.content.Context
import android.media.AudioAttributes
import android.media.AudioFocusRequest
import android.media.AudioManager
import android.net.Uri
import android.os.Build
import androidx.media3.common.C
import androidx.media3.common.MediaItem
import androidx.media3.common.MediaMetadata
import androidx.media3.common.PlaybackException
import androidx.media3.common.Player
import androidx.media3.exoplayer.ExoPlayer
import com.amitia.core.logging.Logger
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

@Singleton
class AudioPlayerImpl @Inject constructor(
    @ApplicationContext private val context: Context,
    private val logger: Logger
) : AudioPlayer {

    private val appContext = context.applicationContext
    private val audioManager = appContext.getSystemService(Context.AUDIO_SERVICE) as AudioManager
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main)

    @Volatile
    private var player: ExoPlayer? = null

    private val stateFlow = MutableStateFlow<AudioPlayer.PlaybackState>(AudioPlayer.PlaybackState.Idle)
    private val progressFlow = MutableStateFlow(AudioPlayer.PlaybackProgress(0L, 0L, 0))

    private var progressJob: Job? = null
    private var currentSource: AudioPlayer.PlaybackSource? = null
    private var audioFocusRequest: AudioFocusRequest? = null
    private var pausedByFocusLoss: Boolean = false

    private val focusListener = AudioManager.OnAudioFocusChangeListener { change ->
        when (change) {
            AudioManager.AUDIOFOCUS_LOSS -> {
                pausedByFocusLoss = false
                stopInternal(releasePlayer = false)
                abandonFocus()
            }
            AudioManager.AUDIOFOCUS_LOSS_TRANSIENT -> {
                pausedByFocusLoss = player?.isPlaying == true
                player?.pause()
                stateFlow.value = AudioPlayer.PlaybackState.Paused
            }
            AudioManager.AUDIOFOCUS_LOSS_TRANSIENT_CAN_DUCK -> {
                player?.volume = 0.2f
            }
            AudioManager.AUDIOFOCUS_GAIN -> {
                player?.volume = 1.0f
                if (pausedByFocusLoss) {
                    pausedByFocusLoss = false
                    if (requestFocus()) player?.play()
                }
            }
        }
    }

    override val state: AudioPlayer.PlaybackState get() = stateFlow.value

    override fun observeState(): StateFlow<AudioPlayer.PlaybackState> = stateFlow.asStateFlow()

    override fun observeProgress(): StateFlow<AudioPlayer.PlaybackProgress> = progressFlow.asStateFlow()

    override suspend fun play(source: AudioPlayer.PlaybackSource) {
        currentSource = source
        ensurePlayer()
        val mediaItem = buildMediaItem(source) ?: run {
            stateFlow.value = AudioPlayer.PlaybackState.Failed("invalid_source")
            return
        }
        if (!requestFocus()) {
            stateFlow.value = AudioPlayer.PlaybackState.Failed("audio_focus_denied")
            return
        }
        stateFlow.value = AudioPlayer.PlaybackState.Buffering
        player?.apply {
            setMediaItem(mediaItem)
            prepare()
            playWhenReady = true
        }
        startProgressTracking()
    }

    override suspend fun pause() {
        player?.pause()
        stateFlow.value = AudioPlayer.PlaybackState.Paused
    }

    override suspend fun resume() {
        if (stateFlow.value !is AudioPlayer.PlaybackState.Paused &&
            stateFlow.value !is AudioPlayer.PlaybackState.Ready) return
        if (!requestFocus()) {
            stateFlow.value = AudioPlayer.PlaybackState.Failed("audio_focus_denied")
            return
        }
        player?.play()
    }

    override suspend fun stop() {
        stopInternal(releasePlayer = false)
    }

    override suspend fun seekTo(positionMillis: Long) {
        player?.seekTo(positionMillis)
        progressFlow.value = progressFlow.value.copy(currentPosition = positionMillis)
    }

    override suspend fun setPlaybackSpeed(speed: Float) {
        player?.setPlaybackSpeed(speed)
    }

    override suspend fun setVolume(left: Float, right: Float) {
        val avg = (left + right) / 2f
        player?.volume = avg.coerceIn(0f, 1f)
    }

    fun release() {
        stopInternal(releasePlayer = true)
        abandonFocus()
        scope.cancel()
    }

    private fun ensurePlayer() {
        if (player != null) return
        val newPlayer = ExoPlayer.Builder(appContext)
            .setAudioAttributes(
                androidx.media3.common.AudioAttributes.Builder()
                    .setContentType(C.AUDIO_CONTENT_TYPE_MUSIC)
                    .setUsage(C.USAGE_MEDIA)
                    .build(),
                false
            )
            .setHandleAudioBecomingNoisy(true)
            .build()
        newPlayer.addListener(object : Player.Listener {
            override fun onPlaybackStateChanged(playbackState: Int) {
                when (playbackState) {
                    Player.STATE_IDLE -> stateFlow.value = AudioPlayer.PlaybackState.Idle
                    Player.STATE_BUFFERING -> stateFlow.value = AudioPlayer.PlaybackState.Buffering
                    Player.STATE_READY -> {
                        if (newPlayer.playWhenReady) {
                            stateFlow.value = AudioPlayer.PlaybackState.Playing
                        } else {
                            stateFlow.value = AudioPlayer.PlaybackState.Ready
                        }
                        progressFlow.value = progressFlow.value.copy(
                            duration = newPlayer.duration.coerceAtLeast(0L)
                        )
                    }
                    Player.STATE_ENDED -> {
                        stateFlow.value = AudioPlayer.PlaybackState.Ended
                        stopProgressTracking()
                        abandonFocus()
                    }
                }
            }

            override fun onIsPlayingChanged(isPlaying: Boolean) {
                if (isPlaying && stateFlow.value !is AudioPlayer.PlaybackState.Failed) {
                    stateFlow.value = AudioPlayer.PlaybackState.Playing
                } else if (!isPlaying && stateFlow.value is AudioPlayer.PlaybackState.Playing) {
                    stateFlow.value = AudioPlayer.PlaybackState.Paused
                }
            }

            override fun onPlayerError(error: PlaybackException) {
                stateFlow.value = AudioPlayer.PlaybackState.Failed(error.message ?: "playback_error")
                logger.e(TAG, "ExoPlayer error", error)
                abandonFocus()
            }
        })
        player = newPlayer
    }

    private fun buildMediaItem(source: AudioPlayer.PlaybackSource): MediaItem? {
        return when (source) {
            is AudioPlayer.PlaybackSource.LocalFile -> {
                MediaItem.Builder()
                    .setUri(Uri.fromFile(java.io.File(source.path)))
                    .build()
            }
            is AudioPlayer.PlaybackSource.ContentUri -> {
                MediaItem.Builder().setUri(Uri.parse(source.uri)).build()
            }
            is AudioPlayer.PlaybackSource.RemoteUrl -> {
                MediaItem.Builder().setUri(source.url).build()
            }
            is AudioPlayer.PlaybackSource.ResourceId -> {
                val resUri = Uri.parse("android.resource://${appContext.packageName}/${source.resId}")
                MediaItem.Builder().setUri(resUri).build()
            }
        }
    }

    private fun startProgressTracking() {
        progressJob?.cancel()
        progressJob = scope.launch {
            while (true) {
                val p = player ?: break
                val current = p.currentPosition.coerceAtLeast(0L)
                val duration = p.duration.coerceAtLeast(0L)
                val buffered = p.bufferedPercentage
                progressFlow.value = AudioPlayer.PlaybackProgress(current, duration, buffered)
                delay(200L)
            }
        }
    }

    private fun stopProgressTracking() {
        progressJob?.cancel()
        progressJob = null
    }

    private fun stopInternal(releasePlayer: Boolean) {
        stopProgressTracking()
        if (releasePlayer) {
            player?.release()
            player = null
        } else {
            player?.stop()
        }
        stateFlow.value = AudioPlayer.PlaybackState.Idle
        progressFlow.value = AudioPlayer.PlaybackProgress(0L, 0L, 0)
    }

    private fun requestFocus(): Boolean {
        val attrs = android.media.AudioAttributes.Builder()
            .setContentType(android.media.AudioAttributes.CONTENT_TYPE_MUSIC)
            .setUsage(android.media.AudioAttributes.USAGE_MEDIA)
            .build()
        val request = AudioFocusRequest.Builder(AudioManager.AUDIOFOCUS_GAIN)
            .setAudioAttributes(attrs)
            .setOnAudioFocusChangeListener(focusListener)
            .build()
        audioFocusRequest = request
        return audioManager.requestAudioFocus(request) == AudioManager.AUDIOFOCUS_REQUEST_GRANTED
    }

    private fun abandonFocus() {
        audioFocusRequest?.let { audioManager.abandonAudioFocusRequest(it) }
        audioFocusRequest = null
    }

    companion object {
        private const val TAG = "AudioPlayerImpl"
    }
}
