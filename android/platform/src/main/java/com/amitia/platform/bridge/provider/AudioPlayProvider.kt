package com.amitia.platform.bridge.provider

import com.amitia.platform.audio.AudioPlayer
import com.amitia.platform.bridge.CapabilityProvider
import com.amitia.platform.bridge.NativeActionRequest
import com.amitia.platform.bridge.NativeActionResult
import kotlinx.coroutines.flow.first
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class AudioPlayProvider @Inject constructor(
    private val audioPlayer: AudioPlayer
) : CapabilityProvider {

    override fun action(): String = "audio_play"

    override fun requiredPermission(): String? = null

    override suspend fun execute(request: NativeActionRequest): NativeActionResult {
        val op = request.params["op"] ?: "status"
        return when (op) {
            "status" -> {
                val progress = audioPlayer.observeProgress().first()
                NativeActionResult.Success(
                    mapOf(
                        "state" to audioPlayer.state::class.simpleName.orEmpty(),
                        "position" to progress.currentPosition.toString(),
                        "duration" to progress.duration.toString(),
                        "buffered" to progress.bufferedPercentage.toString()
                    )
                )
            }
            "play" -> {
                val source = request.params["source"] ?: return NativeActionResult.Failed("source required")
                val type = request.params["type"] ?: "url"
                val playback = when (type) {
                    "file" -> AudioPlayer.PlaybackSource.LocalFile(source)
                    "uri" -> AudioPlayer.PlaybackSource.ContentUri(source)
                    "res" -> AudioPlayer.PlaybackSource.ResourceId(source.toIntOrNull() ?: 0)
                    else -> AudioPlayer.PlaybackSource.RemoteUrl(source)
                }
                audioPlayer.play(playback)
                NativeActionResult.Success(mapOf("state" to "playing"))
            }
            "pause" -> { audioPlayer.pause(); NativeActionResult.Success(mapOf("state" to "paused")) }
            "resume" -> { audioPlayer.resume(); NativeActionResult.Success(mapOf("state" to "playing")) }
            "stop" -> { audioPlayer.stop(); NativeActionResult.Success(mapOf("state" to "stopped")) }
            "seek" -> {
                val pos = request.params["position"]?.toLongOrNull() ?: 0L
                audioPlayer.seekTo(pos)
                NativeActionResult.Success(mapOf("state" to "seeked"))
            }
            "speed" -> {
                val speed = request.params["value"]?.toFloatOrNull() ?: 1.0f
                audioPlayer.setPlaybackSpeed(speed)
                NativeActionResult.Success(mapOf("speed" to speed.toString()))
            }
            "volume" -> {
                val left = request.params["left"]?.toFloatOrNull() ?: 1.0f
                val right = request.params["right"]?.toFloatOrNull() ?: 1.0f
                audioPlayer.setVolume(left, right)
                NativeActionResult.Success(emptyMap())
            }
            else -> NativeActionResult.Failed("unsupported op: $op")
        }
    }
}
